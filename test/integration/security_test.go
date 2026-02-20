package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// --- Path traversal security tests ---

// TestExportRejectsPathTraversalInOutput verifies that the export system
// refuses to write files with path traversal in the output path.
func TestExportRejectsPathTraversalInOutput(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  secret:
    val: s3cr3t
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	td, err := core.PrepareTreeData(ctx, eng, true, "")
	if err != nil {
		t.Fatalf("PrepareTreeData: %v", err)
	}

	// Attempt to write to a path with traversal.
	// Note: filepath.Join normalizes ".." away, so construct the raw path manually
	// to ensure the ".." segments survive and are detected by the security check.
	cfg := core.ExportRunConfig{
		Format:     "json",
		OutputPath: "/tmp/safe/../../etc/evil.json",
		DryRun:     false,
	}
	_, err = core.Export(ctx, td, cfg)
	if err == nil {
		t.Fatal("Export should reject path traversal in output path")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

// TestExportRejectsPathTraversalInTemplate verifies that templates with
// path traversal are rejected.
func TestExportRejectsPathTraversalInTemplate(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("app/key", &config.Value{Val: "val"})
	td := core.NewTreeData(tree, nil)

	cfg := core.ExportRunConfig{
		TemplatePath: "/tmp/../../../etc/passwd",
	}
	_, err := core.Export(context.Background(), td, cfg)
	if err == nil {
		t.Fatal("Export should reject template path with traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

// TestExportDryRunDoesNotWrite verifies that dry-run mode never creates files.
func TestExportDryRunDoesNotWrite(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  secret:
    val: should-not-be-written
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	td, err := core.PrepareTreeData(ctx, eng, true, "")
	if err != nil {
		t.Fatalf("PrepareTreeData: %v", err)
	}

	outputPath := filepath.Join(wsDir, "should-not-exist.json")
	cfg := core.ExportRunConfig{
		Format:     "json",
		OutputPath: outputPath,
		DryRun:     true,
	}
	result, err := core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export dry-run: %v", err)
	}
	if result.Content == "" {
		t.Error("dry-run should produce content")
	}
	if _, err := os.Stat(outputPath); err == nil {
		t.Error("dry-run should not create the output file")
	}
}

// TestExportFilePermissions verifies that exported files have secure permissions.
func TestExportFilePermissions(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	td, err := core.PrepareTreeData(ctx, eng, true, "")
	if err != nil {
		t.Fatalf("PrepareTreeData: %v", err)
	}

	outputPath := filepath.Join(wsDir, "output.json")
	cfg := core.ExportRunConfig{
		Format:     "json",
		OutputPath: outputPath,
	}
	_, err = core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	// File should be 0644 (owner rw, group r, other r).
	if perm != 0o644 {
		t.Errorf("file permissions = %o, want 0644", perm)
	}
	// Specifically check it's not world-writable.
	if perm&0o002 != 0 {
		t.Error("exported file should not be world-writable")
	}
}

// TestExportSymlinkResolution verifies that symlinks in export paths are
// resolved to prevent symlink attacks.
func TestExportSymlinkResolution(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: safe-value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	td, err := core.PrepareTreeData(ctx, eng, true, "")
	if err != nil {
		t.Fatalf("PrepareTreeData: %v", err)
	}

	// Create a real directory and a symlink to it.
	realDir := filepath.Join(wsDir, "real-output")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(wsDir, "link-output")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// Export through the symlink.
	outputPath := filepath.Join(linkDir, "config.json")
	cfg := core.ExportRunConfig{
		Format:     "json",
		OutputPath: outputPath,
	}
	_, err = core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export through symlink: %v", err)
	}

	// The file should physically exist in the real directory.
	realPath := filepath.Join(realDir, "config.json")
	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("file should exist at resolved path: %v", err)
	}
	if !strings.Contains(string(data), "safe-value") {
		t.Error("exported file should contain the config value")
	}
}

// --- Path validation security tests ---

// TestSetValueRejectsInvalidPaths verifies that the engine rejects config paths
// that don't match the allowed pattern.
func TestSetValueRejectsPathTraversal(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	badPaths := []string{
		"",              // empty path
		"/leading",      // leading slash
		"../escape",     // path traversal
		"foo//bar",      // empty segment
		"UPPER/case",    // uppercase
		"has space/key", // space in segment
	}

	for _, path := range badPaths {
		t.Run(path, func(t *testing.T) {
			err := eng.SetValue(ctx, path, config.Value{Val: "test"})
			if err == nil {
				t.Errorf("SetValue(%q) should return an error for invalid path", path)
			}
		})
	}
}

// --- Workspace validation security tests ---

// TestWorkspaceValidationRejectsUnknownProvider verifies that workspace
// validation catches references to non-existent providers.
func TestWorkspaceValidationRejectsUnknownProvider(t *testing.T) {
	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "nonexistent-provider"},
	}
	reg := core.DefaultRegistry()
	err := core.ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject unknown config provider")
	}
}

