package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// Compile-time interface check.
var _ ui.Plugin = (*mcpSSE)(nil)

// ---------- mock controller ----------

type mockController struct {
	workspaceName string
	tree          *config.Tree
}

func newMockController() *mockController {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})
	return &mockController{
		workspaceName: "test-workspace",
		tree:          tree,
	}
}

func (m *mockController) WorkspaceName(context.Context) (string, error) {
	return m.workspaceName, nil
}
func (m *mockController) LoadTree(context.Context) (*config.Tree, error) { return m.tree, nil }
func (m *mockController) SetValue(_ context.Context, path string, value config.Value) error {
	return m.tree.Set(path, &value)
}
func (m *mockController) Validate(context.Context) ([]config.ValidationResult, error) {
	return nil, nil
}
func (m *mockController) SaveTree(context.Context) error { return nil }
func (m *mockController) ExportTemplates(context.Context) ([]ui.ExportTemplate, error) {
	return nil, nil
}
func (m *mockController) Export(context.Context, ui.ExportRequest) (*ui.ExportResult, error) {
	return &ui.ExportResult{Content: "exported"}, nil
}
func (m *mockController) Apply(_ context.Context, _ string, _ func(ui.ApplyEvent)) (*ui.ApplyResult, error) {
	return &ui.ApplyResult{ExitCode: 0}, nil
}
func (m *mockController) ListComponents(context.Context) ([]ui.ComponentInfo, error) { return nil, nil }
func (m *mockController) EnableComponent(context.Context, string) ([]string, error)  { return nil, nil }
func (m *mockController) DisableComponent(context.Context, string) error             { return nil }
func (m *mockController) SearchMarketplace(context.Context, ui.MarketplaceQuery) (*ui.MarketplaceResults, error) {
	return &ui.MarketplaceResults{}, nil
}
func (m *mockController) GetMarketplaceDetail(context.Context, string, string, string) (*ui.MarketplaceDetail, error) {
	return &ui.MarketplaceDetail{}, nil
}
func (m *mockController) InstallPlugin(context.Context, string) (*ui.InstallResult, error) {
	return &ui.InstallResult{}, nil
}
func (m *mockController) UninstallPlugin(context.Context, string, string) error { return nil }
func (m *mockController) ListInstalledPlugins(context.Context) ([]ui.InstalledPlugin, error) {
	return nil, nil
}
func (m *mockController) CheckUpdates(context.Context) ([]ui.PluginUpdate, error) { return nil, nil }
func (m *mockController) UpdatePlugin(context.Context, string, string) (*ui.InstallResult, error) {
	return &ui.InstallResult{}, nil
}
func (m *mockController) RatePlugin(context.Context, string, string, string, ui.Rating) error {
	return nil
}
func (m *mockController) StoreAuthMethods(context.Context) ([]ui.StoreAuthMethod, error) {
	return nil, nil
}
func (m *mockController) StoreLogin(context.Context, string, map[string]string) (*ui.StoreSession, error) {
	return &ui.StoreSession{}, nil
}
func (m *mockController) StoreLoginInteractive(context.Context, string, map[string]string) (*ui.StoreInteractiveChallenge, error) {
	return nil, errors.New("not supported")
}
func (m *mockController) StoreLoginInteractiveCallback(context.Context, string, map[string]string) (*ui.StoreSession, error) {
	return nil, errors.New("not supported")
}
func (m *mockController) StoreAuthStatus(context.Context) (*ui.StoreSession, error) {
	return &ui.StoreSession{}, nil
}
func (m *mockController) StoreLogout(context.Context) error { return nil }

// ---------- test helpers ----------

