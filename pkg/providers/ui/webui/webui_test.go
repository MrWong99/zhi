package webui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// ---------- mock controller ----------

type mockController struct {
	workspaceName   string
	tree            *config.Tree
	components      []ui.ComponentInfo
	exportTemplates []ui.ExportTemplate
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
		exportTemplates: []ui.ExportTemplate{
			{Name: "docker-compose", Format: "yaml", Output: "./docker-compose.override.yml"},
			{Name: "env-file", Format: "dotenv", Output: "./.env", Prefix: "app/env"},
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

func (m *mockController) SaveTree(_ context.Context) error {
	return nil
}

func (m *mockController) ExportTemplates(_ context.Context) ([]ui.ExportTemplate, error) {
	return m.exportTemplates, nil
}

func (m *mockController) Export(_ context.Context, req ui.ExportRequest) (*ui.ExportResult, error) {
	name := req.Format
	if name == "" {
		name = "custom"
	}
	content := "exported-content-" + name
	return &ui.ExportResult{
		Name:       name,
		Content:    content,
		OutputPath: req.OutputPath,
	}, nil
}

func (m *mockController) Apply(_ context.Context, target string, handler func(ui.ApplyEvent)) (*ui.ApplyResult, error) {
	handler(ui.ApplyEvent{Line: "running " + target, Stream: "stdout"})
	handler(ui.ApplyEvent{Line: "done", Stream: "stdout"})
	return &ui.ApplyResult{ExitCode: 0}, nil
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

func (m *mockController) SearchMarketplace(_ context.Context, q ui.MarketplaceQuery) (*ui.MarketplaceResults, error) {
	results := []ui.MarketplaceEntry{
		{
			Name:          "zhi-store-vault",
			Publisher:     "hashicorp",
			Type:          "store",
			Description:   "HashiCorp Vault KV v2 backend",
			LatestVersion: "1.2.0",
			Rating:        4.2,
			RatingCount:   42,
			Downloads:     1200,
			Verified:      true,
			Platforms:     []string{"linux/amd64", "darwin/arm64"},
		},
		{
			Name:          "zhi-config-env",
			Publisher:     "community",
			Type:          "config",
			Description:   "Environment variable config provider",
			LatestVersion: "0.5.0",
			Rating:        3.8,
			RatingCount:   15,
			Downloads:     800,
			Installed:     true,
			InstalledVer:  "0.4.0",
			UpdateAvail:   true,
		},
	}
	// Apply type filter if present.
	if q.Type != "" {
		var filtered []ui.MarketplaceEntry
		for _, r := range results {
			if r.Type == q.Type {
				filtered = append(filtered, r)
			}
		}
		return &ui.MarketplaceResults{Total: len(filtered), Results: filtered}, nil
	}
	return &ui.MarketplaceResults{Total: len(results), Results: results}, nil
}

func (m *mockController) GetMarketplaceDetail(_ context.Context, _, publisher, name string) (*ui.MarketplaceDetail, error) {
	return &ui.MarketplaceDetail{
		MarketplaceEntry: ui.MarketplaceEntry{
			Name:          name,
			Publisher:     publisher,
			Type:          "store",
			Description:   "A test plugin",
			LatestVersion: "1.0.0",
			Rating:        4.5,
			RatingCount:   10,
			Downloads:     500,
			Verified:      true,
			Platforms:     []string{"linux/amd64"},
		},
		LongDescription: "A detailed description of the plugin.",
		License:         "MIT",
		Keywords:        []string{"vault", "store"},
	}, nil
}

func (m *mockController) InstallPlugin(_ context.Context, ref string) (*ui.InstallResult, error) {
	return &ui.InstallResult{Name: ref, Version: "1.0.0"}, nil
}

func (m *mockController) UninstallPlugin(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockController) ListInstalledPlugins(_ context.Context) ([]ui.InstalledPlugin, error) {
	return []ui.InstalledPlugin{
		{
			Name:    "structuredfile",
			Type:    "config",
			Version: "1.0.0",
			Source:  "built-in",
		},
		{
			Name:     "zhi-store-vault",
			Type:     "store",
			Version:  "1.1.0",
			Source:   "registry.zhi.dev/hashicorp/zhi-store-vault",
			Verified: true,
		},
	}, nil
}

func (m *mockController) CheckUpdates(_ context.Context) ([]ui.PluginUpdate, error) {
	return []ui.PluginUpdate{
		{Name: "zhi-store-vault", Type: "store", CurrentVersion: "1.1.0", LatestVersion: "1.2.0"},
	}, nil
}

func (m *mockController) UpdatePlugin(_ context.Context, name, version string) (*ui.InstallResult, error) {
	if version == "" {
		version = "1.2.0"
	}
	return &ui.InstallResult{Name: name, Version: version}, nil
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

	nodes := buildNestedTree(tree, tree, nil)
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

	nodes := buildNestedTree(tree, tree, components)
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
		{map[string]string{}, "{}"},
		{map[string]string{"a": "1", "b": "2"}, "{a: 1, b: 2}"},
		{[]string{}, "[]"},
		{[]string{"x", "y"}, "[x, y]"},
		{[]any{}, "[]"},
		{[]any{"a", "b"}, "[a, b]"},
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
		{map[string]any{"k": "v"}, "map"},
		{map[string]string{"k": "v"}, "map"},
		{[]any{"a", "b"}, "list"},
		{[]string{"a"}, "list"},
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
	csrf := newCSRFMiddleware(true)
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

// ---------- editor tests ----------

// getCSRFToken fetches a page and extracts the CSRF cookie.
func getCSRFToken(t *testing.T, base string) (string, []*http.Cookie) {
	t.Helper()
	resp := get(t, base+"/tree")
	resp.Body.Close()
	token := ""
	for _, c := range resp.Cookies() {
		if c.Name == "_csrf" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("no CSRF cookie found")
	}
	return token, resp.Cookies()
}

func postWithCSRF(t *testing.T, url, body, csrfToken string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestEditForm(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree/edit/db/host")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	// Should contain a form with the current value.
	if !strings.Contains(body, "localhost") {
		t.Error("edit form should contain current value 'localhost'")
	}
	if !strings.Contains(body, `name="value"`) || !strings.Contains(body, `name="value_type"`) {
		t.Error("edit form should contain value and value_type inputs")
	}
	if !strings.Contains(body, "editor-form") {
		t.Error("edit form should have editor-form class")
	}
}

func TestEditFormNotFound(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree/edit/nonexistent/path")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSaveValue(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/tree/values/db/host", "value=newhost&value_type=string", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	// Should return the display fragment with the new value.
	if !strings.Contains(body, "newhost") {
		t.Error("save response should contain updated value 'newhost'")
	}
	// Should trigger unsaved changes.
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "markUnsaved") {
		t.Error("save response should trigger markUnsaved")
	}
}

func TestSaveValueNumber(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/tree/values/db/port", "value=3306&value_type=number", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "3306") {
		t.Error("save response should contain updated number value '3306'")
	}
}

func TestDisplayValue(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree/display/db/host")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "localhost") {
		t.Error("display should contain 'localhost'")
	}
	if !strings.Contains(body, "tree-value") {
		t.Error("display should contain tree-value class")
	}
}

func TestDisplayValueNotFound(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree/display/nonexistent/path")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// ---------- validation tests ----------

func TestValidationPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/validation")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Validation") {
		t.Error("validation page should contain 'Validation'")
	}
}

func TestFullValidation(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/validate", "", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	// Mock controller returns no validation results.
	if !strings.Contains(body, "No validation issues") {
		t.Error("validation results should show no issues when controller returns empty results")
	}
}

func TestInlineValidation(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/validate/inline/db/host", "value=testval&value_type=string", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	// Mock returns no validation results, so badge should be empty.
	_ = body // No error means success.
}

// ---------- save tests ----------

func TestSaveTree(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/tree/save", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "showNotification") {
		t.Error("save should trigger showNotification")
	}
	if !strings.Contains(trigger, "markSaved") {
		t.Error("save should trigger markSaved")
	}
	if !strings.Contains(trigger, "success") {
		t.Error("save should show success notification")
	}
}

