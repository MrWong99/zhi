package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// testPlugin is a full in-memory implementation of Plugin for testing
// the gRPC round-trip. It supports no versioning, auth, or encryption.
type testPlugin struct {
	mu    sync.RWMutex
	trees map[string]map[string]config.Value // tree id -> path -> value
}

func newTestPlugin() *testPlugin {
	return &testPlugin{trees: make(map[string]map[string]config.Value)}
}

func (t *testPlugin) Capabilities(context.Context) (*Capabilities, error) {
	return &Capabilities{
		Versioning:    VersioningNone,
		Encryption:    EncryptionNone,
		Auth:          false,
		AccessControl: false,
	}, nil
}

func (t *testPlugin) AuthMethods(context.Context) ([]AuthMethod, error) {
	return nil, nil
}

func (t *testPlugin) Login(context.Context, string, map[string]string) (*Credential, error) {
	return nil, errors.New("authentication not supported")
}

func (t *testPlugin) ListTrees(_ context.Context) ([]string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ids := make([]string, 0, len(t.trees))
	for id := range t.trees {
		ids = append(ids, id)
	}
	return ids, nil
}

func (t *testPlugin) DeleteTree(_ context.Context, id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.trees, id)
	return nil
}

func (t *testPlugin) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	tree, ok := t.trees[id]
	if !ok {
		return map[string]config.Value{}, nil
	}
	result := make(map[string]config.Value, len(paths))
	for _, p := range paths {
		if v, found := tree[p]; found {
			result[p] = v
		}
	}
	return result, nil
}

func (t *testPlugin) PutValues(_ context.Context, id string, values map[string]config.Value, _ *PutOptions) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	tree, ok := t.trees[id]
	if !ok {
		tree = make(map[string]config.Value)
		t.trees[id] = tree
	}
	for path, v := range values {
		tree[path] = v
	}
	return nil
}

func (t *testPlugin) DeleteValues(_ context.Context, id string, paths []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	tree, ok := t.trees[id]
	if !ok {
		return nil
	}
	for _, p := range paths {
		delete(tree, p)
	}
	return nil
}

func (t *testPlugin) ListTreeVersions(context.Context, string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (t *testPlugin) GetTreeVersion(context.Context, string, string, []string) (map[string]config.Value, error) {
	return nil, errors.New("versioning not supported")
}

func (t *testPlugin) RollbackTree(context.Context, string, string) error {
	return errors.New("versioning not supported")
}

func (t *testPlugin) DeleteTreeVersion(context.Context, string, string) error {
	return errors.New("versioning not supported")
}

func (t *testPlugin) ListValueVersions(context.Context, string, string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (t *testPlugin) GetValueVersion(context.Context, string, string, string) (config.Value, bool, error) {
	return config.Value{}, false, errors.New("versioning not supported")
}

func (t *testPlugin) RollbackValue(context.Context, string, string, string) error {
	return errors.New("versioning not supported")
}

func (t *testPlugin) DeleteValueVersion(context.Context, string, string, string) error {
	return errors.New("versioning not supported")
}

func (t *testPlugin) InitEncryption(context.Context, []byte) error {
	return errors.New("encryption not supported")
}

func (t *testPlugin) RotateEncryption(context.Context, []byte, []byte) error {
	return errors.New("encryption not supported")
}

func (t *testPlugin) GrantAccess(context.Context, string, string, []Permission) error {
	return errors.New("access control not supported")
}

func (t *testPlugin) RevokeAccess(context.Context, string, string, []string) error {
	return errors.New("access control not supported")
}

func (t *testPlugin) ListAccess(context.Context, string) (map[string][]Permission, error) {
	return nil, errors.New("access control not supported")
}

// dispense starts an in-process gRPC plugin and returns a connected client.
func dispense(t *testing.T, impl Plugin) Plugin {
	t.Helper()
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"store": &GRPCPlugin{Impl: impl},
	})
	raw, err := client.Dispense("store")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(Plugin)
}

