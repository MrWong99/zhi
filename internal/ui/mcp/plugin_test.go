package mcp_test

import (
	"context"
	"io"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	mcpplugin "github.com/MrWong99/zhi/internal/ui/mcp"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// Compile-time interface check.
var _ ui.Plugin = (*mcpplugin.StdioPlugin)(nil)

// --- minimal mock controller ---

type mockController struct{}

func (m *mockController) LoadTree(context.Context) (*config.Tree, error) {
	t := config.NewTree()
	_ = t.Set("app/name", &config.Value{Val: "test-app"})
	return t, nil
}

func (m *mockController) SetValue(context.Context, string, config.Value) error { return nil }
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
func (m *mockController) ListComponents(context.Context) ([]ui.ComponentInfo, error) {
	return nil, nil
}
func (m *mockController) EnableComponent(context.Context, string) ([]string, error) {
	return nil, nil
}
func (m *mockController) DisableComponent(context.Context, string) error { return nil }
func (m *mockController) WorkspaceName(context.Context) (string, error) {
	return "test-workspace", nil
}
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
func (m *mockController) CheckUpdates(context.Context) ([]ui.PluginUpdate, error) {
	return nil, nil
}
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
	return &ui.StoreInteractiveChallenge{}, nil
}
func (m *mockController) StoreLoginInteractiveCallback(context.Context, string, map[string]string) (*ui.StoreSession, error) {
	return &ui.StoreSession{}, nil
}
func (m *mockController) StoreAuthStatus(context.Context) (*ui.StoreSession, error) {
	return &ui.StoreSession{}, nil
}
func (m *mockController) StoreLogout(context.Context) error { return nil }

// --- tests ---

func TestCapabilities(t *testing.T) {
	p := &mcpplugin.StdioPlugin{}
	caps, err := p.Capabilities(context.Background())
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

func TestRunEndToEnd(t *testing.T) {
	// Create two pairs of pipes to simulate stdio communication between
	// the plugin (server) and an MCP client.
	//
	// Client writes -> serverIn (plugin reads from Stdin)
	// Plugin writes to Stdout -> clientIn (client reads)
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	plugin := &mcpplugin.StdioPlugin{
		Stdin:  serverIn,
		Stdout: serverOut,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the plugin in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		errCh <- plugin.Run(ctx, &mockController{})
	}()

	// Connect an MCP client through the pipes.
	clientTransport := &gomcp.IOTransport{
		Reader: clientIn,
		Writer: clientOut,
	}

	client := gomcp.NewClient(&gomcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	// List tools — the plugin should register all mcpbridge tools.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	// Verify reload_tree works.
	result, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name: "reload_tree",
	})
	if err != nil {
		t.Fatalf("CallTool reload_tree: %v", err)
	}
	if result.IsError {
		t.Errorf("reload_tree returned error: %v", result.Content)
	}

	// Read workspace name resource.
	resource, err := session.ReadResource(ctx, &gomcp.ReadResourceParams{
		URI: "zhi://workspace/name",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(resource.Contents) == 0 {
		t.Fatal("expected resource content")
	}
	if resource.Contents[0].Text != "test-workspace" {
		t.Errorf("workspace name = %q, want %q", resource.Contents[0].Text, "test-workspace")
	}

	// List prompts.
	prompts, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(prompts.Prompts) == 0 {
		t.Fatal("expected at least one prompt")
	}

	cancel()
	<-errCh
}

func TestRunReadOnly(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	plugin := &mcpplugin.StdioPlugin{
		Stdin:    serverIn,
		Stdout:   serverOut,
		ReadOnly: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- plugin.Run(ctx, &mockController{})
	}()

	clientTransport := &gomcp.IOTransport{
		Reader: clientIn,
		Writer: clientOut,
	}

	client := gomcp.NewClient(&gomcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	// In read-only mode, mutation tools (set_value, save, apply, etc.)
	// should be absent.
	for _, tool := range tools.Tools {
		switch tool.Name {
		case "set_value", "save", "apply", "export",
			"enable_component", "disable_component",
			"marketplace_install", "marketplace_uninstall",
			"marketplace_update", "marketplace_rate",
			"store_login", "store_logout":
			t.Errorf("read-only mode should not have tool %q", tool.Name)
		}
	}

	// Read-only tools should still be present.
	foundReload := false
	for _, tool := range tools.Tools {
		if tool.Name == "reload_tree" {
			foundReload = true
			break
		}
	}
	if !foundReload {
		t.Error("expected reload_tree tool in read-only mode")
	}

	cancel()
	<-errCh
}
