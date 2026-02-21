package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/ui/httpapi"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// mockController is a minimal implementation for the gRPC round-trip test.
type mockController struct{}

func newMockController() *mockController { return &mockController{} }

func (m *mockController) WorkspaceName(_ context.Context) (string, error) {
	return "test-workspace", nil
}
func (m *mockController) LoadTree(_ context.Context) (*config.Tree, error) {
	return config.NewTree(), nil
}
func (m *mockController) SetValue(_ context.Context, _ string, _ config.Value) error { return nil }
func (m *mockController) Validate(_ context.Context) ([]config.ValidationResult, error) {
	return nil, nil
}
func (m *mockController) SaveTree(_ context.Context) error { return nil }
func (m *mockController) ExportTemplates(_ context.Context) ([]ui.ExportTemplate, error) {
	return nil, nil
}
func (m *mockController) Export(_ context.Context, _ ui.ExportRequest) (*ui.ExportResult, error) {
	return &ui.ExportResult{}, nil
}
func (m *mockController) Apply(_ context.Context, _ string, _ func(ui.ApplyEvent)) (*ui.ApplyResult, error) {
	return &ui.ApplyResult{}, nil
}
func (m *mockController) ListComponents(_ context.Context) ([]ui.ComponentInfo, error) {
	return nil, nil
}
func (m *mockController) EnableComponent(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockController) DisableComponent(_ context.Context, _ string) error { return nil }
func (m *mockController) SearchMarketplace(_ context.Context, _ ui.MarketplaceQuery) (*ui.MarketplaceResults, error) {
	return &ui.MarketplaceResults{}, nil
}
func (m *mockController) GetMarketplaceDetail(_ context.Context, _, _, _ string) (*ui.MarketplaceDetail, error) {
	return &ui.MarketplaceDetail{}, nil
}
func (m *mockController) InstallPlugin(_ context.Context, _ string) (*ui.InstallResult, error) {
	return &ui.InstallResult{}, nil
}
func (m *mockController) UninstallPlugin(_ context.Context, _, _ string) error { return nil }
func (m *mockController) ListInstalledPlugins(_ context.Context) ([]ui.InstalledPlugin, error) {
	return nil, nil
}
func (m *mockController) CheckUpdates(_ context.Context) ([]ui.PluginUpdate, error) { return nil, nil }
func (m *mockController) UpdatePlugin(_ context.Context, _, _ string) (*ui.InstallResult, error) {
	return &ui.InstallResult{}, nil
}
func (m *mockController) RatePlugin(_ context.Context, _, _, _ string, _ ui.Rating) error { return nil }
func (m *mockController) StoreAuthMethods(_ context.Context) ([]ui.StoreAuthMethod, error) {
	return nil, nil
}
func (m *mockController) StoreLogin(_ context.Context, _ string, _ map[string]string) (*ui.StoreSession, error) {
	return &ui.StoreSession{Status: ui.StoreSessionNone}, nil
}
func (m *mockController) StoreLoginInteractive(_ context.Context, _ string, _ map[string]string) (*ui.StoreInteractiveChallenge, error) {
	return nil, errors.New("not supported")
}
func (m *mockController) StoreLoginInteractiveCallback(_ context.Context, _ string, _ map[string]string) (*ui.StoreSession, error) {
	return nil, errors.New("not supported")
}
func (m *mockController) StoreAuthStatus(_ context.Context) (*ui.StoreSession, error) {
	return &ui.StoreSession{Status: ui.StoreSessionNone}, nil
}
func (m *mockController) StoreLogout(_ context.Context) error { return nil }

// TestGRPCRoundTrip verifies the httpapi provider works through the full
// gRPC plugin layer.
func TestGRPCRoundTrip(t *testing.T) {
	p, err := httpapi.New(httpapi.Config{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("httpapi.New: %v", err)
	}
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"ui": &ui.GRPCPlugin{Impl: p},
	})
	raw, err := client.Dispense("ui")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	uiPlugin := raw.(ui.Plugin)

	caps, err := uiPlugin.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.RequiresTTY {
		t.Error("RequiresTTY should be false for HTTP UI")
	}

	// Run with the plugin's HTTP server to verify the full round-trip.
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- uiPlugin.Run(ctx, newMockController())
	}()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	addr, err := p.Addr(waitCtx)
	if err != nil {
		cancel()
		t.Fatalf("server did not start: %v", err)
	}

	base := "http://" + addr
	var result map[string]string
	resp, err := http.Get(base + "/api/workspace")
	if err != nil {
		t.Fatalf("GET /api/workspace: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck

	if result["name"] != "test-workspace" {
		t.Errorf("workspace name = %q, want %q", result["name"], "test-workspace")
	}

	cancel()
	if err := <-errCh; err != nil {
		if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "cancelled") {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
	}
}