func TestCapabilities(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	caps, err := p.Capabilities(ctx)
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.Versioning != VersioningNone {
		t.Errorf("Versioning = %v, want VersioningNone", caps.Versioning)
	}
	if caps.Encryption != EncryptionNone {
		t.Errorf("Encryption = %v, want EncryptionNone", caps.Encryption)
	}
	if caps.Auth {
		t.Error("Auth should be false")
	}
	if caps.AccessControl {
		t.Error("AccessControl should be false")
	}
}

func TestPutAndGetValues(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	values := map[string]config.Value{
		"db/host": {Val: "localhost"},
		"db/port": {Val: float64(5432)},
	}
	if err := p.PutValues(ctx, "prod", values, nil); err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := p.GetValues(ctx, "prod", []string{"db/host", "db/port"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d values, want 2", len(got))
	}
	if got["db/host"].Val != "localhost" {
		t.Errorf("db/host = %v, want %q", got["db/host"].Val, "localhost")
	}
}

func TestGetValuesMissing(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	got, err := p.GetValues(ctx, "nope", []string{"a/b"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d values, want 0 for missing tree", len(got))
	}
}

func TestPartialGet(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	values := map[string]config.Value{
		"a/x": {Val: "one"},
		"a/y": {Val: "two"},
		"b/z": {Val: "three"},
	}
	_ = p.PutValues(ctx, "test", values, nil)

	// Only request two of the three paths.
	got, err := p.GetValues(ctx, "test", []string{"a/x", "b/z"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d values, want 2", len(got))
	}
	if got["a/x"].Val != "one" {
		t.Errorf("a/x = %v, want %q", got["a/x"].Val, "one")
	}
	if got["b/z"].Val != "three" {
		t.Errorf("b/z = %v, want %q", got["b/z"].Val, "three")
	}
}

func TestPutOverwrite(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "old"},
	}, nil)
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "new"},
	}, nil)

	got, _ := p.GetValues(ctx, "app", []string{"app/name"})
	if got["app/name"].Val != "new" {
		t.Errorf("Val = %v, want %q", got["app/name"].Val, "new")
	}
}

func TestDeleteValues(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	_ = p.PutValues(ctx, "test", map[string]config.Value{
		"a/x": {Val: "one"},
		"a/y": {Val: "two"},
	}, nil)

	if err := p.DeleteValues(ctx, "test", []string{"a/x"}); err != nil {
		t.Fatalf("DeleteValues: %v", err)
	}

	got, _ := p.GetValues(ctx, "test", []string{"a/x", "a/y"})
	if _, found := got["a/x"]; found {
		t.Error("a/x should have been deleted")
	}
	if got["a/y"].Val != "two" {
		t.Errorf("a/y = %v, want %q", got["a/y"].Val, "two")
	}
}

func TestDeleteTree(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	_ = p.PutValues(ctx, "temp", map[string]config.Value{
		"a/b": {Val: "x"},
	}, nil)

	if err := p.DeleteTree(ctx, "temp"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	got, _ := p.GetValues(ctx, "temp", []string{"a/b"})
	if len(got) != 0 {
		t.Error("tree still has values after DeleteTree")
	}
}

func TestListTrees(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	_ = p.PutValues(ctx, "one", map[string]config.Value{"a/b": {Val: "x"}}, nil)
	_ = p.PutValues(ctx, "two", map[string]config.Value{"a/b": {Val: "y"}}, nil)

	ids, err := p.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("got %d ids, want 2", len(ids))
	}
}

func TestMetadataPreserved(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	_ = p.PutValues(ctx, "meta", map[string]config.Value{
		"key": {Val: "value", Metadata: map[string]any{"description": "test key"}},
	}, nil)

	got, _ := p.GetValues(ctx, "meta", []string{"key"})
	if got["key"].Metadata["description"] != "test key" {
		t.Errorf("description = %v, want %q", got["key"].Metadata["description"], "test key")
	}
}

func TestValidatorsNotStored(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	_ = p.PutValues(ctx, "val", map[string]config.Value{
		"key": {
			Val:        "test",
			Validators: []config.ValidateFunc{func(any, config.TreeReader) []config.ValidationResult { return nil }},
		},
	}, nil)

	got, _ := p.GetValues(ctx, "val", []string{"key"})
	if len(got["key"].Validators) != 0 {
		t.Error("Validators should not be stored (they cannot cross gRPC)")
	}
}

