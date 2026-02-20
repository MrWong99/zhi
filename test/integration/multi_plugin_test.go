package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// --- In-memory store for multi-plugin tests ---

type memStore struct {
	trees map[string]map[string]config.Value
}

func newMemStore() *memStore {
	return &memStore{trees: make(map[string]map[string]config.Value)}
}

func (m *memStore) Capabilities(context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning: store.VersioningNone,
		Encryption: store.EncryptionNone,
	}, nil
}
func (m *memStore) AuthMethods(context.Context) ([]store.AuthMethod, error) {
	return nil, fmt.Errorf("auth not supported")
}
func (m *memStore) Login(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, fmt.Errorf("auth not supported")
}
func (m *memStore) LoginInteractive(context.Context, string, map[string]string) (*store.InteractiveChallenge, error) {
	return nil, fmt.Errorf("interactive login not supported")
}
func (m *memStore) LoginInteractiveCallback(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, fmt.Errorf("interactive login not supported")
}
func (m *memStore) ListTrees(context.Context) ([]string, error) {
	keys := make([]string, 0, len(m.trees))
	for k := range m.trees {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}
func (m *memStore) DeleteTree(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}
func (m *memStore) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
	vals := m.trees[id]
	if vals == nil {
		return nil, nil
	}
	result := make(map[string]config.Value)
	for _, p := range paths {
		if v, ok := vals[p]; ok {
			result[p] = v
		}
	}
	return result, nil
}
func (m *memStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	if m.trees[id] == nil {
		m.trees[id] = make(map[string]config.Value)
	}
	for k, v := range values {
		m.trees[id][k] = v
	}
	return nil
}
func (m *memStore) DeleteValues(_ context.Context, id string, paths []string) error {
	vals := m.trees[id]
	if vals == nil {
		return nil
	}
	for _, p := range paths {
		delete(vals, p)
	}
	return nil
}
func (m *memStore) ListTreeVersions(context.Context, string) ([]string, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *memStore) GetTreeVersion(context.Context, string, string, []string) (map[string]config.Value, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *memStore) RollbackTree(context.Context, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *memStore) DeleteTreeVersion(context.Context, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *memStore) ListValueVersions(context.Context, string, string) ([]string, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *memStore) GetValueVersion(context.Context, string, string, string) (config.Value, bool, error) {
	return config.Value{}, false, fmt.Errorf("versioning not supported")
}
func (m *memStore) RollbackValue(context.Context, string, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *memStore) DeleteValueVersion(context.Context, string, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *memStore) InitEncryption(context.Context, []byte) error           { return nil }
func (m *memStore) RotateEncryption(context.Context, []byte, []byte) error { return nil }
func (m *memStore) GrantAccess(context.Context, string, string, []store.Permission) error {
	return fmt.Errorf("access control not supported")
}
func (m *memStore) RevokeAccess(context.Context, string, string, []string) error {
	return fmt.Errorf("access control not supported")
}
func (m *memStore) ListAccess(context.Context, string) (map[string][]store.Permission, error) {
	return nil, fmt.Errorf("access control not supported")
}

// --- Transforms for testing ---

// prefixTransform adds a "display/" prefix path before display and removes it after save.
type prefixTransform struct{}

func (p *prefixTransform) BeforeDisplay(_ context.Context, tree *config.Tree) error {
	return tree.Set("display/info", &config.Value{Val: "display-only-data"})
}

func (p *prefixTransform) AfterSave(_ context.Context, tree *config.Tree) error {
	tree.Delete("display/info")
	return nil
}

func (p *prefixTransform) ValidatePolicy(context.Context) (transform.ValidatePolicy, error) {
	return transform.ValidateBeforeTransform, nil
}

// uppercaseTransform uppercases all string values before display.
type uppercaseTransform struct{}

func (u *uppercaseTransform) BeforeDisplay(_ context.Context, tree *config.Tree) error {
	for _, path := range tree.List() {
		v, ok := tree.GetPtr(path)
		if !ok {
			continue
		}
		if s, isStr := v.Val.(string); isStr {
			v.Val = strings.ToUpper(s)
		}
	}
	return nil
}

func (u *uppercaseTransform) AfterSave(_ context.Context, _ *config.Tree) error {
	return nil
}

func (u *uppercaseTransform) ValidatePolicy(context.Context) (transform.ValidatePolicy, error) {
	return transform.ValidateBeforeTransform, nil
}

// --- Multi-plugin integration tests ---

