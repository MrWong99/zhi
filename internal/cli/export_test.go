package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "export",
		Short:         "Export configuration",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runExport,
	}
	cmd.Flags().StringVar(&exportTemplate, "template", "", "path to a Go template file")
	cmd.Flags().StringVar(&exportFormat, "format", "", "built-in format (json, yaml, toml, dotenv)")
	cmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file path")
	cmd.Flags().StringVar(&exportPrefix, "prefix", "", "only export paths under this prefix")
	cmd.Flags().BoolVar(&exportAllComponents, "all-components", false, "include all components")
	cmd.Flags().BoolVar(&exportDryRun, "dry-run", false, "print without writing")
	return cmd
}

func resetExportFlags() {
	exportTemplate = ""
	exportFormat = ""
	exportOutput = ""
	exportPrefix = ""
	exportAllComponents = false
	exportDryRun = false
}

func TestExportFormatJSON(t *testing.T) {
	resetExportFlags()
	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--format", "json")
	if err != nil {
		t.Fatalf("export --format json: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "localhost") {
		t.Errorf("JSON output missing localhost: %s", output)
	}
}

func TestExportFormatYAML(t *testing.T) {
	resetExportFlags()
	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--format", "yaml")
	if err != nil {
		t.Fatalf("export --format yaml: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "host: localhost") {
		t.Errorf("YAML output missing host: %s", output)
	}
}

func TestExportFormatDotenv(t *testing.T) {
	resetExportFlags()
	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--format", "dotenv")
	if err != nil {
		t.Fatalf("export --format dotenv: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "DATABASE_HOST=localhost") {
		t.Errorf("dotenv output missing DATABASE_HOST: %s", output)
	}
}

func TestExportTemplate(t *testing.T) {
	resetExportFlags()
	dir := t.TempDir()

	tmplContent := `name={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := setupTestEngine(t, nil)
	eng.SetTestWorkspaceDir(dir)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--template", tmplPath)
	if err != nil {
		t.Fatalf("export --template: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "name=myapp") {
		t.Errorf("template output missing name=myapp: %s", output)
	}
}

func TestExportTemplateToFile(t *testing.T) {
	resetExportFlags()
	dir := t.TempDir()

	tmplContent := `name={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(dir, "output.txt")

	eng := setupTestEngine(t, nil)
	eng.SetTestWorkspaceDir(dir)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--template", tmplPath, "--output", outputPath)
	if err != nil {
		t.Fatalf("export --template --output: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "Exported:") {
		t.Errorf("expected 'Exported:' message, got: %s", output)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "name=myapp" {
		t.Errorf("file content = %q, want %q", string(data), "name=myapp")
	}
}

func TestExportDryRun(t *testing.T) {
	resetExportFlags()
	dir := t.TempDir()

	tmplContent := `name={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(dir, "output.txt")

	eng := setupTestEngine(t, nil)
	eng.SetTestWorkspaceDir(dir)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--template", tmplPath, "--output", outputPath, "--dry-run")
	if err != nil {
		t.Fatalf("export --dry-run: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "name=myapp") {
		t.Errorf("dry-run should print content: %s", output)
	}
	if _, err := os.Stat(outputPath); err == nil {
		t.Error("dry-run should not create output file")
	}
}

func TestExportAllComponents(t *testing.T) {
	resetExportFlags()
	components := []core.ComponentDef{
		{Name: "app", Paths: []string{"app/"}, Mandatory: true},
		{Name: "database", Paths: []string{"database/"}, Mandatory: false},
	}
	eng := setupTestEngine(t, components)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	// Without --all-components: database is disabled, so its paths are excluded.
	output, err := executeCommand(cmd, "--format", "dotenv")
	if err != nil {
		t.Fatalf("export without --all-components: %v", err)
	}
	if strings.Contains(output, "DATABASE_HOST") {
		t.Errorf("without --all-components, disabled component paths should be excluded: %s", output)
	}

	// With --all-components: all paths included.
	resetExportFlags()
	cmd2 := newExportCmd()
	setEngineContext(cmd2, eng, reg)

	output2, err := executeCommand(cmd2, "--format", "dotenv", "--all-components")
	if err != nil {
		t.Fatalf("export --all-components: %v", err)
	}
	if !strings.Contains(output2, "DATABASE_HOST=localhost") {
		t.Errorf("with --all-components, all paths should be included: %s", output2)
	}
}

func TestExportPrefix(t *testing.T) {
	resetExportFlags()
	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--format", "dotenv", "--prefix", "database")
	if err != nil {
		t.Fatalf("export --prefix: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "DATABASE_HOST=localhost") {
		t.Errorf("prefix output missing DATABASE_HOST: %s", output)
	}
	if strings.Contains(output, "APP_NAME") {
		t.Errorf("prefix output should not include APP_NAME: %s", output)
	}
}

func TestExportNoFlagsNoWorkspaceTemplates(t *testing.T) {
	resetExportFlags()
	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	_, err := executeCommand(cmd)
	if err == nil {
		t.Error("export with no flags and no workspace templates should return error")
	}
}

func TestExportComponentConditionalTemplate(t *testing.T) {
	resetExportFlags()
	dir := t.TempDir()

	tmplContent := `app={{ .Get "app/name" }}
{{ if .ComponentEnabled "database" }}db_host={{ .Get "database/host" }}{{ end }}
{{ if .ComponentEnabled "monitoring" }}monitoring=true{{ end }}`

	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	components := []core.ComponentDef{
		{Name: "app", Paths: []string{"app/"}, Mandatory: true},
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "monitoring", Paths: []string{"monitoring/"}, Mandatory: false},
	}
	eng := setupTestEngine(t, components)
	eng.SetTestWorkspaceDir(dir)
	reg := core.NewRegistry()

	cmd := newExportCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--template", tmplPath)
	if err != nil {
		t.Fatalf("export: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "app=myapp") {
		t.Errorf("missing app=myapp: %s", output)
	}
	if !strings.Contains(output, "db_host=localhost") {
		t.Errorf("missing db_host=localhost (database is mandatory): %s", output)
	}
	if strings.Contains(output, "monitoring=true") {
		t.Errorf("monitoring should not appear (disabled): %s", output)
	}
}
