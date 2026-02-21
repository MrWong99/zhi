// Package httpclient provides a thin HTTP client for the HashiCorp Vault API.
// It is shared by the vault store provider and the vault-manager meta-plugin.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Client is a thin HTTP client for the HashiCorp Vault API.
// It handles token management, JSON encoding/decoding, and Vault-specific
// response parsing without importing the heavy vault/api dependency.
type Client struct {
	addr       string
	namespace  string
	httpClient *http.Client

	mu    sync.RWMutex
	token string
}

// Response is the standard Vault API response envelope.
type Response struct {
	Data     json.RawMessage `json:"data"`
	Errors   []string        `json:"errors"`
	Warnings []string        `json:"warnings"`
	Auth     *AuthResponse   `json:"auth"`
	WrapInfo *WrapInfo       `json:"wrap_info"`
}

// AuthResponse holds the auth block from a Vault login response.
type AuthResponse struct {
	ClientToken   string            `json:"client_token"`
	Accessor      string            `json:"accessor"`
	Policies      []string          `json:"policies"`
	LeaseDuration int               `json:"lease_duration"`
	Renewable     bool              `json:"renewable"`
	Metadata      map[string]string `json:"metadata"`
}

// WrapInfo holds the wrap_info block from a Vault response-wrapping response.
type WrapInfo struct {
	Token           string `json:"token"`
	TTL             int    `json:"ttl"`
	CreationTime    string `json:"creation_time"`
	WrappedAccessor string `json:"wrapped_accessor"`
}

// APIError represents an error response from the Vault API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("vault: %s (HTTP %d)", e.Message, e.StatusCode)
}

// New creates a new Vault HTTP client.
func New(addr, token, namespace string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{
		addr:       strings.TrimRight(addr, "/"),
		namespace:  namespace,
		token:      token,
		httpClient: httpClient,
	}
}

// SetToken updates the client's Vault token.
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

// Token returns the client's current Vault token.
func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// Do performs an HTTP request to the Vault API and returns the parsed response.
// Extra headers (e.g. X-Vault-Wrap-TTL) can be passed via extraHeaders.
func (c *Client) Do(ctx context.Context, method, path string, body any, extraHeaders map[string]string) (*Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := c.addr + "/v1/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if token := c.Token(); token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	if c.namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.namespace)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	// 204 No Content is a successful response with no body.
	if resp.StatusCode == http.StatusNoContent {
		return &Response{}, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading vault response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, ParseError(resp.StatusCode, respBody)
	}

	var vresp Response
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &vresp); err != nil {
			return nil, fmt.Errorf("parsing vault response: %w", err)
		}
	}
	return &vresp, nil
}

// Request performs an HTTP request without extra headers.
func (c *Client) Request(ctx context.Context, method, path string, body any) (*Response, error) {
	return c.Do(ctx, method, path, body, nil)
}

// List performs a Vault LIST request.
func (c *Client) List(ctx context.Context, path string) (*Response, error) {
	return c.Request(ctx, "LIST", path, nil)
}

// ParseError converts a Vault HTTP error response into a Go error.
func ParseError(statusCode int, body []byte) error {
	var errResp struct {
		Errors []string `json:"errors"`
	}
	_ = json.Unmarshal(body, &errResp)
	msg := strings.Join(errResp.Errors, "; ")
	if msg == "" {
		msg = http.StatusText(statusCode)
	}
	return &APIError{
		StatusCode: statusCode,
		Message:    msg,
	}
}

// ParseListKeys extracts the "keys" array from a Vault LIST response.
func ParseListKeys(resp *Response) ([]string, error) {
	if resp == nil || len(resp.Data) == 0 {
		return nil, nil
	}
	var data struct {
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parsing list keys: %w", err)
	}
	return data.Keys, nil
}
