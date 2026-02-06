package metadata

import (
	"os"
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

func TestSecurityFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := &InstalledPlugin{
		Name:            "secure-plugin",
		Type:            "config",
		Version:         "2.0.0",
		Ref:             "oci://ghcr.io/zhi-project/zhi-config-secure:v2.0.0",
		Digest:          "sha256:abc123",
		Platform:        "linux/amd64",
		InstalledAt:     time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
		Publisher:       "zhi-project",
		BinaryDigest:    "sha256:def456",
		Signed:          true,
		SigningIdentity: "release@zhi.dev",
	}

	if err := s.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load("secure-plugin")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.BinaryDigest != "sha256:def456" {
		t.Errorf("BinaryDigest = %q, want %q", loaded.BinaryDigest, "sha256:def456")
	}
	if !loaded.Signed {
		t.Error("expected Signed to be true")
	}
	if loaded.SigningIdentity != "release@zhi.dev" {
		t.Errorf("SigningIdentity = %q, want %q", loaded.SigningIdentity, "release@zhi.dev")
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

func TestSetPinned(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := &InstalledPlugin{
		Name:        "pin-test",
		Type:        "config",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
	}
	if err := s.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Pin the plugin.
	if err := s.SetPinned("pin-test", true); err != nil {
		t.Fatalf("SetPinned(true): %v", err)
	}
	loaded, err := s.Load("pin-test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.Pinned {
		t.Error("expected Pinned to be true")
	}

	// Unpin the plugin.
	if err := s.SetPinned("pin-test", false); err != nil {
		t.Fatalf("SetPinned(false): %v", err)
	}
	loaded, err = s.Load("pin-test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Pinned {
		t.Error("expected Pinned to be false")
	}
}

func TestSetPinnedNotInstalled(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	err := s.SetPinned("nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestBackupAndRestoreBinary(t *testing.T) {
	pluginDir := t.TempDir()
	binaryName := "zhi-config-test"

	// Create a fake binary.
	binaryPath := filepath.Join(pluginDir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("binary-v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Back up.
	if err := BackupBinary(pluginDir, binaryName, "1.0.0"); err != nil {
		t.Fatalf("BackupBinary: %v", err)
	}

	// Original should be gone.
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		t.Error("expected original binary to be removed after backup")
	}

	// Backup should exist.
	backupPath := filepath.Join(pluginDir, ".backup", binaryName+".1.0.0")
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("expected backup at %s: %v", backupPath, err)
	}

	// Write a new version as the current binary.
	if err := os.WriteFile(binaryPath, []byte("binary-v2"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Restore backup.
	version, err := RestoreBackup(pluginDir, binaryName)
	if err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("restored version = %q, want %q", version, "1.0.0")
	}

	// Restored binary should contain the old data.
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("reading restored binary: %v", err)
	}
	if string(data) != "binary-v1" {
		t.Errorf("restored binary content = %q, want %q", string(data), "binary-v1")
	}
}

func TestBackupNonexistentBinary(t *testing.T) {
	pluginDir := t.TempDir()
	// Should not error when the binary doesn't exist.
	if err := BackupBinary(pluginDir, "nonexistent", "1.0.0"); err != nil {
		t.Errorf("BackupBinary for nonexistent: %v", err)
	}
}

func TestRestoreBackupNotFound(t *testing.T) {
	pluginDir := t.TempDir()
	_, err := RestoreBackup(pluginDir, "nonexistent")
	if err == nil {
		t.Error("expected error for missing backup")
	}
}

func TestPinnedFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := &InstalledPlugin{
		Name:        "pinned-plugin",
		Type:        "config",
		Version:     "1.0.0",
		Ref:         "oci://example.com/test:v1.0.0",
		InstalledAt: time.Now(),
		Pinned:      true,
	}

	if err := s.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load("pinned-plugin")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if !loaded.Pinned {
		t.Error("expected Pinned to be true after round-trip")
	}
}
