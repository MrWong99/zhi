package tui_test

import (
	"context"
	"fmt"
	"maps"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/internal/ui/tui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
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

// newMockConfigWithLabels creates a mock config with various UI labels set for testing.
func newMockConfigWithLabels() *mockConfig {
	t := config.NewTree()
	_ = t.Set("database/type", &config.Value{
		Val: "postgres",
		Metadata: map[string]any{
			"ui.enum":        []string{"postgres", "mysql", "sqlite"},
			"ui.order":       1,
			"ui.section":     "Connection",
			"ui.displayName": "Database Engine",
		},
	})
	_ = t.Set("database/host", &config.Value{
		Val: "localhost",
		Metadata: map[string]any{
			"core.description": "Database hostname",
			"ui.order":         2,
			"ui.section":       "Connection",
			"ui.placeholder":   "Enter hostname...",
			"config.required":  true,
		},
	})
	_ = t.Set("database/port", &config.Value{
		Val: 5432,
		Metadata: map[string]any{
			"ui.order":   3,
			"ui.section": "Connection",
			"ui.pattern": `^\d+$`,
		},
	})
	_ = t.Set("database/password", &config.Value{
		Val: "s3cret",
		Metadata: map[string]any{
			"ui.password": true,
			"ui.confirm":  true,
			"ui.order":    4,
			"ui.section":  "Connection",
		},
	})
	_ = t.Set("database/internal-key", &config.Value{
		Val:      "should-not-appear",
		Metadata: map[string]any{"ui.hidden": true},
	})
	_ = t.Set("database/pg-options", &config.Value{
		Val: "sslmode=require",
		Metadata: map[string]any{
			"ui.showIf":  "database/type=postgres",
			"ui.order":   5,
			"ui.section": "Connection",
		},
	})
	_ = t.Set("database/mysql-charset", &config.Value{
		Val: "utf8mb4",
		Metadata: map[string]any{
			"ui.showIf":  "database/type=mysql",
			"ui.order":   5,
			"ui.section": "Connection",
		},
	})
	_ = t.Set("app/name", &config.Value{
		Val: "myapp",
		Metadata: map[string]any{
			"ui.readonly":    true,
			"ui.group":       "Application",
			"ui.displayName": "Application Name",
		},
	})
	_ = t.Set("app/version", &config.Value{
		Val: "1.0.0",
		Metadata: map[string]any{
			"core.deprecated": "Use app/release-version instead",
			"ui.group":        "Application",
		},
	})
	_ = t.Set("redis/url", &config.Value{Val: "redis://localhost:6379"})
	return &mockConfig{tree: t}
}

// setupTestControllerWithLabels creates a test controller with label-rich config values.
func setupTestControllerWithLabels(t *testing.T) *ui.UIController {
	t.Helper()

	reg := core.NewRegistry()
	mc := newMockConfigWithLabels()
	ms := newMockStore()

	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterStore("mock", func(string, map[string]any) (store.Plugin, error) {
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

type mockStore struct {
	trees map[string]map[string]config.Value
}

func newMockStore() *mockStore {
	return &mockStore{trees: make(map[string]map[string]config.Value)}
}

func (m *mockStore) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning: store.VersioningNone,
		Encryption: store.EncryptionNone,
	}, nil
}

func (m *mockStore) AuthMethods(_ context.Context) ([]store.AuthMethod, error) { return nil, nil }
func (m *mockStore) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	return nil, nil
}

func (m *mockStore) LoginInteractive(context.Context, string, map[string]string) (*store.InteractiveChallenge, error) {
	return nil, fmt.Errorf("interactive login not supported")
}

func (m *mockStore) LoginInteractiveCallback(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, fmt.Errorf("interactive login not supported")
}

func (m *mockStore) ListTrees(_ context.Context) ([]string, error) {
	var ids []string
	for id := range m.trees {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockStore) DeleteTree(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}

func (m *mockStore) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
	vals, ok := m.trees[id]
	if !ok {
		return nil, nil
	}
	result := make(map[string]config.Value)
	for _, p := range paths {
		if v, exists := vals[p]; exists {
			result[p] = v
		}
	}
	return result, nil
}

func (m *mockStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	if m.trees[id] == nil {
		m.trees[id] = make(map[string]config.Value)
	}
	maps.Copy(m.trees[id], values)
	return nil
}

func (m *mockStore) DeleteValues(_ context.Context, id string, paths []string) error {
	vals := m.trees[id]
	if vals == nil {
		return nil
	}
	for _, p := range paths {
		delete(vals, p)
	}
	return nil
}

func (m *mockStore) ListTreeVersions(_ context.Context, _ string) ([]string, error) { return nil, nil }
func (m *mockStore) GetTreeVersion(_ context.Context, _, _ string, _ []string) (map[string]config.Value, error) {
	return nil, nil
}
func (m *mockStore) RollbackTree(_ context.Context, _, _ string) error      { return nil }
func (m *mockStore) DeleteTreeVersion(_ context.Context, _, _ string) error { return nil }
func (m *mockStore) ListValueVersions(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockStore) GetValueVersion(_ context.Context, _, _, _ string) (config.Value, bool, error) {
	return config.Value{}, false, nil
}
func (m *mockStore) RollbackValue(_ context.Context, _, _, _ string) error      { return nil }
func (m *mockStore) DeleteValueVersion(_ context.Context, _, _, _ string) error { return nil }
func (m *mockStore) InitEncryption(_ context.Context, _ []byte) error           { return nil }
func (m *mockStore) RotateEncryption(_ context.Context, _, _ []byte) error      { return nil }
func (m *mockStore) GrantAccess(_ context.Context, _ string, _ string, _ []store.Permission) error {
	return nil
}
func (m *mockStore) RevokeAccess(_ context.Context, _ string, _ string, _ []string) error {
	return nil
}
func (m *mockStore) ListAccess(_ context.Context, _ string) (map[string][]store.Permission, error) {
	return nil, nil
}

func setupTestController(t *testing.T) *ui.UIController {
	t.Helper()

	reg := core.NewRegistry()
	mc := newMockConfig()
	ms := newMockStore()

	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterStore("mock", func(string, map[string]any) (store.Plugin, error) {
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
