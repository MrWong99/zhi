package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestCheckDriftNoDrift(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `host={{ .Get "database/host" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(dir, "output.txt")
	// Write the expected content so there's no drift.
	if err := os.WriteFile(outputPath, []byte("host=localhost"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newDriftTestEngine(t, dir, []ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "output.txt"},
	})

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if result.HasDrift() {
		t.Error("expected no drift")
	}
	if len(result.InSync) != 1 {
		t.Errorf("expected 1 in-sync entry, got %d", len(result.InSync))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}
}

func TestCheckDriftDetected(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `host={{ .Get "database/host" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(dir, "output.txt")
	// Write different content to simulate drift.
	if err := os.WriteFile(outputPath, []byte("host=remotehost"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newDriftTestEngine(t, dir, []ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "output.txt"},
	})

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if !result.HasDrift() {
		t.Error("expected drift to be detected")
	}
	if len(result.Drifted) != 1 {
		t.Fatalf("expected 1 drifted entry, got %d", len(result.Drifted))
	}
	if result.Drifted[0].Diff == "" {
		t.Error("expected diff output for drifted entry")
	}
	if result.Drifted[0].IsNew {
		t.Error("expected IsNew to be false for existing file")
	}
}

func TestCheckDriftNewFile(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `host={{ .Get "database/host" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Don't create the output file — it's "new".
	eng := newDriftTestEngine(t, dir, []ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "output.txt"},
	})

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if !result.HasDrift() {
		t.Error("expected drift for new file")
	}
	if len(result.Drifted) != 1 {
		t.Fatalf("expected 1 drifted entry, got %d", len(result.Drifted))
	}
	if !result.Drifted[0].IsNew {
		t.Error("expected IsNew to be true")
	}
}

func TestCheckDriftNoTemplates(t *testing.T) {
	dir := t.TempDir()
	eng := newDriftTestEngine(t, dir, nil)

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if result.HasDrift() {
		t.Error("expected no drift with no templates")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestCheckDriftStdoutTemplateSkipped(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `host={{ .Get "database/host" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stdout-only template (no output path).
	eng := newDriftTestEngine(t, dir, []ExportTemplate{
		{Name: "stdout-only", Template: "test.tmpl", Output: ""},
	})

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if result.HasDrift() {
		t.Error("expected no drift for stdout-only template")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning for skipped template, got %d", len(result.Warnings))
	}
}

func TestCheckDriftRenderError(t *testing.T) {
	dir := t.TempDir()

	// Write a malformed template.
	tmplPath := filepath.Join(dir, "bad.tmpl")
	if err := os.WriteFile(tmplPath, []byte(`{{ .Invalid`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newDriftTestEngine(t, dir, []ExportTemplate{
		{Name: "bad", Template: "bad.tmpl", Output: "output.txt"},
	})

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift should not return top-level error: %v", err)
	}
	if !result.HasErrors() {
		t.Error("expected per-template error for malformed template")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestCheckDriftMultipleTemplates(t *testing.T) {
	dir := t.TempDir()

	// Template 1: in sync.
	tmpl1 := filepath.Join(dir, "a.tmpl")
	if err := os.WriteFile(tmpl1, []byte(`a={{ .Get "app/name" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out1 := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(out1, []byte("a=myapp"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Template 2: drifted.
	tmpl2 := filepath.Join(dir, "b.tmpl")
	if err := os.WriteFile(tmpl2, []byte(`b={{ .Get "database/host" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(out2, []byte("b=changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newDriftTestEngine(t, dir, []ExportTemplate{
		{Name: "a", Template: "a.tmpl", Output: "a.txt"},
		{Name: "b", Template: "b.tmpl", Output: "b.txt"},
	})

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if len(result.InSync) != 1 {
		t.Errorf("expected 1 in-sync, got %d", len(result.InSync))
	}
	if len(result.Drifted) != 1 {
		t.Errorf("expected 1 drifted, got %d", len(result.Drifted))
	}
}

func TestCheckDriftIterateTemplate(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `{{ .Key }}`
	tmplPath := filepath.Join(dir, "iter.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create output for one child in sync, leave other as new.
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "databases.yml"), []byte("databases"), 0o644); err != nil {
		t.Fatal(err)
	}
	// webservers.yml doesn't exist → new file drift.

	eng := newDriftTestEngineWithTree(t, dir, []ExportTemplate{
		{
			Name:          "groups",
			Template:      "iter.tmpl",
			Iterate:       "groups",
			OutputPattern: "./out/{{ .Key }}.yml",
		},
	}, newIterateTree())

	result, err := CheckDrift(context.Background(), eng)
	if err != nil {
		t.Fatalf("CheckDrift: %v", err)
	}
	if len(result.InSync) != 1 {
		t.Errorf("expected 1 in-sync (databases), got %d", len(result.InSync))
	}
	if len(result.Drifted) != 1 {
		t.Errorf("expected 1 drifted (webservers new), got %d", len(result.Drifted))
	}
	if len(result.Drifted) > 0 && !result.Drifted[0].IsNew {
		t.Error("expected drifted entry to be IsNew")
	}
}

// newDriftTestEngine creates a test engine with the default test tree and the
// given export templates.
func newDriftTestEngine(t *testing.T, wsDir string, templates []ExportTemplate) *Engine {
	t.Helper()
	return newDriftTestEngineWithTree(t, wsDir, templates, newTestTree())
}

// newDriftTestEngineWithTree creates a test engine with a custom tree and
// export templates.
func newDriftTestEngineWithTree(t *testing.T, wsDir string, templates []ExportTemplate, tree *config.Tree) *Engine {
	t.Helper()

	mc := &mockDriftConfig{tree: tree}

	reg := NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "mock"},
		Dir:     wsDir,
		Export: ExportConfig{
			Templates: templates,
		},
	}

	eng, err := NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng
}

// mockDriftConfig is a minimal config.Plugin for drift tests.
type mockDriftConfig struct {
	tree *config.Tree
}

func (m *mockDriftConfig) List(_ context.Context) ([]string, error) {
	return m.tree.List(), nil
}

func (m *mockDriftConfig) Get(_ context.Context, path string) (config.Value, bool, error) {
	v, ok := m.tree.Get(path)
	return v, ok, nil
}

func (m *mockDriftConfig) Set(_ context.Context, path string, v config.Value) error {
	return m.tree.Set(path, &v)
}

func (m *mockDriftConfig) Validate(_ context.Context, _ string, _ config.TreeReader) ([]config.ValidationResult, error) {
	return nil, nil
}
