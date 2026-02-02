package core

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// mockConfig is a config.Plugin backed by an in-memory tree for testing.
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
	// Return a warning for database/host to verify validation works.
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

// mockTransform is a transform.Plugin for testing.
type mockTransform struct {
	beforeDisplayCalled bool
	afterSaveCalled     bool
}

func (m *mockTransform) BeforeDisplay(_ context.Context, tree *config.Tree) error {
	m.beforeDisplayCalled = true
	// Add a display-only path.
	return tree.Set("display/hint", &config.Value{Val: "visible"})
}

func (m *mockTransform) AfterSave(_ context.Context, tree *config.Tree) error {
	m.afterSaveCalled = true
	tree.Delete("display/hint")
	return nil
}

func (m *mockTransform) ValidatePolicy(context.Context) (transform.ValidatePolicy, error) {
	return transform.ValidateBeforeTransform, nil
}

// mockStore is a store.Plugin backed by an in-memory map for testing.
type mockStore struct {
	trees map[string]map[string]config.Value
}

func newMockStore() *mockStore {
	return &mockStore{trees: make(map[string]map[string]config.Value)}
}

func (m *mockStore) Capabilities(context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning: store.VersioningNone,
		Encryption: store.EncryptionNone,
	}, nil
}

func (m *mockStore) AuthMethods(context.Context) ([]store.AuthMethod, error) {
	return nil, fmt.Errorf("auth not supported")
}
func (m *mockStore) Login(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, fmt.Errorf("auth not supported")
}

func (m *mockStore) ListTrees(context.Context) ([]string, error) {
	var ids []string
	for id := range m.trees {
		ids = append(ids, id)
	}
	sort.Strings(ids)
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
	for p, v := range values {
		m.trees[id][p] = v
	}
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

func (m *mockStore) ListTreeVersions(context.Context, string) ([]string, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *mockStore) GetTreeVersion(context.Context, string, string, []string) (map[string]config.Value, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *mockStore) RollbackTree(context.Context, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) DeleteTreeVersion(context.Context, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) ListValueVersions(context.Context, string, string) ([]string, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *mockStore) GetValueVersion(context.Context, string, string, string) (config.Value, bool, error) {
	return config.Value{}, false, fmt.Errorf("versioning not supported")
}
func (m *mockStore) RollbackValue(context.Context, string, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) DeleteValueVersion(context.Context, string, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) InitEncryption(context.Context, []byte) error           { return nil }
func (m *mockStore) RotateEncryption(context.Context, []byte, []byte) error { return nil }
func (m *mockStore) GrantAccess(context.Context, string, string, []store.Permission) error {
	return fmt.Errorf("access control not supported")
}
func (m *mockStore) RevokeAccess(context.Context, string, string, []string) error {
	return fmt.Errorf("access control not supported")
}
func (m *mockStore) ListAccess(context.Context, string) (map[string][]store.Permission, error) {
	return nil, fmt.Errorf("access control not supported")
}

func setupTestEngine(t *testing.T) (*Engine, *mockConfig, *mockTransform, *mockStore) {
	t.Helper()

	mc := newMockConfig()
	mt := &mockTransform{}
	ms := newMockStore()

	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	})
	_ = reg.RegisterTransform("mock", func(string, map[string]any) (transform.Plugin, error) {
		return mt, nil
	})
	_ = reg.RegisterStore("mock", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	})

	ws := &WorkspaceConfig{
		Version:   "1",
		Config:    ProviderRef{Provider: "mock"},
		Transform: []ProviderRef{{Provider: "mock"}},
		Store:     ProviderRef{Provider: "mock"},
		Components: []ComponentDef{
			{Name: "database", Paths: []string{"database/"}, Mandatory: true},
			{Name: "extras", Paths: []string{"extras/"}},
		},
	}

	eng, err := NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng, mc, mt, ms
}

func TestEngineLoadTree(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	paths := tree.List()
	sort.Strings(paths)
	want := []string{"app/name", "database/host", "database/port"}
	if !slices.Equal(paths, want) {
		t.Errorf("tree paths = %v, want %v", paths, want)
	}

	v, ok := tree.Get("database/host")
	if !ok || v.Val != "localhost" {
		t.Errorf("database/host = %v (ok=%v), want localhost", v.Val, ok)
	}
}

func TestEngineSetValue(t *testing.T) {
	eng, mc, _, _ := setupTestEngine(t)
	ctx := context.Background()

	err := eng.SetValue(ctx, "database/host", config.Value{Val: "remotehost"})
	if err != nil {
		t.Fatalf("SetValue: %v", err)
	}

	v, ok := mc.tree.Get("database/host")
	if !ok || v.Val != "remotehost" {
		t.Errorf("database/host = %v (ok=%v), want remotehost", v.Val, ok)
	}
}

