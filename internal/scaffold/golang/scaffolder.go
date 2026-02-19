// Package golang provides the Go language scaffolder for the zhi plugin
// scaffolding system. It generates a complete, standalone Go project that
// implements a zhi plugin and can be pushed as its own repository.
package golang

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/MrWong99/zhi/internal/scaffold"
)

//go:embed all:templates
var templateFS embed.FS

func init() {
	scaffold.Register(&goScaffolder{})
}

// scaffoldFile maps a template file to its output path. OutputPath may
// contain template variables that are resolved before use.
type scaffoldFile struct {
	// TemplatePath is the path within the embedded FS (relative to templates/).
	TemplatePath string
	// OutputPath is the destination relative to the project root.
	// It is rendered as a template itself to support dynamic paths.
	OutputPath string
}

// commonFiles are generated for every plugin type.
func commonFiles() []scaffoldFile {
	return []scaffoldFile{
		{TemplatePath: "common/go.mod.tmpl", OutputPath: "go.mod"},
		{TemplatePath: "common/Makefile.tmpl", OutputPath: "Makefile"},
		{TemplatePath: "common/gitignore.tmpl", OutputPath: ".gitignore"},
		{TemplatePath: "common/README.md.tmpl", OutputPath: "README.md"},
		{TemplatePath: "common/zhi-plugin.yaml.tmpl", OutputPath: "zhi-plugin.yaml"},
		{TemplatePath: "common/release.yml.tmpl", OutputPath: ".github/workflows/release.yml"},
	}
}

// typeFiles returns the templates specific to the given plugin type.
func typeFiles(pluginType, binaryName string) []scaffoldFile {
	return []scaffoldFile{
		{
			TemplatePath: pluginType + "/main.go.tmpl",
			OutputPath:   "cmd/" + binaryName + "/main.go",
		},
		{
			TemplatePath: pluginType + "/plugin.go.tmpl",
			OutputPath:   "pkg/plugin/plugin.go",
		},
		{
			TemplatePath: pluginType + "/plugin_test.go.tmpl",
			OutputPath:   "pkg/plugin/plugin_test.go",
		},
		{
			TemplatePath: pluginType + "/zhi.yaml.tmpl",
			OutputPath:   "workspace/zhi.yaml",
		},
		{
			TemplatePath: pluginType + "/defaults.yaml.tmpl",
			OutputPath:   "workspace/config/defaults.yaml",
		},
	}
}

type goScaffolder struct{}

func (g *goScaffolder) Language() string { return "go" }

func (g *goScaffolder) Scaffold(data scaffold.Data, outputDir string) error {
	// Refuse to overwrite an existing directory unless it is empty.
	if entries, err := os.ReadDir(outputDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("output directory %q already exists and is not empty", outputDir)
	}

	// Collect the file list.
	files := commonFiles()
	files = append(files, typeFiles(data.PluginType, data.BinaryName)...)

	for _, f := range files {
		if err := renderFile(data, outputDir, f); err != nil {
			return fmt.Errorf("rendering %s: %w", f.OutputPath, err)
		}
	}

	return nil
}

// renderFile reads a template from the embedded FS, renders it with data,
// and writes the result to the output directory.
func renderFile(data scaffold.Data, outputDir string, sf scaffoldFile) error {
	content, err := templateFS.ReadFile("templates/" + sf.TemplatePath)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", sf.TemplatePath, err)
	}

	// Use [[ ]] delimiters to avoid conflicts with Go source braces and
	// GitHub Actions ${{ }} syntax.
	tmpl, err := template.New(sf.TemplatePath).Delims("[[", "]]").Parse(string(content))
	if err != nil {
		return fmt.Errorf("parsing template %s: %w", sf.TemplatePath, err)
	}

	destPath := filepath.Join(outputDir, sf.OutputPath)

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", destPath, err)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("executing template %s: %w", sf.TemplatePath, err)
	}

	return nil
}
