package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// --- testPlugin: in-memory store with versioning, no encryption -----------

type testPlugin struct {
	mu    sync.RWMutex
	trees map[string][]*config.Tree // id -> versions (newest last)
}

func newTestPlugin() *testPlugin {
	return &testPlugin{trees: make(map[string][]*config.Tree)}
}

func (p *testPlugin) Save(_ context.Context, id string, tree config.TreeReader) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := config.NewTree()
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if !ok {
			continue
		}
		_ = cp.Set(path, &v)
	}
	p.trees[id] = append(p.trees[id], cp)
	return nil
}

func (p *testPlugin) Load(_ context.Context, id string) (*config.Tree, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	versions, ok := p.trees[id]
	if !ok || len(versions) == 0 {
		return nil, false, nil
	}
	return versions[len(versions)-1], true, nil
}

func (p *testPlugin) Delete(_ context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.trees, id)
	return nil
}

func (p *testPlugin) ListTrees(_ context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]string, 0, len(p.trees))
	for id := range p.trees {
		ids = append(ids, id)
	}
	return ids, nil
}

func (p *testPlugin) SupportsVersioning(_ context.Context) (bool, error) {
	return true, nil
}

func (p *testPlugin) ListVersions(_ context.Context, id string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	versions, ok := p.trees[id]
	if !ok {
		return nil, nil
	}
	// Newest first: versions slice stores oldest first, so iterate in reverse.
	result := make([]string, len(versions))
	for i := range versions {
		result[len(versions)-1-i] = fmt.Sprintf("v%d", i+1)
	}
	return result, nil
}

func (p *testPlugin) LoadVersion(_ context.Context, id string, version string) (*config.Tree, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	versions, ok := p.trees[id]
	if !ok {
		return nil, false, nil
	}
	for i := range versions {
		label := fmt.Sprintf("v%d", i+1)
		if label == version {
			return versions[i], true, nil
		}
	}
	return nil, false, nil
}

func (p *testPlugin) DeleteVersion(_ context.Context, id string, version string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	versions, ok := p.trees[id]
	if !ok {
		return errors.New("tree not found")
	}
	for i := range versions {
		label := fmt.Sprintf("v%d", i+1)
		if label == version {
			p.trees[id] = slices.Delete(versions, i, i+1)
			if len(p.trees[id]) == 0 {
				delete(p.trees, id)
			}
			return nil
		}
	}
	return errors.New("version not found")
}

func (p *testPlugin) EncryptionStatus(_ context.Context) (EncryptionStatus, error) {
	return EncryptionNone, nil
}

func (p *testPlugin) InitEncryption(_ context.Context, _ []byte) error {
	return errors.New("encryption not supported")
}

func (p *testPlugin) RotateEncryption(_ context.Context, _, _ []byte) error {
	return errors.New("encryption not supported")
}

// --- dispense helper ------------------------------------------------------

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

// --- tests ----------------------------------------------------------------

func TestEncryptionStatusString(t *testing.T) {
	tests := []struct {
		s    EncryptionStatus
		want string
	}{
		{EncryptionNone, "none"},
		{EncryptionSupported, "supported"},
		{EncryptionActive, "active"},
		{EncryptionStatus(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("EncryptionStatus(%d).String() = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost", Metadata: map[string]any{"desc": "host"}})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})

	if err := p.Save(ctx, "production", tree); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, found, err := p.Load(ctx, "production")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Fatal("Load: not found")
	}

	v, ok := loaded.Get("db/host")
	if !ok {
		t.Fatal("db/host missing from loaded tree")
	}
	if v.Val != "localhost" {
		t.Errorf("db/host = %v, want %q", v.Val, "localhost")
	}
	if v.Metadata["desc"] != "host" {
		t.Errorf("metadata[desc] = %v, want %q", v.Metadata["desc"], "host")
	}

	v, ok = loaded.Get("db/port")
	if !ok {
		t.Fatal("db/port missing from loaded tree")
	}
	if v.Val != float64(5432) {
		t.Errorf("db/port = %v, want 5432", v.Val)
	}
}

func TestLoadMissing(t *testing.T) {
	p := dispense(t, newTestPlugin())
	_, found, err := p.Load(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Error("Load returned found=true for missing tree")
	}
}

