package ui_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// --- Mock plugins ---

type mockConfig struct {
	tree *config.Tree
}

func newMockConfig() *mockConfig {
	t := config.NewTree()
	_ = t.Set("database/host", &config.Value{Val: "localhost"})
	_ = t.Set("database/port", &config.Value{Val: 5432})
	_ = t.Set("app/name", &config.Value{Val: "myapp"})
	return &mockConfig{tree: t}
}

func (m *mockConfig) List(context.Context) ([]string, error) {
	return m.tree.List(), nil
}

func (m *mockConfig) Get(_ context.Context, path string) (config.Value, bool, error) {
	v, ok := m.tree.Get(path)
	return v, ok, nil
}

func (m *mockConfig) Set(_ context.Context, path string, v config.Value) error {
	return m.tree.Set(path, &v)
}

func (m *mockConfig) Validate(_ context.Context, path string, tree config.TreeReader) ([]config.ValidationResult, error) {
	if path == "database/host" {
		v, ok := tree.Get(path)
		if ok && v.Val == "localhost" {
			return []config.ValidationResult{
				{Severity: config.Warning, Message: "using localhost"},
			}, nil
		}
	}
	return nil, nil
}

type mockTransform struct{}

func (m *mockTransform) BeforeDisplay(_ context.Context, _ *config.Tree) error { return nil }
func (m *mockTransform) AfterSave(_ context.Context, _ *config.Tree) error     { return nil }
func (m *mockTransform) ValidatePolicy(context.Context) (transform.ValidatePolicy, error) {
	return transform.ValidateBeforeTransform, nil
}

type mockStore struct {
	trees map[string]*config.Tree
}

func newMockStore() *mockStore {
	return &mockStore{trees: make(map[string]*config.Tree)}
}

func (m *mockStore) Save(_ context.Context, id string, tree config.TreeReader) error {
	t := config.NewTree()
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if ok {
			_ = t.Set(path, &v)
		}
	}
	m.trees[id] = t
	return nil
}

func (m *mockStore) Load(_ context.Context, id string) (*config.Tree, bool, error) {
	t, ok := m.trees[id]
	if !ok {
		return nil, false, nil
	}
	return t, true, nil
}

func (m *mockStore) Delete(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}