// ---------- component tests ----------

func TestComponentsPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/components")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "database") {
		t.Error("components page should contain 'database' component")
	}
	if !strings.Contains(body, "application") {
		t.Error("components page should contain 'application' component")
	}
	if !strings.Contains(body, "Mandatory") {
		t.Error("components page should show mandatory badge for database")
	}
}

func TestComponentToggle(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	// Toggle the non-mandatory 'application' component.
	resp := postWithCSRF(t, base+"/components/application/toggle", "", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "component-card") {
		t.Error("toggle response should contain component card")
	}
}

func TestComponentToggleMandatory(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	// Try to toggle the mandatory 'database' component.
	resp := postWithCSRF(t, base+"/components/database/toggle", "", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "Cannot toggle mandatory") {
		t.Error("toggling mandatory component should show error")
	}
}

func TestComponentToggleNotFound(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/components/nonexistent/toggle", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// ---------- shortcuts page test ----------

func TestShortcutsPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/shortcuts")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Keyboard Shortcuts") {
		t.Error("shortcuts page should contain 'Keyboard Shortcuts'")
	}
	if !strings.Contains(body, "Ctrl") {
		t.Error("shortcuts page should document Ctrl+S shortcut")
	}
}

// ---------- tree node has edit button ----------

func TestTreePageHasEditButtons(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "edit-btn") {
		t.Error("tree page should contain edit buttons")
	}
	if !strings.Contains(body, "tree-value") {
		t.Error("tree page should contain tree-value containers")
	}
}

// ---------- sidebar navigation ----------

func TestSidebarNavigation(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)

	for _, link := range []string{"/tree", "/components", "/validation", "/shortcuts"} {
		if !strings.Contains(body, link) {
			t.Errorf("sidebar should contain link to %s", link)
		}
	}
}

// ---------- topbar save button ----------

func TestTopbarHasSaveButton(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if !strings.Contains(body, "save-tree-btn") {
		t.Error("topbar should contain save button")
	}
	if !strings.Contains(body, "unsaved-indicator") {
		t.Error("topbar should contain unsaved changes indicator")
	}
}

// ---------- unit tests for editor helpers ----------

func TestParseFormValue(t *testing.T) {
	tests := []struct {
		raw      string
		valType  string
		expected any
	}{
		{"hello", "string", "hello"},
		{"42", "number", float64(42)},
		{"3.14", "number", float64(3.14)},
		{"invalid", "number", "invalid"},
		{"true", "bool", true},
		{"false", "bool", false},
		{"on", "bool", true},
		{"", "bool", false},
	}
	for _, tt := range tests {
		got := parseFormValue(tt.raw, tt.valType)
		if got != tt.expected {
			t.Errorf("parseFormValue(%q, %q) = %v (%T), want %v (%T)",
				tt.raw, tt.valType, got, got, tt.expected, tt.expected)
		}
	}
}

func TestExtractMapEntries(t *testing.T) {
	t.Parallel()

	entries := extractMapEntries(map[string]string{"b": "2", "a": "1"})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Sorted by key.
	if entries[0].Key != "a" || entries[0].Value != "1" {
		t.Errorf("expected first entry a=1, got %s=%s", entries[0].Key, entries[0].Value)
	}
	if entries[1].Key != "b" || entries[1].Value != "2" {
		t.Errorf("expected second entry b=2, got %s=%s", entries[1].Key, entries[1].Value)
	}

	// map[string]any support.
	entries2 := extractMapEntries(map[string]any{"k": 42})
	if len(entries2) != 1 || entries2[0].Value != "42" {
		t.Errorf("expected entry k=42 for map[string]any, got %v", entries2)
	}

	// Non-map returns empty.
	entries3 := extractMapEntries("not a map")
	if len(entries3) != 0 {
		t.Errorf("expected empty for non-map, got %v", entries3)
	}
}

func TestExtractListItems(t *testing.T) {
	t.Parallel()

	items := extractListItems([]string{"a", "b", "c"})
	if len(items) != 3 || items[0] != "a" {
		t.Errorf("expected [a b c], got %v", items)
	}

	items2 := extractListItems([]any{"x", 42})
	if len(items2) != 2 || items2[1] != "42" {
		t.Errorf("expected [x 42], got %v", items2)
	}

	items3 := extractListItems("not a list")
	if items3 != nil {
		t.Errorf("expected nil for non-list, got %v", items3)
	}
}

func TestParseFormValueFromRequest_Map(t *testing.T) {
	t.Parallel()

	form := url.Values{
		"map_key":   {"host", "port", ""},
		"map_value": {"localhost", "5432", "ignored"},
	}
	req := &http.Request{Form: form}

	result := parseFormValueFromRequest(req, "map")
	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", result)
	}
	if len(m) != 2 {
		t.Errorf("expected 2 entries (empty key skipped), got %d", len(m))
	}
	if m["host"] != "localhost" || m["port"] != "5432" {
		t.Errorf("unexpected map contents: %v", m)
	}
}

func TestParseFormValueFromRequest_List(t *testing.T) {
	t.Parallel()

	form := url.Values{
		"list_item": {"alpha", "", "beta"},
	}
	req := &http.Request{Form: form}

	result := parseFormValueFromRequest(req, "list")
	items, ok := result.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", result)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items (empty skipped), got %d", len(items))
	}
	if items[0] != "alpha" || items[1] != "beta" {
		t.Errorf("unexpected list contents: %v", items)
	}
}

