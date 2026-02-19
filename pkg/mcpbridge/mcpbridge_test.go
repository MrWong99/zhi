package mcpbridge_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/mcpbridge"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// Compile-time interface check.
var _ ui.Controller = (*mockController)(nil)

// --- mock controller ---

type mockController struct {
	workspaceName string
	tree          *config.Tree
	components    []ui.ComponentInfo
	templates     []ui.ExportTemplate
	validations   []config.ValidationResult
	applyEvents   []ui.ApplyEvent
	applyResult   *ui.ApplyResult

	mu           sync.Mutex
	setValuePath string
	enabledName  string
	disabledName string
	exportReq    *ui.ExportRequest
	applyTarget  string
	savedCount   int
}

func newMockController() *mockController {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432), Metadata: map[string]any{"description": "database port"}})
	return &mockController{
		workspaceName: "test-workspace",
		tree:          tree,
		components: []ui.ComponentInfo{
			{Name: "web", Description: "Web server", Enabled: true, Mandatory: true},
			{Name: "cache", Description: "Redis cache", Enabled: false},
		},
		templates: []ui.ExportTemplate{
			{Name: "env", Template: "env.tmpl", Format: "env", Output: ".env"},
		},
		validations: []config.ValidationResult{
			{Severity: config.Warning, Message: "db/port: consider using TLS"},
		},
		applyEvents: []ui.ApplyEvent{
			{Line: "deploying...", Stream: "stdout"},
			{Line: "done", Stream: "stdout"},
		},
		applyResult: &ui.ApplyResult{ExitCode: 0},
	}
}

func (m *mockController) WorkspaceName(_ context.Context) (string, error) {
	return m.workspaceName, nil
}
func (m *mockController) LoadTree(_ context.Context) (*config.Tree, error) {
	return m.tree, nil
}
func (m *mockController) SetValue(_ context.Context, path string, value config.Value) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setValuePath = path
	return m.tree.Set(path, &value)
}
func (m *mockController) Validate(_ context.Context) ([]config.ValidationResult, error) {
	return m.validations, nil
}
func (m *mockController) SaveTree(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedCount++
	return nil
}
func (m *mockController) ExportTemplates(_ context.Context) ([]ui.ExportTemplate, error) {
	return m.templates, nil
}
func (m *mockController) Export(_ context.Context, req ui.ExportRequest) (*ui.ExportResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exportReq = &req
	return &ui.ExportResult{Name: "env", Content: "DB_HOST=localhost", OutputPath: ".env"}, nil
}
func (m *mockController) Apply(_ context.Context, target string, handler func(ui.ApplyEvent)) (*ui.ApplyResult, error) {
	m.mu.Lock()
	m.applyTarget = target
	m.mu.Unlock()
	for _, e := range m.applyEvents {
		handler(e)
	}
	return m.applyResult, nil
}
func (m *mockController) ListComponents(_ context.Context) ([]ui.ComponentInfo, error) {
	return m.components, nil
}
func (m *mockController) EnableComponent(_ context.Context, name string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabledName = name
	return []string{name}, nil
}
func (m *mockController) DisableComponent(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disabledName = name
	return nil
}
func (m *mockController) SearchMarketplace(_ context.Context, _ ui.MarketplaceQuery) (*ui.MarketplaceResults, error) {
	return &ui.MarketplaceResults{}, nil
}
func (m *mockController) GetMarketplaceDetail(_ context.Context, _, _, _ string) (*ui.MarketplaceDetail, error) {
	return &ui.MarketplaceDetail{}, nil
}
func (m *mockController) InstallPlugin(_ context.Context, _ string) (*ui.InstallResult, error) {
	return &ui.InstallResult{}, nil
}
func (m *mockController) UninstallPlugin(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockController) ListInstalledPlugins(_ context.Context) ([]ui.InstalledPlugin, error) {
	return nil, nil
}
func (m *mockController) CheckUpdates(_ context.Context) ([]ui.PluginUpdate, error) {
	return nil, nil
}
func (m *mockController) UpdatePlugin(_ context.Context, _, _ string) (*ui.InstallResult, error) {
	return &ui.InstallResult{}, nil
}
func (m *mockController) RatePlugin(_ context.Context, _, _, _ string, _ ui.Rating) error {
	return nil
}
func (m *mockController) StoreAuthMethods(_ context.Context) ([]ui.StoreAuthMethod, error) {
	return []ui.StoreAuthMethod{{Type: "token", Description: "API token"}}, nil
}
func (m *mockController) StoreLogin(_ context.Context, _ string, _ map[string]string) (*ui.StoreSession, error) {
	return &ui.StoreSession{Status: ui.StoreSessionAuthenticated}, nil
}
func (m *mockController) StoreLoginInteractive(_ context.Context, _ string, _ map[string]string) (*ui.StoreInteractiveChallenge, error) {
	return nil, errors.New("interactive login not supported")
}
func (m *mockController) StoreLoginInteractiveCallback(_ context.Context, _ string, _ map[string]string) (*ui.StoreSession, error) {
	return nil, errors.New("interactive login not supported")
}
func (m *mockController) StoreAuthStatus(_ context.Context) (*ui.StoreSession, error) {
	return &ui.StoreSession{Status: ui.StoreSessionNone}, nil
}
func (m *mockController) StoreLogout(_ context.Context) error {
	return nil
}

