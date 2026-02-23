package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	rollbackDir      = ".zhi-rollback"
	rollbackManifest = "manifest.json"
)

// RollbackManifest records which files were backed up before an export.
type RollbackManifest struct {
	Timestamp time.Time        `json:"timestamp"`
	Files     []RollbackEntry  `json:"files"`
}

// RollbackEntry records a single file backup.
type RollbackEntry struct {
	// OriginalPath is the absolute path to the exported file.
	OriginalPath string `json:"original_path"`
	// BackupName is the filename of the backup in the rollback directory.
	BackupName string `json:"backup_name"`
	// WasNew indicates the file did not exist before export.
	WasNew bool `json:"was_new,omitempty"`
}

// SaveRollbackSnapshot creates a backup of the files that will be overwritten
// by export. Call this before writing export results. The backups are stored
// in workspaceDir/.zhi-rollback/.
func SaveRollbackSnapshot(workspaceDir string, outputPaths []string) error {
	rbDir := filepath.Join(workspaceDir, rollbackDir)

	// Remove previous rollback (we only keep one level).
	if err := os.RemoveAll(rbDir); err != nil {
		return fmt.Errorf("removing old rollback: %w", err)
	}
	if err := os.MkdirAll(rbDir, 0o755); err != nil {
		return fmt.Errorf("creating rollback directory: %w", err)
	}

	manifest := RollbackManifest{
		Timestamp: time.Now(),
	}

	for i, path := range outputPaths {
		if path == "" || path == "-" {
			continue
		}

		entry := RollbackEntry{
			OriginalPath: path,
			BackupName:   fmt.Sprintf("backup_%d_%s", i, filepath.Base(path)),
		}

		// Check if file exists.
		if _, err := os.Stat(path); os.IsNotExist(err) {
			entry.WasNew = true
			manifest.Files = append(manifest.Files, entry)
			continue
		}

		// Copy existing file to backup.
		backupPath := filepath.Join(rbDir, entry.BackupName)
		if err := copyFile(path, backupPath); err != nil {
			return fmt.Errorf("backing up %s: %w", path, err)
		}
		manifest.Files = append(manifest.Files, entry)
	}

	// Write manifest.
	manifestPath := filepath.Join(rbDir, rollbackManifest)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	Logger().Debug("rollback snapshot saved", "dir", rbDir, "files", len(manifest.Files))
	return nil
}

// Rollback restores files from the most recent rollback snapshot.
func Rollback(workspaceDir string) (*RollbackManifest, error) {
	rbDir := filepath.Join(workspaceDir, rollbackDir)
	manifestPath := filepath.Join(rbDir, rollbackManifest)

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no rollback snapshot found (run 'zhi export' first)")
		}
		return nil, fmt.Errorf("reading rollback manifest: %w", err)
	}

	var manifest RollbackManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing rollback manifest: %w", err)
	}

	for _, entry := range manifest.Files {
		if entry.WasNew {
			// File was created by export; remove it.
			if err := os.Remove(entry.OriginalPath); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("removing new file %s: %w", entry.OriginalPath, err)
			}
			Logger().Debug("rollback: removed new file", "path", entry.OriginalPath)
			continue
		}

		// Restore from backup.
		backupPath := filepath.Join(rbDir, entry.BackupName)
		if err := copyFile(backupPath, entry.OriginalPath); err != nil {
			return nil, fmt.Errorf("restoring %s: %w", entry.OriginalPath, err)
		}
		Logger().Debug("rollback: restored file", "path", entry.OriginalPath)
	}

	// Clean up rollback directory after successful restore.
	if err := os.RemoveAll(rbDir); err != nil {
		Logger().Warn("failed to clean up rollback directory", "error", err)
	}

	return &manifest, nil
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