func TestVersioningNotSupported(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	_, err := p.ListTreeVersions(ctx, "any")
	if err == nil {
		t.Error("ListTreeVersions should return error")
	}

	_, err = p.GetTreeVersion(ctx, "any", "v1", nil)
	if err == nil {
		t.Error("GetTreeVersion should return error")
	}

	if err := p.RollbackTree(ctx, "any", "v1"); err == nil {
		t.Error("RollbackTree should return error")
	}

	if err := p.DeleteTreeVersion(ctx, "any", "v1"); err == nil {
		t.Error("DeleteTreeVersion should return error")
	}

	_, err = p.ListValueVersions(ctx, "any", "path")
	if err == nil {
		t.Error("ListValueVersions should return error")
	}

	_, _, err = p.GetValueVersion(ctx, "any", "path", "v1")
	if err == nil {
		t.Error("GetValueVersion should return error")
	}

	if err := p.RollbackValue(ctx, "any", "path", "v1"); err == nil {
		t.Error("RollbackValue should return error")
	}

	if err := p.DeleteValueVersion(ctx, "any", "path", "v1"); err == nil {
		t.Error("DeleteValueVersion should return error")
	}
}

func TestEncryptionNotSupported(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	caps, _ := p.Capabilities(ctx)
	if caps.Encryption != EncryptionNone {
		t.Errorf("Encryption = %v, want EncryptionNone", caps.Encryption)
	}

	if err := p.InitEncryption(ctx, []byte("pass")); err == nil {
		t.Error("InitEncryption should return error")
	}

	if err := p.RotateEncryption(ctx, []byte("old"), []byte("new")); err == nil {
		t.Error("RotateEncryption should return error")
	}
}

func TestAccessControlNotSupported(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	if err := p.GrantAccess(ctx, "id", "user", []Permission{{Path: "a/b", Actions: []Action{ActionRead}}}); err == nil {
		t.Error("GrantAccess should return error")
	}

	if err := p.RevokeAccess(ctx, "id", "user", []string{"a/b"}); err == nil {
		t.Error("RevokeAccess should return error")
	}

	_, err := p.ListAccess(ctx, "id")
	if err == nil {
		t.Error("ListAccess should return error")
	}
}

func TestAuthNotSupported(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	methods, err := p.AuthMethods(ctx)
	if err != nil {
		t.Fatalf("AuthMethods: %v", err)
	}
	if len(methods) != 0 {
		t.Errorf("got %d auth methods, want 0", len(methods))
	}

	_, err = p.Login(ctx, "userpass", map[string]string{"user": "admin"})
	if err == nil {
		t.Error("Login should return error")
	}
}

// --- Versioned tree test plugin ---

type versionedTreePlugin struct {
	testPlugin
	versions map[string][]map[string]config.Value // tree id -> list of snapshots
}

func newVersionedTreePlugin() *versionedTreePlugin {
	return &versionedTreePlugin{
		testPlugin: *newTestPlugin(),
		versions:   make(map[string][]map[string]config.Value),
	}
}

func (v *versionedTreePlugin) Capabilities(context.Context) (*Capabilities, error) {
	return &Capabilities{
		Versioning: VersioningTree,
		Encryption: EncryptionNone,
	}, nil
}

func (v *versionedTreePlugin) PutValues(_ context.Context, id string, values map[string]config.Value, _ *PutOptions) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	tree, ok := v.trees[id]
	if !ok {
		tree = make(map[string]config.Value)
		v.trees[id] = tree
	}
	// Snapshot current state before writing.
	snap := make(map[string]config.Value, len(tree))
	for k, val := range tree {
		snap[k] = val
	}
	v.versions[id] = append(v.versions[id], snap)

	for path, val := range values {
		tree[path] = val
	}
	return nil
}

func (v *versionedTreePlugin) ListTreeVersions(_ context.Context, id string) ([]string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	snaps := v.versions[id]
	versions := make([]string, len(snaps))
	for i := range snaps {
		// Newest first.
		versions[i] = fmt.Sprintf("v%d", len(snaps)-i)
	}
	return versions, nil
}

