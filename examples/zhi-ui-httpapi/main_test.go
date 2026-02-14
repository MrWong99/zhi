package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// ---------- mock controller ----------

// mockController records calls and returns canned responses for testing.
type mockController struct {
	workspaceName string
	tree          *config.Tree
	components    []ui.ComponentInfo
	templates     []ui.ExportTemplate
	validations   []config.ValidationResult
	applyEvents   []ui.ApplyEvent
	applyResult   *ui.ApplyResult

	// recorded calls
	setValue     *config.Value
	setValuePath string
	enabledName  string
	disabledName string
	exportReq    *ui.ExportRequest
	applyTarget  string
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
	m.setValuePath = path
	m.setValue = &value
	return m.tree.Set(path, &value)
}

func (m *mockController) Validate(_ context.Context) ([]config.ValidationResult, error) {
	return m.validations, nil
}

func (m *mockController) SaveTree(_ context.Context) error {
	return nil
}

func (m *mockController) ExportTemplates(_ context.Context) ([]ui.ExportTemplate, error) {
	return m.templates, nil
}

func (m *mockController) Export(_ context.Context, req ui.ExportRequest) (*ui.ExportResult, error) {
	m.exportReq = &req
	return &ui.ExportResult{Name: "env", Content: "DB_HOST=localhost", OutputPath: ".env"}, nil
}

func (m *mockController) Apply(_ context.Context, target string, handler func(ui.ApplyEvent)) (*ui.ApplyResult, error) {
	m.applyTarget = target
	for _, e := range m.applyEvents {
		handler(e)
	}
	return m.applyResult, nil
}

func (m *mockController) ListComponents(_ context.Context) ([]ui.ComponentInfo, error) {
	return m.components, nil
}

func (m *mockController) EnableComponent(_ context.Context, name string) ([]string, error) {
	m.enabledName = name
	return []string{name}, nil
}

func (m *mockController) DisableComponent(_ context.Context, name string) error {
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
	return nil, nil
}

func (m *mockController) StoreLogin(_ context.Context, _ string, _ map[string]string) (*ui.StoreSession, error) {
	return &ui.StoreSession{Status: ui.StoreSessionNone}, nil
}

func (m *mockController) StoreAuthStatus(_ context.Context) (*ui.StoreSession, error) {
	return &ui.StoreSession{Status: ui.StoreSessionNone}, nil
}

func (m *mockController) StoreLogout(_ context.Context) error {
	return nil
}

// ---------- test helpers ----------

// startTestServer starts the HTTP UI plugin directly (no gRPC) and
// returns the base URL and a cancel function.
func startTestServer(t *testing.T, ctrl ui.Controller) (string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	h := &httpUI{addr: "127.0.0.1:0", ready: make(chan struct{})}
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Run(ctx, ctrl)
	}()

	// Wait for the server to be ready using the synchronised Addr method.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	addr, err := h.Addr(waitCtx)
	if err != nil {
		cancel()
		t.Fatalf("server did not start: %v", err)
	}

	base := "http://" + addr
	t.Cleanup(func() {
		cancel()
		<-errCh
	})
	return base, cancel
}

func getJSON(t *testing.T, url string, dst any) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			t.Fatalf("decode response from %s: %v", url, err)
		}
	}
	return resp
}

func postJSON(t *testing.T, url string, body any, dst any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			t.Fatalf("decode response from %s: %v", url, err)
		}
	}
	return resp
}

func putJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	_ = resp.Body.Close()
	return resp
}

// ---------- tests ----------

func TestWorkspaceName(t *testing.T) {
	base, _ := startTestServer(t, newMockController())
	var result map[string]string
	resp := getJSON(t, base+"/api/workspace", &result)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if result["name"] != "test-workspace" {
		t.Errorf("name = %q, want %q", result["name"], "test-workspace")
	}
}

func TestLoadTree(t *testing.T) {
	base, _ := startTestServer(t, newMockController())
	var entries []map[string]any
	resp := getJSON(t, base+"/api/tree", &entries)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	// Find db/host entry.
	found := false
	for _, e := range entries {
		if e["path"] == "db/host" {
			found = true
			if e["value"] != "localhost" {
				t.Errorf("db/host value = %v, want %q", e["value"], "localhost")
			}
		}
	}
	if !found {
		t.Error("db/host entry not found")
	}
}

func TestSetValue(t *testing.T) {
	ctrl := newMockController()
	base, _ := startTestServer(t, ctrl)

	body := map[string]any{"value": "newhost", "metadata": map[string]any{"source": "test"}}
	resp := putJSON(t, base+"/api/tree/values/db/host", body)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if ctrl.setValuePath != "db/host" {
		t.Errorf("path = %q, want %q", ctrl.setValuePath, "db/host")
	}
	if ctrl.setValue.Val != "newhost" {
		t.Errorf("value = %v, want %q", ctrl.setValue.Val, "newhost")
	}
}

func TestValidate(t *testing.T) {
	base, _ := startTestServer(t, newMockController())
	var results []map[string]any
	resp := postJSON(t, base+"/api/tree/validate", nil, &results)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0]["message"].(string), "TLS") {
		t.Errorf("unexpected message: %v", results[0]["message"])
	}
}

