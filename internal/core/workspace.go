package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProviderRef identifies a provider and its options in a workspace config.
type ProviderRef struct {
	Provider string         `yaml:"provider" json:"provider"`
	Options  map[string]any `yaml:"options,omitempty" json:"options,omitempty"`
}

// ExportTemplate describes a template-based export target.
type ExportTemplate struct {
	Name     string `yaml:"name" json:"name"`
	Template string `yaml:"template" json:"template"`
	Output   string `yaml:"output" json:"output"`
}

// ExportConfig holds the export section of a workspace config.
type ExportConfig struct {
	Templates []ExportTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// ApplyConfig holds the apply section of a workspace config.
type ApplyConfig struct {
	Command string `yaml:"command,omitempty" json:"command,omitempty"`
	Workdir string `yaml:"workdir,omitempty" json:"workdir,omitempty"`
}

// WorkspaceConfig is the parsed representation of a zhi.yaml (or zhi.json)
// workspace configuration file.
type WorkspaceConfig struct {
	Version    string         `yaml:"version" json:"version"`
	Config     ProviderRef    `yaml:"config" json:"config"`
	Transform  []ProviderRef  `yaml:"transform,omitempty" json:"transform,omitempty"`
	Store      ProviderRef    `yaml:"store,omitempty" json:"store,omitempty"`
	Components []ComponentDef `yaml:"components,omitempty" json:"components,omitempty"`
	Export     ExportConfig   `yaml:"export,omitempty" json:"export,omitempty"`
	Apply      ApplyConfig    `yaml:"apply,omitempty" json:"apply,omitempty"`

	// Dir is the absolute directory containing the workspace config file.
	// It is set during loading and not part of the config file itself.
	Dir string `yaml:"-" json:"-"`
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
	} else if _, err := reg.ConfigProvider(ws.Config.Provider, nil); err != nil {
		errs = append(errs, fmt.Errorf("config provider: %w", err))
	}

	// Check transform providers exist.
	for i, t := range ws.Transform {
		if t.Provider == "" {
			errs = append(errs, fmt.Errorf("transform[%d]: provider name is required", i))
		} else if _, err := reg.TransformProvider(t.Provider, nil); err != nil {
			errs = append(errs, fmt.Errorf("transform[%d]: %w", i, err))
		}
	}

	// Check store provider exists (if specified).
	if ws.Store.Provider != "" {
		if _, err := reg.StoreProvider(ws.Store.Provider, nil); err != nil {
			errs = append(errs, fmt.Errorf("store provider: %w", err))
		}
	}

	// Validate component definitions.
	if _, err := NewComponentManager(ws.Components); err != nil {
		errs = append(errs, fmt.Errorf("components: %w", err))
	}

	// Check that template files exist on disk.
	for _, tmpl := range ws.Export.Templates {
		tmplPath := tmpl.Template
		if !filepath.IsAbs(tmplPath) {
			tmplPath = filepath.Join(ws.Dir, tmplPath)
		}
		if _, err := os.Stat(tmplPath); err != nil {
			errs = append(errs, fmt.Errorf("export template %q: file %q not found", tmpl.Name, tmpl.Template))
		}
	}

	return errors.Join(errs...)
}
