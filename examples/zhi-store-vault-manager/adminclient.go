package main

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

// adminClient is a thin HTTP client for Vault management APIs.
// It handles policy CRUD, AppRole management, token creation,
// and wrapped secret_id generation.
type adminClient struct {
	addr       string
	namespace  string
	httpClient *http.Client

	mu    sync.RWMutex
	token string
}

func newAdminClient(addr, token, namespace string) *adminClient {
	return &adminClient{
		addr:       strings.TrimRight(addr, "/"),
		namespace:  namespace,
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *adminClient) setToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *adminClient) getToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

type adminResponse struct {
	Data     json.RawMessage `json:"data"`
	Errors   []string        `json:"errors"`
	Warnings []string        `json:"warnings"`
	Auth     *adminAuthResp  `json:"auth"`
	WrapInfo *wrapInfo       `json:"wrap_info"`
}

type adminAuthResp struct {
	ClientToken   string            `json:"client_token"`
	Accessor      string            `json:"accessor"`
	Policies      []string          `json:"policies"`
	LeaseDuration int               `json:"lease_duration"`
	Renewable     bool              `json:"renewable"`
	Metadata      map[string]string `json:"metadata"`
}

type wrapInfo struct {
	Token           string `json:"token"`
	TTL             int    `json:"ttl"`
	CreationTime    string `json:"creation_time"`
	WrappedAccessor string `json:"wrapped_accessor"`
}

// doRequest is the shared HTTP request handler for all Vault API calls.
func (c *adminClient) doRequest(ctx context.Context, method, path string, body any, extraHeaders map[string]string) (*adminResponse, error) {
	url := c.addr + "/v1/" + strings.TrimLeft(path, "/")

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	token := c.getToken()
	if token != "" {
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
		return nil, fmt.Errorf("vault request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return &adminResponse{}, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result adminResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
	}

	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if len(result.Errors) > 0 {
			msg = strings.Join(result.Errors, "; ")
		}
		return nil, fmt.Errorf("vault: %s (%d)", msg, resp.StatusCode)
	}

	return &result, nil
}

func (c *adminClient) request(ctx context.Context, method, path string, body any) (*adminResponse, error) {
	return c.doRequest(ctx, method, path, body, nil)
}

func (c *adminClient) requestWithWrap(ctx context.Context, method, path string, body any, wrapTTL string) (*adminResponse, error) {
	var headers map[string]string
	if wrapTTL != "" {
		headers = map[string]string{"X-Vault-Wrap-TTL": wrapTTL}
	}
	return c.doRequest(ctx, method, path, body, headers)
}

// --- Policy operations ---

// putPolicy creates or updates an ACL policy in Vault.
func (c *adminClient) putPolicy(ctx context.Context, name, hcl string) error {
	_, err := c.request(ctx, http.MethodPut, "sys/policies/acl/"+name, map[string]any{
		"policy": hcl,
	})
	return err
}

// deletePolicy removes an ACL policy from Vault.
func (c *adminClient) deletePolicy(ctx context.Context, name string) error {
	_, err := c.request(ctx, http.MethodDelete, "sys/policies/acl/"+name, nil)
	return err
}

// listPolicies lists all ACL policies matching the given prefix.
func (c *adminClient) listPolicies(ctx context.Context, prefix string) ([]string, error) {
	resp, err := c.request(ctx, "LIST", "sys/policies/acl", nil)
	if err != nil {
		return nil, err
	}
	var data struct {
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("decoding policy list: %w", err)
	}
	var matched []string
	for _, k := range data.Keys {
		if strings.HasPrefix(k, prefix) {
			matched = append(matched, k)
		}
	}
	return matched, nil
}

// --- AppRole operations ---

// readAppRole reads the configuration of an AppRole role.
func (c *adminClient) readAppRole(ctx context.Context, roleName string) (map[string]any, error) {
	resp, err := c.request(ctx, http.MethodGet, "auth/approle/role/"+roleName, nil)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("decoding approle: %w", err)
	}
	return data, nil
}

