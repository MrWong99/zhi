package airgap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportNoArtifacts(t *testing.T) {
	err := Export(ExportOptions{
		Output: "/tmp/test.tar",
	})
	if err == nil {
		t.Error("expected error for empty artifacts")
	}
}

func TestExportNoOutput(t *testing.T) {
	err := Export(ExportOptions{
		Artifacts: []string{"test/plugin:v1.0.0"},
	})
	if err == nil {
		t.Error("expected error for empty output")
	}
}

func TestDigestSHA256(t *testing.T) {
	dgst := digestSHA256([]byte("test"))
	want := "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if dgst != want {
		t.Errorf("digestSHA256(\"test\") = %q, want %q", dgst, want)
	}
}

func TestCreateTarball(t *testing.T) {
	dir := t.TempDir()

	// Create some content.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := createTarball(output, dir); err != nil {
		t.Fatalf("createTarball: %v", err)
	}

	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Error("tarball should be non-empty")
	}
}

func TestParseRefParts(t *testing.T) {
	tests := []struct {
		ref      string
		wantRepo string
		wantTag  string
	}{
		{"zhi-project/plugin:v1.2.0", "zhi-project/plugin", "v1.2.0"},
		{"simple/name:latest", "simple/name", "latest"},
		{"no-tag", "no-tag", ""},
	}

	for _, tt := range tests {
		repo, tag := parseRefParts(tt.ref)
		if repo != tt.wantRepo {
			t.Errorf("parseRefParts(%q) repo = %q, want %q", tt.ref, repo, tt.wantRepo)
		}
		if tag != tt.wantTag {
			t.Errorf("parseRefParts(%q) tag = %q, want %q", tt.ref, tag, tt.wantTag)
		}
	}
}
