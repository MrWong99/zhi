package ui_test

import (
	"context"
	"sort"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// --- mock implementations ---

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

func (m *mockConfig) List(_ context.Context) ([]string, error) {
	return m.tree.List(), nil
}

func (m *mockConfig) Get(_ context.Context, path string) (config.Value, bool, error) {
	v, ok := m.tree.Get(path)
	return v, ok, nil
}

func (m *mockConfig) Set(_ context.Context, path string, v config.Value) error {
	return m.tree.Set(path, &v)
}

func (m *mockConfig) Validate(_ context.Context, _ string, _ config.TreeReader) ([]config.ValidationResult, error) {
	return nil, nil
}

type mockTransform struct{}

func (m *mockTransform) BeforeDisplay(_ context.Context, _ *config.Tree) error { return nil }
func (m *mockTransform) AfterSave(_ context.Context, _ *config.Tree) error    { return nil }
func (m *mockTransform) ValidatePolicy(_ context.Context) (transform.ValidatePolicy, error) {
	return 0, nil
}

type mockStore struct {
	trees map[string]*config.Tree
}

func newMockStore() *mockStore {
	return &mockStore{trees: make(map[string]*config.Tree)}
}

func (m *mockStore) Save(_ context.Context, id string, r config.TreeReader) error {
	t := config.NewTree()
	for _, p := range r.List() {
		v, ok := r.Get(p)
		if ok {
			_ = t.Set(p, &v)
		}
	}
	m.trees[id] = t
	return nil
}

func (m *mockStore) Load(_ context.Context, id string) (*config.Tree, bool, error) {
	t, ok := m.trees[id]
	return t, ok, nil
}

func (m *mockStore) Delete(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}

func (m *mockStore) ListTrees(_ context.Context) ([]string, error) {
	var ids []string
	for id := range m.trees {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockStore) SupportsVersioning(_ context.Context) (bool, error) { return false, nil }
func (m *mockStore) ListVersions(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockStore) LoadVersion(_ context.Context, _, _ string) (*config.Tree, bool, error) {
	return nil, false, nil
}
func (m *mockStore) DeleteVersion(_ context.Context, _, _ string) error { return nil }
func (m *mockStore) EncryptionStatus(_ context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionNone, nil
}
func (m *mockStore) InitEncryption(_ context.Context, _ []byte) error      { return nil }
func (m *mockStore) RotateEncryption(_ context.Context, _, _ []byte) error { return nil }

// --- test helpers ---

func setupTestController(t *testing.T) *ui.UIController {
	t.Helper()

	reg := core.NewRegistry()
	mc := newMockConfig()
	ms := newMockStore()

	if err := reg.RegisterConfig("mock", func(_ map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterTransform("mock", func(_ map[string]any) (transform.Plugin, error) {
		return &mockTransform{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterStore("mock", func(_ map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
		Transform: []core.ProviderRef{
			{Provider: "mock"},
		},
		Store: core.ProviderRef{Provider: "mock"},
		Components: []core.ComponentDef{
			{Name: "database", Paths: []string{"database/"}, Mandatory: true},
			{Name: "app", Paths: []string{"app/"}, Dependencies: []string{"database"}},
		},
		Dir: t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatal(err)
	}

	return ui.NewUIController(eng)
}

// --- tests ---

func TestUIController_LoadTree(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	paths := tree.List()
	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(paths))
	}

	v, ok := tree.Get("database/host")
	if !ok {
		t.Fatal("expected database/host in tree")
	}
	if v.Val != "localhost" {
		t.Errorf("expected localhost, got %v", v.Val)
	}
}

func TestUIController_GetValue(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	// Must load tree first.
	if _, err := ctrl.LoadTree(ctx); err != nil {
		t.Fatal(err)
	}

	v, ok := ctrl.GetValue("database/host")
	if !ok {
		t.Fatal("expected database/host")
	}
	if v.Val != "localhost" {
		t.Errorf("expected localhost, got %v", v.Val)
	}

	_, ok = ctrl.GetValue("nonexistent/path")
	if ok {
		t.Error("expected false for nonexistent path")
	}
}

func TestUIController_SetValue(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	if _, err := ctrl.LoadTree(ctx); err != nil {
		t.Fatal(err)
	}

	err := ctrl.SetValue(ctx, "database/host", config.Value{Val: "remotehost"})
	if err != nil {
		t.Fatalf("SetValue: %v", err)
	}

	v, ok := ctrl.GetValue("database/host")
	if !ok {
		t.Fatal("expected database/host after set")
	}
	if v.Val != "remotehost" {
		t.Errorf("expected remotehost, got %v", v.Val)
	}
}

func TestUIController_ListComponents(t *testing.T) {
	ctrl := setupTestController(t)

	components := ctrl.ListComponents()
	if len(components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(components))
	}

	names := make([]string, len(components))
	for i, c := range components {
		names[i] = c.Name
	}
	sort.Strings(names)

	if names[0] != "app" || names[1] != "database" {
		t.Errorf("expected [app database], got %v", names)
	}
}

func TestUIController_EnableDisableComponent(t *testing.T) {
	ctrl := setupTestController(t)

	// app starts disabled (not mandatory), database is mandatory.
	if ctrl.IsComponentEnabled("app") {
		t.Error("app should start disabled")
	}
	if !ctrl.IsComponentEnabled("database") {
		t.Error("database should be enabled (mandatory)")
	}

	// Enable app - should also auto-enable database (its dependency).
	enabled, err := ctrl.EnableComponent("app")
	if err != nil {
		t.Fatalf("EnableComponent: %v", err)
	}
	if !ctrl.IsComponentEnabled("app") {
		t.Error("app should be enabled after Enable")
	}
	// database was already enabled, so enabled list should just be ["app"]
	if len(enabled) != 1 || enabled[0] != "app" {
		t.Errorf("expected [app], got %v", enabled)
	}

	// Try to disable mandatory database.
	err = ctrl.DisableComponent("database")
	if err == nil {
		t.Error("expected error disabling mandatory component")
	}
}

func TestUIController_Validate(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	if _, err := ctrl.LoadTree(ctx); err != nil {
		t.Fatal(err)
	}

	results, err := ctrl.Validate(ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// With our mock, validation shouldn't return errors unless app has
	// dependency issues. app starts disabled but database is enabled, so
	// there shouldn't be dependency errors.
	_ = results // May or may not have results depending on component state.
}

func TestUIController_PathBelongsToComponent(t *testing.T) {
	ctrl := setupTestController(t)

	comp, ok := ctrl.PathBelongsToComponent("database/host")
	if !ok {
		t.Fatal("expected database/host to belong to a component")
	}
	if comp != "database" {
		t.Errorf("expected database, got %s", comp)
	}

	_, ok = ctrl.PathBelongsToComponent("unknown/path")
	if ok {
		t.Error("expected unknown/path to not belong to any component")
	}
}

func TestUIController_SaveTree(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	if _, err := ctrl.LoadTree(ctx); err != nil {
		t.Fatal(err)
	}

	err := ctrl.SaveTree(ctx, "test")
	if err != nil {
		t.Fatalf("SaveTree: %v", err)
	}
}

func TestUIController_WorkspaceName(t *testing.T) {
	ctrl := setupTestController(t)

	name := ctrl.WorkspaceName()
	if name == "" || name == "unknown" {
		t.Errorf("expected non-empty workspace name, got %q", name)
	}
}