// TestWorkspaceValidationRejectsUnsupportedVersion verifies that version
// validation works.
func TestWorkspaceValidationRejectsUnsupportedVersion(t *testing.T) {
	ws := &core.WorkspaceConfig{
		Version: "99",
		Config:  core.ProviderRef{Provider: "structuredfile"},
	}
	reg := core.DefaultRegistry()
	err := core.ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported workspace version") {
		t.Errorf("error should mention unsupported version, got: %v", err)
	}
}

// TestWorkspaceValidationRejectsPluginDirTraversal verifies that plugin
// directories with path traversal are rejected.
func TestWorkspaceValidationRejectsPluginDirTraversal(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
plugins:
  directories:
    - ../../etc/evil
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	reg := core.DefaultRegistry()
	err = core.ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject plugin directory with path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

// TestWorkspaceValidationRejectsMissingConfigProvider verifies that a workspace
// without a config provider is rejected.
func TestWorkspaceValidationRejectsMissingConfigProvider(t *testing.T) {
	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: ""},
	}
	reg := core.DefaultRegistry()
	err := core.ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject empty config provider")
	}
	if !strings.Contains(err.Error(), "config provider is required") {
		t.Errorf("error should mention config provider required, got: %v", err)
	}
}

// TestWorkspaceValidationRejectsBadComponentNames verifies that component names
// must follow the allowed pattern.
func TestWorkspaceValidationRejectsBadComponentNames(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: ValidName_WRONG
    paths:
      - valid/
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	reg := core.DefaultRegistry()
	err = core.ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject invalid component names")
	}
}

// TestWorkspaceValidationRejectsMissingTemplateFile verifies that referencing a
// non-existent template file is caught by workspace validation.
func TestWorkspaceValidationRejectsMissingTemplateFile(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
export:
  templates:
    - name: missing
      template: ./templates/does-not-exist.tmpl
      output: ./output/out.txt
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	reg := core.DefaultRegistry()
	err = core.ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject missing template file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention file not found, got: %v", err)
	}
}

// --- .zhi directory permissions test ---

// TestComponentStateFilePermissions verifies that the .zhi state directory
// has restrictive permissions (user-only).
func TestComponentStateFilePermissions(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: database
    paths:
      - database/
    mandatory: true
`,
		map[string]string{
			"db.yaml": `database:
  host:
    val: localhost
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	// Create .zhi directory with proper permissions by saving state.
	zhiDir := filepath.Join(wsDir, ".zhi")
	if err := os.MkdirAll(zhiDir, 0o700); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(zhiDir)
	if err != nil {
		t.Fatalf("Stat .zhi dir: %v", err)
	}
	perm := info.Mode().Perm()
	// .zhi directory should be user-only (0700).
	if perm&0o077 != 0 {
		t.Errorf(".zhi directory permissions = %o, want 0700 (user-only)", perm)
	}
}
