package tui_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/internal/ui/tui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// --- mock implementations for TUI tests ---

type mockConfig struct {
	tree *config.Tree
}

func newMockConfig() *mockConfig {
	t := config.NewTree()
	_ = t.Set("database/host", &config.Value{Val: "localhost", Metadata: map[string]any{"description": "Database hostname"}})
	_ = t.Set("database/port", &config.Value{Val: 5432})
	_ = t.Set("app/name", &config.Value{Val: "myapp"})
	_ = t.Set("redis/url", &config.Value{Val: "redis://localhost:6379"})
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

func (m *mockConfig) Validate(_ context.Context, path string, _ config.TreeReader) ([]config.ValidationResult, error) {
	if path == "database/host" {
		return []config.ValidationResult{
			{Severity: config.Warning, Message: "using localhost is not recommended for production"},
		}, nil
	}
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

func (m *mockStore) SupportsVersioning(_ context.Context) (bool, error)           { return false, nil }
func (m *mockStore) ListVersions(_ context.Context, _ string) ([]string, error)   { return nil, nil }
func (m *mockStore) LoadVersion(_ context.Context, _, _ string) (*config.Tree, bool, error) {
	return nil, false, nil
}
func (m *mockStore) DeleteVersion(_ context.Context, _, _ string) error { return nil }
func (m *mockStore) EncryptionStatus(_ context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionNone, nil
}
func (m *mockStore) InitEncryption(_ context.Context, _ []byte) error      { return nil }
func (m *mockStore) RotateEncryption(_ context.Context, _, _ []byte) error { return nil }

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
	if err := reg.RegisterStore("mock", func(_ map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
		Store:   core.ProviderRef{Provider: "mock"},
		Components: []core.ComponentDef{
			{Name: "database", Paths: []string{"database/"}, Mandatory: true, Description: "PostgreSQL database"},
			{Name: "app", Paths: []string{"app/"}, Dependencies: []string{"database"}, Description: "Application config"},
			{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}, Description: "Redis cache"},
		},
		Dir: t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatal(err)
	}

	return ui.NewUIController(eng)
}

func TestNewApp(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	// Should render without panic.
	view := app.View()
	if view == "" {
		t.Error("expected non-empty initial view")
	}
}

func TestApp_ViewSwitching(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatal(err)
	}

	// Set a window size first.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Should start in tree view. Press 'c' to go to component view.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	app = model.(*tui.App)

	view := app.View()
	if view == "" {
		t.Error("expected non-empty view after switching to components")
	}

	// Press Esc to go back to tree view.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	app = model.(*tui.App)

	view = app.View()
	if view == "" {
		t.Error("expected non-empty view after returning to tree")
	}
}

func TestApp_QuitOnQ(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatal(err)
	}

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command when pressing q")
	}
}

func TestApp_QuitOnCtrlC(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatal(err)
	}

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit command on ctrl+c")
	}
}

func TestApp_ValidationView(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatal(err)
	}

	// Set window size.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Press 'v' to validate.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	app = model.(*tui.App)

	view := app.View()
	if view == "" {
		t.Error("expected non-empty validation view")
	}
}

func TestApp_SaveTree(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatal(err)
	}

	// Set window size.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Press 's' to save.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = model.(*tui.App)

	view := app.View()
	if view == "" {
		t.Error("expected non-empty view after save")
	}
}

func TestApp_ReloadTree(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatal(err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Press 'r' to reload - should return a command.
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("expected reload command")
	}
}

func TestTUIDriver_Registration(t *testing.T) {
	// Importing tui package triggers init() registration.
	driver, err := ui.Get("tui")
	if err != nil {
		t.Fatalf("Get tui driver: %v", err)
	}
	if driver == nil {
		t.Fatal("expected non-nil tui driver")
	}
}