func TestFormatEditValue(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
	}
	for _, tt := range tests {
		got := formatEditValue(tt.input)
		if got != tt.expected {
			t.Errorf("formatEditValue(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestPathToID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"db/host", "db--host"},
		{"app/tls/cert.pem", "app--tls--cert.pem"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		got := pathToID(tt.path)
		if got != tt.expected {
			t.Errorf("pathToID(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestGroupValidationResults(t *testing.T) {
	results := []config.ValidationResult{
		{Severity: config.Blocking, Message: "blocking error", Metadata: map[string]any{"path": "db/host"}},
		{Severity: config.Warning, Message: "a warning", Metadata: map[string]any{"path": "db/port"}},
		{Severity: config.Info, Message: "info message"},
		{Severity: config.Blocking, Message: "another blocking"},
	}

	groups := groupValidationResults(results)
	if len(groups) != 3 {
		t.Fatalf("got %d groups, want 3", len(groups))
	}
	// Should be in order: Blocking, Warning, Info.
	if groups[0].Severity != "Blocking" || len(groups[0].Results) != 2 {
		t.Errorf("first group: severity=%s count=%d, want Blocking/2", groups[0].Severity, len(groups[0].Results))
	}
	if groups[1].Severity != "Warning" || len(groups[1].Results) != 1 {
		t.Errorf("second group: severity=%s count=%d, want Warning/1", groups[1].Severity, len(groups[1].Results))
	}
	if groups[2].Severity != "Info" || len(groups[2].Results) != 1 {
		t.Errorf("third group: severity=%s count=%d, want Info/1", groups[2].Severity, len(groups[2].Results))
	}
}

func TestFilterValidationForPath(t *testing.T) {
	results := []config.ValidationResult{
		{Severity: config.Blocking, Message: "error on db/host", Metadata: map[string]any{"path": "db/host"}},
		{Severity: config.Warning, Message: "warning on db/port", Metadata: map[string]any{"path": "db/port"}},
		{Severity: config.Info, Message: "info on app/name", Metadata: map[string]any{"path": "app/name"}},
		{Severity: config.Info, Message: "no path metadata"},
	}

	filtered := filterValidationForPath(results, "db/host")
	if len(filtered) != 1 {
		t.Fatalf("got %d results, want 1", len(filtered))
	}
	if filtered[0].Message != "error on db/host" {
		t.Errorf("filtered message = %q, want 'error on db/host'", filtered[0].Message)
	}
}

func TestHasBlockingResults(t *testing.T) {
	noBlocking := []config.ValidationResult{
		{Severity: config.Warning, Message: "warn"},
		{Severity: config.Info, Message: "info"},
	}
	if hasBlockingResults(noBlocking) {
		t.Error("should not have blocking results")
	}

	withBlocking := []config.ValidationResult{
		{Severity: config.Warning, Message: "warn"},
		{Severity: config.Blocking, Message: "block"},
	}
	if !hasBlockingResults(withBlocking) {
		t.Error("should have blocking results")
	}
}

func TestToComponentViewData(t *testing.T) {
	components := []ui.ComponentInfo{
		{Name: "db", Description: "Database", Enabled: true, Mandatory: true, Paths: []string{"db"}, Dependencies: []string{"core"}},
		{Name: "app", Description: "App", Enabled: false},
	}
	result := toComponentViewData(components)
	if len(result) != 2 {
		t.Fatalf("got %d items, want 2", len(result))
	}
	if result[0].Name != "db" || !result[0].Mandatory || !result[0].Enabled {
		t.Error("first component should be db, mandatory, enabled")
	}
	if result[1].Name != "app" || result[1].Enabled {
		t.Error("second component should be app, disabled")
	}
}

// ---------- tree node PathID ----------

func TestTreeNodeHasPathID(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})

	nodes := buildNestedTree(tree, tree, nil)
	leaf := nodes[0].Children[0]
	if leaf.PathID != "db--host" {
		t.Errorf("PathID = %q, want 'db--host'", leaf.PathID)
	}
}

// ---------- export tests ----------

func TestExportPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/export")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Export") {
		t.Error("export page should contain 'Export'")
	}
	if !strings.Contains(body, "docker-compose") {
		t.Error("export page should contain 'docker-compose' template")
	}
	if !strings.Contains(body, "env-file") {
		t.Error("export page should contain 'env-file' template")
	}
	if !strings.Contains(body, "JSON") {
		t.Error("export page should contain quick export JSON button")
	}
	if !strings.Contains(body, "YAML") {
		t.Error("export page should contain quick export YAML button")
	}
}

func TestExportPageHasExportAllButton(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/export")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Export All") {
		t.Error("export page should contain 'Export All' button when templates exist")
	}
}

func TestExportPreview(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/export/preview", "format=yaml", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "export-preview") {
		t.Error("preview response should contain export-preview class")
	}
	if !strings.Contains(body, "exported-content-yaml") {
		t.Error("preview response should contain exported content")
	}
}

func TestExportExecution(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/export", "format=json&output=/tmp/test.json", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "showNotification") {
		t.Error("export should trigger showNotification")
	}
	if !strings.Contains(trigger, "success") {
		t.Error("export result should indicate success")
	}
}

func TestExportAll(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/export/all", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "showNotification") {
		t.Error("export all should trigger showNotification")
	}
	if !strings.Contains(trigger, "Exported all") {
		t.Error("export all should show success message")
	}
}

func TestExportHTMXFragment(t *testing.T) {
	base, _ := startTestServer(t)
	req, err := http.NewRequest("GET", base+"/export", nil)
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
	if !strings.Contains(body, "docker-compose") {
		t.Error("fragment should contain template names")
	}
}

// ---------- apply tests ----------

func TestApplyPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/apply")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Apply") {
		t.Error("apply page should contain 'Apply'")
	}
	if !strings.Contains(body, "apply-terminal") {
		t.Error("apply page should contain terminal element")
	}
	if !strings.Contains(body, "default") {
		t.Error("apply page should contain default target")
	}
}

func TestApplyRunSSE(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/apply/run", "target=default", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}

	// Verify SSE content type.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Verify SSE format: should contain event: output and event: done.
	if !strings.Contains(body, "event: output") {
		t.Error("SSE stream should contain 'event: output'")
	}
	if !strings.Contains(body, "event: done") {
		t.Error("SSE stream should contain 'event: done'")
	}
	if !strings.Contains(body, "running default") {
		t.Error("SSE stream should contain mock output 'running default'")
	}
	if !strings.Contains(body, `"exit_code":0`) {
		t.Error("SSE done event should contain exit_code 0")
	}
}

func TestApplyRunPreExport(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/apply/run", "target=default&pre_export=true", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	// Should still produce the SSE stream after pre-export.
	if !strings.Contains(body, "event: done") {
		t.Error("SSE stream should complete after pre-export")
	}
}

// ---------- sidebar navigation ----------

func TestSidebarHasExportAndApplyLinks(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if !strings.Contains(body, "/export") {
		t.Error("sidebar should contain link to /export")
	}
	if !strings.Contains(body, "/apply") {
		t.Error("sidebar should contain link to /apply")
	}
}

// ---------- shortcuts page ----------

func TestShortcutsPageHasExportAndApply(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/shortcuts")
	body := bodyString(t, resp)
	if !strings.Contains(body, "Export") {
		t.Error("shortcuts page should document Export shortcut")
	}
	if !strings.Contains(body, "Apply") {
		t.Error("shortcuts page should document Apply shortcut")
	}
}

// ---------- unit tests ----------

