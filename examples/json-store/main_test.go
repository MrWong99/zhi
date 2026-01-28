package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// newTestStore returns a jsonStore that writes to a temporary directory
// cleaned up automatically when the test finishes.
func newTestStore(t *testing.T) *jsonStore {
	t.Helper()
	dir := t.TempDir()
	return &jsonStore{dir: dir}
}

// dispense starts an in-process gRPC plugin server and returns a connected
// store.Plugin client. No subprocess is spawned. The returned dir is the
// temporary directory used by the store so tests can inspect the files.
func dispense(t *testing.T) (store.Plugin, string) {
	t.Helper()
	s := newTestStore(t)
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"store": &store.GRPCPlugin{Impl: s},
	})
	raw, err := client.Dispense("store")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(store.Plugin), s.dir
}

func TestSaveAndLoad(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{Val: "localhost"})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})

	if err := p.Save(ctx, "prod", tree); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, found, err := p.Load(ctx, "prod")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Fatal("Load: not found")
	}

	v, ok := loaded.Get("db/host")
	if !ok {
		t.Fatal("db/host missing")
	}
	if v.Val != "localhost" {
		t.Errorf("Val = %v, want %q", v.Val, "localhost")
	}
}

func TestLoadMissing(t *testing.T) {
	p, _ := dispense(t)
	_, found, err := p.Load(context.Background(), "nope")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Error("found should be false for missing tree")
	}
}

func TestDelete(t *testing.T) {
	p, _ := dispense(t)
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
	p, _ := dispense(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("a/b", &config.Value{Val: "x"})

	_ = p.Save(ctx, "one", tree)
	_ = p.Save(ctx, "two", tree)

	ids, err := p.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"one", "two"}) {
		t.Errorf("ListTrees() = %v, want [one, two]", ids)
	}
}

func TestSaveOverwrites(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	tree1 := config.NewTree()
	_ = tree1.Set("app/name", &config.Value{Val: "old"})
	_ = p.Save(ctx, "app", tree1)

	tree2 := config.NewTree()
	_ = tree2.Set("app/name", &config.Value{Val: "new"})
	_ = p.Save(ctx, "app", tree2)

	loaded, found, _ := p.Load(ctx, "app")
	if !found {
		t.Fatal("not found")
	}
	v, _ := loaded.Get("app/name")
	if v.Val != "new" {
		t.Errorf("Val = %v, want %q", v.Val, "new")
	}
}

func TestJSONFileCreatedOnDisk(t *testing.T) {
	p, dir := dispense(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("key", &config.Value{Val: "value"})
	_ = p.Save(ctx, "disk-check", tree)

	path := filepath.Join(dir, "disk-check.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if len(data) == 0 {
		t.Error("JSON file is empty")
	}
}

func TestMetadataPreserved(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("key", &config.Value{
		Val:      "value",
		Metadata: map[string]any{"description": "test key"},
	})
	_ = p.Save(ctx, "meta", tree)

	loaded, found, _ := p.Load(ctx, "meta")
	if !found {
		t.Fatal("not found")
	}
	v, _ := loaded.Get("key")
	if v.Metadata["description"] != "test key" {
		t.Errorf("description = %v, want %q", v.Metadata["description"], "test key")
	}
}

func TestVersioningNotSupported(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	supported, err := p.SupportsVersioning(ctx)
	if err != nil {
		t.Fatalf("SupportsVersioning: %v", err)
	}
	if supported {
		t.Error("json store should not support versioning")
	}

	_, err = p.ListVersions(ctx, "any")
	if err == nil {
		t.Error("ListVersions should return error")
	}

	_, _, err = p.LoadVersion(ctx, "any", "v1")
	if err == nil {
		t.Error("LoadVersion should return error")
	}

	err = p.DeleteVersion(ctx, "any", "v1")
	if err == nil {
		t.Error("DeleteVersion should return error")
	}
}

func TestEncryptionNotSupported(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	status, err := p.EncryptionStatus(ctx)
	if err != nil {
		t.Fatalf("EncryptionStatus: %v", err)
	}
	if status != store.EncryptionNone {
		t.Errorf("EncryptionStatus() = %v, want EncryptionNone", status)
	}

	if err := p.InitEncryption(ctx, []byte("pass")); err == nil {
		t.Error("InitEncryption should return error")
	}

	if err := p.RotateEncryption(ctx, []byte("old"), []byte("new")); err == nil {
		t.Error("RotateEncryption should return error")
	}
}
