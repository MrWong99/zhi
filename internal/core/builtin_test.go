package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestNewJSONFileStoreProvider(t *testing.T) {
	dir := t.TempDir()

	sp, err := NewJSONFileStoreProvider(dir, nil)
	if err != nil {
		t.Fatalf("NewJSONFileStoreProvider: %v", err)
	}

	ctx := context.Background()

	// Should be able to put and get values.
	vals := map[string]config.Value{
		"database/host": {Val: "localhost"},
		"database/port": {Val: 5432},
	}
	if err := sp.PutValues(ctx, "test-tree", vals, nil); err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := sp.GetValues(ctx, "test-tree", []string{"database/host", "database/port"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %d", len(got))
	}
	if got["database/host"].Val != "localhost" {
		t.Errorf("expected localhost, got %v", got["database/host"].Val)
	}

	// Verify the JSON file was created in .zhi/store/.
	storeDir := filepath.Join(dir, DefaultStoreDir)
	if _, err := os.Stat(filepath.Join(storeDir, "test-tree.json")); err != nil {
		t.Errorf("expected store file to exist: %v", err)
	}
}

func TestNewJSONFileStoreProvider_CustomDirectory(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "custom-store")

	sp, err := NewJSONFileStoreProvider(dir, map[string]any{
		"directory": customDir,
	})
	if err != nil {
		t.Fatalf("NewJSONFileStoreProvider: %v", err)
	}

	ctx := context.Background()
	vals := map[string]config.Value{"test/key": {Val: "value"}}
	if err := sp.PutValues(ctx, "tree1", vals, nil); err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	if _, err := os.Stat(filepath.Join(customDir, "tree1.json")); err != nil {
		t.Errorf("expected store file in custom dir: %v", err)
	}
}

func TestNewJSONFileStoreProvider_InvalidOption(t *testing.T) {
	_, err := NewJSONFileStoreProvider("", map[string]any{
		"directory": 123,
	})
	if err == nil {
		t.Error("expected error for non-string directory option")
	}
}

func TestEngineFallbackStore(t *testing.T) {
	// When workspace.Dir is set but no store provider is configured,
	// the engine should create a fallback jsonfile store.
	dir := t.TempDir()

	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Dir:     dir,
		Config:  ProviderRef{Provider: "mock"},
		// No Store configured.
	}

	eng, err := NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx := context.Background()

	// Store should work (fallback is active).
	tree := config.NewTree()
	v := config.Value{Val: "test-value"}
	if err := tree.Set("test/key", &v); err != nil {
		t.Fatalf("tree.Set: %v", err)
	}

	if err := eng.SaveTree(ctx, tree); err != nil {
		t.Fatalf("SaveTree should work with fallback store: %v", err)
	}

	// Verify we can load it back.
	trees, err := eng.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	if len(trees) == 0 {
		t.Error("expected at least one tree after save")
	}
}

func TestDefaultRegistryHasJSONFileStore(t *testing.T) {
	reg := DefaultRegistry()

	// The jsonfile store should be registered.
	dir := t.TempDir()
	sp, err := reg.StoreProvider(dir, "jsonfile", nil)
	if err != nil {
		t.Fatalf("StoreProvider(jsonfile): %v", err)
	}
	if sp == nil {
		t.Error("expected non-nil store plugin")
	}
}
