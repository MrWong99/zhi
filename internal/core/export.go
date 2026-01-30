package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// ExportRunConfig holds the runtime configuration for a single export
// operation. It extends ExportTemplate with resolved paths and options.
type ExportRunConfig struct {
	// TemplatePath is the absolute path to a .tmpl file (empty for built-in formats).
	TemplatePath string
	// Format is a built-in format name (json, yaml, toml, dotenv). Empty when using a template file.
	Format string
	// OutputPath is the file to write to. "-" or empty means stdout.
	OutputPath string
	// Prefix limits export to paths under this prefix.
	Prefix string
	// AllComponents disables component filtering when true.
	AllComponents bool
	// DryRun prints output without writing to disk.
	DryRun bool
}

// ExportResult holds the result of a single export operation.
type ExportResult struct {
	// Name is a descriptive name for the export (template name or format).
	Name string
	// Content is the rendered output.
	Content string
	// OutputPath is the file path that was (or would be) written.
	OutputPath string
}

// Export renders a single export template or built-in format against the
// given tree and component manager. The tree is filtered through the
// ComponentManager unless AllComponents is set.
func Export(_ context.Context, tree *TreeData, cfg ExportRunConfig) (*ExportResult, error) {
	var content string
	var err error

	if cfg.Format != "" {
		// Use a built-in format.
		content, err = renderBuiltinFormat(tree, cfg.Format, cfg.Prefix)
		if err != nil {
			return nil, fmt.Errorf("rendering format %q: %w", cfg.Format, err)
		}
	} else if cfg.TemplatePath != "" {
		// Use a template file.
		content, err = renderTemplateFile(tree, cfg.TemplatePath)
		if err != nil {
			return nil, fmt.Errorf("rendering template %q: %w", cfg.TemplatePath, err)
		}
	} else {
		return nil, fmt.Errorf("export config must specify either a template or a format")
	}

	result := &ExportResult{
		Content:    content,
		OutputPath: cfg.OutputPath,
	}

	if cfg.Format != "" {
		result.Name = cfg.Format
	} else {
		result.Name = filepath.Base(cfg.TemplatePath)
	}

	// Write output if not dry-run and output is a file path.
	if !cfg.DryRun && cfg.OutputPath != "" && cfg.OutputPath != "-" {
		if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
			return nil, fmt.Errorf("creating output directory: %w", err)
		}
		if err := os.WriteFile(cfg.OutputPath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("writing output file: %w", err)
		}
	}

	return result, nil
}

// ExportAll runs all exports defined in the workspace configuration.
func ExportAll(ctx context.Context, tree *TreeData, configs []ExportRunConfig) ([]*ExportResult, error) {
	var results []*ExportResult
	for _, cfg := range configs {
		r, err := Export(ctx, tree, cfg)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}

// PrepareTreeData creates a TreeData from an engine, applying component
// filtering and optional prefix filtering.
func PrepareTreeData(ctx context.Context, eng *Engine, allComponents bool, prefix string) (*TreeData, error) {
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading tree: %w", err)
	}

	if err := eng.TransformForDisplay(ctx, tree); err != nil {
		return nil, fmt.Errorf("transforming tree: %w", err)
	}

	cm := eng.Components()
	var filtered *TreeData
	if allComponents {
		filtered = NewTreeData(tree, cm)
	} else {
		filtered = NewTreeData(cm.FilterTree(tree), cm)
	}

	if prefix != "" {
		filtered = newPrefixedTreeData(filtered, prefix)
	}

	return filtered, nil
}

// newPrefixedTreeData wraps a TreeData to limit it to paths under a prefix.
func newPrefixedTreeData(td *TreeData, prefix string) *TreeData {
	return &TreeData{
		tree:       &prefixTree{reader: td.tree, prefix: prefix},
		components: td.components,
	}
}

// prefixTree is a TreeReader that only exposes paths under a given prefix.
type prefixTree struct {
	reader config.TreeReader
	prefix string
}

func (pt *prefixTree) Get(path string) (config.Value, bool) {
	return pt.reader.Get(path)
}

func (pt *prefixTree) List() []string {
	p := pt.prefix
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	var out []string
	for _, path := range pt.reader.List() {
		if strings.HasPrefix(path, p) || path == pt.prefix {
			out = append(out, path)
		}
	}
	return out
}

// renderTemplateFile reads a template file and executes it with the TreeData.
func renderTemplateFile(td *TreeData, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New(filepath.Base(path)).Funcs(templateFuncMap()).Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, td); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}

// renderBuiltinFormat renders the tree using a built-in format template.
func renderBuiltinFormat(td *TreeData, format, prefix string) (string, error) {
	var tmplText string
	switch format {
	case "json":
		tmplText = `{{ .Nested "" | toJSON }}`
		if prefix != "" {
			tmplText = fmt.Sprintf(`{{ .Nested %q | toJSON }}`, prefix)
		}
	case "yaml":
		tmplText = `{{ .Nested "" | toYAML }}`
		if prefix != "" {
			tmplText = fmt.Sprintf(`{{ .Nested %q | toYAML }}`, prefix)
		}
	case "toml":
		tmplText = `{{ .Nested "" | toTOML }}`
		if prefix != "" {
			tmplText = fmt.Sprintf(`{{ .Nested %q | toTOML }}`, prefix)
		}
	case "dotenv":
		tmplText = `{{ .All | toDotenv }}`
	default:
		return "", fmt.Errorf("unknown format: %q (supported: json, yaml, toml, dotenv)", format)
	}

	tmpl, err := template.New("builtin").Funcs(templateFuncMap()).Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parsing built-in template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, td); err != nil {
		return "", fmt.Errorf("executing built-in template: %w", err)
	}
	return buf.String(), nil
}

// templateFuncMap returns the function map available in all export templates.
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"toJSON":        toJSON,
		"toJSONCompact": toJSONCompact,
		"toYAML":        toYAML,
		"toTOML":        toTOML,
		"toDotenv":      toDotenv,
		"quote":         shellQuote,
		"indent":        indentStr,
		"upper":         strings.ToUpper,
		"lower":         strings.ToLower,
		"replace":       strings.ReplaceAll,
	}
}

func toJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func toJSONCompact(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func toYAML(v any) (string, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func toTOML(v any) (string, error) {
	b, err := toml.Marshal(v)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// toDotenv renders a flat map as KEY=value lines. Keys are uppercased and
// "/" is replaced with "_".
func toDotenv(v any) (string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", fmt.Errorf("toDotenv requires a map[string]any, got %T", v)
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		envKey := strings.ToUpper(strings.ReplaceAll(k, "/", "_"))
		sb.WriteString(fmt.Sprintf("%s=%v\n", envKey, m[k]))
	}
	return strings.TrimSpace(sb.String()), nil
}

// shellQuote returns a shell-safe single-quoted string.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// indentStr indents every line of s by n spaces.
func indentStr(n int, s string) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}