func TestDeleteTree(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("a/b", &config.Value{Val: "x"})
	_ = p.Save(ctx, "temp", tree)

	if err := p.Delete(ctx, "temp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, found, _ := p.Load(ctx, "temp")
	if found {
		t.Error("tree still found after Delete")
	}
}

func TestListTrees(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("a/b", &config.Value{Val: "x"})

	_ = p.Save(ctx, "alpha", tree)
	_ = p.Save(ctx, "beta", tree)

	ids, err := p.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	slices.Sort(ids)
	want := []string{"alpha", "beta"}
	if !slices.Equal(ids, want) {
		t.Errorf("ListTrees() = %v, want %v", ids, want)
	}
}

func TestSupportsVersioning(t *testing.T) {
	p := dispense(t, newTestPlugin())
	supported, err := p.SupportsVersioning(context.Background())
	if err != nil {
		t.Fatalf("SupportsVersioning: %v", err)
	}
	if !supported {
		t.Error("SupportsVersioning() = false, want true")
	}
}

func TestVersioning(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	// Save two versions.
	tree1 := config.NewTree()
	_ = tree1.Set("app/version", &config.Value{Val: "1.0"})
	_ = p.Save(ctx, "myapp", tree1)

	tree2 := config.NewTree()
	_ = tree2.Set("app/version", &config.Value{Val: "2.0"})
	_ = p.Save(ctx, "myapp", tree2)

	// Latest should be v2.
	latest, found, err := p.Load(ctx, "myapp")
	if err != nil || !found {
		t.Fatalf("Load latest: err=%v, found=%v", err, found)
	}
	v, _ := latest.Get("app/version")
	if v.Val != "2.0" {
		t.Errorf("latest version = %v, want %q", v.Val, "2.0")
	}

	// List versions.
	versions, err := p.ListVersions(ctx, "myapp")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	// Newest first.
	if versions[0] != "v2" || versions[1] != "v1" {
		t.Errorf("versions = %v, want [v2, v1]", versions)
	}

	// Load older version.
	old, found, err := p.LoadVersion(ctx, "myapp", "v1")
	if err != nil || !found {
		t.Fatalf("LoadVersion v1: err=%v, found=%v", err, found)
	}
	v, _ = old.Get("app/version")
	if v.Val != "1.0" {
		t.Errorf("v1 version = %v, want %q", v.Val, "1.0")
	}
}

func TestLoadVersionMissing(t *testing.T) {
	p := dispense(t, newTestPlugin())
	_, found, err := p.LoadVersion(context.Background(), "nope", "v1")
	if err != nil {
		t.Fatalf("LoadVersion: %v", err)
	}
	if found {
		t.Error("LoadVersion returned found=true for missing tree")
	}
}

func TestDeleteVersion(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	// Save three versions.
	for i := 1; i <= 3; i++ {
		tree := config.NewTree()
		_ = tree.Set("app/version", &config.Value{Val: fmt.Sprintf("%d.0", i)})
		_ = p.Save(ctx, "myapp", tree)
	}

	// Delete the middle version.
	if err := p.DeleteVersion(ctx, "myapp", "v2"); err != nil {
		t.Fatalf("DeleteVersion: %v", err)
	}

	versions, err := p.ListVersions(ctx, "myapp")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions after delete, want 2", len(versions))
	}

	// Latest should still be the last saved version.
	latest, found, _ := p.Load(ctx, "myapp")
	if !found {
		t.Fatal("tree not found after DeleteVersion")
	}
	v, _ := latest.Get("app/version")
	if v.Val != "3.0" {
		t.Errorf("latest = %v, want %q", v.Val, "3.0")
	}
}

func TestDeleteVersionNonexistent(t *testing.T) {
	p := dispense(t, newTestPlugin())
	err := p.DeleteVersion(context.Background(), "nope", "v1")
	if err == nil {
		t.Error("DeleteVersion should return error for missing tree")
	}
}

func TestEncryptionStatusOverGRPC(t *testing.T) {
	p := dispense(t, newTestPlugin())
	status, err := p.EncryptionStatus(context.Background())
	if err != nil {
		t.Fatalf("EncryptionStatus: %v", err)
	}
	if status != EncryptionNone {
		t.Errorf("EncryptionStatus() = %v, want EncryptionNone", status)
	}
}

func TestInitEncryptionUnsupported(t *testing.T) {
	p := dispense(t, newTestPlugin())
	err := p.InitEncryption(context.Background(), []byte("secret"))
	if err == nil {
		t.Error("InitEncryption should return error for unsupported store")
	}
}

func TestRotateEncryptionUnsupported(t *testing.T) {
	p := dispense(t, newTestPlugin())
	err := p.RotateEncryption(context.Background(), []byte("old"), []byte("new"))
	if err == nil {
		t.Error("RotateEncryption should return error for unsupported store")
	}
}

func TestValidatorsNotStored(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("key", &config.Value{
		Val: "value",
		Validators: []config.ValidateFunc{
			func(any, config.TreeReader) []config.ValidationResult {
				return []config.ValidationResult{{Severity: config.Blocking, Message: "fail"}}
			},
		},
	})

	_ = p.Save(ctx, "test", tree)

	loaded, found, _ := p.Load(ctx, "test")
	if !found {
		t.Fatal("not found")
	}

	v, ok := loaded.Get("key")
	if !ok {
		t.Fatal("key missing")
	}
	if len(v.Validators) != 0 {
		t.Errorf("got %d validators, want 0 (validators must not be stored)", len(v.Validators))
	}
}

func TestMetadataPreserved(t *testing.T) {
	p := dispense(t, newTestPlugin())
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{
		Val:      "localhost",
		Metadata: map[string]any{"description": "Database host", "required": true},
	})

	_ = p.Save(ctx, "test", tree)

	loaded, found, _ := p.Load(ctx, "test")
	if !found {
		t.Fatal("not found")
	}

	v, ok := loaded.Get("db/host")
	if !ok {
		t.Fatal("db/host missing")
	}
	if v.Metadata["description"] != "Database host" {
		t.Errorf("description = %v, want %q", v.Metadata["description"], "Database host")
	}
	if v.Metadata["required"] != true {
		t.Errorf("required = %v, want true", v.Metadata["required"])
	}
}
