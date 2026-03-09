package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

func TestTemplateFuncMapContainsSprigFunctions(t *testing.T) {
	fm := templateFuncMap(nil)

	// Verify a selection of Sprig-specific functions are present.
	sprigFuncs := []string{
		"trim", "trimAll", "trimSuffix", "trimPrefix",
		"nospace", "title", "snakecase", "camelcase", "kebabcase",
		"contains", "hasPrefix", "hasSuffix",
		"repeat", "substr", "trunc",
		"add", "sub", "mul", "div", "mod", "max", "min",
		"list", "dict", "hasKey", "pluck", "keys", "values",
		"default", "empty", "coalesce", "ternary",
		"toDate", "now", "date",
		"b64enc", "b64dec",
		"join", "sortAlpha",
		"regexMatch", "regexReplaceAll",
		"env", "expandenv",
		"sha256sum",
		"semver", "semverCompare",
	}
	for _, name := range sprigFuncs {
		if _, ok := fm[name]; !ok {
			t.Errorf("templateFuncMap() missing Sprig function %q", name)
		}
	}
}

func TestTemplateFuncMapZhiFunctionsPresent(t *testing.T) {
	fm := templateFuncMap(nil)

	// Verify zhi-specific functions that have no Sprig equivalent are present.
	zhiFuncs := []string{
		"toYAML", "toTOML", "toDotenv", "shellQuote", "fileACL", "fileMode",
	}
	for _, name := range zhiFuncs {
		if _, ok := fm[name]; !ok {
			t.Errorf("templateFuncMap() missing zhi function %q", name)
		}
	}
}