func TestToExportTemplateData(t *testing.T) {
	templates := []ui.ExportTemplate{
		{Name: "t1", Format: "yaml", Output: "out.yaml"},
		{Name: "t2", Template: "tmpl.txt", Output: "out.txt", Prefix: "app"},
	}
	result := toExportTemplateData(templates)
	if len(result) != 2 {
		t.Fatalf("got %d items, want 2", len(result))
	}
	if result[0].Name != "t1" || result[0].Format != "yaml" {
		t.Error("first template should be t1/yaml")
	}
	if result[1].Name != "t2" || result[1].Template != "tmpl.txt" || result[1].Prefix != "app" {
		t.Error("second template should be t2 with template and prefix")
	}
}

func TestFormatForCSS(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"json", "json"},
		{"yaml", "yaml"},
		{"toml", "toml"},
		{"dotenv", "dotenv"},
		{"custom", "text"},
		{"", "text"},
	}
	for _, tt := range tests {
		got := formatForCSS(tt.input)
		if got != tt.expected {
			t.Errorf("formatForCSS(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestToApplyTargetData(t *testing.T) {
	targets := toApplyTargetData(nil)
	if len(targets) != 1 {
		t.Fatalf("got %d targets, want 1", len(targets))
	}
	if targets[0].Name != "default" {
		t.Errorf("target name = %q, want 'default'", targets[0].Name)
	}
}

// ---------- marketplace tests ----------

func TestMarketplacePage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/marketplace")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Marketplace") {
		t.Error("marketplace page should contain 'Marketplace'")
	}
	if !strings.Contains(body, "zhi-store-vault") {
		t.Error("marketplace page should contain 'zhi-store-vault'")
	}
	if !strings.Contains(body, "zhi-config-env") {
		t.Error("marketplace page should contain 'zhi-config-env'")
	}
	if !strings.Contains(body, "hashicorp") {
		t.Error("marketplace page should contain publisher 'hashicorp'")
	}
	if !strings.Contains(body, "2 results") {
		t.Error("marketplace page should show '2 results'")
	}
}

func TestMarketplaceHTMXFragment(t *testing.T) {
	base, _ := startTestServer(t)
	req, err := http.NewRequest("GET", base+"/marketplace?q=vault", nil)
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
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX fragment should not contain full HTML document")
	}
	if !strings.Contains(body, "zhi-store-vault") {
		t.Error("fragment should contain search results")
	}
}

func TestMarketplaceTypeFilter(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/marketplace?type=store")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "zhi-store-vault") {
		t.Error("filtered results should contain store plugin")
	}
	if !strings.Contains(body, "1 result") {
		t.Error("filtered results should show '1 result'")
	}
}

func TestMarketplaceVerifiedBadge(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/marketplace")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "verified-badge") {
		t.Error("marketplace should show verified badge for verified plugins")
	}
}

func TestMarketplaceInstalledBadge(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/marketplace")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// The zhi-config-env plugin is installed with an update available.
	if !strings.Contains(body, "Update") {
		t.Error("marketplace should show Update button for plugins with updates")
	}
}

func TestPluginDetailPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/marketplace/store/hashicorp/zhi-store-vault")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "zhi-store-vault") {
		t.Error("detail page should contain plugin name")
	}
	if !strings.Contains(body, "hashicorp") {
		t.Error("detail page should contain publisher")
	}
	if !strings.Contains(body, "A detailed description") {
		t.Error("detail page should contain long description")
	}
	if !strings.Contains(body, "MIT") {
		t.Error("detail page should contain license")
	}
	if !strings.Contains(body, "vault") {
		t.Error("detail page should contain keywords")
	}
}

func TestPluginDetailHasRatingForm(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/marketplace/store/hashicorp/zhi-store-vault")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "star-rating") {
		t.Error("detail page should contain star rating form")
	}
	if !strings.Contains(body, "Submit Rating") {
		t.Error("detail page should contain rating submit button")
	}
}

func TestRatePlugin(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/marketplace/store/hashicorp/zhi-store-vault/rate", "score=5&comment=Great+plugin", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "showNotification") {
		t.Error("rate should trigger showNotification")
	}
	if !strings.Contains(trigger, "success") {
		t.Error("rate should show success notification")
	}
}

func TestRatePluginInvalidScore(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/marketplace/store/hashicorp/zhi-store-vault/rate", "score=0", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// ---------- installed plugins tests ----------

func TestPluginsPage(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/plugins")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Installed Plugins") {
		t.Error("plugins page should contain 'Installed Plugins'")
	}
	if !strings.Contains(body, "structuredfile") {
		t.Error("plugins page should list 'structuredfile' plugin")
	}
	if !strings.Contains(body, "zhi-store-vault") {
		t.Error("plugins page should list 'zhi-store-vault' plugin")
	}
}

func TestPluginsPageUpdateBadge(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/plugins")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "1.2.0") {
		t.Error("plugins page should show available update version")
	}
	if !strings.Contains(body, "Update All") {
		t.Error("plugins page should show 'Update All' button when updates available")
	}
}

func TestPluginsPageHTMXFragment(t *testing.T) {
	base, _ := startTestServer(t)
	req, err := http.NewRequest("GET", base+"/plugins", nil)
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
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX fragment should not contain full HTML document")
	}
}

func TestInstallPlugin(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/plugins/install", "ref=hashicorp/zhi-store-vault", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "showNotification") {
		t.Error("install should trigger showNotification")
	}
	if !strings.Contains(trigger, "success") {
		t.Error("install should show success notification")
	}
}

func TestInstallPluginMissingRef(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/plugins/install", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUninstallPlugin(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/plugins/zhi-store-vault/uninstall", "type=store", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "Uninstalled") {
		t.Error("uninstall should show success message")
	}
}

func TestUpdatePlugin(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/plugins/zhi-store-vault/update", "version=1.2.0", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "Updated") {
		t.Error("update should show success message")
	}
}

func TestUpdateAllPlugins(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/plugins/update-all", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "showNotification") {
		t.Error("update all should trigger showNotification")
	}
	if !strings.Contains(trigger, "Updated all") {
		t.Error("update all should show success message")
	}
}

// ---------- sidebar navigation ----------

func TestSidebarHasMarketplaceAndPluginsLinks(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if !strings.Contains(body, "/marketplace") {
		t.Error("sidebar should contain link to /marketplace")
	}
	if !strings.Contains(body, "/plugins") {
		t.Error("sidebar should contain link to /plugins")
	}
}

// ---------- shortcuts ----------

func TestShortcutsPageHasMarketplaceAndPlugins(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/shortcuts")
	body := bodyString(t, resp)
	if !strings.Contains(body, "Marketplace") {
		t.Error("shortcuts page should document Marketplace shortcut")
	}
	if !strings.Contains(body, "Plugins") {
		t.Error("shortcuts page should document Plugins shortcut")
	}
}

// ---------- capabilities ----------

func TestCapabilitiesSupportsMarketplace(t *testing.T) {
	w := New(DefaultConfig())
	caps, err := w.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if !caps.SupportsMarketplace {
		t.Error("SupportsMarketplace should be true for web UI")
	}
}

// ---------- accessibility ----------

func TestLayoutHasSkipLink(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if !strings.Contains(body, "skip-link") {
		t.Error("layout should contain skip-to-content link")
	}
	if !strings.Contains(body, `role="main"`) {
		t.Error("content area should have role=main")
	}
}