func TestEngineValidate(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	results, err := eng.Validate(ctx, tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Should have at least the "using localhost" warning.
	found := false
	for _, r := range results {
		if r.Message == "using localhost" && r.Severity == config.Warning {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'using localhost' warning in results: %v", results)
	}
}

func TestEngineTransform(t *testing.T) {
	eng, _, mt, _ := setupTestEngine(t)
	ctx := context.Background()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	if err := eng.TransformForDisplay(ctx, tree); err != nil {
		t.Fatalf("TransformForDisplay: %v", err)
	}
	if !mt.beforeDisplayCalled {
		t.Error("BeforeDisplay was not called")
	}
	if _, ok := tree.Get("display/hint"); !ok {
		t.Error("expected display/hint to be added by BeforeDisplay")
	}

	if err := eng.TransformForSave(ctx, tree); err != nil {
		t.Fatalf("TransformForSave: %v", err)
	}
	if !mt.afterSaveCalled {
		t.Error("AfterSave was not called")
	}
	if _, ok := tree.Get("display/hint"); ok {
		t.Error("expected display/hint to be removed by AfterSave")
	}
}

func TestEngineSaveAndLoadTree(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	if err := eng.SaveTree(ctx, "test-tree", tree); err != nil {
		t.Fatalf("SaveTree: %v", err)
	}

	loaded, found, err := eng.LoadStoredTree(ctx, "test-tree")
	if err != nil {
		t.Fatalf("LoadStoredTree: %v", err)
	}
	if !found {
		t.Fatal("tree not found after save")
	}

	loadedPaths := loaded.List()
	sort.Strings(loadedPaths)
	origPaths := tree.List()
	sort.Strings(origPaths)

	if !slices.Equal(loadedPaths, origPaths) {
		t.Errorf("loaded paths = %v, want %v", loadedPaths, origPaths)
	}
}

func TestEngineListTrees(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	tree, _ := eng.LoadTree(ctx)
	_ = eng.SaveTree(ctx, "alpha", tree)
	_ = eng.SaveTree(ctx, "beta", tree)

	ids, err := eng.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	sort.Strings(ids)
	want := []string{"alpha", "beta"}
	if !slices.Equal(ids, want) {
		t.Errorf("ListTrees() = %v, want %v", ids, want)
	}
}

func TestEngineComponents(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	cm := eng.Components()

	if cm == nil {
		t.Fatal("Components() returned nil")
	}
	if !cm.IsEnabled("database") {
		t.Error("mandatory component 'database' should be enabled")
	}
	if cm.IsEnabled("extras") {
		t.Error("non-mandatory component 'extras' should be disabled")
	}
}

func TestEngineFilteredTree(t *testing.T) {
	eng, mc, _, _ := setupTestEngine(t)
	ctx := context.Background()

	// Add an extras path to the mock config.
	_ = mc.tree.Set("extras/feature", &config.Value{Val: "cool"})

	filtered, err := eng.FilteredTree(ctx)
	if err != nil {
		t.Fatalf("FilteredTree: %v", err)
	}

	paths := filtered.List()
	sort.Strings(paths)
	// extras is disabled, so extras/feature should be excluded.
	// app/name is unmanaged, so it's included.
	want := []string{"app/name", "database/host", "database/port"}
	if !slices.Equal(paths, want) {
		t.Errorf("FilteredTree paths = %v, want %v", paths, want)
	}

	// Enable extras and check again.
	if err := eng.Components().Enable("extras"); err != nil {
		t.Fatalf("Enable extras: %v", err)
	}
	filtered2, err := eng.FilteredTree(ctx)
	if err != nil {
		t.Fatalf("FilteredTree: %v", err)
	}
	paths2 := filtered2.List()
	sort.Strings(paths2)
	want2 := []string{"app/name", "database/host", "database/port", "extras/feature"}
	if !slices.Equal(paths2, want2) {
		t.Errorf("FilteredTree paths = %v, want %v", paths2, want2)
	}
}

func TestEngineNoStore(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "mock"},
	}

	eng, err := NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx := context.Background()

	if err := eng.SaveTree(ctx, "test", config.NewTree()); err == nil {
		t.Error("expected error saving without store")
	}
	if _, _, err := eng.LoadStoredTree(ctx, "test"); err == nil {
		t.Error("expected error loading without store")
	}
	if _, err := eng.ListTrees(ctx); err == nil {
		t.Error("expected error listing without store")
	}
}