// TestMultiPluginConfigTransformStore tests the full lifecycle:
// config loads data -> transform mutates for display -> store persists -> reload merges.
func TestMultiPluginConfigTransformStore(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: myapp
  version:
    val: "1.0"
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	ms := newMemStore()
	pt := &prefixTransform{}

	reg := core.DefaultRegistry()
	if err := reg.RegisterStore("memory", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterTransform("prefix", func(string, map[string]any) (transform.Plugin, error) {
		return pt, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws.Store = core.ProviderRef{Provider: "memory"}
	ws.Transform = []core.ProviderRef{{Provider: "prefix"}}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	// Step 1: Load tree from config.
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	if _, ok := tree.Get("app/name"); !ok {
		t.Fatal("app/name should exist from config")
	}

	// Step 2: Transform for display should add display/info.
	if err := eng.TransformForDisplay(ctx, tree); err != nil {
		t.Fatalf("TransformForDisplay: %v", err)
	}
	if v, ok := tree.Get("display/info"); !ok {
		t.Error("display/info should exist after BeforeDisplay")
	} else if v.Val != "display-only-data" {
		t.Errorf("display/info = %v, want display-only-data", v.Val)
	}

	// Step 3: Transform for save should remove display/info.
	if err := eng.TransformForSave(ctx, tree); err != nil {
		t.Fatalf("TransformForSave: %v", err)
	}
	if _, ok := tree.Get("display/info"); ok {
		t.Error("display/info should be removed after AfterSave")
	}

	// Step 4: Save to store.
	if err := eng.SaveTree(ctx, tree); err != nil {
		t.Fatalf("SaveTree: %v", err)
	}

	// Step 5: Verify store has the values.
	treeID := eng.TreeID()
	ids, err := eng.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	found := false
	for _, id := range ids {
		if id == treeID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tree ID %q not in store trees: %v", treeID, ids)
	}

	// Step 6: Reload tree - stored values should merge.
	tree2, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree (second): %v", err)
	}
	v, ok := tree2.Get("app/name")
	if !ok || v.Val != "myapp" {
		t.Errorf("app/name after reload = %v, want myapp", v.Val)
	}
}

// TestMultipleTransformChaining verifies that multiple transforms are applied
// in order, each building on the previous result.
func TestMultipleTransformChaining(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  greeting:
    val: hello
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	reg := core.DefaultRegistry()
	// Register two transforms in order: prefix then uppercase.
	if err := reg.RegisterTransform("prefix", func(string, map[string]any) (transform.Plugin, error) {
		return &prefixTransform{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterTransform("uppercase", func(string, map[string]any) (transform.Plugin, error) {
		return &uppercaseTransform{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws.Transform = []core.ProviderRef{
		{Provider: "prefix"},
		{Provider: "uppercase"},
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	if err := eng.TransformForDisplay(ctx, tree); err != nil {
		t.Fatalf("TransformForDisplay: %v", err)
	}

	// After both transforms:
	// 1. prefix added display/info with "display-only-data"
	// 2. uppercase uppercased all strings
	v, ok := tree.Get("app/greeting")
	if !ok {
		t.Fatal("app/greeting not found")
	}
	if v.Val != "HELLO" {
		t.Errorf("app/greeting = %v, want HELLO (uppercased by transform)", v.Val)
	}

	v, ok = tree.Get("display/info")
	if !ok {
		t.Fatal("display/info not found")
	}
	if v.Val != "DISPLAY-ONLY-DATA" {
		t.Errorf("display/info = %v, want DISPLAY-ONLY-DATA", v.Val)
	}
}

// TestStoreSaveReloadPreservesTypes verifies that values round-trip through
// the store with their types preserved.
func TestStoreSaveReloadPreservesTypes(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"types.yaml": `types:
  str:
    val: hello
  num:
    val: 42
  flag:
    val: true
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	ms := newMemStore()
	reg := core.DefaultRegistry()
	if err := reg.RegisterStore("memory", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}
	ws.Store = core.ProviderRef{Provider: "memory"}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	// Save and reload.
	if err := eng.SaveTree(ctx, tree); err != nil {
		t.Fatalf("SaveTree: %v", err)
	}

	tree2, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree (reload): %v", err)
	}

	// Verify types.
	v, ok := tree2.Get("types/str")
	if !ok || v.Val != "hello" {
		t.Errorf("types/str = %v, want hello", v.Val)
	}

	v, ok = tree2.Get("types/flag")
	if !ok || v.Val != true {
		t.Errorf("types/flag = %v, want true", v.Val)
	}
}

// TestStoreDeleteValues verifies that deleting values from the store works
// correctly and a subsequent load reflects the deletion.
func TestStoreDeleteValues(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: myapp
  debug:
    val: true
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	ms := newMemStore()
	reg := core.DefaultRegistry()
	if err := reg.RegisterStore("memory", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}
	ws.Store = core.ProviderRef{Provider: "memory"}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	// Save initial tree.
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	if err := eng.SaveTree(ctx, tree); err != nil {
		t.Fatalf("SaveTree: %v", err)
	}

	// Delete app/debug from store.
	if err := eng.DeleteValues(ctx, eng.TreeID(), []string{"app/debug"}); err != nil {
		t.Fatalf("DeleteValues: %v", err)
	}

	// Verify deleted from store.
	storedTree, found, err := eng.LoadStoredTree(ctx)
	if err != nil {
		t.Fatalf("LoadStoredTree: %v", err)
	}
	if !found {
		t.Fatal("stored tree not found")
	}
	if _, ok := storedTree.Get("app/debug"); ok {
		t.Error("app/debug should have been deleted from store")
	}
	// app/name should still be there.
	if _, ok := storedTree.Get("app/name"); !ok {
		t.Error("app/name should still exist in store")
	}
}

// TestStoreDeleteTree verifies that deleting an entire tree works.
func TestStoreDeleteTree(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	ms := newMemStore()
	reg := core.DefaultRegistry()
	if err := reg.RegisterStore("memory", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}
	ws.Store = core.ProviderRef{Provider: "memory"}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	if err := eng.SaveTree(ctx, tree); err != nil {
		t.Fatalf("SaveTree: %v", err)
	}

	// Verify tree exists.
	ids, err := eng.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected at least one tree")
	}

	// Delete it.
	if err := eng.DeleteTree(ctx, eng.TreeID()); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	// Verify gone.
	ids, err = eng.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees after delete: %v", err)
	}
	for _, id := range ids {
		if id == eng.TreeID() {
			t.Error("tree should have been deleted")
		}
	}
}
