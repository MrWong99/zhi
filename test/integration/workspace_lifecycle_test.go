package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// TestWorkspaceLoadAndListPaths verifies that a workspace with a structuredfile
// config provider correctly loads a zhi.yaml and serves config paths from
// YAML/JSON files in the config directory.
func TestWorkspaceLoadAndListPaths(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"database.yaml": `database:
  host:
    val: localhost
    metadata:
      description: Database hostname
  port:
    val: 5432
    metadata:
      description: Database port
`,
			"app.json": `{
  "app": {
    "name": {
      "val": "myapp",
      "metadata": {"description": "Application name"}
    },
    "debug": {
      "val": false
    }
  }
}`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if ws.Version != "1" {
		t.Errorf("version = %q, want %q", ws.Version, "1")
	}
	if ws.Config.Provider != "structuredfile" {
		t.Errorf("config provider = %q, want structuredfile", ws.Config.Provider)
	}

	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	paths := tree.List()
	sort.Strings(paths)
	want := []string{"app/debug", "app/name", "database/host", "database/port"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}

	// Verify specific values.
	v, ok := tree.Get("database/host")
	if !ok || v.Val != "localhost" {
		t.Errorf("database/host = %v (ok=%v), want localhost", v.Val, ok)
	}
	v, ok = tree.Get("database/port")
	if !ok {
		t.Fatal("database/port not found")
	}
	// structuredfile parses YAML integers; type may be int or float64.
	switch p := v.Val.(type) {
	case int:
		if p != 5432 {
			t.Errorf("database/port = %d, want 5432", p)
		}
	case float64:
		if int(p) != 5432 {
			t.Errorf("database/port = %f, want 5432", p)
		}
	default:
		t.Errorf("database/port has unexpected type %T = %v", v.Val, v.Val)
	}
}

// TestWorkspaceSetAndReloadValue tests that Set updates the in-memory config
// and a subsequent LoadTree reflects the change.
func TestWorkspaceSetAndReloadValue(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: original
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

	// Set a new value.
	err = eng.SetValue(ctx, "app/name", config.Value{Val: "modified"})
	if err != nil {
		t.Fatalf("SetValue: %v", err)
	}

	// Reload and verify.
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	v, ok := tree.Get("app/name")
	if !ok {
		t.Fatal("app/name not found after set")
	}
	if v.Val != "modified" {
		t.Errorf("app/name = %v, want modified", v.Val)
	}
}

