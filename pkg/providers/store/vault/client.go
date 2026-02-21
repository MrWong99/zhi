package vault

import (
	"context"
	"net/http"

	"github.com/MrWong99/zhi/pkg/providers/store/vault/httpclient"
)

// Type aliases for the shared httpclient types, preserving internal names.
type (
	vaultResponse = httpclient.Response
	vaultAPIError = httpclient.APIError
)

// vaultClient wraps the shared httpclient.Client, providing the same
// method signatures used throughout vault.go.
type vaultClient struct {
	*httpclient.Client
}

func newVaultClient(addr, token, namespace string, httpClient *http.Client) *vaultClient {
	return &vaultClient{
		Client: httpclient.New(addr, token, namespace, httpClient),
	}
}

func (c *vaultClient) setToken(token string) { c.SetToken(token) }

func (c *vaultClient) request(ctx context.Context, method, path string, body any) (*vaultResponse, error) {
	return c.Request(ctx, method, path, body)
}

func (c *vaultClient) list(ctx context.Context, path string) (*vaultResponse, error) {
	return c.List(ctx, path)
}

// parseListKeys delegates to the shared implementation.
var parseListKeys = httpclient.ParseListKeys
