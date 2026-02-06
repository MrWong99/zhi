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

// ---------- Phase 2: editor tests ----------

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

// ---------- Phase 2: validation tests ----------

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

// ---------- Phase 2: save tests ----------

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

// ---------- Phase 2: component tests ----------

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

// ---------- Phase 2: shortcuts page test ----------

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

// ---------- Phase 2: tree node has edit button ----------

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

// ---------- Phase 2: sidebar navigation ----------

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

// ---------- Phase 2: topbar save button ----------

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

// ---------- Phase 2: unit tests for editor helpers ----------

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

// ---------- Phase 2: tree node PathID ----------

func TestTreeNodeHasPathID(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})

	nodes := buildNestedTree(tree, nil)
	leaf := nodes[0].Children[0]
	if leaf.PathID != "db--host" {
		t.Errorf("PathID = %q, want 'db--host'", leaf.PathID)
	}
}

// ---------- compile-time interface checks ----------

var _ ui.Plugin = (*WebUI)(nil)
var _ ui.Controller = (*mockController)(nil)
