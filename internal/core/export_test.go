package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// newTestTree creates a tree with test data.
func newTestTree() *config.Tree {
	t := config.NewTree()
	_ = t.Set("database/host", &config.Value{Val: "localhost", Metadata: map[string]any{"description": "DB host"}})
	_ = t.Set("database/port", &config.Value{Val: 5432})
	_ = t.Set("app/name", &config.Value{Val: "myapp"})
	_ = t.Set("app/log-level", &config.Value{Val: "info"})
	_ = t.Set("redis/url", &config.Value{Val: "redis://localhost:6379"})
	_ = t.Set("monitoring/scrape-interval", &config.Value{Val: "15s"})
	return t
}

// newTestComponents creates a ComponentManager with test definitions.
func newTestComponents(t *testing.T) *ComponentManager {
	t.Helper()
	defs := []ComponentDef{
		{Name: "app", Paths: []string{"app/"}, Mandatory: true},
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/"}, Mandatory: false},
		{Name: "monitoring", Paths: []string{"monitoring/"}, Mandatory: false},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}
	// Enable redis, leave monitoring disabled.
	_ = cm.Enable("redis")
	return cm
}

func TestTreeDataGet(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	// Existing path.
	if got := td.Get("database/host"); got != "localhost" {
		t.Errorf("Get(database/host) = %q, want %q", got, "localhost")
	}

	// Missing path returns empty string.
	if got := td.Get("nonexistent"); got != "" {
		t.Errorf("Get(nonexistent) = %q, want empty", got)
	}

	// Disabled component path returns empty string.
	if got := td.Get("monitoring/scrape-interval"); got != "" {
		t.Errorf("Get(monitoring/scrape-interval) = %q, want empty (disabled)", got)
	}
}

func TestTreeDataGetOr(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	if got := td.GetOr("database/host", "fallback"); got != "localhost" {
		t.Errorf("GetOr(database/host) = %q, want %q", got, "localhost")
	}
	if got := td.GetOr("nonexistent", "fallback"); got != "fallback" {
		t.Errorf("GetOr(nonexistent) = %q, want %q", got, "fallback")
	}
}

func TestTreeDataHas(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	if !td.Has("database/host") {
		t.Error("Has(database/host) = false, want true")
	}
	if td.Has("nonexistent") {
		t.Error("Has(nonexistent) = true, want false")
	}
	// Disabled component path.
	if td.Has("monitoring/scrape-interval") {
		t.Error("Has(monitoring/scrape-interval) = true, want false (disabled)")
	}
}

func TestTreeDataAll(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	all := td.All()

	// Should include enabled components.
	if _, ok := all["database/host"]; !ok {
		t.Error("All() missing database/host")
	}
	if _, ok := all["app/name"]; !ok {
		t.Error("All() missing app/name")
	}
	if _, ok := all["redis/url"]; !ok {
		t.Error("All() missing redis/url")
	}

	// Should exclude disabled components.
	if _, ok := all["monitoring/scrape-interval"]; ok {
		t.Error("All() should not include monitoring/scrape-interval (disabled)")
	}
}

func TestTreeDataPrefix(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	p := td.Prefix("database")
	if v, ok := p["host"]; !ok || v != "localhost" {
		t.Errorf("Prefix(database)[host] = %v, want localhost", v)
	}
	if v, ok := p["port"]; !ok || v != 5432 {
		t.Errorf("Prefix(database)[port] = %v, want 5432", v)
	}
	if len(p) != 2 {
		t.Errorf("Prefix(database) has %d entries, want 2", len(p))
	}
}

func TestTreeDataNested(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("database/connection/host", &config.Value{Val: "localhost"})
	_ = tree.Set("database/connection/port", &config.Value{Val: 5432})
	_ = tree.Set("database/name", &config.Value{Val: "mydb"})

	td := NewTreeData(tree, nil)
	nested := td.Nested("database")

	conn, ok := nested["connection"].(map[string]any)
	if !ok {
		t.Fatal("Nested(database)[connection] is not a map")
	}
	if conn["host"] != "localhost" {
		t.Errorf("nested connection/host = %v, want localhost", conn["host"])
	}
	if conn["port"] != 5432 {
		t.Errorf("nested connection/port = %v, want 5432", conn["port"])
	}
	if nested["name"] != "mydb" {
		t.Errorf("nested name = %v, want mydb", nested["name"])
	}
}