// writeAppRole creates or updates an AppRole role with the given parameters.
func (c *adminClient) writeAppRole(ctx context.Context, roleName string, params map[string]any) error {
	_, err := c.request(ctx, http.MethodPost, "auth/approle/role/"+roleName, params)
	return err
}

// readRoleID reads the role_id for an AppRole role.
func (c *adminClient) readRoleID(ctx context.Context, roleName string) (string, error) {
	resp, err := c.request(ctx, http.MethodGet, "auth/approle/role/"+roleName+"/role-id", nil)
	if err != nil {
		return "", err
	}
	var data struct {
		RoleID string `json:"role_id"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("decoding role_id: %w", err)
	}
	return data.RoleID, nil
}

// generateWrappedSecretID generates a new secret_id for an AppRole role,
// returned as a response-wrapped token.
func (c *adminClient) generateWrappedSecretID(ctx context.Context, roleName, wrapTTL string) (string, error) {
	resp, err := c.requestWithWrap(ctx, http.MethodPost, "auth/approle/role/"+roleName+"/secret-id", nil, wrapTTL)
	if err != nil {
		return "", err
	}
	if resp.WrapInfo == nil {
		return "", fmt.Errorf("no wrap_info in response for role %s", roleName)
	}
	return resp.WrapInfo.Token, nil
}

// --- Token operations ---

// createToken creates a new Vault token with the given policies and TTL.
func (c *adminClient) createToken(ctx context.Context, policies []string, ttl string) (string, error) {
	resp, err := c.request(ctx, http.MethodPost, "auth/token/create", map[string]any{
		"policies": policies,
		"ttl":      ttl,
		"num_uses": 0,
	})
	if err != nil {
		return "", err
	}
	if resp.Auth == nil {
		return "", fmt.Errorf("no auth block in token create response")
	}
	return resp.Auth.ClientToken, nil
}

// createScopedToken creates a short-lived token for delegating a single
// operation to the child plugin. The policyHCL parameter is reserved for
// future inline-policy support; currently the token inherits the parent's
// policies and relies on the short TTL for scoping.
func (c *adminClient) createScopedToken(ctx context.Context, _ string, ttl string) (string, error) {
	return c.createToken(ctx, nil, ttl)
}

// --- Auth operations ---

// login authenticates to Vault using the specified method and credentials.
// Supported methods: "token", "userpass", "approle".
func (c *adminClient) login(ctx context.Context, method string, credentials map[string]string) (*adminAuthResp, error) {
	var path string
	var body map[string]any

	switch method {
	case "token":
		token, ok := credentials["token"]
		if !ok {
			return nil, fmt.Errorf("token credential required")
		}
		c.setToken(token)
		// Verify with token lookup.
		resp, err := c.request(ctx, http.MethodGet, "auth/token/lookup-self", nil)
		if err != nil {
			return nil, fmt.Errorf("token validation failed: %w", err)
		}
		var data struct {
			Policies []string `json:"policies"`
		}
		if resp.Data != nil {
			_ = json.Unmarshal(resp.Data, &data)
		}
		return &adminAuthResp{
			ClientToken: token,
			Policies:    data.Policies,
		}, nil
	case "userpass":
		username := credentials["username"]
		path = "auth/userpass/login/" + username
		body = map[string]any{"password": credentials["password"]}
	case "approle":
		path = "auth/approle/login"
		body = map[string]any{
			"role_id":   credentials["role_id"],
			"secret_id": credentials["secret_id"],
		}
	default:
		return nil, fmt.Errorf("unsupported admin auth method: %s", method)
	}

	resp, err := c.request(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	if resp.Auth == nil {
		return nil, fmt.Errorf("no auth block in login response")
	}
	c.setToken(resp.Auth.ClientToken)
	return resp.Auth, nil
}
