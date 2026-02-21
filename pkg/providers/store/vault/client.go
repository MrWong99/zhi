package vault

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

// vaultClient is a thin HTTP client for the HashiCorp Vault API.
// It handles token management, JSON encoding/decoding, and Vault-specific
// response parsing without importing the heavy vault/api dependency.
type vaultClient struct {
	addr       string
	namespace  string
	httpClient *http.Client

	mu    sync.RWMutex
	token string
}

// vaultResponse is the standard Vault API response envelope.
type vaultResponse struct {
	Data     json.RawMessage `json:"data"`
	Errors   []string        `json:"errors"`
	Warnings []string        `json:"warnings"`
	Auth     *vaultAuthResp  `json:"auth"`
}

// vaultAuthResp holds the auth block from a Vault login response.
type vaultAuthResp struct {
	ClientToken   string            `json:"client_token"`
	Accessor      string            `json:"accessor"`
	Policies      []string          `json:"policies"`
	LeaseDuration int               `json:"lease_duration"`
	Renewable     bool              `json:"renewable"`
	Metadata      map[string]string `json:"metadata"`
}

// vaultAPIError represents an error response from the Vault API.
type vaultAPIError struct {
	StatusCode int
	Message    string
}

func (e *vaultAPIError) Error() string {
	return fmt.Sprintf("vault: %s (HTTP %d)", e.Message, e.StatusCode)
}

func newVaultClient(addr, token, namespace string, httpClient *http.Client) *vaultClient {
	return &vaultClient{
		addr:       strings.TrimRight(addr, "/"),
		namespace:  namespace,
		token:      token,
		httpClient: httpClient,
	}
}

func (c *vaultClient) setToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *vaultClient) getToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// request performs an HTTP request to the Vault API and returns the
// parsed response envelope.
func (c *vaultClient) request(ctx context.Context, method, path string, body any) (*vaultResponse, error) {
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

	if token := c.getToken(); token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	if c.namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.namespace)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	// 204 No Content is a successful response with no body.
	if resp.StatusCode == http.StatusNoContent {
		return &vaultResponse{}, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading vault response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, parseVaultError(resp.StatusCode, respBody)
	}

	var vresp vaultResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &vresp); err != nil {
			return nil, fmt.Errorf("parsing vault response: %w", err)
		}
	}
	return &vresp, nil
}

// list performs a Vault LIST request using the LIST HTTP method.
func (c *vaultClient) list(ctx context.Context, path string) (*vaultResponse, error) {
	return c.request(ctx, "LIST", path, nil)
}

// parseVaultError converts a Vault HTTP error response into a Go error.
func parseVaultError(statusCode int, body []byte) error {
	var errResp struct {
		Errors []string `json:"errors"`
	}
	_ = json.Unmarshal(body, &errResp)
	msg := strings.Join(errResp.Errors, "; ")
	if msg == "" {
		msg = http.StatusText(statusCode)
	}
	return &vaultAPIError{
		StatusCode: statusCode,
		Message:    msg,
	}
}

// parseListKeys extracts the "keys" array from a Vault LIST response.
func parseListKeys(resp *vaultResponse) ([]string, error) {
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