func (m *mockStore) ListTrees(context.Context) ([]string, error) {
	var ids []string
	for id := range m.trees {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (m *mockStore) SupportsVersioning(context.Context) (bool, error)       { return false, nil }
func (m *mockStore) ListVersions(context.Context, string) ([]string, error) { return nil, nil }
func (m *mockStore) LoadVersion(context.Context, string, string) (*config.Tree, bool, error) {
	return nil, false, nil
}
func (m *mockStore) DeleteVersion(context.Context, string, string) error { return nil }
func (m *mockStore) EncryptionStatus(context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionNone, nil
}
func (m *mockStore) InitEncryption(context.Context, []byte) error           { return nil }
func (m *mockStore) RotateEncryption(context.Context, []byte, []byte) error { return nil }

// --- Test helpers ---

func setupTestEngine(t *testing.T, components []core.ComponentDef) *core.Engine {
	t.Helper()

	mc := newMockConfig()
	mt := &mockTransform{}
	ms := newMockStore()

	reg := core.NewRegistry()
	_ = reg.RegisterConfig("mock", func(map[string]any) (config.Plugin, error) {
		return mc, nil
	})
	_ = reg.RegisterTransform("mock", func(map[string]any) (transform.Plugin, error) {
		return mt, nil
	})
	_ = reg.RegisterStore("mock", func(map[string]any) (store.Plugin, error) {
		return ms, nil
	})

	ws := &core.WorkspaceConfig{
		Version:    "1",
		Config:     core.ProviderRef{Provider: "mock"},
		Transform:  []core.ProviderRef{{Provider: "mock"}},
		Store:      core.ProviderRef{Provider: "mock"},
		Components: components,
		Dir:        t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng
}

// --- UIController tests ---

func TestUIController_LoadTree(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)

	tree, err := ctrl.LoadTree(context.Background())
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	paths := tree.List()
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d: %v", len(paths), paths)
	}

	v, ok := tree.Get("database/host")
	if !ok {
		t.Fatal("expected database/host to exist")
	}
	if v.Val != "localhost" {
		t.Errorf("expected database/host = localhost, got %v", v.Val)
	}
}

func TestUIController_FilteredTree(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "db", Paths: []string{"database/"}, Mandatory: false},
		{Name: "app", Paths: []string{"app/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)
	ctrl := ui.NewUIController(eng)

	// Only app (mandatory) is enabled, db is disabled.
	tree, err := ctrl.FilteredTree(context.Background())
	if err != nil {
		t.Fatalf("FilteredTree: %v", err)
	}

	paths := tree.List()
	// Should only have app/name since db/ is disabled.
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "app/name" {
		t.Errorf("expected app/name, got %s", paths[0])
	}
}

func TestUIController_GetValue(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)
	ctx := context.Background()

	v, ok, err := ctrl.GetValue(ctx, "database/host")
	if err != nil {
		t.Fatalf("GetValue: %v", err)
	}
	if !ok {
		t.Fatal("expected database/host to exist")
	}
	if v.Val != "localhost" {
		t.Errorf("expected localhost, got %v", v.Val)
	}

	_, ok, err = ctrl.GetValue(ctx, "nonexistent/path")
	if err != nil {
		t.Fatalf("GetValue nonexistent: %v", err)
	}
	if ok {
		t.Error("expected nonexistent path to not be found")
	}
}

func TestUIController_SetValue(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)
	ctx := context.Background()

	err := ctrl.SetValue(ctx, "database/host", config.Value{Val: "example.com"})
	if err != nil {
		t.Fatalf("SetValue: %v", err)
	}

	// Verify the value was updated.
	v, ok, err := ctrl.GetValue(ctx, "database/host")
	if err != nil {
		t.Fatalf("GetValue after set: %v", err)
	}
	if !ok {
		t.Fatal("expected database/host to exist after set")
	}
	if v.Val != "example.com" {
		t.Errorf("expected example.com, got %v", v.Val)
	}
}

func TestUIController_Validate(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)
	ctx := context.Background()

	results, err := ctrl.Validate(ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// The mock validator returns a warning for database/host=localhost.
	if len(results) == 0 {
		t.Fatal("expected at least one validation result")
	}

	found := false
	for _, r := range results {
		if r.Message == "using localhost" && r.Severity == config.Warning {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about localhost")
	}
}

func TestUIController_ValidateIncludesComponentDeps(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "base", Paths: []string{"base/"}},
		{Name: "dependent", Paths: []string{"dep/"}, Dependencies: []string{"base"}},
	}
	eng := setupTestEngine(t, components)
	ctrl := ui.NewUIController(eng)

	// Force an inconsistent state: dependent enabled but its dependency
	// base disabled. LoadState allows this because neither is mandatory.
	eng.Components().LoadState(map[string]bool{
		"base":      false,
		"dependent": true,
	})

	results, err := ctrl.Validate(context.Background())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	foundDepError := false
	for _, r := range results {
		if r.Severity == config.Blocking {
			foundDepError = true
		}
	}
	if !foundDepError {
		t.Error("expected blocking validation result for dependency violation")
	}
}

func TestUIController_SaveTree(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)
	ctx := context.Background()

	err := ctrl.SaveTree(ctx, "test-tree")
	if err != nil {
		t.Fatalf("SaveTree: %v", err)
	}

	// Verify via engine that the tree was saved.
	loaded, found, err := eng.LoadStoredTree(ctx, "test-tree")
	if err != nil {
		t.Fatalf("LoadStoredTree: %v", err)
	}
	if !found {
		t.Fatal("expected saved tree to be found")
	}
	if len(loaded.List()) != 3 {
		t.Errorf("expected 3 paths in saved tree, got %d", len(loaded.List()))
	}
}

