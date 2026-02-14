package ui_test

import (
	"context"
	"maps"
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
func (m *mockTransform) AfterSave(_ context.Context, _ *config.Tree) error     { return nil }
func (m *mockTransform) ValidatePolicy(_ context.Context) (transform.ValidatePolicy, error) {
	return 0, nil
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

// --- test helpers ---

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
	if err := reg.RegisterTransform("mock", func(string, map[string]any) (transform.Plugin, error) {
		return &mockTransform{}, nil
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

	err := ctrl.SaveTree(ctx)
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

func TestUIController_StoreAuthMethods_NoAuth(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	methods, err := ctrl.StoreAuthMethods(ctx)
	if err != nil {
		t.Fatalf("StoreAuthMethods: %v", err)
	}
	// mockStore returns nil for AuthMethods, so result should be empty.
	if len(methods) != 0 {
		t.Errorf("expected 0 methods, got %d", len(methods))
	}
}

func TestUIController_StoreAuthStatus_NoAuth(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	session, err := ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	// mockStore doesn't require auth, so after AuthRequired check the
	// session should reflect that. But since setupTestController doesn't
	// call AuthRequired, the session defaults to Unauthenticated.
	if session == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestUIController_StoreLogout(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := context.Background()

	err := ctrl.StoreLogout(ctx)
	if err != nil {
		t.Fatalf("StoreLogout: %v", err)
	}

	// After logout, status should be unauthenticated.
	session, err := ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus after logout: %v", err)
	}
	if session.Status != "unauthenticated" {
		t.Errorf("expected unauthenticated status after logout, got %q", session.Status)
	}
}

func TestUIController_StoreAuthMethods_WithAuthStore(t *testing.T) {
	reg := core.NewRegistry()
	mc := newMockConfig()
	ms := &authMockStore{mockStore: newMockStore()}

	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterTransform("mock", func(string, map[string]any) (transform.Plugin, error) {
		return &mockTransform{}, nil
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
		Transform: []core.ProviderRef{
			{Provider: "mock"},
		},
		Store: core.ProviderRef{Provider: "mock"},
		Dir:   t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatal(err)
	}

	ctrl := ui.NewUIController(eng)
	ctx := context.Background()

	methods, err := ctrl.StoreAuthMethods(ctx)
	if err != nil {
		t.Fatalf("StoreAuthMethods: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if methods[0].Type != "userpass" {
		t.Errorf("expected userpass method, got %q", methods[0].Type)
	}
	if len(methods[0].Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(methods[0].Fields))
	}

	// Login
	session, err := ctrl.StoreLogin(ctx, "userpass", map[string]string{
		"username": "admin",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("StoreLogin: %v", err)
	}
	if session.Status != "authenticated" {
		t.Errorf("expected authenticated, got %q", session.Status)
	}
	if session.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	// Status check
	status, err := ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	if status.Status != "authenticated" {
		t.Errorf("expected authenticated, got %q", status.Status)
	}

	// Logout
	if err := ctrl.StoreLogout(ctx); err != nil {
		t.Fatalf("StoreLogout: %v", err)
	}
	status, err = ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus after logout: %v", err)
	}
	if status.Status != "unauthenticated" {
		t.Errorf("expected unauthenticated after logout, got %q", status.Status)
	}
}

// authMockStore wraps mockStore and adds auth support.
type authMockStore struct {
	*mockStore
}

func (m *authMockStore) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning: store.VersioningNone,
		Encryption: store.EncryptionNone,
		Auth:       true,
	}, nil
}

func (m *authMockStore) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	return []store.AuthMethod{
		{
			Type:        "userpass",
			Description: "Username and password",
			Fields: []store.AuthField{
				{Name: "username", Description: "Username", Required: true},
				{Name: "password", Description: "Password", Required: true, Secret: true},
			},
		},
	}, nil
}

func (m *authMockStore) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	return &store.Credential{
		Token:    "mock-token",
		Metadata: map[string]string{"username": "admin"},
	}, nil
}