// --- test helpers ---

func connectTestServer(t *testing.T, ctrl ui.Controller) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcpbridge.NewMCPServer(ctrl)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)

	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// --- tests ---

func TestToolsList(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	var names []string
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}

	// Should have all 17 tools.
	expected := []string{
		"apply", "check_updates", "disable_component", "enable_component",
		"export", "marketplace_detail", "marketplace_install",
		"marketplace_rate", "marketplace_search", "marketplace_uninstall",
		"marketplace_update", "reload_tree", "save", "set_value",
		"store_login", "store_logout", "validate",
	}
	if len(names) != len(expected) {
		t.Fatalf("expected %d tools, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("tool[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestToolsReadOnly(t *testing.T) {
	ctx := context.Background()

	server := mcpbridge.NewMCPServer(newMockController(), mcpbridge.WithReadOnly(true))
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)

	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	var names []string
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}

	// Read-only mode should only have 5 tools.
	expected := []string{
		"check_updates", "marketplace_detail", "marketplace_search",
		"reload_tree", "validate",
	}
	if len(names) != len(expected) {
		t.Fatalf("expected %d tools in read-only, got %d: %v", len(expected), len(names), names)
	}
}

func TestCallSetValue(t *testing.T) {
	mc := newMockController()
	cs := connectTestServer(t, mc)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "set_value",
		Arguments: json.RawMessage(`{"path":"app/name","value":"my-app"}`),
	})
	if err != nil {
		t.Fatalf("call set_value: %v", err)
	}
	if result.IsError {
		t.Fatalf("set_value returned error: %v", result.Content)
	}
	mc.mu.Lock()
	if mc.setValuePath != "app/name" {
		t.Errorf("expected path %q, got %q", "app/name", mc.setValuePath)
	}
	mc.mu.Unlock()
}

func TestCallSave(t *testing.T) {
	mc := newMockController()
	cs := connectTestServer(t, mc)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "save",
		Arguments: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("call save: %v", err)
	}
	if result.IsError {
		t.Fatalf("save returned error: %v", result.Content)
	}
	mc.mu.Lock()
	if mc.savedCount != 1 {
		t.Errorf("expected 1 save, got %d", mc.savedCount)
	}
	mc.mu.Unlock()
}

func TestCallValidate(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "validate",
		Arguments: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("call validate: %v", err)
	}
	if result.IsError {
		t.Fatalf("validate returned error: %v", result.Content)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !json.Valid([]byte(text)) {
		t.Fatalf("validate did not return JSON: %s", text)
	}
}

func TestCallApply(t *testing.T) {
	mc := newMockController()
	cs := connectTestServer(t, mc)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "apply",
		Arguments: json.RawMessage(`{"target":"production"}`),
	})
	if err != nil {
		t.Fatalf("call apply: %v", err)
	}
	if result.IsError {
		t.Fatalf("apply returned error: %v", result.Content)
	}
	mc.mu.Lock()
	if mc.applyTarget != "production" {
		t.Errorf("expected target %q, got %q", "production", mc.applyTarget)
	}
	mc.mu.Unlock()
}