// ---------- unit tests ----------

func TestBuildMarketplaceQuery(t *testing.T) {
	r, _ := http.NewRequest("GET", "/marketplace?q=vault&type=store&sort=downloads&verified=true&page=2", nil)
	q := buildMarketplaceQuery(r)
	if q.Query != "vault" {
		t.Errorf("Query = %q, want 'vault'", q.Query)
	}
	if q.Type != "store" {
		t.Errorf("Type = %q, want 'store'", q.Type)
	}
	if q.Sort != "downloads" {
		t.Errorf("Sort = %q, want 'downloads'", q.Sort)
	}
	if !q.Verified {
		t.Error("Verified should be true")
	}
	if q.Page != 2 {
		t.Errorf("Page = %d, want 2", q.Page)
	}
}

func TestBuildMarketplaceQueryDefaults(t *testing.T) {
	r, _ := http.NewRequest("GET", "/marketplace", nil)
	q := buildMarketplaceQuery(r)
	if q.Sort != "relevance" {
		t.Errorf("default Sort = %q, want 'relevance'", q.Sort)
	}
	if q.Page != 1 {
		t.Errorf("default Page = %d, want 1", q.Page)
	}
	if q.PerPage != 20 {
		t.Errorf("default PerPage = %d, want 20", q.PerPage)
	}
}

func TestPageCount(t *testing.T) {
	tests := []struct {
		total   int
		perPage int
		want    int
	}{
		{0, 20, 1},
		{10, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{40, 20, 2},
		{41, 20, 3},
		{100, 0, 1},
	}
	for _, tt := range tests {
		got := pageCount(tt.total, tt.perPage)
		if got != tt.want {
			t.Errorf("pageCount(%d, %d) = %d, want %d", tt.total, tt.perPage, got, tt.want)
		}
	}
}

func TestRatingStars(t *testing.T) {
	tests := []struct {
		rating float64
		want   string
	}{
		{0, "\u2606\u2606\u2606\u2606\u2606"},
		{3.0, "\u2605\u2605\u2605\u2606\u2606"},
		{5.0, "\u2605\u2605\u2605\u2605\u2605"},
		{4.7, "\u2605\u2605\u2605\u2605\u2606"},
	}
	for _, tt := range tests {
		got := ratingStars(tt.rating)
		if got != tt.want {
			t.Errorf("ratingStars(%v) = %q, want %q", tt.rating, got, tt.want)
		}
	}
}

func TestFormatDownloads(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0k"},
		{1200, "1.2k"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}
	for _, tt := range tests {
		got := formatDownloads(tt.n)
		if got != tt.want {
			t.Errorf("formatDownloads(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	got := formatTime(time.Time{})
	if got != "" {
		t.Errorf("formatTime(zero) = %q, want empty", got)
	}
	tm := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	got = formatTime(tm)
	if got != "2025-01-15" {
		t.Errorf("formatTime = %q, want '2025-01-15'", got)
	}
}

func TestSeqFunc(t *testing.T) {
	got := seqFunc(1, 5)
	if len(got) != 5 {
		t.Fatalf("seqFunc(1,5) returned %d items, want 5", len(got))
	}
	if got[0] != 1 || got[4] != 5 {
		t.Errorf("seqFunc(1,5) = %v, want [1,2,3,4,5]", got)
	}
	empty := seqFunc(5, 1)
	if len(empty) != 0 {
		t.Errorf("seqFunc(5,1) should be empty, got %v", empty)
	}
}

func TestTriggerNotification(t *testing.T) {
	got := triggerNotification("success", "test message")
	if !strings.Contains(got, "success") {
		t.Error("should contain notification type")
	}
	if !strings.Contains(got, "test message") {
		t.Error("should contain notification message")
	}
}

// ---------- error mock controller ----------

// errorMockController wraps mockController and allows configuring errors
// for any method.
type errorMockController struct {
	mockController
	loadTreeErr          error
	listComponentsErr    error
	validateErr          error
	saveTreeErr          error
	exportTemplatesErr   error
	exportErr            error
	applyErr             error
	searchMarketplaceErr error
	getDetailErr         error
	installPluginErr     error
	listPluginsErr       error
	checkUpdatesErr      error
	ratePluginErr        error
}

func (m *errorMockController) LoadTree(ctx context.Context) (*config.Tree, error) {
	if m.loadTreeErr != nil {
		return nil, m.loadTreeErr
	}
	return m.mockController.LoadTree(ctx)
}

func (m *errorMockController) ListComponents(ctx context.Context) ([]ui.ComponentInfo, error) {
	if m.listComponentsErr != nil {
		return nil, m.listComponentsErr
	}
	return m.mockController.ListComponents(ctx)
}

func (m *errorMockController) Validate(ctx context.Context) ([]config.ValidationResult, error) {
	if m.validateErr != nil {
		return nil, m.validateErr
	}
	return m.mockController.Validate(ctx)
}

func (m *errorMockController) SaveTree(ctx context.Context) error {
	if m.saveTreeErr != nil {
		return m.saveTreeErr
	}
	return m.mockController.SaveTree(ctx)
}

func (m *errorMockController) ExportTemplates(ctx context.Context) ([]ui.ExportTemplate, error) {
	if m.exportTemplatesErr != nil {
		return nil, m.exportTemplatesErr
	}
	return m.mockController.ExportTemplates(ctx)
}

func (m *errorMockController) Export(ctx context.Context, req ui.ExportRequest) (*ui.ExportResult, error) {
	if m.exportErr != nil {
		return nil, m.exportErr
	}
	return m.mockController.Export(ctx, req)
}

func (m *errorMockController) Apply(ctx context.Context, target string, handler func(ui.ApplyEvent)) (*ui.ApplyResult, error) {
	if m.applyErr != nil {
		return nil, m.applyErr
	}
	return m.mockController.Apply(ctx, target, handler)
}

func (m *errorMockController) SearchMarketplace(ctx context.Context, q ui.MarketplaceQuery) (*ui.MarketplaceResults, error) {
	if m.searchMarketplaceErr != nil {
		return nil, m.searchMarketplaceErr
	}
	return m.mockController.SearchMarketplace(ctx, q)
}

func (m *errorMockController) GetMarketplaceDetail(ctx context.Context, pluginType, publisher, name string) (*ui.MarketplaceDetail, error) {
	if m.getDetailErr != nil {
		return nil, m.getDetailErr
	}
	return m.mockController.GetMarketplaceDetail(ctx, pluginType, publisher, name)
}

func (m *errorMockController) InstallPlugin(ctx context.Context, ref string) (*ui.InstallResult, error) {
	if m.installPluginErr != nil {
		return nil, m.installPluginErr
	}
	return m.mockController.InstallPlugin(ctx, ref)
}

func (m *errorMockController) ListInstalledPlugins(ctx context.Context) ([]ui.InstalledPlugin, error) {
	if m.listPluginsErr != nil {
		return nil, m.listPluginsErr
	}
	return m.mockController.ListInstalledPlugins(ctx)
}

func (m *errorMockController) CheckUpdates(ctx context.Context) ([]ui.PluginUpdate, error) {
	if m.checkUpdatesErr != nil {
		return nil, m.checkUpdatesErr
	}
	return m.mockController.CheckUpdates(ctx)
}

func (m *errorMockController) RatePlugin(ctx context.Context, pluginType, pub, name string, r ui.Rating) error {
	if m.ratePluginErr != nil {
		return m.ratePluginErr
	}
	return m.mockController.RatePlugin(ctx, pluginType, pub, name, r)
}

// startTestServerWithCtrl is like startTestServer but with a custom controller.
func startTestServerWithCtrl(t *testing.T, ctrl ui.Controller) (string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	cfg := Config{Addr: "127.0.0.1:0", AutoOpen: false}
	srv, err := NewServer(ctrl, cfg)
	if err != nil {
		cancel()
		t.Fatalf("NewServer: %v", err)
	}

	w := &WebUI{config: cfg, ready: make(chan struct{})}
	w.server = srv
	close(w.ready)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer waitCancel()
	addr, err := srv.Addr(waitCtx)
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

// ---------- error path tests ----------

func TestLoadTreeError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		loadTreeErr:    errors.New("tree unavailable"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "tree unavailable") {
		t.Error("error page should contain error message")
	}
}

func TestListComponentsError(t *testing.T) {
	ctrl := &errorMockController{
		mockController:    *newMockController(),
		listComponentsErr: errors.New("components broken"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/components")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "components broken") {
		t.Error("error page should contain error message")
	}
}

func TestValidateError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		validateErr:    errors.New("validation boom"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/validation")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "validation boom") {
		t.Error("error page should contain error message")
	}
}

func TestSaveTreeError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		saveTreeErr:    errors.New("save failed"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/tree/save", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestExportTemplatesError(t *testing.T) {
	ctrl := &errorMockController{
		mockController:     *newMockController(),
		exportTemplatesErr: errors.New("templates broken"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/export")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body = %s", resp.StatusCode, body)
	}
}

func TestExportError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		exportErr:      errors.New("export failed"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/export", "format=json", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	trigger := resp.Header.Get("HX-Trigger")
	if !strings.Contains(trigger, "export failed") {
		t.Error("export result should contain error message")
	}
}

func TestApplyError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		applyErr:       errors.New("apply boom"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/apply/run", "target=default", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "apply boom") {
		t.Error("SSE stream should contain error message")
	}
}

func TestSearchMarketplaceError(t *testing.T) {
	ctrl := &errorMockController{
		mockController:       *newMockController(),
		searchMarketplaceErr: errors.New("marketplace down"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/marketplace")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Marketplace is currently unavailable") {
		t.Error("marketplace page should show error message")
	}
}

func TestPluginDetailError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		getDetailErr:   errors.New("not found"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/marketplace/config/test/plugin")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body = %s", resp.StatusCode, body)
	}
}

func TestInstallPluginError(t *testing.T) {
	ctrl := &errorMockController{
		mockController:   *newMockController(),
		installPluginErr: errors.New("install failed"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/plugins/install", "ref=test/plugin", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestListInstalledPluginsError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		listPluginsErr: errors.New("list broken"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/plugins")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body = %s", resp.StatusCode, body)
	}
}

func TestRatePluginError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		ratePluginErr:  errors.New("rating failed"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/marketplace/config/test/plugin/rate", "score=5", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

// ---------- CSRF rejection tests ----------

func TestCSRFRejectionAllPOSTs(t *testing.T) {
	base, _ := startTestServer(t)

	endpoints := []string{
		"/tree/values/db/host",
		"/validate/inline/db/host",
		"/validate",
		"/tree/save",
		"/components/database/toggle",
		"/export/preview",
		"/export",
		"/export/all",
		"/apply/run",
		"/marketplace/store/hashicorp/zhi-store-vault/rate",
		"/plugins/install",
		"/plugins/zhi-store-vault/uninstall",
		"/plugins/zhi-store-vault/update",
		"/plugins/update-all",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req, err := http.NewRequest("POST", base+ep, strings.NewReader(""))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("POST %s without CSRF: status = %d, want 403", ep, resp.StatusCode)
			}
		})
	}
}

