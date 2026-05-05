package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrWong99/zhi/pkg/sharing/lockfile"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
)

func TestCheckExternalToolMultiWordName(t *testing.T) {
	// "docker compose" must look up "docker" on PATH, not the literal
	// "docker compose" string. We exercise both branches by picking a
	// binary that almost certainly exists ("sh") and one that does not.
	tests := []struct {
		name     string
		tool     manifest.ToolRequirement
		wantSym  string
		wantWord string
	}{
		{
			name:     "single-word existing binary",
			tool:     manifest.ToolRequirement{Name: "sh"},
			wantSym:  "+",
			wantWord: "found",
		},
		{
			name:     "multi-word resolves to first segment",
			tool:     manifest.ToolRequirement{Name: "sh subcommand"},
			wantSym:  "+",
			wantWord: "found",
		},
		{
			name:     "missing multi-word binary",
			tool:     manifest.ToolRequirement{Name: "definitely-not-a-real-tool subcmd", Version: "1.0"},
			wantSym:  "!",
			wantWord: "not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			checkExternalTool(&buf, tt.tool)
			out := buf.String()
			if !strings.Contains(out, tt.wantSym) {
				t.Errorf("output %q missing symbol %q", out, tt.wantSym)
			}
			if !strings.Contains(out, tt.wantWord) {
				t.Errorf("output %q missing %q", out, tt.wantWord)
			}
			if !strings.Contains(out, tt.tool.Name) {
				t.Errorf("output %q should mention tool name %q", out, tt.tool.Name)
			}
			if tt.tool.Version != "" && !strings.Contains(out, tt.tool.Version) {
				t.Errorf("output %q should mention required version %q", out, tt.tool.Version)
			}
		})
	}
}

func TestStripTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"oci://ghcr.io/org/plugin:v1.0.0", "ghcr.io/org/plugin"},
		{"ghcr.io/org/plugin:v1.0.0", "ghcr.io/org/plugin"},
		{"ghcr.io/org/plugin", "ghcr.io/org/plugin"},
		{"oci://ghcr.io/org/plugin", "ghcr.io/org/plugin"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripTag(tt.input)
			if got != tt.want {
				t.Errorf("stripTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWorkspaceLockFilePath(t *testing.T) {
	got := workspaceLockFilePath("/home/user/workspace")
	want := filepath.Join("/home/user/workspace", "zhi-plugins.lock")
	if got != want {
		t.Errorf("workspaceLockFilePath = %q, want %q", got, want)
	}
}

func TestWorkspaceLockGeneration(t *testing.T) {
	dir := t.TempDir()
	metaDir := filepath.Join(dir, "metadata")
	metaStore := metadata.NewStore(metaDir)

	now := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	plugins := []*metadata.InstalledPlugin{
		{
			Name:        "alpha",
			Type:        "config",
			Version:     "1.0.0",
			Ref:         "oci://ghcr.io/org/zhi-config-alpha:v1.0.0",
			Digest:      "sha256:abc123",
			Platform:    "linux/amd64",
			InstalledAt: now,
		},
		{
			Name:        "beta",
			Type:        "store",
			Version:     "2.0.0",
			Ref:         "oci://ghcr.io/org/zhi-store-beta:v2.0.0",
			Digest:      "sha256:def456",
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

	lf := &lockfile.LockFile{
		Version:     lockfile.Version,
		GeneratedAt: now,
		ZhiVersion:  "0.8.0",
	}

	for _, p := range list {
		entry := lockfile.LockedPlugin{
			Name:   p.Name,
			Type:   p.Type,
			Ref:    p.Ref,
			Digest: p.Digest,
		}
		if p.Platform != "" {
			entry.Platforms = map[string]string{
				p.Platform: p.Digest,
			}
		}
		lf.Plugins = append(lf.Plugins, entry)
	}

	lockPath := filepath.Join(dir, "zhi-plugins.lock")
	if err := lf.SaveFile(lockPath); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	// Verify we can load it back.
	loaded, err := lockfile.LoadFile(lockPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(loaded.Plugins) != 2 {
		t.Errorf("Plugins = %d, want 2", len(loaded.Plugins))
	}
	if loaded.Plugins[0].Name != "alpha" {
		t.Errorf("Plugins[0].Name = %q", loaded.Plugins[0].Name)
	}
}

func TestLoadWorkspaceMetadataNotFound(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	_, err := loadWorkspaceMetadata()
	if err == nil {
		t.Error("expected error when no workspace files exist")
	}
}

func TestLoadWorkspaceMetadataNoManifest(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Create only zhi.yaml (no zhi-workspace.yaml).
	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte("version: \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadWorkspaceMetadata()
	if err == nil {
		t.Error("expected error when zhi-workspace.yaml is missing")
	}
}