// startTestServer starts the MCP SSE plugin directly (no gRPC) and
// returns the base URL and a cancel function.
func startTestServer(t *testing.T, ctrl ui.Controller, token string) (string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	m := &mcpSSE{
		addr:      "127.0.0.1:0",
		authToken: token,
		ready:     make(chan struct{}),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Run(ctx, ctrl)
	}()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	addr, err := m.Addr(waitCtx)
	if err != nil {
		cancel()
		t.Fatalf("server did not start: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	return "http://" + addr, cancel
}

// ---------- tests ----------

func TestCapabilities(t *testing.T) {
	m := newMCPSSE()
	caps, err := m.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.RequiresTTY {
		t.Error("expected RequiresTTY=false")
	}
	if !caps.SupportsMarketplace {
		t.Error("expected SupportsMarketplace=true")
	}
}

func TestMCPProtocol(t *testing.T) {
	base, _ := startTestServer(t, newMockController(), "")
	ctx := context.Background()

	transport := &mcp.StreamableClientTransport{
		Endpoint: base + "/mcp",
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	// List tools.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	// Call reload_tree.
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "reload_tree",
	})
	if err != nil {
		t.Fatalf("CallTool reload_tree: %v", err)
	}
	if result.IsError {
		t.Errorf("reload_tree returned error: %v", result.Content)
	}

	// Read workspace name resource.
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "zhi://workspace/name",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(resource.Contents) == 0 || resource.Contents[0].Text != "test-workspace" {
		t.Errorf("workspace name = %v, want test-workspace", resource.Contents)
	}

	// List prompts.
	prompts, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(prompts.Prompts) == 0 {
		t.Fatal("expected at least one prompt")
	}
}

// bearerRoundTripper wraps an http.RoundTripper and adds a Bearer token
// to each request's Authorization header.
type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(req)
}

func TestAuthValidToken(t *testing.T) {
	token := "test-secret-token"
	base, _ := startTestServer(t, newMockController(), token)
	ctx := context.Background()

	transport := &mcp.StreamableClientTransport{
		Endpoint: base + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{
				token: token,
				base:  http.DefaultTransport,
			},
		},
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Connect with valid token: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("expected tools with valid token")
	}
}

func TestAuthInvalidToken(t *testing.T) {
	token := "correct-token"
	base, _ := startTestServer(t, newMockController(), token)

	// Try with wrong token via raw HTTP.
	req, err := http.NewRequest("POST", base+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAuthMissingToken(t *testing.T) {
	base, _ := startTestServer(t, newMockController(), "required-token")

	// Try without any Authorization header.
	resp, err := http.Post(base+"/mcp", "application/json", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAuthDisabled(t *testing.T) {
	// Empty token means no auth required.
	base, _ := startTestServer(t, newMockController(), "")

	resp, err := http.Post(base+"/mcp", "application/json", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	// Without auth, the request should reach the MCP handler (not 401/403).
	// The MCP handler will return some response (possibly 400 for bad JSON,
	// but not an auth error).
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Errorf("should not get auth error when token is empty, got %d", resp.StatusCode)
	}
}

func TestDefaultAddr(t *testing.T) {
	// Verify the default address when ZHI_MCP_ADDR is not set.
	t.Setenv("ZHI_MCP_ADDR", "")
	m := newMCPSSE()
	if m.addr != "127.0.0.1:8091" {
		t.Errorf("default addr = %q, want %q", m.addr, "127.0.0.1:8091")
	}
}

func TestCustomAddr(t *testing.T) {
	t.Setenv("ZHI_MCP_ADDR", "0.0.0.0:9999")
	m := newMCPSSE()
	if m.addr != "0.0.0.0:9999" {
		t.Errorf("addr = %q, want %q", m.addr, "0.0.0.0:9999")
	}
}

func TestTokenFromEnv(t *testing.T) {
	t.Setenv("ZHI_MCP_TOKEN", "my-secret")
	m := newMCPSSE()
	if m.authToken != "my-secret" {
		t.Errorf("authToken = %q, want %q", m.authToken, "my-secret")
	}
}

func TestCallToolSetValue(t *testing.T) {
	base, _ := startTestServer(t, newMockController(), "")
	ctx := context.Background()

	transport := &mcp.StreamableClientTransport{Endpoint: base + "/mcp"}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "set_value",
		Arguments: map[string]any{
			"path":  "app/name",
			"value": "new-value",
		},
	})
	if err != nil {
		t.Fatalf("CallTool set_value: %v", err)
	}
	if result.IsError {
		t.Errorf("set_value returned error: %s", fmtContent(result.Content))
	}
}

func fmtContent(c []mcp.Content) string {
	var parts []string
	for _, item := range c {
		if tc, ok := item.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return fmt.Sprintf("%v", parts)
}
