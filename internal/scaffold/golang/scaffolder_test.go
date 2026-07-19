package golang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/scaffold"
)

func testData() scaffold.Data {
	return scaffold.Data{
		PluginName:  "my-plugin",
		PluginType:  "config",
		BinaryName:  "zhi-config-my-plugin",
		ModulePath:  "github.com/testorg/zhi-config-my-plugin",
		PackageName: "myplugin",
		Author:      "testorg",
		License:     "MIT",
		Version:     "0.1.0",
		Description: "A test config plugin",
		Registry:    "ghcr.io/testorg",
		GoVersion:   "1.26",
		ZhiModule:   "github.com/MrWong99/zhi",
		Year:        "2026",
	}
}

func TestScaffoldConfig(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "zhi-config-my-plugin")

	s := &goScaffolder{}
	data := testData()

	if err := s.Scaffold(data, outputDir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	// Verify expected files exist.
	expectedFiles := []string{
		"go.mod",
		"Makefile",
		".gitignore",
		"README.md",
		"zhi-plugin.yaml",
		".github/workflows/release.yml",
		"cmd/zhi-config-my-plugin/main.go",
		"pkg/plugin/plugin.go",
		"pkg/plugin/plugin_test.go",
		"workspace/zhi.yaml",
		"workspace/config/defaults.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestScaffoldStore(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "zhi-store-my-plugin")

	s := &goScaffolder{}
	data := testData()
	data.PluginType = "store"
	data.BinaryName = "zhi-store-my-plugin"

	if err := s.Scaffold(data, outputDir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	expectedFiles := []string{
		"go.mod",
		"Makefile",
		"cmd/zhi-store-my-plugin/main.go",
		"pkg/plugin/plugin.go",
		"pkg/plugin/plugin_test.go",
		"workspace/zhi.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// All types include workspace config defaults for the sample workspace.
	defaults := filepath.Join(outputDir, "workspace/config/defaults.yaml")
	if _, err := os.Stat(defaults); os.IsNotExist(err) {
		t.Error("workspace/config/defaults.yaml should exist for store plugin")
	}
}

func TestScaffoldTransform(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "zhi-transform-my-plugin")

	s := &goScaffolder{}
	data := testData()
	data.PluginType = "transform"
	data.BinaryName = "zhi-transform-my-plugin"

	if err := s.Scaffold(data, outputDir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	expectedFiles := []string{
		"cmd/zhi-transform-my-plugin/main.go",
		"pkg/plugin/plugin.go",
		"pkg/plugin/plugin_test.go",
		"workspace/zhi.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestScaffoldUI(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "zhi-ui-my-plugin")

	s := &goScaffolder{}
	data := testData()
	data.PluginType = "ui"
	data.BinaryName = "zhi-ui-my-plugin"

	if err := s.Scaffold(data, outputDir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	expectedFiles := []string{
		"cmd/zhi-ui-my-plugin/main.go",
		"pkg/plugin/plugin.go",
		"pkg/plugin/plugin_test.go",
		"workspace/zhi.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestScaffoldNonEmpty(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "existing")

	// Create the directory with a file in it.
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "existing.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &goScaffolder{}
	err := s.Scaffold(testData(), outputDir)
	if err == nil {
		t.Error("Scaffold should fail on non-empty directory")
	}
}

func TestScaffoldTemplateRendering(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "project")

	s := &goScaffolder{}
	data := testData()

	if err := s.Scaffold(data, outputDir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	// Check go.mod content.
	goMod, err := os.ReadFile(filepath.Join(outputDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	goModStr := string(goMod)
	if !strings.Contains(goModStr, "github.com/testorg/zhi-config-my-plugin") {
		t.Error("go.mod should contain the module path")
	}
	if !strings.Contains(goModStr, "go 1.26") {
		t.Error("go.mod should contain Go version")
	}
	if !strings.Contains(goModStr, "github.com/MrWong99/zhi") {
		t.Error("go.mod should reference zhi module")
	}

	// Check zhi-plugin.yaml content.
	manifest, err := os.ReadFile(filepath.Join(outputDir, "zhi-plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	manifestStr := string(manifest)
	if !strings.Contains(manifestStr, "name: my-plugin") {
		t.Error("manifest should contain plugin name")
	}
	if !strings.Contains(manifestStr, "type: config") {
		t.Error("manifest should contain plugin type")
	}
	if !strings.Contains(manifestStr, `author: "testorg"`) {
		t.Error("manifest should contain quoted author")
	}

	// Check main.go references the correct plugin type.
	mainGo, err := os.ReadFile(filepath.Join(outputDir, "cmd/zhi-config-my-plugin/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	mainGoStr := string(mainGo)
	if !strings.Contains(mainGoStr, `"config": &config.GRPCPlugin{Impl: plugin.New()}`) {
		t.Error("main.go should register config plugin")
	}

	// Check Makefile references the binary name.
	makefile, err := os.ReadFile(filepath.Join(outputDir, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(makefile), "BIN_NAME := zhi-config-my-plugin") {
		t.Error("Makefile should contain binary name")
	}
}

func TestScaffoldLanguage(t *testing.T) {
	s := &goScaffolder{}
	if s.Language() != "go" {
		t.Errorf("Language() = %q, want %q", s.Language(), "go")
	}
}

func TestScaffoldRegistered(t *testing.T) {
	s, err := scaffold.Get("go")
	if err != nil {
		t.Fatalf("Get(go): %v", err)
	}
	if s.Language() != "go" {
		t.Errorf("Language() = %q, want %q", s.Language(), "go")
	}
}

func TestScaffoldGitHubActions(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "project")

	s := &goScaffolder{}
	data := testData()

	if err := s.Scaffold(data, outputDir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	workflow, err := os.ReadFile(filepath.Join(outputDir, ".github/workflows/release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflowStr := string(workflow)
	// GitHub Actions ${{ }} syntax should be preserved (not treated as Go template).
	if !strings.Contains(workflowStr, "${{ github.event_name }}") {
		t.Error("workflow should contain GitHub Actions expressions intact")
	}
	if !strings.Contains(workflowStr, "ghcr.io/testorg") {
		t.Error("workflow should contain the registry")
	}
}

func TestScaffoldEmptyDir(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "empty-project")

	// Create the output dir but leave it empty.
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &goScaffolder{}
	if err := s.Scaffold(testData(), outputDir); err != nil {
		t.Fatalf("Scaffold should succeed on empty directory: %v", err)
	}
}

func TestScaffoldNoOptionalFields(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "project")

	s := &goScaffolder{}
	data := testData()
	data.Author = ""
	data.License = ""
	data.Registry = ""

	if err := s.Scaffold(data, outputDir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	manifest, err := os.ReadFile(filepath.Join(outputDir, "zhi-plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), "author:") {
		t.Error("manifest should not contain author when empty")
	}
}
