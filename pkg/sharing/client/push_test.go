package client

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePlatform(t *testing.T) {
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
		{"/amd64", "", "", true},
		{"linux/", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			osName, arch, err := parsePlatform(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePlatform(%q): %v", tt.input, err)
			}
			if osName != tt.os {
				t.Errorf("OS = %q, want %q", osName, tt.os)
			}
			if arch != tt.arch {
				t.Errorf("Arch = %q, want %q", arch, tt.arch)
			}
		})
	}
}

func TestDigestOf(t *testing.T) {
	data := []byte("hello world")
	d := digestOf(data)
	if d == "" {
		t.Error("digest is empty")
	}
	// The digest should start with sha256:
	if d[:7] != "sha256:" {
		t.Errorf("digest prefix = %q, want sha256:", string(d[:7]))
	}
	// Same data should produce same digest.
	d2 := digestOf(data)
	if d != d2 {
		t.Errorf("digest mismatch: %s vs %s", d, d2)
	}
	// Different data should produce different digest.
	d3 := digestOf([]byte("different"))
	if d == d3 {
		t.Error("different data produced same digest")
	}
}

func TestCreateWorkspaceBundle(t *testing.T) {
	dir := t.TempDir()

	// Create workspace files.
	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte("version: \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "config.tmpl"), []byte("{{.key}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "apply"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "apply", "setup.sh"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file that will be gitignored.
	if err := os.WriteFile(filepath.Join(dir, "random.txt"), []byte("random"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a .gitignore that excludes random.txt.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("random.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create files that should NOT be included (hidden dirs).
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("git config"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := createWorkspaceBundle(dir)
	if err != nil {
		t.Fatalf("createWorkspaceBundle: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("bundle is empty")
	}

	// Extract and verify contents.
	files := extractTarGzEntries(t, data)
	expectedFiles := map[string]bool{
		"zhi.yaml":              true,
		"templates":             true,
		"templates/config.tmpl": true,
		"apply":                 true,
		"apply/setup.sh":        true,
	}
	for _, f := range files {
		if !expectedFiles[f] {
			t.Errorf("unexpected file in bundle: %s", f)
		}
		delete(expectedFiles, f)
	}
	for f := range expectedFiles {
		t.Errorf("expected file not in bundle: %s", f)
	}
}

func TestCreateWorkspaceBundleEmpty(t *testing.T) {
	dir := t.TempDir()

	data, err := createWorkspaceBundle(dir)
	if err != nil {
		t.Fatalf("createWorkspaceBundle: %v", err)
	}

	// Should produce a valid (but empty) tarball.
	if len(data) == 0 {
		t.Fatal("bundle is empty")
	}
}

func TestLoadGitignorePatterns(t *testing.T) {
	t.Run("with gitignore", func(t *testing.T) {
		dir := t.TempDir()
		content := "# comment\n\nrandom.txt\n/keys\n*.log\n"
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		patterns := loadGitignorePatterns(dir)
		want := []string{"random.txt", "/keys", "*.log"}
		if len(patterns) != len(want) {
			t.Fatalf("got %d patterns, want %d: %v", len(patterns), len(want), patterns)
		}
		for i, p := range patterns {
			if p != want[i] {
				t.Errorf("pattern[%d] = %q, want %q", i, p, want[i])
			}
		}
	})

	t.Run("no gitignore", func(t *testing.T) {
		dir := t.TempDir()
		patterns := loadGitignorePatterns(dir)
		if patterns != nil {
			t.Errorf("got %v, want nil", patterns)
		}
	})
}

func TestCreateWorkspaceBundleNoGitignore(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte("version: \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := createWorkspaceBundle(dir)
	if err != nil {
		t.Fatalf("createWorkspaceBundle: %v", err)
	}

	files := extractTarGzEntries(t, data)
	expected := map[string]bool{"zhi.yaml": true, "extra.txt": true}
	for _, f := range files {
		if !expected[f] {
			t.Errorf("unexpected file in bundle: %s", f)
		}
		delete(expected, f)
	}
	for f := range expected {
		t.Errorf("expected file not in bundle: %s", f)
	}
}

func TestExtractTarGz(t *testing.T) {
	// Create a tar.gz in memory.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	files := map[string]string{
		"zhi.yaml":         "version: \"1\"\n",
		"templates/a.tmpl": "{{.key}}",
	}

	// Add directory entry.
	tw.WriteHeader(&tar.Header{
		Name:     "templates/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	})

	for name, content := range files {
		tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0o644,
		})
		tw.Write([]byte(content))
	}
	tw.Close()
	gw.Close()

	targetDir := t.TempDir()
	if err := extractTarGz(buf.Bytes(), targetDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	for name, content := range files {
		data, err := os.ReadFile(filepath.Join(targetDir, name))
		if err != nil {
			t.Errorf("reading %s: %v", name, err)
			continue
		}
		if string(data) != content {
			t.Errorf("content of %s = %q, want %q", name, data, content)
		}
	}
}

func TestExtractTarGzPathTraversal(t *testing.T) {
	// Create a malicious tar.gz with path traversal.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	tw.WriteHeader(&tar.Header{
		Name: "../../../etc/passwd",
		Size: 4,
		Mode: 0o644,
	})
	tw.Write([]byte("evil"))
	tw.Close()
	gw.Close()

	targetDir := t.TempDir()
	err := extractTarGz(buf.Bytes(), targetDir)
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestMediaTypeWorkspaceConstants(t *testing.T) {
	if MediaTypeWorkspaceConfig != "application/vnd.zhi.workspace.config.v1+json" {
		t.Errorf("MediaTypeWorkspaceConfig = %q", MediaTypeWorkspaceConfig)
	}
	if MediaTypeWorkspaceBundle != "application/vnd.zhi.workspace.bundle.v1+tar.gz" {
		t.Errorf("MediaTypeWorkspaceBundle = %q", MediaTypeWorkspaceBundle)
	}
	if MediaTypeWorkspaceReadme != "application/vnd.zhi.workspace.readme.v1+markdown" {
		t.Errorf("MediaTypeWorkspaceReadme = %q", MediaTypeWorkspaceReadme)
	}
}

// extractTarGzEntries returns all file names in a tar.gz archive.
func extractTarGzEntries(t *testing.T, data []byte) []string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var names []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		names = append(names, header.Name)
	}
	return names
}
