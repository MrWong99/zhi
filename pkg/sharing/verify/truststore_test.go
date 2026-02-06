package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListKeysEmpty(t *testing.T) {
	dir := t.TempDir()
	ts := NewTrustStore(dir)
	keys, err := ts.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestListKeysWithFiles(t *testing.T) {
	dir := t.TempDir()

	// Create top-level key.
	if err := os.WriteFile(filepath.Join(dir, "zhi-project.pub"), []byte("key1"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Create custom key.
	customDir := filepath.Join(dir, "custom")
	if err := os.MkdirAll(customDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "myorg.pub"), []byte("key2"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Create non-.pub file (should be ignored).
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}

	ts := NewTrustStore(dir)
	keys, err := ts.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	// Keys should be sorted by name.
	if keys[0].Name != "myorg" {
		t.Errorf("first key = %q, want 'myorg'", keys[0].Name)
	}
	if !keys[0].Custom {
		t.Error("myorg key should be custom")
	}
	if keys[1].Name != "zhi-project" {
		t.Errorf("second key = %q, want 'zhi-project'", keys[1].Name)
	}
	if keys[1].Custom {
		t.Error("zhi-project key should not be custom")
	}
}

func TestGetKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.pub"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	ts := NewTrustStore(dir)

	key, err := ts.GetKey("test")
	if err != nil {
		t.Fatalf("GetKey: %v", err)
	}
	if key.Name != "test" {
		t.Errorf("key.Name = %q, want 'test'", key.Name)
	}
}

func TestGetKeyNotFound(t *testing.T) {
	dir := t.TempDir()
	ts := NewTrustStore(dir)

	_, err := ts.GetKey("nonexistent")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestAddKey(t *testing.T) {
	dir := t.TempDir()
	ts := NewTrustStore(dir)

	if err := ts.AddKey("myorg", []byte("public-key-data")); err != nil {
		t.Fatalf("AddKey: %v", err)
	}

	// Verify it's in custom/.
	key, err := ts.GetKey("myorg")
	if err != nil {
		t.Fatalf("GetKey: %v", err)
	}
	if !key.Custom {
		t.Error("added key should be custom")
	}

	// Verify file contents.
	data, err := os.ReadFile(key.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "public-key-data" {
		t.Errorf("key data = %q, want 'public-key-data'", data)
	}
}

func TestRemoveKey(t *testing.T) {
	dir := t.TempDir()
	ts := NewTrustStore(dir)

	if err := ts.AddKey("myorg", []byte("key")); err != nil {
		t.Fatal(err)
	}

	if err := ts.RemoveKey("myorg"); err != nil {
		t.Fatalf("RemoveKey: %v", err)
	}

	_, err := ts.GetKey("myorg")
	if err == nil {
		t.Error("expected error after removing key")
	}
}

func TestRemoveKeyNotFound(t *testing.T) {
	dir := t.TempDir()
	ts := NewTrustStore(dir)

	if err := ts.RemoveKey("nonexistent"); err == nil {
		t.Error("expected error for removing nonexistent key")
	}
}

func TestDefaultTrustStoreDir(t *testing.T) {
	dir := DefaultTrustStoreDir()
	if dir == "" {
		t.Skip("cannot determine home directory")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected absolute path, got %q", dir)
	}
}
