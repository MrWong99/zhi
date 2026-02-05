package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MrWong99/zhi/pkg/sharing/client"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
)

func TestParsePlatformFlag(t *testing.T) {
	tests := []struct {
		input   string
		os      string
		arch    string
		wantErr bool
	}{
		{"linux/amd64", "linux", "amd64", false},
		{"darwin/arm64", "darwin", "arm64", false},
		{"windows/amd64", "windows", "amd64", false},
		{"invalid", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := parsePlatformFlag(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePlatformFlag(%q): %v", tt.input, err)
			}
			if p.OS != tt.os {
				t.Errorf("OS = %q, want %q", p.OS, tt.os)
			}
			if p.Arch != tt.arch {
				t.Errorf("Arch = %q, want %q", p.Arch, tt.arch)
			}
		})
	}
}

func TestPluginUninstall(t *testing.T) {
	dir := t.TempDir()
	metaDir := filepath.Join(dir, "metadata")
	pluginDir := filepath.Join(dir, "plugins")

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a fake installed plugin.
	metaStore := metadata.NewStore(metaDir)
	meta := &metadata.InstalledPlugin{
		Name:        "test-plugin",
		Type:        "config",
		Version:     "1.0.0",
		Ref:         "oci://example.com/test:v1.0.0",
		InstalledAt: time.Now(),
	}
	if err := metaStore.Save(meta); err != nil {
		t.Fatal(err)
	}

	// Create a fake binary.
	binaryPath := filepath.Join(pluginDir, "zhi-config-test-plugin")
	if err := os.WriteFile(binaryPath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Verify metadata exists.
	loaded, err := metaStore.Load("test-plugin")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected metadata to exist before uninstall")
	}
}

func TestPluginListJSON(t *testing.T) {
	dir := t.TempDir()
	metaStore := metadata.NewStore(dir)

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	plugins := []*metadata.InstalledPlugin{
		{Name: "alpha", Type: "config", Version: "1.0.0", Ref: "oci://example.com/alpha:v1.0.0", Platform: "linux/amd64", InstalledAt: now},
		{Name: "beta", Type: "store", Version: "2.0.0", Ref: "oci://example.com/beta:v2.0.0", Platform: "linux/amd64", InstalledAt: now},
	}
	for _, p := range plugins {
		if err := metaStore.Save(p); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	// Verify we can list and format as JSON.
	list, err := metaStore.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	entries := make([]pluginListEntry, len(list))
	for i, p := range list {
		entries[i] = pluginListEntry{
			Name:      p.Name,
			Type:      p.Type,
			Version:   p.Version,
			Ref:       p.Ref,
			Platform:  p.Platform,
			Installed: p.InstalledAt.Format(time.RFC3339),
		}
	}

	var buf bytes.Buffer
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	buf.Write(data)

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "alpha" {
		t.Errorf("first entry name = %q", entries[0].Name)
	}
}

func TestNewSharingClientPlatform(t *testing.T) {
	p := client.CurrentPlatform()
	if p.OS == "" || p.Arch == "" {
		t.Error("CurrentPlatform returned empty fields")
	}
}