func TestTreeDataMeta(t *testing.T) {
	tree := newTestTree()
	td := NewTreeData(tree, nil)

	if got := td.Meta("database/host", "description"); got != "DB host" {
		t.Errorf("Meta(database/host, description) = %q, want %q", got, "DB host")
	}
	if got := td.Meta("database/host", "nonexistent"); got != "" {
		t.Errorf("Meta(database/host, nonexistent) = %q, want empty", got)
	}
	if got := td.Meta("nonexistent", "description"); got != "" {
		t.Errorf("Meta(nonexistent, description) = %q, want empty", got)
	}
}

func TestTreeDataComponentEnabled(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	td := NewTreeData(tree, cm)

	if !td.ComponentEnabled("app") {
		t.Error("ComponentEnabled(app) = false, want true")
	}
	if !td.ComponentEnabled("database") {
		t.Error("ComponentEnabled(database) = false, want true")
	}
	if !td.ComponentEnabled("redis") {
		t.Error("ComponentEnabled(redis) = false, want true")
	}
	if td.ComponentEnabled("monitoring") {
		t.Error("ComponentEnabled(monitoring) = true, want false")
	}
	if td.ComponentEnabled("nonexistent") {
		t.Error("ComponentEnabled(nonexistent) = true, want false")
	}
}

func TestTreeDataEnabledComponents(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	td := NewTreeData(tree, cm)

	enabled := td.EnabledComponents()
	if len(enabled) != 3 {
		t.Fatalf("EnabledComponents() has %d entries, want 3", len(enabled))
	}
	expected := map[string]bool{"app": true, "database": true, "redis": true}
	for _, name := range enabled {
		if !expected[name] {
			t.Errorf("unexpected enabled component: %q", name)
		}
	}
}

func TestTreeDataDisabledComponents(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	td := NewTreeData(tree, cm)

	disabled := td.DisabledComponents()
	if len(disabled) != 1 {
		t.Fatalf("DisabledComponents() has %d entries, want 1", len(disabled))
	}
	if disabled[0] != "monitoring" {
		t.Errorf("DisabledComponents()[0] = %q, want monitoring", disabled[0])
	}
}

func TestTreeDataComponentPaths(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	td := NewTreeData(tree, cm)

	paths := td.ComponentPaths("database")
	if len(paths) != 1 || paths[0] != "database/" {
		t.Errorf("ComponentPaths(database) = %v, want [database/]", paths)
	}

	paths = td.ComponentPaths("nonexistent")
	if paths != nil {
		t.Errorf("ComponentPaths(nonexistent) = %v, want nil", paths)
	}
}

func TestFormatToJSON(t *testing.T) {
	result, err := toJSON(map[string]any{"host": "localhost", "port": 5432})
	if err != nil {
		t.Fatalf("toJSON: %v", err)
	}
	if !strings.Contains(result, "\"host\": \"localhost\"") {
		t.Errorf("toJSON output missing host: %s", result)
	}
	if !strings.Contains(result, "\"port\": 5432") {
		t.Errorf("toJSON output missing port: %s", result)
	}
}

func TestFormatToJSONCompact(t *testing.T) {
	result, err := toJSONCompact(map[string]any{"host": "localhost"})
	if err != nil {
		t.Fatalf("toJSONCompact: %v", err)
	}
	if strings.Contains(result, "\n") {
		t.Errorf("toJSONCompact should not contain newlines: %s", result)
	}
}

func TestFormatToYAML(t *testing.T) {
	result, err := toYAML(map[string]any{"host": "localhost", "port": 5432})
	if err != nil {
		t.Fatalf("toYAML: %v", err)
	}
	if !strings.Contains(result, "host: localhost") {
		t.Errorf("toYAML output missing host: %s", result)
	}
}

func TestFormatToTOML(t *testing.T) {
	result, err := toTOML(map[string]any{"host": "localhost", "port": int64(5432)})
	if err != nil {
		t.Fatalf("toTOML: %v", err)
	}
	if !strings.Contains(result, "host") {
		t.Errorf("toTOML output missing host: %s", result)
	}
}

func TestFormatToDotenv(t *testing.T) {
	result, err := toDotenv(map[string]any{
		"database/host": "localhost",
		"database/port": 5432,
	})
	if err != nil {
		t.Fatalf("toDotenv: %v", err)
	}
	if !strings.Contains(result, "DATABASE_HOST=localhost") {
		t.Errorf("toDotenv output missing DATABASE_HOST: %s", result)
	}
	if !strings.Contains(result, "DATABASE_PORT=5432") {
		t.Errorf("toDotenv output missing DATABASE_PORT: %s", result)
	}
}

