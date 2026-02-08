package core

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// --- 8.1: Input Validation Tests ---

func TestSetValueRejectsInvalidPaths(t *testing.T) {
	mc := newMockConfig()
	eng := &Engine{
		configPlugin: mc,
		components:   mustComponents(t, nil),
		workspace:    &WorkspaceConfig{},
	}

	tests := []struct {
		path string
	}{
		{""},             // empty path
		{"/leading"},     // leading slash
		{"has space/ok"}, // space in segment
		{"UPPER/case"},   // uppercase
		{"../escape"},    // path traversal
		{"foo//bar"},     // empty segment
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := eng.SetValue(context.Background(), tt.path, config.Value{Val: "test"})
			if err == nil {
				t.Errorf("SetValue(%q) should return an error for invalid path", tt.path)
			}
		})
	}
}

func TestSetValueAcceptsValidPaths(t *testing.T) {
	mc := newMockConfig()
	eng := &Engine{
		configPlugin: mc,
		components:   mustComponents(t, nil),
		workspace:    &WorkspaceConfig{},
	}

	tests := []string{
		"database/host",
		"app/name",
		"a",
		"app/tls/cert.pem",
		"my-app/log-level",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			err := eng.SetValue(context.Background(), path, config.Value{Val: "test"})
			if err != nil {
				t.Errorf("SetValue(%q) unexpected error: %v", path, err)
			}
		})
	}
}

// --- 8.1: Workspace Validation Tests ---

func TestValidateWorkspaceRejectsInvalidComponentNames(t *testing.T) {
	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "structuredfile"},
		Components: []ComponentDef{
			{Name: "valid-name", Paths: []string{"valid/"}},
			{Name: "Invalid", Paths: []string{"invalid/"}},
		},
	}

	reg := NewRegistry()
	_ = reg.RegisterConfig("structuredfile", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	err := ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject invalid component names")
	}
	if !strings.Contains(err.Error(), "Invalid") {
		t.Errorf("error should mention the invalid name, got: %v", err)
	}
}

func TestValidateWorkspaceAcceptsValidComponentNames(t *testing.T) {
	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "structuredfile"},
		Components: []ComponentDef{
			{Name: "app", Paths: []string{"app/"}},
			{Name: "my-database", Paths: []string{"db/"}},
			{Name: "redis2", Paths: []string{"redis/"}},
		},
	}

	reg := NewRegistry()
	_ = reg.RegisterConfig("structuredfile", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	err := ValidateWorkspace(ws, reg)
	if err != nil {
		t.Fatalf("ValidateWorkspace should accept valid component names, got: %v", err)
	}
}

func TestValidateWorkspaceRejectsPluginDirTraversal(t *testing.T) {
	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "structuredfile"},
		Plugins: PluginsConfig{
			Directories: []string{"../../etc/evil"},
		},
	}

	reg := NewRegistry()
	_ = reg.RegisterConfig("structuredfile", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	err := ValidateWorkspace(ws, reg)
	if err == nil {
		t.Fatal("ValidateWorkspace should reject plugin directories with path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

func TestValidateWorkspaceAcceptsNormalPluginDirs(t *testing.T) {
	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "structuredfile"},
		Plugins: PluginsConfig{
			Directories: []string{"~/.zhi/plugins", "./local-plugins", "/opt/zhi/plugins"},
		},
	}

	reg := NewRegistry()
	_ = reg.RegisterConfig("structuredfile", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfig(), nil
	})

	err := ValidateWorkspace(ws, reg)
	if err != nil {
		t.Fatalf("ValidateWorkspace should accept normal plugin dirs, got: %v", err)
	}
}

// --- 8.2: Plugin Execution Security Tests ---

func TestAuditPluginBinaryWorldWritable(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "test-plugin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Explicitly set world-writable permissions (os.WriteFile applies umask).
	if err := os.Chmod(binPath, 0o777); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	SetLogOutput(&buf, slog.LevelWarn)
	defer SetLogLevel(slog.LevelWarn) // restore

	auditPluginBinary(binPath)

	if !strings.Contains(buf.String(), "world-writable") {
		t.Errorf("should warn about world-writable binary, got: %s", buf.String())
	}
}

func TestAuditPluginBinaryNonWorldWritable(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "test-plugin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	SetLogOutput(&buf, slog.LevelWarn)
	defer SetLogLevel(slog.LevelWarn) // restore

	auditPluginBinary(binPath)

	if strings.Contains(buf.String(), "world-writable") {
		t.Errorf("should not warn about non-world-writable binary, got: %s", buf.String())
	}
}

func TestAuditPluginBinaryLogsHash(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "test-plugin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	SetLogOutput(&buf, slog.LevelInfo)
	defer SetLogLevel(slog.LevelWarn) // restore

	auditPluginBinary(binPath)

	if !strings.Contains(buf.String(), "sha256") {
		t.Errorf("should log SHA-256 hash, got: %s", buf.String())
	}
}

// --- 8.3: File System Security Tests ---