func TestUIController_ListComponents(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "db", Paths: []string{"database/"}, Mandatory: false},
		{Name: "app", Paths: []string{"app/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)
	ctrl := ui.NewUIController(eng)

	states := ctrl.ListComponents()
	if len(states) != 2 {
		t.Fatalf("expected 2 components, got %d", len(states))
	}

	stateMap := make(map[string]core.ComponentState)
	for _, s := range states {
		stateMap[s.Name] = s
	}

	if !stateMap["app"].Enabled {
		t.Error("expected app to be enabled (mandatory)")
	}
	if stateMap["db"].Enabled {
		t.Error("expected db to be disabled initially")
	}
}

func TestUIController_EnableDisableComponent(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "db", Paths: []string{"database/"}, Mandatory: false},
		{Name: "app", Paths: []string{"app/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)
	ctrl := ui.NewUIController(eng)

	// Enable db.
	if err := ctrl.EnableComponent("db"); err != nil {
		t.Fatalf("EnableComponent: %v", err)
	}
	if !ctrl.IsComponentEnabled("db") {
		t.Error("expected db to be enabled after EnableComponent")
	}

	// Disable db.
	if err := ctrl.DisableComponent("db"); err != nil {
		t.Fatalf("DisableComponent: %v", err)
	}
	if ctrl.IsComponentEnabled("db") {
		t.Error("expected db to be disabled after DisableComponent")
	}

	// Cannot disable mandatory component.
	err := ctrl.DisableComponent("app")
	if err == nil {
		t.Error("expected error when disabling mandatory component")
	}
}

func TestUIController_ExportPreview(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)
	ctx := context.Background()

	cfg := core.ExportRunConfig{
		Format: "json",
	}
	content, err := ctrl.ExportPreview(ctx, cfg)
	if err != nil {
		t.Fatalf("ExportPreview: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty export preview")
	}
}

func TestUIController_Export(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)
	ctx := context.Background()

	cfg := core.ExportRunConfig{
		Format: "json",
		DryRun: true,
	}
	result, err := ctrl.Export(ctx, cfg)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Content == "" {
		t.Error("expected non-empty content")
	}
	if result.Name != "json" {
		t.Errorf("expected name 'json', got %q", result.Name)
	}
}

func TestUIController_ApplyConfig(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)

	ac := ctrl.ApplyConfig()
	if ac == nil {
		t.Fatal("expected non-nil ApplyConfig")
	}
}

func TestUIController_WorkspaceName(t *testing.T) {
	eng := setupTestEngine(t, nil)
	eng.SetTestWorkspaceDir("/some/workspace/dir")
	ctrl := ui.NewUIController(eng)

	name := ctrl.WorkspaceName()
	if name != "dir" {
		t.Errorf("expected workspace name 'dir', got %q", name)
	}
}

func TestUIController_WorkspaceNameEmpty(t *testing.T) {
	eng := setupTestEngine(t, nil)
	eng.SetTestWorkspaceDir("")
	ctrl := ui.NewUIController(eng)

	name := ctrl.WorkspaceName()
	if name != "" {
		t.Errorf("expected empty workspace name, got %q", name)
	}
}

func TestUIController_ProviderInfo(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)

	info := ctrl.ProviderInfo()
	if info == nil {
		t.Fatal("expected non-nil provider info")
	}
	if info["config"] != "mock" {
		t.Errorf("expected config provider 'mock', got %q", info["config"])
	}
	if info["store"] != "mock" {
		t.Errorf("expected store provider 'mock', got %q", info["store"])
	}
	if info["transform[0]"] != "mock" {
		t.Errorf("expected transform[0] provider 'mock', got %q", info["transform[0]"])
	}
}

// --- UIDriver interface test ---

// testDriver is a minimal UIDriver for testing that records calls.
type testDriver struct {
	runCalled  bool
	controller *ui.UIController
	err        error
}

func (d *testDriver) Run(_ context.Context, controller *ui.UIController) error {
	d.runCalled = true
	d.controller = controller
	return d.err
}

func TestUIDriver_Interface(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)

	driver := &testDriver{}
	err := driver.Run(context.Background(), ctrl)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !driver.runCalled {
		t.Error("expected Run to be called")
	}
	if driver.controller != ctrl {
		t.Error("expected controller to be passed through")
	}
}

func TestUIDriver_RunError(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctrl := ui.NewUIController(eng)

	driver := &testDriver{err: fmt.Errorf("test error")}
	err := driver.Run(context.Background(), ctrl)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %q", err.Error())
	}
}
