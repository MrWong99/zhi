package webui

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// ---------- mock controller ----------

type mockController struct {
	workspaceName string
	tree          *config.Tree
	components    []ui.ComponentInfo
}

func newMockController() *mockController {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})
	_ = tree.Set("app/name", &config.Value{Val: "myapp"})
	return &mockController{
		workspaceName: "test-workspace",
		tree:          tree,
		components: []ui.ComponentInfo{
			{Name: "database", Description: "Database config", Enabled: true, Mandatory: true, Paths: []string{"db"}},
			{Name: "application", Description: "App config", Enabled: false, Paths: []string{"app"}},
		},
	}
}

func (m *mockController) WorkspaceName(_ context.Context) (string, error) {
	return m.workspaceName, nil
}

func (m *mockController) LoadTree(_ context.Context) (*config.Tree, error) {
	return m.tree, nil
}

func (m *mockController) SetValue(_ context.Context, path string, value config.Value) error {
	return m.tree.Set(path, &value)
}

func (m *mockController) Validate(_ context.Context) ([]config.ValidationResult, error) {
	return nil, nil
}

func (m *mockController) SaveTree(_ context.Context, _ string) error {
	return nil
}

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
	return m.components, nil
}

func (m *mockController) EnableComponent(_ context.Context, name string) ([]string, error) {
	return []string{name}, nil
}

func (m *mockController) DisableComponent(_ context.Context, _ string) error {
	return nil
}

func (m *mockController) SearchMarketplace(_ context.Context, _ ui.MarketplaceQuery) (*ui.MarketplaceResults, error) {
	return &ui.MarketplaceResults{}, nil
}

func (m *mockController) GetMarketplaceDetail(_ context.Context, _, _ string) (*ui.MarketplaceDetail, error) {
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

func (m *mockController) RatePlugin(_ context.Context, _, _ string, _ ui.Rating) error {
	return nil
}

// ---------- test helpers ----------

func startTestServer(t *testing.T) (string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ctrl := newMockController()

	cfg := Config{Addr: "127.0.0.1:0", AutoOpen: false}
	w := New(cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Run(ctx, ctrl)
	}()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer waitCancel()
	addr, err := w.Addr(waitCtx)
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

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(b)
}

// ---------- tests ----------

func TestRootRedirect(t *testing.T) {
	base, _ := startTestServer(t)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	loc := resp.Header.Get("Location")
	if loc != "/tree" {
		t.Errorf("Location = %q, want /tree", loc)
	}
}

func TestTreePage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "db") {
		t.Error("tree page does not contain 'db'")
	}
	if !strings.Contains(body, "localhost") {
		t.Error("tree page does not contain 'localhost'")
	}
	if !strings.Contains(body, "test-workspace") {
		t.Error("tree page does not contain workspace name")
	}
}

func TestTreeFilter(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree?filter=app")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "myapp") {
		t.Error("filtered tree should contain 'myapp'")
	}
}

func TestTreeHTMXFragment(t *testing.T) {
	base, _ := startTestServer(t)
	req, err := http.NewRequest("GET", base+"/tree?filter=db", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("HX-Request", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// Fragment should not contain the full layout.
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX fragment should not contain full HTML document")
	}
	if !strings.Contains(body, "host") {
		t.Error("fragment should contain 'host'")
	}
}

func TestNotFound(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/nonexistent")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if !strings.Contains(body, "404") {
		t.Error("404 page should contain '404'")
	}
}

func TestSecurityHeaders(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	resp.Body.Close()

	checks := map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "strict-origin-when-cross-origin",
		"Cross-Origin-Opener-Policy": "same-origin",
	}
	for header, expected := range checks {
		got := resp.Header.Get(header)
		if got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header missing")
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Error("CSP should contain frame-ancestors 'none'")
	}
}

func TestCSRFCookieSet(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	resp.Body.Close()

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "_csrf" {
			found = true
			if c.Value == "" {
				t.Error("CSRF cookie is empty")
			}
		}
	}
	if !found {
		t.Error("CSRF cookie not set")
	}
}

func TestCSRFTokenInPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if !strings.Contains(body, `name="csrf-token"`) {
		t.Error("page should contain CSRF token meta tag")
	}
}

func TestStaticFiles(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/static/css/tokens.css")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "--color-bg") {
		t.Error("tokens.css should contain CSS custom properties")
	}
	cc := resp.Header.Get("Cache-Control")
	if cc == "" {
		t.Error("static files should have Cache-Control header")
	}
}

func TestCapabilities(t *testing.T) {
	w := New(DefaultConfig())
	caps, err := w.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.RequiresTTY {
		t.Error("RequiresTTY should be false for web UI")
	}
}

// ---------- tree building tests ----------

func TestBuildNestedTree(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})
	_ = tree.Set("app/name", &config.Value{Val: "myapp"})

	nodes := buildNestedTree(tree, nil)
	if len(nodes) != 2 {
		t.Fatalf("got %d root nodes, want 2", len(nodes))
	}

	// Find the db node.
	var dbNode *treeNode
	for _, n := range nodes {
		if n.Name == "db" {
			dbNode = n
		}
	}
	if dbNode == nil {
		t.Fatal("db node not found")
	}
	if dbNode.IsLeaf {
		t.Error("db should not be a leaf")
	}
	if len(dbNode.Children) != 2 {
		t.Errorf("db has %d children, want 2", len(dbNode.Children))
	}
}

func TestBuildNestedTreeWithComponents(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})

	components := []ui.ComponentInfo{
		{Name: "database", Enabled: true, Paths: []string{"db"}},
	}

	nodes := buildNestedTree(tree, components)
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	// The leaf should have the component badge.
	leaf := nodes[0].Children[0]
	if leaf.Component != "database" {
		t.Errorf("component = %q, want 'database'", leaf.Component)
	}
	if !leaf.ComponentEnabled {
		t.Error("component should be enabled")
	}
}

func TestFilterTree(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})
	_ = tree.Set("app/name", &config.Value{Val: "myapp"})

	filtered := filterTree(tree, "app")
	paths := filtered.List()
	if len(paths) != 1 {
		t.Fatalf("filtered tree has %d paths, want 1", len(paths))
	}
	if paths[0] != "app/name" {
		t.Errorf("path = %q, want 'app/name'", paths[0])
	}
}

func TestFilterTreeCaseInsensitive(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})

	filtered := filterTree(tree, "DB")
	paths := filtered.List()
	if len(paths) != 1 {
		t.Fatalf("filtered tree has %d paths, want 1", len(paths))
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "hello"},
		{float64(42), "42"},
		{true, "true"},
		{nil, "null"},
	}
	for _, tt := range tests {
		got := formatValue(tt.input)
		if got != tt.expected {
			t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValueType(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "string"},
		{float64(42), "number"},
		{true, "bool"},
		{[]string{}, "other"},
	}
	for _, tt := range tests {
		got := valueType(tt.input)
		if got != tt.expected {
			t.Errorf("valueType(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ---------- middleware tests ----------

func TestCSRFValidation(t *testing.T) {
	csrf := newCSRFMiddleware()
	token := csrf.generateToken()

	if !csrf.validateToken(token) {
		t.Error("valid token rejected")
	}
	if csrf.validateToken("invalid-token") {
		t.Error("invalid token accepted")
	}
	if csrf.validateToken("") {
		t.Error("empty token accepted")
	}
	if csrf.validateToken("only.one.dot.too.many") {
		t.Error("malformed token accepted")
	}
}

func TestGenerateNonce(t *testing.T) {
	n1 := generateNonce()
	n2 := generateNonce()
	if n1 == "" {
		t.Error("nonce is empty")
	}
	if n1 == n2 {
		t.Error("nonces should be unique")
	}
}

// ---------- compile-time interface checks ----------

var _ ui.Plugin = (*WebUI)(nil)
var _ ui.Controller = (*mockController)(nil)