func TestFormatToDotenvNonMap(t *testing.T) {
	_, err := toDotenv("not a map")
	if err == nil {
		t.Error("toDotenv(string) should return error")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("hello world"); got != "'hello world'" {
		t.Errorf("shellQuote = %q, want %q", got, "'hello world'")
	}
	if got := shellQuote("it's"); got != "'it'\\''s'" {
		t.Errorf("shellQuote = %q, want %q", got, "'it'\\''s'")
	}
}

func TestIndent(t *testing.T) {
	result := indentStr(4, "line1\nline2\nline3")
	if !strings.HasPrefix(result, "    line1") {
		t.Errorf("indent missing prefix: %q", result)
	}
	if !strings.Contains(result, "\n    line2") {
		t.Errorf("indent missing second line: %q", result)
	}
}

func TestExportWithTemplate(t *testing.T) {
	dir := t.TempDir()
	tmplContent := `host={{ .Get "database/host" }}
port={{ .Get "database/port" }}
{{ if .ComponentEnabled "redis" }}redis={{ .Get "redis/url" }}{{ end }}
{{ if .ComponentEnabled "monitoring" }}monitoring=true{{ end }}`

	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	result, err := Export(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if !strings.Contains(result.Content, "host=localhost") {
		t.Errorf("missing host=localhost in output: %s", result.Content)
	}
	if !strings.Contains(result.Content, "port=5432") {
		t.Errorf("missing port=5432 in output: %s", result.Content)
	}
	if !strings.Contains(result.Content, "redis=redis://localhost:6379") {
		t.Errorf("missing redis URL in output: %s", result.Content)
	}
	if strings.Contains(result.Content, "monitoring=true") {
		t.Errorf("monitoring should not be in output (disabled): %s", result.Content)
	}
}

func TestExportWriteToFile(t *testing.T) {
	dir := t.TempDir()
	tmplContent := `hello={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	outputPath := filepath.Join(dir, "output.txt")

	tree := newTestTree()
	td := NewTreeData(tree, nil)

	_, err := Export(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		OutputPath:   outputPath,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "hello=myapp" {
		t.Errorf("output = %q, want %q", string(data), "hello=myapp")
	}
}

func TestExportDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	tmplContent := `hello={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	outputPath := filepath.Join(dir, "output.txt")

	tree := newTestTree()
	td := NewTreeData(tree, nil)

	result, err := Export(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		OutputPath:   outputPath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if result.Content != "hello=myapp" {
		t.Errorf("content = %q, want %q", result.Content, "hello=myapp")
	}

	if _, err := os.Stat(outputPath); err == nil {
		t.Error("dry-run should not create output file")
	}
}

func TestExportNoTemplateOrFormat(t *testing.T) {
	td := NewTreeData(config.NewTree(), nil)
	_, err := Export(context.Background(), td, ExportRunConfig{})
	if err == nil {
		t.Error("Export with no template or format should fail")
	}
}

func TestExportMissingPathReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	tmplContent := `value={{ .Get "nonexistent/path" }}`
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	td := NewTreeData(config.NewTree(), nil)
	result, err := Export(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if result.Content != "value=" {
		t.Errorf("content = %q, want %q", result.Content, "value=")
	}
}

func TestExportAll(t *testing.T) {
	dir := t.TempDir()

	tmpl1 := filepath.Join(dir, "a.tmpl")
	if err := os.WriteFile(tmpl1, []byte(`a={{ .Get "app/name" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpl2 := filepath.Join(dir, "b.tmpl")
	if err := os.WriteFile(tmpl2, []byte(`b={{ .Get "database/host" }}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tree := newTestTree()
	td := NewTreeData(tree, nil)

	configs := []ExportRunConfig{
		{TemplatePath: tmpl1, DryRun: true},
		{TemplatePath: tmpl2, DryRun: true},
	}

	results, err := ExportAll(context.Background(), td, configs)
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("ExportAll returned %d results, want 2", len(results))
	}
	if results[0].Content != "a=myapp" {
		t.Errorf("result[0] = %q, want a=myapp", results[0].Content)
	}
	if results[1].Content != "b=localhost" {
		t.Errorf("result[1] = %q, want b=localhost", results[1].Content)
	}
}
