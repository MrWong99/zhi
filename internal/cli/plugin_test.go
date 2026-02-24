package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestPluginListJSONWithSigning(t *testing.T) {
	dir := t.TempDir()
	metaStore := metadata.NewStore(dir)

	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	plugins := []*metadata.InstalledPlugin{
		{
			Name:            "signed-plugin",
			Type:            "config",
			Version:         "1.0.0",
			Ref:             "oci://ghcr.io/org/signed:v1.0.0",
			Platform:        "linux/amd64",
			InstalledAt:     now,
			Signed:          true,
			SigningIdentity: "release@zhi.dev",
		},
		{
			Name:        "unsigned-plugin",
			Type:        "store",
			Version:     "2.0.0",
			Ref:         "oci://ghcr.io/org/unsigned:v2.0.0",
			Platform:    "linux/amd64",
			InstalledAt: now,
		},
	}
	for _, p := range plugins {
		if err := metaStore.Save(p); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	list, err := metaStore.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	entries := make([]pluginListEntry, len(list))
	for i, p := range list {
		entries[i] = pluginListEntry{
			Name:            p.Name,
			Type:            p.Type,
			Version:         p.Version,
			Ref:             p.Ref,
			Platform:        p.Platform,
			Installed:       p.InstalledAt.Format(time.RFC3339),
			Signed:          p.Signed,
			SigningIdentity: p.SigningIdentity,
		}
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Entries are sorted by name.
	if entries[0].Name != "signed-plugin" {
		t.Errorf("first entry name = %q, want signed-plugin", entries[0].Name)
	}
	if !entries[0].Signed {
		t.Error("expected signed-plugin to have Signed=true")
	}
	if entries[0].SigningIdentity != "release@zhi.dev" {
		t.Errorf("SigningIdentity = %q, want release@zhi.dev", entries[0].SigningIdentity)
	}
	if entries[1].Signed {
		t.Error("expected unsigned-plugin to have Signed=false")
	}
}

func TestPluginInfoOutput(t *testing.T) {
	dir := t.TempDir()
	metaStore := metadata.NewStore(dir)

	p := &metadata.InstalledPlugin{
		Name:            "test-info",
		Type:            "config",
		Version:         "1.0.0",
		Ref:             "oci://ghcr.io/org/test:v1.0.0",
		Digest:          "sha256:abc123",
		BinaryDigest:    "sha256:def456",
		Platform:        "linux/amd64",
		Publisher:       "test-org",
		Signed:          true,
		SigningIdentity: "ci@test.org",
		InstalledAt:     time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := metaStore.Save(p); err != nil {
		t.Fatal(err)
	}

	loaded, err := metaStore.Load("test-info")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Verify the info output struct can be populated.
	out := pluginInfoOutput{
		Name:            loaded.Name,
		Type:            loaded.Type,
		Version:         loaded.Version,
		Ref:             loaded.Ref,
		Digest:          loaded.Digest,
		BinaryDigest:    loaded.BinaryDigest,
		Platform:        loaded.Platform,
		Publisher:       loaded.Publisher,
		Signed:          loaded.Signed,
		SigningIdentity: loaded.SigningIdentity,
		InstalledAt:     loaded.InstalledAt.Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}
	if out.BinaryDigest != "sha256:def456" {
		t.Errorf("BinaryDigest = %q, want sha256:def456", out.BinaryDigest)
	}
}

func TestRunPluginInstallLocal(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a fake binary.
	binDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(binDir, "my-plugin")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a manifest next to the binary.
	manifestContent := `schemaVersion: "1"
name: my-plugin
type: store
version: 1.0.0
author: test-author
`
	if err := os.WriteFile(filepath.Join(binDir, "zhi-plugin.yaml"), []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override core.DefaultPluginDir and metadata dir for testing.
	// We test via the internal function directly.
	t.Setenv("HOME", dir)

	// Ensure plugin dir exists at the expected path.
	zhiPluginDir := filepath.Join(dir, ".zhi", "plugins")
	zhiMetaDir := filepath.Join(dir, ".zhi", "metadata")
	if err := os.MkdirAll(zhiPluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Run the install command.
	cmd := pluginInstallCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--path", binaryPath})

	// Reset flags.
	pluginInstallForce = false
	pluginInstallPath = binaryPath

	err := runPluginInstallLocal(cmd, binaryPath)
	if err != nil {
		t.Fatalf("runPluginInstallLocal: %v", err)
	}

	// Verify binary was copied.
	destPath := filepath.Join(zhiPluginDir, "zhi-store-my-plugin")
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected binary at %s: %v", destPath, err)
	}

	// Verify binary content.
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "#!/bin/sh\necho hello" {
		t.Errorf("unexpected binary content: %q", content)
	}

	// Verify metadata was saved.
	metaStore := metadata.NewStore(zhiMetaDir)
	meta, err := metaStore.Load("my-plugin")
	if err != nil {
		t.Fatalf("Load metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata to exist")
	}
	if meta.Name != "my-plugin" {
		t.Errorf("Name = %q, want my-plugin", meta.Name)
	}
	if meta.Type != "store" {
		t.Errorf("Type = %q, want store", meta.Type)
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", meta.Version)
	}
	if meta.Publisher != "test-author" {
		t.Errorf("Publisher = %q, want test-author", meta.Publisher)
	}
	if !strings.HasPrefix(meta.Ref, "local://") {
		t.Errorf("Ref = %q, want local:// prefix", meta.Ref)
	}
	if meta.BinaryDigest == "" {
		t.Error("expected non-empty BinaryDigest")
	}

	// Cleanup flag state.
	pluginInstallPath = ""
}

func TestRunPluginInstallLocalErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Ensure plugin dir exists.
	if err := os.MkdirAll(filepath.Join(dir, ".zhi", "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := pluginInstallCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	t.Run("binary not found", func(t *testing.T) {
		err := runPluginInstallLocal(cmd, filepath.Join(dir, "nonexistent"))
		if err == nil || !strings.Contains(err.Error(), "binary not found") {
			t.Errorf("expected 'binary not found' error, got: %v", err)
		}
	})

	t.Run("path is directory", func(t *testing.T) {
		err := runPluginInstallLocal(cmd, dir)
		if err == nil || !strings.Contains(err.Error(), "directory") {
			t.Errorf("expected 'directory' error, got: %v", err)
		}
	})

	t.Run("not executable", func(t *testing.T) {
		nonExec := filepath.Join(dir, "nonexec")
		os.WriteFile(nonExec, []byte("data"), 0o644)
		err := runPluginInstallLocal(cmd, nonExec)
		if err == nil || !strings.Contains(err.Error(), "not executable") {
			t.Errorf("expected 'not executable' error, got: %v", err)
		}
	})

	t.Run("missing manifest", func(t *testing.T) {
		binDir := filepath.Join(dir, "noManifest")
		os.MkdirAll(binDir, 0o755)
		bin := filepath.Join(binDir, "plugin")
		os.WriteFile(bin, []byte("data"), 0o755)
		err := runPluginInstallLocal(cmd, bin)
		if err == nil || !strings.Contains(err.Error(), "manifest") {
			t.Errorf("expected manifest error, got: %v", err)
		}
	})

	t.Run("already exists without force", func(t *testing.T) {
		binDir := filepath.Join(dir, "exists")
		os.MkdirAll(binDir, 0o755)
		bin := filepath.Join(binDir, "plugin")
		os.WriteFile(bin, []byte("data"), 0o755)
		os.WriteFile(filepath.Join(binDir, "zhi-plugin.yaml"), []byte("schemaVersion: \"1\"\nname: existing\ntype: config\nversion: 1.0.0\n"), 0o644)

		// Pre-create the destination binary.
		dest := filepath.Join(dir, ".zhi", "plugins", "zhi-config-existing")
		os.WriteFile(dest, []byte("old"), 0o755)

		pluginInstallForce = false
		err := runPluginInstallLocal(cmd, bin)
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}
	})
}

func TestRunPluginInstallLocalScheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	binDir := filepath.Join(dir, "src")
	os.MkdirAll(binDir, 0o755)

	bin := filepath.Join(binDir, "my-plugin")
	os.WriteFile(bin, []byte("binary"), 0o755)
	os.WriteFile(filepath.Join(binDir, "zhi-plugin.yaml"), []byte("schemaVersion: \"1\"\nname: scheme-test\ntype: config\nversion: 0.1.0\n"), 0o644)

	os.MkdirAll(filepath.Join(dir, ".zhi", "plugins"), 0o755)

	cmd := pluginInstallCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test local:// scheme parsing.
	ref := "local://" + bin
	localPath := strings.TrimPrefix(ref, "local://")
	if localPath != bin {
		t.Fatalf("expected %q, got %q", bin, localPath)
	}

	err := runPluginInstallLocal(cmd, localPath)
	if err != nil {
		t.Fatalf("runPluginInstallLocal: %v", err)
	}

	// Verify installed.
	destPath := filepath.Join(dir, ".zhi", "plugins", "zhi-config-scheme-test")
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected binary at %s: %v", destPath, err)
	}
}

func TestNewSharingClientPlatform(t *testing.T) {
	p := client.CurrentPlatform()
	if p.OS == "" || p.Arch == "" {
		t.Error("CurrentPlatform returned empty fields")
	}
}
