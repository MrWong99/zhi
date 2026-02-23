package core

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
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
	Logger().Debug("exporting configuration",
		"format", cfg.Format,
		"template", cfg.TemplatePath,
		"output", cfg.OutputPath,
		"prefix", cfg.Prefix,
		"dry_run", cfg.DryRun)

	var content string
	var err error
	var opts *exportFileOptions

	if cfg.Format != "" {
		// Use a built-in format.
		content, err = renderBuiltinFormat(tree, cfg.Format, cfg.Prefix)
		if err != nil {
			return nil, fmt.Errorf("rendering format %q: %w", cfg.Format, err)
		}
	} else if cfg.TemplatePath != "" {
		// Use a template file. Pass opts so fileACL/fileMode can capture values.
		opts = &exportFileOptions{}
		content, err = renderTemplateFile(tree, cfg.TemplatePath, opts)
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
		if err := writeExportFile(cfg.OutputPath, []byte(content), opts); err != nil {
			return nil, err
		}
		Logger().Debug("export written", "path", cfg.OutputPath)
	}

	return result, nil
}

// ExportAll runs all exports defined in the workspace configuration.
// If workspaceDir is provided in any config and no configs are dry-run,
// a rollback snapshot is saved before writing files.
func ExportAll(ctx context.Context, tree *TreeData, configs []ExportRunConfig) ([]*ExportResult, error) {
	Logger().Info("running all exports", "count", len(configs))

	// Save rollback snapshot before writing (skip for dry-run).
	anyWrite := false
	var outputPaths []string
	for _, cfg := range configs {
		if !cfg.DryRun && cfg.OutputPath != "" && cfg.OutputPath != "-" {
			anyWrite = true
		}
		outputPaths = append(outputPaths, cfg.OutputPath)
	}
	if anyWrite && len(outputPaths) > 0 {
		// Determine workspace dir from first output path's parent.
		wsDir := filepath.Dir(outputPaths[0])
		if err := SaveRollbackSnapshot(wsDir, outputPaths); err != nil {
			Logger().Warn("failed to save rollback snapshot", "error", err)
			// Non-fatal: continue with export even if snapshot fails.
		}
	}

	results := make([]*ExportResult, 0, len(configs))
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
// If opts is non-nil, template functions like fileGroupID and fileMode will
// capture their values into it.
func renderTemplateFile(td *TreeData, path string, opts *exportFileOptions) (string, error) {
	if containsPathTraversal(path) {
		return "", fmt.Errorf("template path %q: path traversal (.. segments) is not allowed", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New(filepath.Base(path)).Funcs(templateFuncMap(opts)).Parse(string(data))
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
		tmplText = `{{ .Nested "" | mustToPrettyJson }}`
		if prefix != "" {
			tmplText = fmt.Sprintf(`{{ .Nested %q | mustToPrettyJson }}`, prefix)
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

	tmpl, err := template.New("builtin").Funcs(templateFuncMap(nil)).Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parsing built-in template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, td); err != nil {
		return "", fmt.Errorf("executing built-in template: %w", err)
	}
	return buf.String(), nil
}

// exportFileOptions holds file-level overrides that template functions can set
// during rendering. These are applied when writing the exported file to disk.
type exportFileOptions struct {
	acl  []string    // POSIX ACL entries to apply via setfacl -m
	mode os.FileMode // file permissions; 0 means use default 0o644
}

// getACL returns the ACL entries, or nil if opts is nil.
func (o *exportFileOptions) getACL() []string {
	if o == nil {
		return nil
	}
	return o.acl
}

// templateFuncMap returns the function map available in all export templates.
// It starts with all Sprig template functions and adds zhi-specific functions
// for formats that Sprig does not provide (YAML, TOML, dotenv, shell quoting).
// If opts is non-nil, the fileACL and fileMode functions will capture their
// values into opts for use during file writing.
func templateFuncMap(opts *exportFileOptions) template.FuncMap {
	// Start with Sprig's text/template function map as the base.
	// This provides 100+ functions including string manipulation, math,
	// date/time, crypto, regex, list/dict operations, and more.
	// See https://masterminds.github.io/sprig/ for the full list.
	fm := sprig.TxtFuncMap()

	// Add zhi-specific functions that have no Sprig equivalent.
	fm["toYAML"] = toYAML
	fm["toTOML"] = toTOML
	fm["toDotenv"] = toDotenv
	fm["shellQuote"] = shellQuote
	fm["fileACL"] = func(entry string) string {
		if opts != nil {
			opts.acl = append(opts.acl, entry)
		}
		return ""
	}
	fm["fileMode"] = func(mode int) string {
		if opts != nil {
			opts.mode = os.FileMode(mode)
		}
		return ""
	}

	return fm
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
	keys := slices.Sorted(maps.Keys(m))

	var sb strings.Builder
	for _, k := range keys {
		envKey := strings.ToUpper(strings.ReplaceAll(k, "/", "_"))
		fmt.Fprintf(&sb, "%s=%v\n", envKey, m[k])
	}
	return strings.TrimSpace(sb.String()), nil
}

// shellQuote returns a shell-safe single-quoted string.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// writeExportFile writes data to outputPath with security hardening:
//   - Rejects paths containing ".." traversal
//   - Resolves symlinks to determine the real destination
//   - Uses atomic writes (temp file + rename) to prevent partial writes
//   - Sets file permissions (default 0644, overridable via opts.mode)
//   - Optionally sets file group ID via opts.groupID
func writeExportFile(outputPath string, data []byte, opts *exportFileOptions) error {
	// Reject path traversal in the raw (un-normalized) path.
	if containsPathTraversal(outputPath) {
		return fmt.Errorf("export output path %q: path traversal (.. segments) is not allowed", outputPath)
	}
	// Normalize to a canonical path for all subsequent operations.
	outputPath = filepath.Clean(outputPath)

	// Resolve the directory, creating it if needed.
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Resolve symlinks in the output path to determine real destination.
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}
	realPath := filepath.Join(realDir, filepath.Base(outputPath))

	// Atomic write: write to temp file, then rename.
	tmp, err := os.CreateTemp(realDir, ".zhi-export-*")
	if err != nil {
		return fmt.Errorf("creating temp file for export: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing export temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing export temp file: %w", err)
	}

	// Set permissions before rename.
	mode := os.FileMode(0o644)
	if opts != nil && opts.mode != 0 {
		mode = opts.mode
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting export file permissions: %w", err)
	}

	if err := os.Rename(tmpName, realPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming export temp file: %w", err)
	}

	// Apply POSIX ACL entries if requested.
	for _, entry := range opts.getACL() {
		// Apply to the file itself.
		cmd := exec.Command("setfacl", "-m", entry, realPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("setting ACL %q on %s: %w (%s)", entry, realPath, err, strings.TrimSpace(string(out)))
		}
		// Apply to the parent directory with execute permission added,
		// so that the ACL subject can traverse the directory and delete
		// the file (e.g. for Docker volume mounts where the container
		// needs to remove consumed files).
		dirEntry := aclEntryWithExecute(entry)
		cmd = exec.Command("setfacl", "-m", dirEntry, realDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("setting ACL %q on directory %s: %w (%s)", dirEntry, realDir, err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

// aclEntryWithExecute takes an ACL entry like "g:10668:rw" and returns it
// with execute permission added: "g:10668:rwx". This is needed for directory
// ACLs where execute means traverse permission. If the entry doesn't match
// the expected format, it is returned unchanged.
func aclEntryWithExecute(entry string) string {
	// ACL entries have the form type:qualifier:perms (e.g. "g:10668:rw").
	i := strings.LastIndex(entry, ":")
	if i < 0 {
		return entry
	}
	perms := entry[i+1:]
	if !strings.Contains(perms, "x") {
		perms += "x"
	}
	return entry[:i+1] + perms
}