func (v *versionedTreePlugin) GetTreeVersion(_ context.Context, id string, version string, paths []string) (map[string]config.Value, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	snaps := v.versions[id]
	idx, err := v.versionIndex(id, version)
	if err != nil {
		return nil, err
	}
	snap := snaps[idx]
	result := make(map[string]config.Value)
	for _, p := range paths {
		if val, ok := snap[p]; ok {
			result[p] = val
		}
	}
	return result, nil
}

func (v *versionedTreePlugin) RollbackTree(_ context.Context, id string, version string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	idx, err := v.versionIndex(id, version)
	if err != nil {
		return err
	}
	snap := v.versions[id][idx]
	tree := make(map[string]config.Value, len(snap))
	for k, val := range snap {
		tree[k] = val
	}
	v.trees[id] = tree
	return nil
}

func (v *versionedTreePlugin) DeleteTreeVersion(_ context.Context, id string, version string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	idx, err := v.versionIndex(id, version)
	if err != nil {
		return err
	}
	snaps := v.versions[id]
	v.versions[id] = append(snaps[:idx], snaps[idx+1:]...)
	return nil
}

func (v *versionedTreePlugin) versionIndex(id string, version string) (int, error) {
	snaps := v.versions[id]
	for i := range snaps {
		label := fmt.Sprintf("v%d", i+1)
		if label == version {
			return i, nil
		}
	}
	return 0, fmt.Errorf("version %q not found for tree %q", version, id)
}

func TestTreeVersioning(t *testing.T) {
	impl := newVersionedTreePlugin()
	p := dispense(t, impl)
	ctx := context.Background()

	caps, _ := p.Capabilities(ctx)
	if caps.Versioning != VersioningTree {
		t.Fatalf("Versioning = %v, want VersioningTree", caps.Versioning)
	}

	// Write first version.
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "v1"},
	}, nil)
	// Write second version.
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "v2"},
	}, nil)

	versions, err := p.ListTreeVersions(ctx, "app")
	if err != nil {
		t.Fatalf("ListTreeVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}

	// Get first version's data.
	v1, err := p.GetTreeVersion(ctx, "app", "v1", []string{"app/name"})
	if err != nil {
		t.Fatalf("GetTreeVersion v1: %v", err)
	}
	// v1 is the snapshot taken before the first PutValues (empty tree).
	if len(v1) != 0 {
		t.Errorf("v1 should be empty (snapshot before first write), got %v", v1)
	}

	// Rollback to v1.
	if err := p.RollbackTree(ctx, "app", "v1"); err != nil {
		t.Fatalf("RollbackTree: %v", err)
	}
	got, _ := p.GetValues(ctx, "app", []string{"app/name"})
	if _, found := got["app/name"]; found {
		t.Error("after rollback to v1, app/name should not exist")
	}

	// Delete a version.
	if err := p.DeleteTreeVersion(ctx, "app", "v2"); err != nil {
		t.Fatalf("DeleteTreeVersion: %v", err)
	}
}

// --- CAS test plugin ---

// casPlugin extends testPlugin with tree-level CAS support.
type casPlugin struct {
	testPlugin
	version string // current tree version, bumped on each PutValues
}

func newCASPlugin() *casPlugin {
	return &casPlugin{
		testPlugin: *newTestPlugin(),
		version:    "v0",
	}
}

func (c *casPlugin) Capabilities(context.Context) (*Capabilities, error) {
	return &Capabilities{
		Versioning: VersioningTree,
		Encryption: EncryptionNone,
	}, nil
}

func (c *casPlugin) PutValues(_ context.Context, id string, values map[string]config.Value, opts *PutOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if opts != nil && opts.CASVersion != "" {
		if opts.CASVersion != c.version {
			return &ErrCASConflict{
				Expected: opts.CASVersion,
				Actual:   c.version,
			}
		}
	}

	tree, ok := c.trees[id]
	if !ok {
		tree = make(map[string]config.Value)
		c.trees[id] = tree
	}
	for path, v := range values {
		tree[path] = v
	}
	// Bump version.
	c.version = fmt.Sprintf("v%d", len(tree))
	return nil
}

func TestCASConflict(t *testing.T) {
	impl := newCASPlugin()
	p := dispense(t, impl)
	ctx := context.Background()

	// Initial write without CAS succeeds.
	err := p.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "first"},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues (no CAS): %v", err)
	}

	// Write with correct CAS version succeeds.
	err = p.PutValues(ctx, "app", map[string]config.Value{
		"app/version": {Val: "second"},
	}, &PutOptions{CASVersion: "v1"})
	if err != nil {
		t.Fatalf("PutValues (correct CAS): %v", err)
	}

	// Write with wrong CAS version fails.
	err = p.PutValues(ctx, "app", map[string]config.Value{
		"app/bad": {Val: "nope"},
	}, &PutOptions{CASVersion: "v1"})
	if err == nil {
		t.Fatal("expected CAS conflict error, got nil")
	}
	if !IsCASConflict(err) {
		t.Errorf("expected ErrCASConflict, got %T: %v", err, err)
	}
}

