package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffExport_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	result, err := DiffExport(path, "line1\nline2\n")
	if err != nil {
		t.Fatalf("DiffExport: %v", err)
	}
	if !result.HasChanges {
		t.Error("expected HasChanges=true for new file")
	}
	if !result.IsNew {
		t.Error("expected IsNew=true")
	}
	if !strings.Contains(result.Diff, "+line1") {
		t.Errorf("diff should show additions: %s", result.Diff)
	}
}

func TestDiffExport_NoChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "same.txt")
	content := "line1\nline2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := DiffExport(path, content)
	if err != nil {
		t.Fatalf("DiffExport: %v", err)
	}
	if result.HasChanges {
		t.Error("expected HasChanges=false for identical content")
	}
}

func TestDiffExport_WithChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "changed.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := DiffExport(path, "line1\nmodified\nline3\n")
	if err != nil {
		t.Fatalf("DiffExport: %v", err)
	}
	if !result.HasChanges {
		t.Error("expected HasChanges=true")
	}
	if !strings.Contains(result.Diff, "-line2") {
		t.Errorf("diff should show removal of line2: %s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+modified") {
		t.Errorf("diff should show addition of modified: %s", result.Diff)
	}
}