func TestSprigFunctionsInTemplate(t *testing.T) {
	dir := t.TempDir()

	// Use several Sprig functions in a template: trim, upper (zhi override),
	// default, repeat, and snakecase.
	tmplContent := `name={{ .Get "app/name" | trim }}
default={{ .GetOr "missing/key" "" | default "fallback" }}
repeated={{ "ab" | repeat 3 }}
snake={{ "myApp" | snakecase }}`

	tmplPath := filepath.Join(dir, "sprig.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newTestTree()
	td := NewTreeData(tree, nil)

	result, err := Export(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if !strings.Contains(result.Content, "name=myapp") {
		t.Errorf("expected name=myapp, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "default=fallback") {
		t.Errorf("expected default=fallback, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "repeated=ababab") {
		t.Errorf("expected repeated=ababab, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "snake=my_app") {
		t.Errorf("expected snake=my_app, got: %s", result.Content)
	}
}

func TestShellQuoteInTemplate(t *testing.T) {
	dir := t.TempDir()

	// shellQuote is zhi's shell-safe single-quoting function.
	// Sprig's quote (double-quoting) and squote (simple single-quoting)
	// remain available under their own names.
	tmplContent := `shell={{ .Get "app/name" | shellQuote }}`

	tmplPath := filepath.Join(dir, "quote.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newTestTree()
	td := NewTreeData(tree, nil)

	result, err := Export(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// shellQuote wraps in single quotes with proper escaping.
	if result.Content != "shell='myapp'" {
		t.Errorf("shellQuote got: %q, want: %q", result.Content, "shell='myapp'")
	}
}

func TestFileACLRendersEmpty(t *testing.T) {
	opts := &exportFileOptions{}
	fm := templateFuncMap(opts)

	fn := fm["fileACL"].(func(string) string)
	result := fn("g:10668:rw")

	if result != "" {
		t.Errorf("fileACL returned %q, want empty string", result)
	}
	if len(opts.acl) != 1 || opts.acl[0] != "g:10668:rw" {
		t.Errorf("opts.acl = %v, want [g:10668:rw]", opts.acl)
	}
}

func TestFileACLNilOpts(t *testing.T) {
	fm := templateFuncMap(nil)
	fn := fm["fileACL"].(func(string) string)

	// Should not panic with nil opts.
	result := fn("g:10668:rw")
	if result != "" {
		t.Errorf("fileACL returned %q, want empty string", result)
	}
}

func TestFileACLAccumulates(t *testing.T) {
	opts := &exportFileOptions{}
	fm := templateFuncMap(opts)

	fn := fm["fileACL"].(func(string) string)
	fn("g:10668:rw")
	fn("u:1000:r")

	if len(opts.acl) != 2 {
		t.Fatalf("opts.acl has %d entries, want 2", len(opts.acl))
	}
	if opts.acl[0] != "g:10668:rw" {
		t.Errorf("opts.acl[0] = %q, want g:10668:rw", opts.acl[0])
	}
	if opts.acl[1] != "u:1000:r" {
		t.Errorf("opts.acl[1] = %q, want u:1000:r", opts.acl[1])
	}
}

func TestFileModeRendersEmpty(t *testing.T) {
	opts := &exportFileOptions{}
	fm := templateFuncMap(opts)

	fn := fm["fileMode"].(func(int) string)
	result := fn(0o755)

	if result != "" {
		t.Errorf("fileMode returned %q, want empty string", result)
	}
	if opts.mode != 0o755 {
		t.Errorf("opts.mode = %o, want 755", opts.mode)
	}
}

func TestFileModeNilOpts(t *testing.T) {
	fm := templateFuncMap(nil)
	fn := fm["fileMode"].(func(int) string)

	// Should not panic with nil opts.
	result := fn(0o600)
	if result != "" {
		t.Errorf("fileMode returned %q, want empty string", result)
	}
}

func TestExportWriteWithFileMode(t *testing.T) {
	dir := t.TempDir()
	tmplContent := `{{ fileMode 0600 }}secret={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "mode.tmpl")
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

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode = %o, want 600", got)
	}
}

func TestExportWriteDefaultMode(t *testing.T) {
	dir := t.TempDir()
	tmplContent := `hello={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "default.tmpl")
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

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("file mode = %o, want 644", got)
	}
}

func TestExportFileModeRendersCleanContent(t *testing.T) {
	dir := t.TempDir()
	tmplContent := `{{ fileMode 0600 }}hello={{ .Get "app/name" }}`
	tmplPath := filepath.Join(dir, "clean.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newTestTree()
	td := NewTreeData(tree, nil)

	result, err := Export(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if result.Content != "hello=myapp" {
		t.Errorf("content = %q, want %q", result.Content, "hello=myapp")
	}
}

func TestExportWriteWithACL(t *testing.T) {
	if _, err := exec.LookPath("setfacl"); err != nil {
		t.Skip("setfacl not available, skipping ACL test")
	}
	if _, err := exec.LookPath("getfacl"); err != nil {
		t.Skip("getfacl not available, skipping ACL test")
	}

	dir := t.TempDir()
	gid := os.Getgid()
	aclEntry := fmt.Sprintf("g:%d:rw", gid)
	tmplContent := fmt.Sprintf(`{{ fileACL %q }}hello={{ .Get "app/name" }}`, aclEntry)
	tmplPath := filepath.Join(dir, "acl.tmpl")
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

	// Verify ACL was applied to the file using getfacl --numeric.
	out, err := exec.Command("getfacl", "-c", "--numeric", outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("getfacl file: %v (%s)", err, out)
	}
	expected := fmt.Sprintf("group:%d:rw-", gid)
	if !strings.Contains(string(out), expected) {
		t.Errorf("file getfacl output missing %q:\n%s", expected, out)
	}

	// Verify ACL was also applied to the parent directory with execute.
	out, err = exec.Command("getfacl", "-c", "--numeric", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("getfacl dir: %v (%s)", err, out)
	}
	expectedDir := fmt.Sprintf("group:%d:rwx", gid)
	if !strings.Contains(string(out), expectedDir) {
		t.Errorf("directory getfacl output missing %q:\n%s", expectedDir, out)
	}

	// Content should be clean (no ACL artifacts).
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "hello=myapp" {
		t.Errorf("content = %q, want %q", string(data), "hello=myapp")
	}
}

func TestAclEntryWithExecute(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"g:10668:rw", "g:10668:rwx"},
		{"u:1000:r", "u:1000:rx"},
		{"g:10668:rwx", "g:10668:rwx"}, // already has x
		{"g:10668:x", "g:10668:x"},     // already has x
		{"g:10668:", "g:10668:x"},      // empty perms
		{"nocolon", "nocolon"},         // malformed, returned as-is
	}
	for _, tt := range tests {
		if got := aclEntryWithExecute(tt.input); got != tt.want {
			t.Errorf("aclEntryWithExecute(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// newIterateTree creates a tree simulating Ansible groups with per-group vars.
func newIterateTree() *config.Tree {
	t := config.NewTree()
	_ = t.Set("groups/webservers/vars/http_port", &config.Value{Val: "8080"})
	_ = t.Set("groups/webservers/vars/max_clients", &config.Value{Val: "200"})
	_ = t.Set("groups/databases/vars/pg_max_connections", &config.Value{Val: "100"})
	_ = t.Set("groups/databases/vars/backup_schedule", &config.Value{Val: "0 2 * * *"})
	_ = t.Set("hosts/web1/ansible_host", &config.Value{Val: "10.0.1.10"})
	return t
}

func TestDirectChildren(t *testing.T) {
	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	children := directChildren(td, "groups")
	if len(children) != 2 {
		t.Fatalf("directChildren(groups) = %v, want 2 entries", children)
	}
	if children[0] != "databases" || children[1] != "webservers" {
		t.Errorf("directChildren(groups) = %v, want [databases webservers]", children)
	}
}

func TestDirectChildrenEmpty(t *testing.T) {
	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	children := directChildren(td, "nonexistent")
	if len(children) != 0 {
		t.Errorf("directChildren(nonexistent) = %v, want empty", children)
	}
}

func TestMaterializeChildTree(t *testing.T) {
	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	child := materializeChildTree(td, "groups", "webservers")
	paths := child.List()
	if len(paths) != 2 {
		t.Fatalf("materializeChildTree(groups, webservers) has %d paths, want 2: %v", len(paths), paths)
	}

	v, ok := child.Get("vars/http_port")
	if !ok || fmt.Sprintf("%v", v.Val) != "8080" {
		t.Errorf("child tree vars/http_port = %v, want 8080", v)
	}

	v, ok = child.Get("vars/max_clients")
	if !ok || fmt.Sprintf("%v", v.Val) != "200" {
		t.Errorf("child tree vars/max_clients = %v, want 200", v)
	}
}

func TestExportIterate(t *testing.T) {
	dir := t.TempDir()

	// Template that uses IterateData: .Key and .Value (a *TreeData).
	tmplContent := `# {{ .Key }}
{{ range $k, $v := .Value.Prefix "vars" }}{{ $k }}: {{ $v }}
{{ end }}`

	tmplPath := filepath.Join(dir, "group-vars.yml.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	results, err := ExportIterate(context.Background(), td, ExportRunConfig{
		TemplatePath:  tmplPath,
		Iterate:       "groups",
		OutputPattern: "./group_vars/{{ .Key }}.yml",
		WorkspaceDir:  dir,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("ExportIterate: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("ExportIterate returned %d results, want 2", len(results))
	}

	// Results are sorted by child name: databases, webservers.
	if !strings.Contains(results[0].Name, "databases") {
		t.Errorf("result[0].Name = %q, want to contain 'databases'", results[0].Name)
	}
	if !strings.Contains(results[0].Content, "# databases") {
		t.Errorf("result[0] missing '# databases': %s", results[0].Content)
	}
	if !strings.Contains(results[0].Content, "pg_max_connections: 100") {
		t.Errorf("result[0] missing pg_max_connections: %s", results[0].Content)
	}

	if !strings.Contains(results[1].Name, "webservers") {
		t.Errorf("result[1].Name = %q, want to contain 'webservers'", results[1].Name)
	}
	if !strings.Contains(results[1].Content, "# webservers") {
		t.Errorf("result[1] missing '# webservers': %s", results[1].Content)
	}
	if !strings.Contains(results[1].Content, "http_port: 8080") {
		t.Errorf("result[1] missing http_port: %s", results[1].Content)
	}
}

func TestExportIterateOutputPaths(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `{{ .Key }}`
	tmplPath := filepath.Join(dir, "simple.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	results, err := ExportIterate(context.Background(), td, ExportRunConfig{
		TemplatePath:  tmplPath,
		Iterate:       "groups",
		OutputPattern: "./out/{{ .Key }}.yml",
		WorkspaceDir:  dir,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("ExportIterate: %v", err)
	}

	// Verify output paths are resolved relative to workspace dir.
	for _, r := range results {
		if !filepath.IsAbs(r.OutputPath) {
			t.Errorf("output path %q is not absolute", r.OutputPath)
		}
		if !strings.HasPrefix(r.OutputPath, dir) {
			t.Errorf("output path %q is not under workspace dir %q", r.OutputPath, dir)
		}
	}

	expected := map[string]bool{
		filepath.Join(dir, "out/databases.yml"):  true,
		filepath.Join(dir, "out/webservers.yml"): true,
	}
	for _, r := range results {
		if !expected[r.OutputPath] {
			t.Errorf("unexpected output path: %q", r.OutputPath)
		}
	}
}

func TestExportIterateWritesToDisk(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `name={{ .Key }}`
	tmplPath := filepath.Join(dir, "name.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	_, err := ExportIterate(context.Background(), td, ExportRunConfig{
		TemplatePath:  tmplPath,
		Iterate:       "groups",
		OutputPattern: "./out/{{ .Key }}.txt",
		WorkspaceDir:  dir,
		DryRun:        false,
	})
	if err != nil {
		t.Fatalf("ExportIterate: %v", err)
	}

	// Verify files were written.
	for _, name := range []string{"databases", "webservers"} {
		outPath := filepath.Join(dir, "out", name+".txt")
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Errorf("reading %s: %v", outPath, err)
			continue
		}
		want := "name=" + name
		if string(data) != want {
			t.Errorf("file %s = %q, want %q", name, string(data), want)
		}
	}
}

func TestExportIterateNoChildren(t *testing.T) {
	dir := t.TempDir()

	tmplContent := `{{ .Key }}`
	tmplPath := filepath.Join(dir, "empty.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	results, err := ExportIterate(context.Background(), td, ExportRunConfig{
		TemplatePath:  tmplPath,
		Iterate:       "nonexistent",
		OutputPattern: "./out/{{ .Key }}.yml",
		WorkspaceDir:  dir,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("ExportIterate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty iterate, got %d", len(results))
	}
}

func TestExportIterateRequiresTemplate(t *testing.T) {
	td := NewTreeData(config.NewTree(), nil)
	_, err := ExportIterate(context.Background(), td, ExportRunConfig{
		Format:        "json",
		Iterate:       "groups",
		OutputPattern: "./out/{{ .Key }}.yml",
	})
	if err == nil {
		t.Error("ExportIterate with format should fail")
	}
}

func TestExportIterateRequiresOutputPattern(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	td := NewTreeData(config.NewTree(), nil)
	_, err := ExportIterate(context.Background(), td, ExportRunConfig{
		TemplatePath: tmplPath,
		Iterate:      "groups",
	})
	if err == nil {
		t.Error("ExportIterate without output-pattern should fail")
	}
}

func TestExportAllWithIterate(t *testing.T) {
	dir := t.TempDir()

	// Regular template.
	regularTmpl := filepath.Join(dir, "regular.tmpl")
	if err := os.WriteFile(regularTmpl, []byte(`app={{ .Get "hosts/web1/ansible_host" }}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Iterate template.
	iterTmpl := filepath.Join(dir, "iter.tmpl")
	if err := os.WriteFile(iterTmpl, []byte(`group={{ .Key }}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tree := newIterateTree()
	td := NewTreeData(tree, nil)

	configs := []ExportRunConfig{
		{TemplatePath: regularTmpl, DryRun: true},
		{TemplatePath: iterTmpl, Iterate: "groups", OutputPattern: "./out/{{ .Key }}.yml", WorkspaceDir: dir, DryRun: true},
	}

	results, err := ExportAll(context.Background(), td, configs)
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}

	// 1 regular + 2 iterate children = 3 results.
	if len(results) != 3 {
		t.Fatalf("ExportAll returned %d results, want 3", len(results))
	}

	if results[0].Content != "app=10.0.1.10" {
		t.Errorf("result[0] = %q, want app=10.0.1.10", results[0].Content)
	}
	if !strings.Contains(results[1].Content, "group=databases") {
		t.Errorf("result[1] = %q, want group=databases", results[1].Content)
	}
	if !strings.Contains(results[2].Content, "group=webservers") {
		t.Errorf("result[2] = %q, want group=webservers", results[2].Content)
	}
}

func TestExportIterateOutputPatternTraversal(t *testing.T) {
	// Path traversal in output-pattern is caught at workspace validation time
	// (ValidateWorkspace rejects output-pattern containing ".."), not at
	// runtime, because filepath.Join normalizes ".." away. This test verifies
	// that containsPathTraversal detects the raw pattern.
	if !containsPathTraversal("../escape/{{ .Key }}.yml") {
		t.Error("containsPathTraversal should detect '..' in output-pattern")
	}
	if containsPathTraversal("./safe/{{ .Key }}.yml") {
		t.Error("containsPathTraversal should not flag safe output-pattern")
	}
}

func TestExportIterateWithFormat(t *testing.T) {
	td := NewTreeData(newIterateTree(), nil)
	// iterate + built-in format (no template) should fail.
	_, err := ExportIterate(context.Background(), td, ExportRunConfig{
		Format:        "yaml",
		Iterate:       "groups",
		OutputPattern: "./out/{{ .Key }}.yml",
	})
	if err == nil {
		t.Error("ExportIterate with format (no template) should fail")
	}
}

func TestIterateWithComponentFiltering(t *testing.T) {
	// Build a tree with two groups; disable one via components.
	tree := config.NewTree()
	_ = tree.Set("groups/webservers/vars/port", &config.Value{Val: "80"})
	_ = tree.Set("groups/databases/vars/port", &config.Value{Val: "5432"})

	defs := []ComponentDef{
		{Name: "web", Paths: []string{"groups/webservers/"}, Mandatory: false},
		{Name: "db", Paths: []string{"groups/databases/"}, Mandatory: false},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatal(err)
	}
	_ = cm.Enable("web")
	// db stays disabled

	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "g.tmpl")
	if err := os.WriteFile(tmplPath, []byte("{{ .Key }}"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := ExportIterate(context.Background(), td, ExportRunConfig{
		TemplatePath:  tmplPath,
		Iterate:       "groups",
		OutputPattern: "./out/{{ .Key }}.yml",
		WorkspaceDir:  dir,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("ExportIterate: %v", err)
	}

	// Only webservers should appear (databases is disabled).
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (only enabled component)", len(results))
	}
	if results[0].Content != "webservers" {
		t.Errorf("result content = %q, want webservers", results[0].Content)
	}
}

func TestExpandTemplatesWithIterate(t *testing.T) {
	templates := []ExportTemplate{
		{Name: "regular", Template: "a.tmpl", Output: "out.txt"},
		{Name: "groups", Template: "b.tmpl", Iterate: "groups", OutputPattern: "./gv/{{ .Key }}.yml"},
	}

	configs := ExpandTemplates(templates, "/workspace", false)
	if len(configs) != 2 {
		t.Fatalf("ExpandTemplates returned %d configs, want 2", len(configs))
	}

	// Regular template: iterate fields empty.
	if configs[0].Iterate != "" {
		t.Errorf("configs[0].Iterate = %q, want empty", configs[0].Iterate)
	}

	// Iterate template: iterate and output-pattern set, workspace dir set.
	if configs[1].Iterate != "groups" {
		t.Errorf("configs[1].Iterate = %q, want groups", configs[1].Iterate)
	}
	if configs[1].OutputPattern != "./gv/{{ .Key }}.yml" {
		t.Errorf("configs[1].OutputPattern = %q", configs[1].OutputPattern)
	}
	if configs[1].WorkspaceDir != "/workspace" {
		t.Errorf("configs[1].WorkspaceDir = %q, want /workspace", configs[1].WorkspaceDir)
	}
}