// TestWorkspaceValidation tests workspace-level validation through the engine.
func TestWorkspaceValidation(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: testapp
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	if err := core.ValidateWorkspace(ws, reg); err != nil {
		t.Fatalf("ValidateWorkspace: %v", err)
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	// Validate should return no blocking errors for valid config.
	results, err := eng.Validate(ctx, tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, r := range results {
		if r.Severity == config.Blocking {
			t.Errorf("unexpected blocking validation result: %s", r.Message)
		}
	}
}

// TestWorkspaceExportJSON tests exporting config as JSON using a workspace config.
func TestWorkspaceExportJSON(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
export:
  templates:
    - name: json-export
      format: json
      output: ./output/config.json
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: testapp
  port:
    val: 8080
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

	// Export as JSON to a file.
	outputPath := filepath.Join(wsDir, "output", "config.json")
	cfg := core.ExportRunConfig{
		Format:     "json",
		OutputPath: outputPath,
	}
	result, err := core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if result.Content == "" {
		t.Fatal("export produced empty content")
	}

	// Parse the exported JSON.
	var exported map[string]any
	if err := json.Unmarshal([]byte(result.Content), &exported); err != nil {
		t.Fatalf("parsing exported JSON: %v", err)
	}
	// Check nested structure.
	app, ok := exported["app"].(map[string]any)
	if !ok {
		t.Fatalf("expected app key in exported JSON, got %v", exported)
	}
	if name, ok := app["name"].(string); !ok || name != "testapp" {
		t.Errorf("app.name = %v, want testapp", app["name"])
	}

	// Verify the file was written.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading export output: %v", err)
	}
	if !strings.Contains(string(data), "testapp") {
		t.Error("exported file should contain 'testapp'")
	}
}

// TestWorkspaceExportYAML tests exporting config as YAML.
func TestWorkspaceExportYAML(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"db.yaml": `database:
  host:
    val: db.example.com
  port:
    val: 3306
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

	cfg := core.ExportRunConfig{
		Format: "yaml",
		DryRun: true,
	}
	result, err := core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export YAML: %v", err)
	}
	if !strings.Contains(result.Content, "db.example.com") {
		t.Errorf("YAML export should contain db.example.com, got:\n%s", result.Content)
	}
}

// TestWorkspaceExportDotenv tests exporting config as dotenv format.
func TestWorkspaceExportDotenv(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: myservice
  debug:
    val: true
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

	cfg := core.ExportRunConfig{
		Format: "dotenv",
		DryRun: true,
	}
	result, err := core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export dotenv: %v", err)
	}
	// Dotenv replaces "/" with "_" and uppercases.
	if !strings.Contains(result.Content, "APP_NAME=myservice") {
		t.Errorf("dotenv export should contain APP_NAME=myservice, got:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "APP_DEBUG=true") {
		t.Errorf("dotenv export should contain APP_DEBUG=true, got:\n%s", result.Content)
	}
}

// TestWorkspaceExportCustomTemplate tests using a custom Go template.
func TestWorkspaceExportCustomTemplate(t *testing.T) {
	tmplContent := `# Generated config
{{- range $k, $v := .All }}
{{ $k }}={{ $v }}
{{- end }}
`
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
export:
  templates:
    - name: custom
      template: ./templates/config.tmpl
      output: ./output/config.txt
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: templated-app
`,
		},
		map[string]string{
			"templates/config.tmpl": tmplContent,
		},
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

	tmplPath := filepath.Join(wsDir, "templates", "config.tmpl")
	outputPath := filepath.Join(wsDir, "output", "config.txt")
	cfg := core.ExportRunConfig{
		TemplatePath: tmplPath,
		OutputPath:   outputPath,
	}
	result, err := core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export template: %v", err)
	}

	if !strings.Contains(result.Content, "app/name=templated-app") {
		t.Errorf("template export should contain app/name=templated-app, got:\n%s", result.Content)
	}

	// Verify file was written.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !strings.Contains(string(data), "templated-app") {
		t.Error("output file should contain templated-app")
	}
}

// TestWorkspaceFileDiscovery verifies that LoadWorkspace walks up from a
// subdirectory to find zhi.yaml in a parent directory.
func TestWorkspaceFileDiscovery(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: discovered
`,
		},
		nil,
	)

	// Create a subdirectory and try to discover workspace from there.
	subDir := filepath.Join(wsDir, "sub", "deep")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	ws, err := core.LoadWorkspace(subDir)
	if err != nil {
		t.Fatalf("LoadWorkspace from subdir: %v", err)
	}
	if ws.Config.Provider != "structuredfile" {
		t.Errorf("provider = %q, want structuredfile", ws.Config.Provider)
	}

	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	v, ok := tree.Get("app/name")
	if !ok || v.Val != "discovered" {
		t.Errorf("app/name = %v (ok=%v), want discovered", v.Val, ok)
	}
}

// TestWorkspaceJSONFormat tests that zhi.json is supported as an alternative
// to zhi.yaml.
func TestWorkspaceJSONFormat(t *testing.T) {
	dir := t.TempDir()

	zhiJSON := `{
  "version": "1",
  "config": {
    "provider": "structuredfile",
    "options": {"directory": "./config"}
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "zhi.json"), []byte(zhiJSON), 0o644); err != nil {
		t.Fatalf("writing zhi.json: %v", err)
	}

	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.yaml"), []byte(`app:
  key:
    val: from-json-workspace
`), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := core.LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	tree, err := eng.LoadTree(context.Background())
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	v, ok := tree.Get("app/key")
	if !ok || v.Val != "from-json-workspace" {
		t.Errorf("app/key = %v, want from-json-workspace", v.Val)
	}
}

// TestWorkspaceNoConfigDir verifies that a missing config directory is handled
// gracefully by the structuredfile provider: it starts with an empty tree.
func TestWorkspaceNoConfigDir(t *testing.T) {
	dir := t.TempDir()
	zhiYAML := `version: "1"
config:
  provider: structuredfile
  options:
    directory: ./nonexistent-config
`
	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte(zhiYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := core.LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// structuredfile with missing directory starts with no values.
	tree, err := eng.LoadTree(context.Background())
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	if len(tree.List()) != 0 {
		t.Errorf("expected empty tree for missing config dir, got %d paths", len(tree.List()))
	}
}