// ---------- path traversal tests ----------

func TestStaticPathTraversal(t *testing.T) {
	base, _ := startTestServer(t)

	// Table of path traversal attempts.
	attacks := []string{
		"/static/../embed.go",
		"/static/%2e%2e/embed.go",
		"/static/..%2f..%2f..%2fetc/passwd",
		"/static/css/../../../embed.go",
		"/static/css/%2e%2e/%2e%2e/embed.go",
	}

	for _, path := range attacks {
		t.Run(path, func(t *testing.T) {
			resp := get(t, base+path)
			body := bodyString(t, resp)
			if resp.StatusCode == http.StatusOK && strings.Contains(body, "package webui") {
				t.Errorf("path traversal succeeded with %s", path)
			}
		})
	}
}

// ---------- error page tests ----------

func TestErrorPageNoStackTrace(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/nonexistent")
	body := bodyString(t, resp)
	// Verify no Go-internal stack trace leaks.
	if strings.Contains(body, "goroutine") {
		t.Error("error page should not contain goroutine stack traces")
	}
	if strings.Contains(body, "runtime/") {
		t.Error("error page should not contain Go runtime paths")
	}
	if strings.Contains(body, ".go:") {
		t.Error("error page should not contain Go file references")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	engine, err := newTemplateEngine(false, "")
	if err != nil {
		t.Fatalf("newTemplateEngine: %v", err)
	}

	panicHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})

	handler := recoveryMiddleware(engine)(panicHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "test panic") {
		t.Error("recovery response should not contain panic message")
	}
	if !strings.Contains(body, "unexpected error") {
		t.Error("recovery response should contain generic error message")
	}
}

// ---------- save with blocking validation ----------

