// Package vaultmanager provides a meta-plugin store that wraps a child vault
// store plugin with automatic credential management for deployed applications.
package vaultmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/MrWong99/zhi/pkg/providers/store/vault/httpclient"
)

// AdminClient wraps the shared Vault HTTP client with management API methods
// (policy CRUD, AppRole management, token creation, wrapped secret_id).
type AdminClient struct {
	*httpclient.Client
}

// NewAdminClient creates a new AdminClient for Vault management APIs.
func NewAdminClient(addr, token, namespace string, httpClient *http.Client) *AdminClient {
	return &AdminClient{
		Client: httpclient.New(addr, token, namespace, httpClient),
	}
}

// --- Policy operations ---

// PutPolicy creates or updates an ACL policy in Vault.
func (c *AdminClient) PutPolicy(ctx context.Context, name, hcl string) error {
	_, err := c.Request(ctx, http.MethodPut, "sys/policies/acl/"+name, map[string]any{
		"policy": hcl,
	})
	return err
}

// DeletePolicy removes an ACL policy from Vault.
func (c *AdminClient) DeletePolicy(ctx context.Context, name string) error {
	_, err := c.Request(ctx, http.MethodDelete, "sys/policies/acl/"+name, nil)
	return err
}

// ListPolicies lists all ACL policies matching the given prefix.
func (c *AdminClient) ListPolicies(ctx context.Context, prefix string) ([]string, error) {
	resp, err := c.Request(ctx, "LIST", "sys/policies/acl", nil)
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

// ReadAppRole reads the configuration of an AppRole role.
func (c *AdminClient) ReadAppRole(ctx context.Context, roleName string) (map[string]any, error) {
	resp, err := c.Request(ctx, http.MethodGet, "auth/approle/role/"+roleName, nil)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("decoding approle: %w", err)
	}
	return data, nil
}

// WriteAppRole creates or updates an AppRole role with the given parameters.
func (c *AdminClient) WriteAppRole(ctx context.Context, roleName string, params map[string]any) error {
	_, err := c.Request(ctx, http.MethodPost, "auth/approle/role/"+roleName, params)
	return err
}

// ReadRoleID reads the role_id for an AppRole role.
func (c *AdminClient) ReadRoleID(ctx context.Context, roleName string) (string, error) {
	resp, err := c.Request(ctx, http.MethodGet, "auth/approle/role/"+roleName+"/role-id", nil)
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

// GenerateSecretID generates a new secret_id for an AppRole role.
// If wrapTTL is non-empty, the secret_id is returned as a response-wrapped
// token. Otherwise, the plain secret_id is returned.
func (c *AdminClient) GenerateSecretID(ctx context.Context, roleName, wrapTTL string) (string, bool, error) {
	var headers map[string]string
	if wrapTTL != "" {
		headers = map[string]string{"X-Vault-Wrap-TTL": wrapTTL}
	}
	resp, err := c.Do(ctx, http.MethodPost, "auth/approle/role/"+roleName+"/secret-id", nil, headers)
	if err != nil {
		return "", false, err
	}

	if wrapTTL != "" {
		if resp.WrapInfo == nil {
			return "", false, fmt.Errorf("no wrap_info in response for role %s", roleName)
		}
		return resp.WrapInfo.Token, true, nil
	}

	var data struct {
		SecretID string `json:"secret_id"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", false, fmt.Errorf("decoding secret_id for role %s: %w", roleName, err)
	}
	if data.SecretID == "" {
		return "", false, fmt.Errorf("empty secret_id in response for role %s", roleName)
	}
	return data.SecretID, false, nil
}

// --- Token operations ---

// CreateToken creates a new orphan Vault token with the given policies and TTL.
// It uses the auth/token/create-orphan endpoint so the child token's policies
// do not need to be a subset of the parent token's policies.
func (c *AdminClient) CreateToken(ctx context.Context, policies []string, ttl string) (string, error) {
	resp, err := c.Request(ctx, http.MethodPost, "auth/token/create-orphan", map[string]any{
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

// --- Auth operations ---

// Login authenticates to Vault using the specified method and credentials.
// Supported methods: "token", "userpass", "approle".
func (c *AdminClient) Login(ctx context.Context, method string, credentials map[string]string) (*httpclient.AuthResponse, error) {
	var path string
	var body map[string]any

	switch method {
	case "token":
		token, ok := credentials["token"]
		if !ok {
			return nil, fmt.Errorf("token credential required")
		}
		c.SetToken(token)
		resp, err := c.Request(ctx, http.MethodGet, "auth/token/lookup-self", nil)
		if err != nil {
			return nil, fmt.Errorf("token validation failed: %w", err)
		}
		var data struct {
			Policies []string `json:"policies"`
		}
		if resp.Data != nil {
			_ = json.Unmarshal(resp.Data, &data)
		}
		return &httpclient.AuthResponse{
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

	resp, err := c.Request(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	if resp.Auth == nil {
		return nil, fmt.Errorf("no auth block in login response")
	}
	c.SetToken(resp.Auth.ClientToken)
	return resp.Auth, nil
}
