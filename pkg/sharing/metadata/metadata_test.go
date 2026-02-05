package metadata

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := &InstalledPlugin{
		Name:        "ansible-config",
		Type:        "config",
		Version:     "1.2.0",
		Ref:         "oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
		Digest:      "sha256:abc123",
		Platform:    "linux/amd64",
		InstalledAt: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Publisher:   "zhi-project",
	}

	if err := s.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load("ansible-config")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.Name != p.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, p.Name)
	}
	if loaded.Version != p.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, p.Version)
	}
	if loaded.Ref != p.Ref {
		t.Errorf("Ref = %q, want %q", loaded.Ref, p.Ref)
	}
	if loaded.Digest != p.Digest {
		t.Errorf("Digest = %q, want %q", loaded.Digest, p.Digest)
	}
	if loaded.Platform != p.Platform {
		t.Errorf("Platform = %q, want %q", loaded.Platform, p.Platform)
	}
	if loaded.Publisher != p.Publisher {
		t.Errorf("Publisher = %q, want %q", loaded.Publisher, p.Publisher)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p, err := s.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil for nonexistent plugin, got %+v", p)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := &InstalledPlugin{
		Name:        "test-plugin",
		Type:        "store",
		Version:     "0.1.0",
		Ref:         "oci://example.com/test:v0.1.0",
		InstalledAt: time.Now(),
	}

	if err := s.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.Delete("test-plugin"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	loaded, err := s.Load("test-plugin")
	if err != nil {
		t.Fatalf("Load after delete: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	// Should not error for nonexistent.
	if err := s.Delete("nonexistent"); err != nil {
		t.Errorf("Delete nonexistent: %v", err)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	now := time.Now()
	plugins := []*InstalledPlugin{
		{Name: "beta", Type: "config", Version: "1.0.0", InstalledAt: now},
		{Name: "alpha", Type: "store", Version: "2.0.0", InstalledAt: now},
		{Name: "gamma", Type: "transform", Version: "0.5.0", InstalledAt: now},
	}

	for _, p := range plugins {
		if err := s.Save(p); err != nil {
			t.Fatalf("Save %s: %v", p.Name, err)
		}
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(list))
	}

	// Should be sorted by name.
	if list[0].Name != "alpha" {
		t.Errorf("first = %q, want %q", list[0].Name, "alpha")
	}
	if list[1].Name != "beta" {
		t.Errorf("second = %q, want %q", list[1].Name, "beta")
	}
	if list[2].Name != "gamma" {
		t.Errorf("third = %q, want %q", list[2].Name, "gamma")
	}
}

func TestListEmpty(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestListNonexistentDir(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nonexistent"))

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list != nil {
		t.Errorf("expected nil list, got %v", list)
	}
}

func TestDefaultMetadataDir(t *testing.T) {
	dir := DefaultMetadataDir()
	if dir == "" {
		t.Skip("cannot determine home directory")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DefaultMetadataDir() = %q, want absolute path", dir)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "metadata")
	s := NewStore(dir)

	p := &InstalledPlugin{
		Name:        "test",
		Type:        "config",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
	}

	if err := s.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load("test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil after save to nested directory")
	}
}