func TestSaveValueWithBlockingValidation(t *testing.T) {
	ctrl := &blockingValidationController{
		mockController: *newMockController(),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)

	resp := postWithCSRF(t, base+"/tree/values/db/host", "value=bad&value_type=string", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// Should return the form (not the display) with validation errors.
	if !strings.Contains(body, "Validation errors must be resolved") {
		t.Error("response should contain validation error message")
	}
	if !strings.Contains(body, "value is blocked") {
		t.Error("response should contain the blocking validation message")
	}
}

// blockingValidationController returns blocking validation results.
type blockingValidationController struct {
	mockController
}

func (m *blockingValidationController) Validate(_ context.Context) ([]config.ValidationResult, error) {
	return []config.ValidationResult{
		{
			Severity: config.Blocking,
			Message:  "value is blocked",
			Metadata: map[string]any{"path": "db/host"},
		},
	}, nil
}

// ---------- middleware unit tests ----------

func TestResponseTimeHeader(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	resp.Body.Close()
	rt := resp.Header.Get("X-Response-Time")
	if rt == "" {
		t.Error("X-Response-Time header should be set")
	}
}

func TestETagHeaderStaticFile(t *testing.T) {
	base, _ := startTestServer(t)

	// Static files should get ETag headers (HTML pages don't because of
	// per-request CSP nonces).
	resp, err := http.Get(base + "/static/css/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	bodyString(t, resp)
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("ETag header should be set for static CSS responses")
	}

	// Second request with If-None-Match should return 304.
	req, err := http.NewRequest("GET", base+"/static/css/tokens.css", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", resp2.StatusCode)
	}
}

// ---------- asset manifest tests ----------

func TestAssetManifestHasEntries(t *testing.T) {
	m := newAssetManifest()
	if len(m.hashed) == 0 {
		t.Fatal("asset manifest should have entries")
	}

	// Check that tokens.css is hashed.
	p := m.hashedPath("css/tokens.css")
	if p == "/static/css/tokens.css" {
		t.Error("tokens.css should have a hashed path")
	}
	if !strings.HasPrefix(p, "/static/css/tokens.") {
		t.Errorf("hashed path = %q, want prefix /static/css/tokens.", p)
	}
}

func TestIsHashedPath(t *testing.T) {
	tests := []struct {
		path   string
		want   string
		hashed bool
	}{
		{"css/tokens.a1b2c3d4.css", "css/tokens.css", true},
		{"js/app.12345678.js", "js/app.js", true},
		{"css/tokens.css", "css/tokens.css", false},
		{"css/tokens.toolong123.css", "css/tokens.toolong123.css", false},
		{"css/tokens.short.css", "css/tokens.short.css", false},
		{"no-extension", "no-extension", false},
	}
	for _, tt := range tests {
		got, hashed := isHashedPath(tt.path)
		if got != tt.want || hashed != tt.hashed {
			t.Errorf("isHashedPath(%q) = (%q, %v), want (%q, %v)",
				tt.path, got, hashed, tt.want, tt.hashed)
		}
	}
}

func TestHashedStaticImmutableCache(t *testing.T) {
	base, _ := startTestServer(t)
	// Get a hashed path from the manifest.
	m := newAssetManifest()
	hashedPath := m.hashedPath("css/tokens.css")
	resp := get(t, base+hashedPath)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "--color-bg") {
		t.Error("hashed static file should serve correct content")
	}
	cc := resp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, should contain 'immutable'", cc)
	}
}

// ---------- config tests ----------

func TestDevModeConfig(t *testing.T) {
	t.Setenv("ZHI_WEBUI_DEV", "true")
	t.Setenv("ZHI_WEBUI_TEMPLATE_DIR", "/tmp/templates")
	cfg := DefaultConfig()
	if !cfg.DevMode {
		t.Error("DevMode should be true")
	}
	if cfg.TemplateDir != "/tmp/templates" {
		t.Errorf("TemplateDir = %q, want /tmp/templates", cfg.TemplateDir)
	}
}

// ---------- benchmarks ----------

func BenchmarkTemplateRenderPage(b *testing.B) {
	engine, err := newTemplateEngine(false, "")
	if err != nil {
		b.Fatalf("newTemplateEngine: %v", err)
	}

	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})
	_ = tree.Set("app/name", &config.Value{Val: "myapp"})
	nodes := buildNestedTree(tree, tree, nil)

	data := pageData{
		WorkspaceName: "bench",
		ActiveNav:     "tree",
		TreeNodes:     nodes,
	}

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		_ = engine.renderPage(w, "tree", data)
	}
}

func BenchmarkTemplateRenderFragment(b *testing.B) {
	engine, err := newTemplateEngine(false, "")
	if err != nil {
		b.Fatalf("newTemplateEngine: %v", err)
	}

	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})
	nodes := buildNestedTree(tree, tree, nil)

	data := pageData{
		TreeNodes: nodes,
	}

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		_ = engine.renderFragment(w, "tree_content", data)
	}
}

func BenchmarkBuildNestedTree(b *testing.B) {
	tree := config.NewTree()
	for i := range 100 {
		path := "section" + strings.Repeat("/sub", i%5+1) + "/key"
		_ = tree.Set(path+string(rune('a'+i%26)), &config.Value{Val: "value"})
	}

	b.ResetTimer()
	for b.Loop() {
		buildNestedTree(tree, tree, nil)
	}
}

// ---------- server lifecycle test ----------

func TestServerLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctrl := newMockController()

	cfg := Config{Addr: "127.0.0.1:0", AutoOpen: false}
	w := New(cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Run(ctx, ctrl)
	}()

	// Wait for server to be ready.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer waitCancel()
	addr, err := w.Addr(waitCtx)
	if err != nil {
		cancel()
		t.Fatalf("server did not start: %v", err)
	}

	base := "http://" + addr

	// Make a request to verify the server is working.
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "db") {
		t.Error("tree page should contain 'db'")
	}

	// Cancel context to trigger shutdown.
	cancel()

	// Wait for server to stop.
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("server returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within timeout")
	}

	// Verify the server is no longer accepting connections.
	client := &http.Client{Timeout: 1 * time.Second}
	_, err = client.Get(base + "/tree")
	if err == nil {
		t.Error("server should not be accepting connections after shutdown")
	}
}

// ---------- template function unit tests ----------

func TestCsrfFieldFunc(t *testing.T) {
	got := csrfFieldFunc(pageData{CSRFToken: "testtoken123"})
	if !strings.Contains(string(got), "testtoken123") {
		t.Error("csrfFieldFunc should contain the token")
	}
	if !strings.Contains(string(got), `type="hidden"`) {
		t.Error("csrfFieldFunc should produce a hidden input")
	}
	// Non-pageData should return an empty-value hidden field.
	empty := csrfFieldFunc("not a pageData")
	if !strings.Contains(string(empty), `value=""`) {
		t.Error("csrfFieldFunc with non-pageData should have empty value")
	}
}

func TestJsonFunc(t *testing.T) {
	got := jsonFunc(map[string]string{"key": "value"})
	if !strings.Contains(got, "key") || !strings.Contains(got, "value") {
		t.Errorf("jsonFunc = %q, want JSON with key/value", got)
	}
}

func TestPathSegmentsFunc(t *testing.T) {
	got := pathSegmentsFunc("db/host/port")
	if len(got) != 3 || got[0] != "db" || got[1] != "host" || got[2] != "port" {
		t.Errorf("pathSegmentsFunc = %v, want [db host port]", got)
	}
}

func TestFullValidationHTMXFragment(t *testing.T) {
	base, _ := startTestServer(t)
	token, cookies := getCSRFToken(t, base)

	req, err := http.NewRequest("POST", base+"/validate", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("X-CSRF-Token", token)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX fragment should not contain full HTML document")
	}
}

