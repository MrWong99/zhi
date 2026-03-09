package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// componentNamePattern defines valid component names: starts with a lowercase
// letter, may contain lowercase letters, digits, and hyphens.
var componentNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ProviderRef identifies a provider and its options in a workspace config.
type ProviderRef struct {
	Provider string         `yaml:"provider" json:"provider"`
	Options  map[string]any `yaml:"options,omitempty" json:"options,omitempty"`
}

// ExportTemplate describes a template-based export target.
type ExportTemplate struct {
	Name          string `yaml:"name" json:"name"`
	Template      string `yaml:"template,omitempty" json:"template,omitempty"`
	Format        string `yaml:"format,omitempty" json:"format,omitempty"` // built-in format: json, yaml, toml, dotenv
	Output        string `yaml:"output" json:"output"`
	Prefix        string `yaml:"prefix,omitempty" json:"prefix,omitempty"`                // only export paths under this prefix
	Iterate       string `yaml:"iterate,omitempty" json:"iterate,omitempty"`              // tree prefix to iterate over (direct children)
	OutputPattern string `yaml:"output-pattern,omitempty" json:"outputPattern,omitempty"` // Go template for per-child output path
}

// ExportConfig holds the export section of a workspace config.
type ExportConfig struct {
	Templates []ExportTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// ApplyTargetConfig holds the configuration for a single apply target.
type ApplyTargetConfig struct {
	Command   string            `yaml:"command,omitempty" json:"command,omitempty"`
	Workdir   string            `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	PreExport bool              `yaml:"pre-export,omitempty" json:"pre-export,omitempty"`
	PreCheck  []string          `yaml:"pre-check,omitempty" json:"pre-check,omitempty"`
	Env       map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Timeout   int               `yaml:"timeout,omitempty" json:"timeout,omitempty"` // seconds, 0 = no timeout
}

// ApplyConfig holds the apply section of a workspace config. It supports
// both a simple form (single target) and named targets.
//
// Simple form:
//
//	apply:
//	  command: "docker compose up -d"
//	  pre-export: true
//
// Named targets:
//
//	apply:
//	  default:
//	    command: "docker compose up -d"
//	  destroy:
//	    command: "docker compose down -v"
type ApplyConfig struct {
	// Simple form fields (used when no named targets).
	Command   string            `yaml:"command,omitempty" json:"command,omitempty"`
	Workdir   string            `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	PreExport bool              `yaml:"pre-export,omitempty" json:"pre-export,omitempty"`
	PreCheck  []string          `yaml:"pre-check,omitempty" json:"pre-check,omitempty"`
	Env       map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Timeout   int               `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Named targets (keyed by target name).
	Targets map[string]ApplyTargetConfig `yaml:"targets,omitempty" json:"targets,omitempty"`
}

// ResolveTarget returns the ApplyTargetConfig for the given target name.
// If name is empty, it uses "default". For the simple form (no named
// targets), the top-level fields are returned as the "default" target.
func (ac *ApplyConfig) ResolveTarget(name string) (ApplyTargetConfig, error) {
	if name == "" {
		name = "default"
	}

	// If named targets exist, look up by name.
	if len(ac.Targets) > 0 {
		t, ok := ac.Targets[name]
		if !ok {
			return ApplyTargetConfig{}, fmt.Errorf("unknown apply target %q (available: %v)", name, slices.Sorted(maps.Keys(ac.Targets)))
		}
		return t, nil
	}

	// Simple form: only "default" is valid.
	if name != "default" {
		return ApplyTargetConfig{}, fmt.Errorf("unknown apply target %q (no named targets defined)", name)
	}

	if ac.Command == "" {
		return ApplyTargetConfig{}, fmt.Errorf("no apply command configured; add an apply section to zhi.yaml")
	}

	return ApplyTargetConfig{
		Command:   ac.Command,
		Workdir:   ac.Workdir,
		PreExport: ac.PreExport,
		PreCheck:  ac.PreCheck,
		Env:       ac.Env,
		Timeout:   ac.Timeout,
	}, nil
}

// PluginsConfig holds the optional plugin discovery configuration.
type PluginsConfig struct {
	Directories []string `yaml:"directories,omitempty" json:"directories,omitempty"`
}

// WorkspaceConfig is the parsed representation of a zhi.yaml (or zhi.json)
// workspace configuration file.
type WorkspaceConfig struct {
	Version    string         `yaml:"version" json:"version"`
	Config     ProviderRef    `yaml:"config" json:"config"`
	Transform  []ProviderRef  `yaml:"transform,omitempty" json:"transform,omitempty"`
	Store      ProviderRef    `yaml:"store,omitempty" json:"store"`
	UI         ProviderRef    `yaml:"ui,omitempty" json:"ui"`
	Components []ComponentDef `yaml:"components,omitempty" json:"components,omitempty"`
	Export     ExportConfig   `yaml:"export,omitempty" json:"export"`
	Apply      ApplyConfig    `yaml:"apply,omitempty" json:"apply"`
	Plugins    PluginsConfig  `yaml:"plugins,omitempty" json:"plugins"`

	// Dir is the absolute directory containing the workspace config file.
	// It is set during loading and not part of the config file itself.
	Dir string `yaml:"-" json:"-"`
}

// PluginDirectories returns the configured plugin directories, falling
// back to the default (~/.zhi/plugins/) if none are specified.
func (ws *WorkspaceConfig) PluginDirectories() []string {
	if len(ws.Plugins.Directories) > 0 {
		return ws.Plugins.Directories
	}
	if d := DefaultPluginDir(); d != "" {
		return []string{d}
	}
	return nil
}

// LoadWorkspace finds and parses a zhi.yaml (or zhi.json) in dir or any
// parent directory. It returns the parsed workspace configuration.
func LoadWorkspace(dir string) (*WorkspaceConfig, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}

	configPath, err := findWorkspaceFile(absDir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading workspace config: %w", err)
	}

	ws, err := parseWorkspaceConfig(configPath, data)
	if err != nil {
		return nil, err
	}
	ws.Dir = filepath.Dir(configPath)
	if abs, err := filepath.Abs(ws.Dir); err == nil {
		ws.Dir = abs
	}
	return ws, nil
}

// findWorkspaceFile walks up from dir looking for zhi.yaml or zhi.json.
func findWorkspaceFile(dir string) (string, error) {
	candidates := []string{"zhi.yaml", "zhi.yml", "zhi.json"}
	current := dir
	for {
		for _, name := range candidates {
			p := filepath.Join(current, name)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // reached filesystem root
		}
		current = parent
	}
	return "", fmt.Errorf("no zhi.yaml or zhi.json found in %s or any parent directory", dir)
}

// parseWorkspaceConfig parses either YAML or JSON based on the file extension.
func parseWorkspaceConfig(path string, data []byte) (*WorkspaceConfig, error) {
	var ws WorkspaceConfig
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &ws); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &ws); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s", ext)
	}
	return &ws, nil
}

// ValidateWorkspace checks a workspace configuration for correctness. It
// verifies that referenced providers exist in the registry, component
// definitions are valid, and template files exist on disk.
func ValidateWorkspace(ws *WorkspaceConfig, reg *Registry) error {
	var errs []error

	if ws.Version != "1" {
		errs = append(errs, fmt.Errorf("unsupported workspace version: %q", ws.Version))
	}

	// Check config provider exists.
	if ws.Config.Provider == "" {
		errs = append(errs, errors.New("config provider is required"))
	} else if _, err := reg.ConfigProvider(ws.Dir, ws.Config.Provider, nil); err != nil {
		errs = append(errs, fmt.Errorf("config provider: %w", err))
	}

	// Check transform providers exist.
	for i, t := range ws.Transform {
		if t.Provider == "" {
			errs = append(errs, fmt.Errorf("transform[%d]: provider name is required", i))
		} else if _, err := reg.TransformProvider(ws.Dir, t.Provider, nil); err != nil {
			errs = append(errs, fmt.Errorf("transform[%d]: %w", i, err))
		}
	}

	// Check store provider exists (if specified).
	if ws.Store.Provider != "" {
		if _, err := reg.StoreProvider(ws.Dir, ws.Store.Provider, nil); err != nil {
			errs = append(errs, fmt.Errorf("store provider: %w", err))
		}
	}

	// Check UI provider exists (if specified).
	if ws.UI.Provider != "" {
		if _, err := reg.UIProvider(ws.Dir, ws.UI.Provider, nil); err != nil {
			errs = append(errs, fmt.Errorf("ui provider: %w", err))
		}
	}

	// Validate component definitions.
	for i, c := range ws.Components {
		if c.Name == "" {
			errs = append(errs, fmt.Errorf("components[%d]: name is required", i))
		} else if !componentNamePattern.MatchString(c.Name) {
			errs = append(errs, fmt.Errorf("components[%d]: invalid name %q (must match [a-z][a-z0-9-]*)", i, c.Name))
		}
	}
	if _, err := NewComponentManager(ws.Components); err != nil {
		errs = append(errs, fmt.Errorf("components: %w", err))
	}

	// Validate plugin directories: reject paths containing ".." traversal.
	for _, dir := range ws.Plugins.Directories {
		if containsPathTraversal(dir) {
			errs = append(errs, fmt.Errorf("plugin directory %q: path traversal (.. segments) is not allowed", dir))
		}
	}

	// Validate export templates.
	for i, tmpl := range ws.Export.Templates {
		// Check that template files exist on disk (only for templates with a file, not built-in formats).
		if tmpl.Template != "" {
			tmplPath := tmpl.Template
			if !filepath.IsAbs(tmplPath) {
				tmplPath = filepath.Join(ws.Dir, tmplPath)
			}
			if _, err := os.Stat(tmplPath); err != nil {
				errs = append(errs, fmt.Errorf("export template %q: file %q not found", tmpl.Name, tmpl.Template))
			}
		}

		// Validate iterate/output-pattern consistency.
		if tmpl.Iterate != "" && tmpl.OutputPattern == "" {
			errs = append(errs, fmt.Errorf("export[%d] %q: iterate requires output-pattern", i, tmpl.Name))
		}
		if tmpl.OutputPattern != "" && tmpl.Iterate == "" {
			errs = append(errs, fmt.Errorf("export[%d] %q: output-pattern requires iterate", i, tmpl.Name))
		}
		if tmpl.Iterate != "" && tmpl.Output != "" {
			errs = append(errs, fmt.Errorf("export[%d] %q: iterate templates must not set output (use output-pattern)", i, tmpl.Name))
		}
		if tmpl.OutputPattern != "" {
			if containsPathTraversal(tmpl.OutputPattern) {
				errs = append(errs, fmt.Errorf("export[%d] %q: output-pattern %q contains path traversal", i, tmpl.Name, tmpl.OutputPattern))
			}
			if _, parseErr := template.New("").Parse(tmpl.OutputPattern); parseErr != nil {
				errs = append(errs, fmt.Errorf("export[%d] %q: invalid output-pattern template: %w", i, tmpl.Name, parseErr))
			}
		}
	}

	return errors.Join(errs...)
}

// containsPathTraversal reports whether a path contains ".." segments.
func containsPathTraversal(path string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..")
}
