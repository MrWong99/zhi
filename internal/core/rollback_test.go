package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveRollbackAndRestore(t *testing.T) {
	dir := t.TempDir()

	// Create original files.
	file1 := filepath.Join(dir, "docker-compose.yml")
	file2 := filepath.Join(dir, "apply.sh")
	if err := os.WriteFile(file1, []byte("original-compose"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("original-apply"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save snapshot.
	if err := SaveRollbackSnapshot(dir, []string{file1, file2}); err != nil {
		t.Fatalf("SaveRollbackSnapshot: %v", err)
	}

	// Verify snapshot directory exists.
	rbDir := filepath.Join(dir, rollbackDir)
	if _, err := os.Stat(rbDir); os.IsNotExist(err) {
		t.Fatal("rollback directory not created")
	}

	// Overwrite the files (simulating export).
	if err := os.WriteFile(file1, []byte("new-compose"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("new-apply"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Rollback.
	manifest, err := Rollback(dir)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(manifest.Files) != 2 {
		t.Errorf("expected 2 files in manifest, got %d", len(manifest.Files))
	}

	// Verify files are restored.
	data1, _ := os.ReadFile(file1)
	if string(data1) != "original-compose" {
		t.Errorf("file1 not restored: %q", string(data1))
	}
	data2, _ := os.ReadFile(file2)
	if string(data2) != "original-apply" {
		t.Errorf("file2 not restored: %q", string(data2))
	}

	// Verify rollback dir cleaned up.
	if _, err := os.Stat(rbDir); !os.IsNotExist(err) {
		t.Error("rollback directory should be removed after restore")
	}
}

func TestRollback_NewFilesRemoved(t *testing.T) {
	dir := t.TempDir()

	// Snapshot with a file that doesn't exist yet.
	newFile := filepath.Join(dir, "new-file.sh")
	if err := SaveRollbackSnapshot(dir, []string{newFile}); err != nil {
		t.Fatalf("SaveRollbackSnapshot: %v", err)
	}

	// Create the file (simulating export creating it).
	if err := os.WriteFile(newFile, []byte("created by export"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Rollback should remove the new file.
	manifest, err := Rollback(dir)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(manifest.Files) != 1 || !manifest.Files[0].WasNew {
		t.Errorf("expected 1 new file entry, got: %+v", manifest.Files)
	}

	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("new file should be removed after rollback")
	}
}

func TestRollback_NoSnapshot(t *testing.T) {
	dir := t.TempDir()
	_, err := Rollback(dir)
	if err == nil {
		t.Error("expected error when no snapshot exists")
	}
}