func TestExportPreviewError(t *testing.T) {
	ctrl := &errorMockController{
		mockController: *newMockController(),
		exportErr:      errors.New("preview failed"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/export/preview", "format=json", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "preview failed") {
		t.Error("preview should contain error message")
	}
}

func TestExportAllError(t *testing.T) {
	ctrl := &errorMockController{
		mockController:     *newMockController(),
		exportTemplatesErr: errors.New("templates unavailable"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/export/all", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestCheckUpdatesError(t *testing.T) {
	ctrl := &errorMockController{
		mockController:  *newMockController(),
		checkUpdatesErr: errors.New("updates broken"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/plugins/update-all", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

// ---------- auth mock controller ----------

// authMockController wraps mockController with configurable auth behavior.
type authMockController struct {
	mockController
	authStatus  ui.StoreSessionStatus
	authMethods []ui.StoreAuthMethod
	loginErr    error
}

func (m *authMockController) StoreAuthStatus(_ context.Context) (*ui.StoreSession, error) {
	return &ui.StoreSession{Status: m.authStatus}, nil
}

func (m *authMockController) StoreAuthMethods(_ context.Context) ([]ui.StoreAuthMethod, error) {
	return m.authMethods, nil
}

func (m *authMockController) StoreLogin(_ context.Context, _ string, _ map[string]string) (*ui.StoreSession, error) {
	if m.loginErr != nil {
		return nil, m.loginErr
	}
	return &ui.StoreSession{Status: ui.StoreSessionAuthenticated}, nil
}

func (m *authMockController) StoreLoginInteractive(_ context.Context, _ string, _ map[string]string) (*ui.StoreInteractiveChallenge, error) {
	return nil, errors.New("interactive login not supported")
}

func (m *authMockController) StoreLoginInteractiveCallback(_ context.Context, _ string, _ map[string]string) (*ui.StoreSession, error) {
	return nil, errors.New("interactive login not supported")
}

func (m *authMockController) StoreLogout(_ context.Context) error {
	return nil
}

// ---------- login page tests ----------

func TestLoginPageRender(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionUnauthenticated,
		authMethods: []ui.StoreAuthMethod{
			{
				Type:        "userpass",
				Description: "Username & Password",
				Fields: []ui.StoreAuthField{
					{Name: "username", Description: "Username", Required: true},
					{Name: "password", Description: "Password", Required: true, Secret: true},
				},
			},
		},
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/login")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "Username") {
		t.Error("login page should contain 'Username' field")
	}
	if !strings.Contains(body, `type="password"`) {
		t.Error("login page should have password input type")
	}
	if !strings.Contains(body, "Login") {
		t.Error("login page should contain Login button")
	}
	cc := resp.Header.Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("Cache-Control = %q, want 'no-store'", cc)
	}
}

func TestLoginPageRedirectsWhenAuthenticated(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionAuthenticated,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(base + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	if loc != "/tree" {
		t.Errorf("Location = %q, want /tree", loc)
	}
}

func TestLoginPageMultipleMethods(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionUnauthenticated,
		authMethods: []ui.StoreAuthMethod{
			{Type: "userpass", Description: "Username & Password", Fields: []ui.StoreAuthField{
				{Name: "username", Description: "Username", Required: true},
				{Name: "password", Description: "Password", Required: true, Secret: true},
			}},
			{Type: "token", Description: "API Token", Fields: []ui.StoreAuthField{
				{Name: "token", Description: "Token", Required: true, Secret: true},
			}},
		},
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/login")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "userpass") {
		t.Error("login page should contain 'userpass' method option")
	}
	if !strings.Contains(body, "token") {
		t.Error("login page should contain 'token' method option")
	}
}

func TestLoginSuccess(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionUnauthenticated,
		authMethods: []ui.StoreAuthMethod{
			{Type: "userpass", Description: "Username & Password", Fields: []ui.StoreAuthField{
				{Name: "username", Description: "Username", Required: true},
				{Name: "password", Description: "Password", Required: true, Secret: true},
			}},
		},
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/login", "method=userpass&cred_username=admin&cred_password=secret", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	if loc != "/tree" {
		t.Errorf("Location = %q, want /tree", loc)
	}
}

func TestLoginFailure(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionUnauthenticated,
		authMethods: []ui.StoreAuthMethod{
			{Type: "userpass", Description: "Username & Password", Fields: []ui.StoreAuthField{
				{Name: "username", Description: "Username", Required: true},
				{Name: "password", Description: "Password", Required: true, Secret: true},
			}},
		},
		loginErr: errors.New("invalid credentials"),
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/login", "method=userpass&cred_username=admin&cred_password=wrong", token, cookies)
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if !strings.Contains(body, "invalid credentials") {
		t.Error("login page should show error message")
	}
}

func TestLoginCSRFRequired(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionUnauthenticated,
		authMethods:    []ui.StoreAuthMethod{{Type: "token", Fields: []ui.StoreAuthField{}}},
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	// POST without CSRF token should be rejected.
	resp, err := http.Post(base+"/login", "application/x-www-form-urlencoded", strings.NewReader("method=token"))
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestLogout(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionAuthenticated,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	token, cookies := getCSRFToken(t, base)
	resp := postWithCSRF(t, base+"/logout", "", token, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	if loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestAuthStatus(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionAuthenticated,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/auth/status")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, `"status":"authenticated"`) {
		t.Errorf("body = %q, want status authenticated", body)
	}
}

// ---------- requireAuth middleware tests ----------

func TestRequireAuthPassthroughWhenNoAuth(t *testing.T) {
	// Default mock returns StoreSessionNone, so all routes should work.
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "db") {
		t.Error("tree page should render normally when auth is none")
	}
}

func TestRequireAuthRedirectWhenUnauthenticated(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionUnauthenticated,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(base + "/tree")
	if err != nil {
		t.Fatalf("GET /tree: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	if loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestRequireAuthRedirectWhenExpired(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionExpired,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(base + "/components")
	if err != nil {
		t.Fatalf("GET /components: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	loc := resp.Header.Get("Location")
	if loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestRequireAuthHTMXRedirect(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionUnauthenticated,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	req, err := http.NewRequest("GET", base+"/tree", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("HX-Request", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	hxRedirect := resp.Header.Get("HX-Redirect")
	if hxRedirect != "/login" {
		t.Errorf("HX-Redirect = %q, want /login", hxRedirect)
	}
}

func TestRequireAuthAllowsAuthenticated(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionAuthenticated,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "db") {
		t.Error("tree page should render when authenticated")
	}
}

func TestLogoutButtonVisibleWhenAuthenticated(t *testing.T) {
	ctrl := &authMockController{
		mockController: *newMockController(),
		authStatus:     ui.StoreSessionAuthenticated,
	}
	base, _ := startTestServerWithCtrl(t, ctrl)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if !strings.Contains(body, "Logout") {
		t.Error("sidebar should contain Logout button when authenticated")
	}
}

func TestLogoutButtonHiddenWhenNoAuth(t *testing.T) {
	base, _ := startTestServer(t)
	resp := get(t, base+"/tree")
	body := bodyString(t, resp)
	if strings.Contains(body, "Logout") {
		t.Error("sidebar should not contain Logout button when auth is none")
	}
}

// ---------- compile-time interface checks ----------

var _ ui.Plugin = (*WebUI)(nil)
var _ ui.Controller = (*mockController)(nil)
var _ ui.Controller = (*errorMockController)(nil)
var _ ui.Controller = (*blockingValidationController)(nil)
var _ ui.Controller = (*authMockController)(nil)