func TestWriteExportFileRejectsTraversal(t *testing.T) {
	err := writeExportFile("/tmp/../../../etc/passwd", []byte("test"))
	if err == nil {
		t.Fatal("writeExportFile should reject path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

func TestWriteExportFileAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "output.json")

	err := writeExportFile(outPath, []byte(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("writeExportFile: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Errorf("content = %q, want %q", string(data), `{"key":"value"}`)
	}

	// Check permissions: should be 0644.
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// Mask to get the last 9 bits (rwx permissions).
	perm := info.Mode().Perm()
	if perm != 0o644 {
		t.Errorf("permissions = %o, want 0644", perm)
	}
}

func TestWriteExportFileCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "subdir", "deep", "output.txt")

	err := writeExportFile(outPath, []byte("hello"))
	if err != nil {
		t.Fatalf("writeExportFile: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", string(data), "hello")
	}
}

func TestWriteExportFileResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create symlink to real directory.
	linkDir := filepath.Join(dir, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	outPath := filepath.Join(linkDir, "output.txt")
	err := writeExportFile(outPath, []byte("content"))
	if err != nil {
		t.Fatalf("writeExportFile: %v", err)
	}

	// The file should exist at the real path.
	realPath := filepath.Join(realDir, "output.txt")
	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("ReadFile real path: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("content = %q, want %q", string(data), "content")
	}
}

// --- 8.3: .zhi Directory Permissions ---

func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"./local-plugins", false},
		{"~/.zhi/plugins", false},
		{"/opt/zhi/plugins", false},
		{"../evil", true},
		{"some/../path", true},
		{"../../etc/evil", true},
		{"foo/bar/../baz", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := containsPathTraversal(tt.path)
			if got != tt.want {
				t.Errorf("containsPathTraversal(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// --- 8.9: Edge Case Tests ---

func TestValidatePathMissingFromTree(t *testing.T) {
	mc := newMockConfig()
	eng := &Engine{
		configPlugin: mc,
		components:   mustComponents(t, nil),
		workspace:    &WorkspaceConfig{},
	}

	tree := config.NewTree()

	// Validating a non-existent path should return nil, not error.
	results, err := eng.ValidatePath(context.Background(), "nonexistent/path", tree)
	if err != nil {
		t.Fatalf("ValidatePath should not error for missing path: %v", err)
	}
	if results != nil {
		t.Errorf("ValidatePath should return nil for missing path, got %v", results)
	}
}

func TestEngineWithEmptyTree(t *testing.T) {
	mc := &mockConfig{tree: config.NewTree()}
	eng := &Engine{
		configPlugin: mc,
		components:   mustComponents(t, nil),
		workspace:    &WorkspaceConfig{},
	}

	ctx := context.Background()
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	if len(tree.List()) != 0 {
		t.Errorf("tree should be empty, got %d paths", len(tree.List()))
	}

	// Validate empty tree should not error.
	results, err := eng.Validate(ctx, tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("validation of empty tree should produce no results, got %d", len(results))
	}
}

func TestEngineNoStoreConfigured(t *testing.T) {
	mc := newMockConfig()
	eng := &Engine{
		configPlugin: mc,
		components:   mustComponents(t, nil),
		workspace:    &WorkspaceConfig{},
		storePlugin:  nil,
	}

	ctx := context.Background()

	// Save should fail gracefully.
	err := eng.SaveTree(ctx, config.NewTree())
	if err == nil {
		t.Fatal("SaveTree should error when no store is configured")
	}
	if !strings.Contains(err.Error(), "no store provider") {
		t.Errorf("error should mention no store provider, got: %v", err)
	}

	// Load should fail gracefully.
	_, _, err = eng.LoadStoredTree(ctx)
	if err == nil {
		t.Fatal("LoadStoredTree should error when no store is configured")
	}

	// ListTrees should fail gracefully.
	_, err = eng.ListTrees(ctx)
	if err == nil {
		t.Fatal("ListTrees should error when no store is configured")
	}
}

func TestExportPathTraversalInConfig(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("app/name", &config.Value{Val: "test"})
	td := NewTreeData(tree, nil)

	cfg := ExportRunConfig{
		Format:     "json",
		OutputPath: "/tmp/safe/../../../etc/evil",
		DryRun:     false,
	}

	_, err := Export(context.Background(), td, cfg)
	if err == nil {
		t.Fatal("Export should reject path traversal in output path")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

func TestExportDryRunDoesNotWriteSecurityCheck(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "should-not-exist.json")

	tree := config.NewTree()
	_ = tree.Set("app/name", &config.Value{Val: "test"})
	td := NewTreeData(tree, nil)

	cfg := ExportRunConfig{
		Format:     "json",
		OutputPath: outPath,
		DryRun:     true,
	}

	result, err := Export(context.Background(), td, cfg)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if result.Content == "" {
		t.Error("dry-run should still produce content")
	}

	if _, err := os.Stat(outPath); err == nil {
		t.Error("dry-run should not create the output file")
	}
}

// --- 8.6: Logging Tests ---

func TestSetLogLevel(t *testing.T) {
	// Just verify it doesn't panic.
	SetLogLevel(slog.LevelDebug)
	Logger().Debug("test debug")
	SetLogLevel(slog.LevelWarn) // restore
}

func TestSetLogOutput(t *testing.T) {
	var buf bytes.Buffer
	SetLogOutput(&buf, slog.LevelInfo)
	Logger().Info("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log output to contain 'test message', got: %s", buf.String())
	}
	SetLogLevel(slog.LevelWarn) // restore
}

// --- Discovery Path Traversal Tests ---

func TestDiscoveryRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	SetLogOutput(&buf, slog.LevelWarn)
	defer SetLogLevel(slog.LevelWarn) // restore

	plugins, err := Discover(DiscoveryConfig{
		Directories: []string{"../../etc/evil"},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Should skip the traversal directory and return no plugins.
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}

	if !strings.Contains(buf.String(), "path traversal") {
		t.Errorf("should log a warning about path traversal, got: %s", buf.String())
	}
}

// --- Helper ---

func mustComponents(t *testing.T, defs []ComponentDef) *ComponentManager {
	t.Helper()
	if defs == nil {
		defs = []ComponentDef{}
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}
	return cm
}