func TestCASConflictErrorType(t *testing.T) {
	err := &ErrCASConflict{Path: "db/host", Expected: "v1", Actual: "v2"}
	if !IsCASConflict(err) {
		t.Error("IsCASConflict should return true")
	}
	if !IsCASConflict(fmt.Errorf("wrapped: %w", err)) {
		t.Error("IsCASConflict should match wrapped errors")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

// --- Value-level versioning test plugin ---

// versionedValuePlugin stores independent version histories per value.
type versionedValuePlugin struct {
	testPlugin
	// versions: tree id -> path -> list of versioned values
	valueVersions map[string]map[string][]config.Value
}

func newVersionedValuePlugin() *versionedValuePlugin {
	return &versionedValuePlugin{
		testPlugin:    *newTestPlugin(),
		valueVersions: make(map[string]map[string][]config.Value),
	}
}

func (v *versionedValuePlugin) Capabilities(context.Context) (*Capabilities, error) {
	return &Capabilities{
		Versioning: VersioningValue,
		Encryption: EncryptionNone,
	}, nil
}

func (v *versionedValuePlugin) PutValues(_ context.Context, id string, values map[string]config.Value, opts *PutOptions) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	tree, ok := v.trees[id]
	if !ok {
		tree = make(map[string]config.Value)
		v.trees[id] = tree
	}
	if v.valueVersions[id] == nil {
		v.valueVersions[id] = make(map[string][]config.Value)
	}

	for path, val := range values {
		if opts != nil && opts.CASVersions != nil {
			if expected, hasExpected := opts.CASVersions[path]; hasExpected {
				actual := fmt.Sprintf("v%d", len(v.valueVersions[id][path])+1)
				if expected != actual {
					return &ErrCASConflict{
						Path:     path,
						Expected: expected,
						Actual:   actual,
					}
				}
			}
		}
		// Snapshot current value before overwriting.
		if existing, ok := tree[path]; ok {
			v.valueVersions[id][path] = append(v.valueVersions[id][path], existing)
		}
		tree[path] = val
	}
	return nil
}

func (v *versionedValuePlugin) ListValueVersions(_ context.Context, id string, path string) ([]string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	versions := v.valueVersions[id][path]
	result := make([]string, len(versions))
	for i := range versions {
		result[i] = fmt.Sprintf("v%d", len(versions)-i)
	}
	return result, nil
}

func (v *versionedValuePlugin) GetValueVersion(_ context.Context, id string, path string, version string) (config.Value, bool, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	versions := v.valueVersions[id][path]
	idx, err := v.versionIdx(versions, version)
	if err != nil {
		return config.Value{}, false, nil
	}
	return versions[idx], true, nil
}

func (v *versionedValuePlugin) RollbackValue(_ context.Context, id string, path string, version string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	versions := v.valueVersions[id][path]
	idx, err := v.versionIdx(versions, version)
	if err != nil {
		return err
	}
	old := versions[idx]
	tree := v.trees[id]
	if tree == nil {
		return fmt.Errorf("tree %q not found", id)
	}
	// Snapshot current before rollback.
	if current, ok := tree[path]; ok {
		v.valueVersions[id][path] = append(versions, current)
	}
	tree[path] = old
	return nil
}

func (v *versionedValuePlugin) DeleteValueVersion(_ context.Context, id string, path string, version string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	versions := v.valueVersions[id][path]
	idx, err := v.versionIdx(versions, version)
	if err != nil {
		return err
	}
	v.valueVersions[id][path] = append(versions[:idx], versions[idx+1:]...)
	return nil
}

func (v *versionedValuePlugin) versionIdx(versions []config.Value, version string) (int, error) {
	for i := range versions {
		label := fmt.Sprintf("v%d", i+1)
		if label == version {
			return i, nil
		}
	}
	return 0, fmt.Errorf("version %q not found", version)
}

// Tree-level versioning stubs return errors since this is a value-level plugin.
func (v *versionedValuePlugin) ListTreeVersions(context.Context, string) ([]string, error) {
	return nil, &ErrNotSupported{Feature: "tree versioning"}
}
func (v *versionedValuePlugin) GetTreeVersion(context.Context, string, string, []string) (map[string]config.Value, error) {
	return nil, &ErrNotSupported{Feature: "tree versioning"}
}
func (v *versionedValuePlugin) RollbackTree(context.Context, string, string) error {
	return &ErrNotSupported{Feature: "tree versioning"}
}
func (v *versionedValuePlugin) DeleteTreeVersion(context.Context, string, string) error {
	return &ErrNotSupported{Feature: "tree versioning"}
}

func TestValueVersioning(t *testing.T) {
	impl := newVersionedValuePlugin()
	p := dispense(t, impl)
	ctx := context.Background()

	caps, _ := p.Capabilities(ctx)
	if caps.Versioning != VersioningValue {
		t.Fatalf("Versioning = %v, want VersioningValue", caps.Versioning)
	}

	// Write initial value.
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"db/host": {Val: "host1"},
	}, nil)

	// Update the value (creating a version).
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"db/host": {Val: "host2"},
	}, nil)

	// Update again.
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"db/host": {Val: "host3"},
	}, nil)

	// List versions.
	versions, err := p.ListValueVersions(ctx, "app", "db/host")
	if err != nil {
		t.Fatalf("ListValueVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2 (host1 and host2 are historical)", len(versions))
	}

	// Get historical version v1 (should be "host1").
	v, found, err := p.GetValueVersion(ctx, "app", "db/host", "v1")
	if err != nil {
		t.Fatalf("GetValueVersion v1: %v", err)
	}
	if !found {
		t.Fatal("expected v1 to be found")
	}
	if v.Val != "host1" {
		t.Errorf("v1 = %v, want host1", v.Val)
	}

	// Get v2 (should be "host2").
	v, found, err = p.GetValueVersion(ctx, "app", "db/host", "v2")
	if err != nil {
		t.Fatalf("GetValueVersion v2: %v", err)
	}
	if !found {
		t.Fatal("expected v2 to be found")
	}
	if v.Val != "host2" {
		t.Errorf("v2 = %v, want host2", v.Val)
	}

	// Rollback to v1.
	if err := p.RollbackValue(ctx, "app", "db/host", "v1"); err != nil {
		t.Fatalf("RollbackValue: %v", err)
	}
	got, _ := p.GetValues(ctx, "app", []string{"db/host"})
	if got["db/host"].Val != "host1" {
		t.Errorf("after rollback, db/host = %v, want host1", got["db/host"].Val)
	}

	// Delete version v2.
	if err := p.DeleteValueVersion(ctx, "app", "db/host", "v2"); err != nil {
		t.Fatalf("DeleteValueVersion: %v", err)
	}
}