func TestCallExport(t *testing.T) {
	mc := newMockController()
	cs := connectTestServer(t, mc)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "export",
		Arguments: json.RawMessage(`{"template_name":"env","dry_run":true}`),
	})
	if err != nil {
		t.Fatalf("call export: %v", err)
	}
	if result.IsError {
		t.Fatalf("export returned error: %v", result.Content)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if text != "DB_HOST=localhost" {
		t.Errorf("expected %q, got %q", "DB_HOST=localhost", text)
	}
}

func TestCallExportNotFound(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "export",
		Arguments: json.RawMessage(`{"template_name":"nonexistent"}`),
	})
	if err != nil {
		t.Fatalf("call export: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for nonexistent template")
	}
}

func TestResourceWorkspaceName(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://workspace/name"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}
	if result.Contents[0].Text != "test-workspace" {
		t.Errorf("expected %q, got %q", "test-workspace", result.Contents[0].Text)
	}
}

func TestResourceTree(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://tree"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	text := result.Contents[0].Text
	var tree map[string]any
	if err := json.Unmarshal([]byte(text), &tree); err != nil {
		t.Fatalf("tree is not valid JSON: %v", err)
	}
	if _, ok := tree["db/host"]; !ok {
		t.Error("expected db/host in tree")
	}
}

func TestResourceTreePath(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://tree/db/host"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	text := result.Contents[0].Text
	var entry map[string]any
	if err := json.Unmarshal([]byte(text), &entry); err != nil {
		t.Fatalf("entry is not valid JSON: %v", err)
	}
	if entry["value"] != "localhost" {
		t.Errorf("expected value %q, got %v", "localhost", entry["value"])
	}
}

func TestResourceTreePathNotFound(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	_, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://tree/nonexistent/path"})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestResourceComponents(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://components"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	var components []ui.ComponentInfo
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &components); err != nil {
		t.Fatalf("components is not valid JSON: %v", err)
	}
	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
}

func TestResourceExportTemplates(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://export/templates"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	var templates []ui.ExportTemplate
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &templates); err != nil {
		t.Fatalf("templates is not valid JSON: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(templates))
	}
}

func TestResourceExportPreview(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://export/env"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	if result.Contents[0].Text != "DB_HOST=localhost" {
		t.Errorf("expected %q, got %q", "DB_HOST=localhost", result.Contents[0].Text)
	}
}

func TestResourceStoreAuthMethods(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "zhi://store/auth/methods"})
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	var methods []ui.StoreAuthMethod
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &methods); err != nil {
		t.Fatalf("auth methods is not valid JSON: %v", err)
	}
	if len(methods) != 1 {
		t.Errorf("expected 1 auth method, got %d", len(methods))
	}
}

func TestPromptExploreWorkspace(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "explore-workspace"})
	if err != nil {
		t.Fatalf("get prompt: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("expected role %q, got %q", "user", result.Messages[0].Role)
	}
}

func TestPromptReviewConfigWithPath(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "review-config",
		Arguments: map[string]string{"path": "db"},
	})
	if err != nil {
		t.Fatalf("get prompt: %v", err)
	}
	text := result.Messages[0].Content.(*mcp.TextContent).Text
	if !contains(text, "zhi://tree/db") {
		t.Errorf("expected tree URI with path filter, got: %s", text)
	}
}

func TestPromptSetupComponent(t *testing.T) {
	cs := connectTestServer(t, newMockController())
	ctx := context.Background()

	result, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "setup-component",
		Arguments: map[string]string{"component": "cache"},
	})
	if err != nil {
		t.Fatalf("get prompt: %v", err)
	}
	text := result.Messages[0].Content.(*mcp.TextContent).Text
	if !contains(text, "cache") {
		t.Errorf("expected component name in prompt, got: %s", text)
	}
}

func TestSafeControllerConcurrency(t *testing.T) {
	mc := newMockController()
	safe := mcpbridge.NewSafeController(mc)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 4 {
			case 0:
				_, _ = safe.LoadTree(ctx)
			case 1:
				_ = safe.SetValue(ctx, "test/key", config.Value{Val: n})
			case 2:
				_, _ = safe.Validate(ctx)
			case 3:
				_ = safe.SaveTree(ctx)
			}
		}(i)
	}
	wg.Wait()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
