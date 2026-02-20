package integration

import (
	"os"
	"path/filepath"
	"testing"
)

// setupWorkspaceDir creates a temporary workspace directory with:
//   - zhi.yaml from the given content
//   - a config/ subdirectory with structured YAML/JSON files
//   - optional template files
//   - optional additional files via the files map
//
// Returns the absolute path to the workspace directory.
func setupWorkspaceDir(t *testing.T, zhiYAML string, configFiles map[string]string, extras map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	// Write zhi.yaml
	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte(zhiYAML), 0o644); err != nil {
		t.Fatalf("writing zhi.yaml: %v", err)
	}

	// Write config files into config/ subdirectory
	if len(configFiles) > 0 {
		configDir := filepath.Join(dir, "config")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("creating config dir: %v", err)
		}
		for name, content := range configFiles {
			p := filepath.Join(configDir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatalf("creating parent dir for %s: %v", name, err)
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatalf("writing config file %s: %v", name, err)
			}
		}
	}

	// Write extra files (templates, scripts, etc.)
	for relPath, content := range extras {
		p := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("creating parent dir for %s: %v", relPath, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("writing extra file %s: %v", relPath, err)
		}
	}

	return dir
}