func TestEngineWithStructuredFileProvider(t *testing.T) {
	// Use a dedicated testdata directory with valid fixture files.
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("resolving testdata path: %v", err)
	}

	reg := DefaultRegistry()
	ws := &WorkspaceConfig{
		Version: "1",
		Config: ProviderRef{
			Provider: "structuredfile",
			Options:  map[string]any{"directory": testdataDir},
		},
		Components: []ComponentDef{
			{Name: "pokedex", Paths: []string{"pokedex/"}, Mandatory: true},
		},
	}

	eng, err := NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx := context.Background()
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	paths := tree.List()
	if len(paths) == 0 {
		t.Fatal("expected non-empty tree from structuredfile testdata")
	}

	// Verify a known value from basic.yaml.
	v, ok := tree.Get("pokedex/starter")
	if !ok {
		t.Fatal("expected pokedex/starter to exist")
	}
	if v.Val != "pikachu" {
		t.Errorf("pokedex/starter = %v, want pikachu", v.Val)
	}
}

func TestEngineValidateIncludesComponentDeps(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	// Force an inconsistent state: enable extras (depends on nothing),
	// but manually add a dep check.
	cm := eng.Components()
	cm.state["extras"] = true

	// Create a tree with extras path.
	tree := config.NewTree()
	_ = tree.Set("extras/val", &config.Value{Val: "x"})

	results, err := eng.Validate(ctx, tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Since extras has no deps, there should be no component dep errors.
	for _, r := range results {
		if r.Severity == config.Blocking {
			t.Errorf("unexpected blocking result: %s", r.Message)
		}
	}
}

func TestNewEngineUnknownConfigProvider(t *testing.T) {
	reg := NewRegistry()
	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "nonexistent"},
	}
	_, err := NewEngine(reg, ws)
	if err == nil {
		t.Error("expected error for unknown config provider")
	}
}

func TestNewEngineUnknownTransformProvider(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	ws := &WorkspaceConfig{
		Version:   "1",
		Config:    ProviderRef{Provider: "mock"},
		Transform: []ProviderRef{{Provider: "nonexistent"}},
	}
	_, err := NewEngine(reg, ws)
	if err == nil {
		t.Error("expected error for unknown transform provider")
	}
}

func TestNewEngineUnknownStoreProvider(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "mock"},
		Store:   ProviderRef{Provider: "nonexistent"},
	}
	_, err := NewEngine(reg, ws)
	if err == nil {
		t.Error("expected error for unknown store provider")
	}
}

// --- Engine store operation tests ---

func TestEngineDeleteTree(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	tree, _ := eng.LoadTree(ctx)
	_ = eng.SaveTree(ctx, "to-delete", tree)

	if err := eng.DeleteTree(ctx, "to-delete"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	ids, _ := eng.ListTrees(ctx)
	for _, id := range ids {
		if id == "to-delete" {
			t.Error("tree should have been deleted")
		}
	}
}

func TestEngineDeleteValues(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	tree, _ := eng.LoadTree(ctx)
	_ = eng.SaveTree(ctx, "test", tree)

	if err := eng.DeleteValues(ctx, "test", []string{"database/host"}); err != nil {
		t.Fatalf("DeleteValues: %v", err)
	}

	loaded, found, _ := eng.LoadStoredTree(ctx, "test")
	if !found {
		t.Fatal("tree not found after DeleteValues")
	}
	if _, ok := loaded.Get("database/host"); ok {
		t.Error("database/host should have been deleted")
	}
}

func TestEngineStoreCapabilities(t *testing.T) {
	eng, _, _, _ := setupTestEngine(t)
	ctx := context.Background()

	caps, err := eng.StoreCapabilities(ctx)
	if err != nil {
		t.Fatalf("StoreCapabilities: %v", err)
	}
	if caps.Versioning != store.VersioningNone {
		t.Errorf("Versioning = %v, want VersioningNone", caps.Versioning)
	}
}

func TestEngineNoStoreVersionOps(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "mock"},
	}

	eng, err := NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	ctx := context.Background()

	if _, err := eng.StoreCapabilities(ctx); err == nil {
		t.Error("expected error for StoreCapabilities without store")
	}
	if err := eng.DeleteTree(ctx, "x"); err == nil {
		t.Error("expected error for DeleteTree without store")
	}
	if err := eng.DeleteValues(ctx, "x", nil); err == nil {
		t.Error("expected error for DeleteValues without store")
	}
	if _, err := eng.ListTreeVersions(ctx, "x"); err == nil {
		t.Error("expected error for ListTreeVersions without store")
	}
	if _, _, err := eng.GetTreeVersion(ctx, "x", "v1"); err == nil {
		t.Error("expected error for GetTreeVersion without store")
	}
	if err := eng.RollbackTree(ctx, "x", "v1"); err == nil {
		t.Error("expected error for RollbackTree without store")
	}
	if err := eng.DeleteTreeVersion(ctx, "x", "v1"); err == nil {
		t.Error("expected error for DeleteTreeVersion without store")
	}
	if _, err := eng.ListValueVersions(ctx, "x", "p"); err == nil {
		t.Error("expected error for ListValueVersions without store")
	}
	if _, _, err := eng.GetValueVersion(ctx, "x", "p", "v1"); err == nil {
		t.Error("expected error for GetValueVersion without store")
	}
	if err := eng.RollbackValue(ctx, "x", "p", "v1"); err == nil {
		t.Error("expected error for RollbackValue without store")
	}
	if err := eng.DeleteValueVersion(ctx, "x", "p", "v1"); err == nil {
		t.Error("expected error for DeleteValueVersion without store")
	}
}