func TestValueVersioningCASConflict(t *testing.T) {
	impl := newVersionedValuePlugin()
	p := dispense(t, impl)
	ctx := context.Background()

	// Write initial value.
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"db/host": {Val: "host1"},
	}, nil)

	// CAS write with wrong version.
	err := p.PutValues(ctx, "app", map[string]config.Value{
		"db/host": {Val: "host2"},
	}, &PutOptions{CASVersions: map[string]string{"db/host": "v99"}})
	if err == nil {
		t.Fatal("expected CAS conflict error")
	}
	if !IsCASConflict(err) {
		t.Errorf("expected ErrCASConflict, got %T: %v", err, err)
	}
}

func TestValueVersioningTreeMethodsError(t *testing.T) {
	impl := newVersionedValuePlugin()
	p := dispense(t, impl)
	ctx := context.Background()

	_, err := p.ListTreeVersions(ctx, "any")
	if err == nil {
		t.Error("ListTreeVersions should error for value-level plugin")
	}
	if !IsNotSupported(err) {
		t.Errorf("expected ErrNotSupported, got %T: %v", err, err)
	}
}

// --- Error type unit tests ---

func TestErrorTypeCheckers(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		checker func(error) bool
	}{
		{"CASConflict", &ErrCASConflict{}, IsCASConflict},
		{"AuthRequired", &ErrAuthRequired{}, IsAuthRequired},
		{"AccessDenied", &ErrAccessDenied{}, IsAccessDenied},
		{"PathNotFound", &ErrPathNotFound{}, IsPathNotFound},
		{"EncryptionNotInitialized", &ErrEncryptionNotInitialized{}, IsEncryptionNotInitialized},
		{"NotSupported", &ErrNotSupported{Feature: "versioning"}, IsNotSupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.checker(tt.err) {
				t.Errorf("checker should return true for %T", tt.err)
			}
			// Also test wrapped errors.
			wrapped := fmt.Errorf("wrapped: %w", tt.err)
			if !tt.checker(wrapped) {
				t.Errorf("checker should return true for wrapped %T", tt.err)
			}
			// Negative test: a plain error should not match.
			if tt.checker(errors.New("unrelated")) {
				t.Errorf("checker should return false for plain error")
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		err      error
		contains string
	}{
		{&ErrCASConflict{Expected: "v1", Actual: "v2"}, "CAS conflict"},
		{&ErrCASConflict{Path: "db/host", Expected: "v1", Actual: "v2"}, "db/host"},
		{&ErrAuthRequired{}, "authentication required"},
		{&ErrAuthRequired{Reason: "token expired"}, "token expired"},
		{&ErrAccessDenied{Action: "write"}, "access denied"},
		{&ErrAccessDenied{Path: "db/host", Action: "read"}, "db/host"},
		{&ErrPathNotFound{TreeID: "prod", Path: "db/host"}, "db/host"},
		{&ErrEncryptionNotInitialized{}, "encryption not initialized"},
		{&ErrNotSupported{Feature: "versioning"}, "versioning not supported"},
	}
	for _, tt := range tests {
		msg := tt.err.Error()
		if msg == "" {
			t.Errorf("error message should not be empty for %T", tt.err)
		}
		found := false
		for _, sub := range []string{tt.contains} {
			if containsSubstring(msg, sub) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("error %q should contain %q", msg, tt.contains)
		}
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Error round-trip through gRPC ---

func TestNotSupportedErrorRoundTrip(t *testing.T) {
	// The versionedValuePlugin returns ErrNotSupported for tree versioning.
	impl := newVersionedValuePlugin()
	p := dispense(t, impl)
	ctx := context.Background()

	_, err := p.ListTreeVersions(ctx, "any")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotSupported(err) {
		t.Errorf("expected ErrNotSupported after gRPC round-trip, got %T: %v", err, err)
	}
}

func TestCASConflictErrorRoundTrip(t *testing.T) {
	impl := newCASPlugin()
	p := dispense(t, impl)
	ctx := context.Background()

	// Write initial value.
	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"app/key": {Val: "val"},
	}, nil)

	// CAS conflict.
	err := p.PutValues(ctx, "app", map[string]config.Value{
		"app/key": {Val: "val2"},
	}, &PutOptions{CASVersion: "wrong"})
	if err == nil {
		t.Fatal("expected CAS conflict error")
	}
	if !IsCASConflict(err) {
		t.Errorf("expected ErrCASConflict after gRPC round-trip, got %T: %v", err, err)
	}
}
