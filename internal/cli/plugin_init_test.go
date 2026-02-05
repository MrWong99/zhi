package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MrWong99/zhi/pkg/sharing/manifest"
)

func TestDetectBinaries(t *testing.T) {
	dir := t.TempDir()

	// Change to temp dir for the test.
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	distDir := filepath.Join(dir, "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create fake binaries.
	files := []string{
		"zhi-config-myplugin_linux_amd64",
		"zhi-config-myplugin_darwin_arm64",
		"zhi-config-myplugin_windows_amd64",
		"unrelated-file",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(distDir, f), []byte("fake"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	result := detectBinaries("config", "myplugin")
	if len(result) != 3 {
		t.Errorf("detectBinaries = %d entries, want 3", len(result))
	}
	if _, ok := result["linux/amd64"]; !ok {
		t.Error("missing linux/amd64")
	}
	if _, ok := result["darwin/arm64"]; !ok {
		t.Error("missing darwin/arm64")
	}
	if _, ok := result["windows/amd64"]; !ok {
		t.Error("missing windows/amd64")
	}
}

func TestDetectBinariesNoDist(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	result := detectBinaries("config", "myplugin")
	if result != nil {
		t.Errorf("expected nil for missing dist/, got %v", result)
	}
}

func TestPluginInitManifestCreation(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Simulate what runPluginInit does.
	m := &manifest.PluginManifest{
		SchemaVersion:      manifest.SchemaVersion,
		Name:               "test-plugin",
		Type:               "config",
		Version:            "1.0.0",
		ZhiProtocolVersion: "1",
		Description:        "A test plugin",
		Author:             "testorg",
		License:            "MIT",
	}

	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	path := filepath.Join(dir, "zhi-plugin.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify we can read it back.
	loaded, err := manifest.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if loaded.Name != "test-plugin" {
		t.Errorf("Name = %q", loaded.Name)
	}
	if loaded.Type != "config" {
		t.Errorf("Type = %q", loaded.Type)
	}
}