// --- Versioned mock store for engine tests ---

type versionedMockStore struct {
	mockStore
	versions map[string][]map[string]config.Value // tree id -> list of snapshots
}

func newVersionedMockStore() *versionedMockStore {
	return &versionedMockStore{
		mockStore: *newMockStore(),
		versions:  make(map[string][]map[string]config.Value),
	}
}

func (m *versionedMockStore) Capabilities(context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning: store.VersioningTree,
		Encryption: store.EncryptionNone,
	}, nil
}

func (m *versionedMockStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	if m.trees[id] == nil {
		m.trees[id] = make(map[string]config.Value)
	}
	// Snapshot before writing.
	snap := make(map[string]config.Value, len(m.trees[id]))
	for k, v := range m.trees[id] {
		snap[k] = v
	}
	m.versions[id] = append(m.versions[id], snap)

	for p, v := range values {
		m.trees[id][p] = v
	}
	return nil
}

func (m *versionedMockStore) ListTreeVersions(_ context.Context, id string) ([]string, error) {
	snaps := m.versions[id]
	versions := make([]string, len(snaps))
	for i := range snaps {
		versions[i] = fmt.Sprintf("v%d", len(snaps)-i)
	}
	return versions, nil
}

func (m *versionedMockStore) GetTreeVersion(_ context.Context, id string, version string, paths []string) (map[string]config.Value, error) {
	snaps := m.versions[id]
	for i := range snaps {
		if fmt.Sprintf("v%d", i+1) == version {
			result := make(map[string]config.Value)
			for _, p := range paths {
				if v, ok := snaps[i][p]; ok {
					result[p] = v
				}
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("version %q not found", version)
}

func (m *versionedMockStore) RollbackTree(_ context.Context, id string, version string) error {
	snaps := m.versions[id]
	for i := range snaps {
		if fmt.Sprintf("v%d", i+1) == version {
			tree := make(map[string]config.Value, len(snaps[i]))
			for k, v := range snaps[i] {
				tree[k] = v
			}
			m.trees[id] = tree
			return nil
		}
	}
	return fmt.Errorf("version %q not found", version)
}

func (m *versionedMockStore) DeleteTreeVersion(_ context.Context, id string, version string) error {
	snaps := m.versions[id]
	for i := range snaps {
		if fmt.Sprintf("v%d", i+1) == version {
			m.versions[id] = append(snaps[:i], snaps[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("version %q not found", version)
}

func setupVersionedTestEngine(t *testing.T) (*Engine, *mockConfig, *versionedMockStore) {
	t.Helper()

	mc := newMockConfig()
	ms := newVersionedMockStore()

	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	})
	_ = reg.RegisterStore("vmock", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "mock"},
		Store:   ProviderRef{Provider: "vmock"},
	}

	eng, err := NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng, mc, ms
}

func TestEngineTreeVersioning(t *testing.T) {
	eng, _, _ := setupVersionedTestEngine(t)
	ctx := context.Background()

	// Save two versions.
	tree1, _ := eng.LoadTree(ctx)
	_ = eng.SaveTree(ctx, "app", tree1)

	_ = eng.SetValue(ctx, "database/host", config.Value{Val: "remote"})
	tree2, _ := eng.LoadTree(ctx)
	_ = eng.SaveTree(ctx, "app", tree2)

	// List versions.
	versions, err := eng.ListTreeVersions(ctx, "app")
	if err != nil {
		t.Fatalf("ListTreeVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}

	// Get version 1 (empty snapshot before first write).
	v1tree, found, err := eng.GetTreeVersion(ctx, "app", "v1")
	if err != nil {
		t.Fatalf("GetTreeVersion v1: %v", err)
	}
	// v1 is snapshot before first write, should be empty.
	if found {
		t.Logf("v1 tree has %d paths (empty snapshot expected)", len(v1tree.List()))
	}

	// Rollback to v1.
	if err := eng.RollbackTree(ctx, "app", "v1"); err != nil {
		t.Fatalf("RollbackTree: %v", err)
	}

	// Delete version v2.
	if err := eng.DeleteTreeVersion(ctx, "app", "v2"); err != nil {
		t.Fatalf("DeleteTreeVersion: %v", err)
	}

	versions, _ = eng.ListTreeVersions(ctx, "app")
	if len(versions) != 1 {
		t.Errorf("got %d versions after delete, want 1", len(versions))
	}
}