func TestSaveTree(t *testing.T) {
	ctrl := newMockController()
	base, _ := startTestServer(t, ctrl)

	resp := postJSON(t, base+"/api/tree/save", nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

func TestExportTemplates(t *testing.T) {
	base, _ := startTestServer(t, newMockController())
	var templates []map[string]any
	resp := getJSON(t, base+"/api/export/templates", &templates)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(templates) != 1 {
		t.Fatalf("got %d templates, want 1", len(templates))
	}
	if templates[0]["name"] != "env" {
		t.Errorf("template name = %v, want %q", templates[0]["name"], "env")
	}
}

func TestExport(t *testing.T) {
	ctrl := newMockController()
	base, _ := startTestServer(t, ctrl)

	reqBody := map[string]any{"template_path": "env.tmpl", "format": "env", "dry_run": true}
	var result map[string]any
	resp := postJSON(t, base+"/api/export", reqBody, &result)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if result["content"] != "DB_HOST=localhost" {
		t.Errorf("content = %v, want %q", result["content"], "DB_HOST=localhost")
	}
	if ctrl.exportReq == nil || !ctrl.exportReq.DryRun {
		t.Error("expected DryRun to be true")
	}
}

func TestApplySSE(t *testing.T) {
	ctrl := newMockController()
	base, _ := startTestServer(t, ctrl)

	data, _ := json.Marshal(map[string]string{"target": "production"})
	resp, err := http.Post(base+"/api/apply", "application/json", strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("POST /api/apply: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	// Parse SSE events.
	scanner := bufio.NewScanner(resp.Body)
	var outputEvents []map[string]string
	var doneEvent map[string]any

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: output") {
			if scanner.Scan() {
				dataLine := strings.TrimPrefix(scanner.Text(), "data: ")
				var ev map[string]string
				if err := json.Unmarshal([]byte(dataLine), &ev); err == nil {
					outputEvents = append(outputEvents, ev)
				}
			}
		} else if strings.HasPrefix(line, "event: done") {
			if scanner.Scan() {
				dataLine := strings.TrimPrefix(scanner.Text(), "data: ")
				json.Unmarshal([]byte(dataLine), &doneEvent) //nolint:errcheck
			}
		}
	}

	if len(outputEvents) != 2 {
		t.Fatalf("got %d output events, want 2", len(outputEvents))
	}
	if outputEvents[0]["line"] != "deploying..." {
		t.Errorf("first event line = %q, want %q", outputEvents[0]["line"], "deploying...")
	}
	if outputEvents[0]["stream"] != "stdout" {
		t.Errorf("first event stream = %q, want %q", outputEvents[0]["stream"], "stdout")
	}
	if doneEvent == nil {
		t.Fatal("no done event received")
	}
	if exitCode, ok := doneEvent["exit_code"].(float64); !ok || exitCode != 0 {
		t.Errorf("exit_code = %v, want 0", doneEvent["exit_code"])
	}
	if ctrl.applyTarget != "production" {
		t.Errorf("applyTarget = %q, want %q", ctrl.applyTarget, "production")
	}
}

func TestListComponents(t *testing.T) {
	base, _ := startTestServer(t, newMockController())
	var components []map[string]any
	resp := getJSON(t, base+"/api/components", &components)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(components) != 2 {
		t.Fatalf("got %d components, want 2", len(components))
	}
}

func TestEnableComponent(t *testing.T) {
	ctrl := newMockController()
	base, _ := startTestServer(t, ctrl)

	var result map[string]any
	resp := postJSON(t, base+"/api/components/cache/enable", nil, &result)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ctrl.enabledName != "cache" {
		t.Errorf("enabledName = %q, want %q", ctrl.enabledName, "cache")
	}
}

func TestDisableComponent(t *testing.T) {
	ctrl := newMockController()
	base, _ := startTestServer(t, ctrl)

	resp := postJSON(t, base+"/api/components/cache/disable", nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if ctrl.disabledName != "cache" {
		t.Errorf("disabledName = %q, want %q", ctrl.disabledName, "cache")
	}
}

// TestGRPCRoundTrip verifies the plugin works over gRPC via go-plugin.
func TestGRPCRoundTrip(t *testing.T) {
	h := &httpUI{addr: "127.0.0.1:0", ready: make(chan struct{})}
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"ui": &ui.GRPCPlugin{Impl: h},
	})
	raw, err := client.Dispense("ui")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	p := raw.(ui.Plugin)

	caps, err := p.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.RequiresTTY {
		t.Error("RequiresTTY should be false for HTTP UI")
	}

	// Run with a mock controller to verify the full round-trip.
	ctrl := newMockController()
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, ctrl)
	}()

	// Wait for the HTTP server to be ready using the synchronised method.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	addr, err := h.Addr(waitCtx)
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
	defer func() {
		_ = resp.Body.Close()
	}()
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck

	if result["name"] != "test-workspace" {
		t.Errorf("workspace name = %q, want %q", result["name"], "test-workspace")
	}

	cancel()
	// gRPC cancellation wraps the error; accept nil or context-related errors.
	if err := <-errCh; err != nil {
		if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "cancelled") {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
	}
}

// ---------- compile-time interface checks ----------

var _ ui.Plugin = (*httpUI)(nil)
var _ ui.Controller = (*mockController)(nil)
