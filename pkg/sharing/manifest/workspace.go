package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkspaceManifest describes a published workspace. It is serialised as the
// OCI config blob for workspace artifacts and optionally as
// zhi-workspace.yaml in the project root.
type WorkspaceManifest struct {
	// Name is the workspace's short name.
	Name string `yaml:"name" json:"name"`
	// Version is a semver version string.
	Version string `yaml:"version" json:"version"`
	// Description is a short summary of the workspace.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Author is the workspace publisher's name or organisation.
	Author string `yaml:"author,omitempty" json:"author,omitempty"`
	// License is the SPDX license identifier.
	License string `yaml:"license,omitempty" json:"license,omitempty"`
	// Dependencies lists plugins required by this workspace.
	Dependencies []PluginDependency `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	// Tools lists external tools required by the workspace.
	Tools []ToolRequirement `yaml:"tools,omitempty" json:"tools,omitempty"`
	// Keywords are search terms for marketplace discovery.
	Keywords []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
}

// PluginDependency is an OCI reference to a required plugin.
type PluginDependency struct {
	// Ref is the OCI reference (e.g. "oci://ghcr.io/org/plugin:v1.0").
	Ref string `yaml:"ref" json:"ref"`
	// Type is the plugin type (config, transform, store, ui).
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
	// Optional indicates the workspace works without this plugin.
	Optional bool `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// ToolRequirement declares an external tool needed by the workspace.
type ToolRequirement struct {
	// Name is the tool binary name (e.g. "kubectl").
	Name string `yaml:"name" json:"name"`
	// Version is a version constraint (e.g. ">=1.28").
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// LoadWorkspaceFile reads and parses a workspace manifest from the given path.
func LoadWorkspaceFile(path string) (*WorkspaceManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workspace manifest: %w", err)
	}
	return ParseWorkspace(data)
}

// ParseWorkspace unmarshals YAML data into a WorkspaceManifest and validates it.
func ParseWorkspace(data []byte) (*WorkspaceManifest, error) {
	var m WorkspaceManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing workspace manifest YAML: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks that the workspace manifest has all required fields.
func (m *WorkspaceManifest) Validate() error {
	var errs []string

	if m.Name == "" {
		errs = append(errs, "name is required")
	} else if !namePattern.MatchString(m.Name) {
		errs = append(errs, fmt.Sprintf("name %q must match [a-z][a-z0-9-]*", m.Name))
	}

	if m.Version == "" {
		errs = append(errs, "version is required")
	} else if !semverPattern.MatchString(m.Version) {
		errs = append(errs, fmt.Sprintf("version %q is not valid semver", m.Version))
	}

	for i, dep := range m.Dependencies {
		if dep.Ref == "" {
			errs = append(errs, fmt.Sprintf("dependency[%d]: ref is required", i))
		}
		if dep.Type != "" && !validTypes[dep.Type] {
			errs = append(errs, fmt.Sprintf("dependency[%d]: type %q must be one of: config, transform, store, ui", i, dep.Type))
		}
	}

	for i, tool := range m.Tools {
		if tool.Name == "" {
			errs = append(errs, fmt.Sprintf("tools[%d]: name is required", i))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid workspace manifest: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Marshal serialises the workspace manifest as YAML.
func (m *WorkspaceManifest) Marshal() ([]byte, error) {
	return yaml.Marshal(m)
}

// MarshalConfigJSON serialises the workspace manifest as JSON for OCI config blobs.
func (m *WorkspaceManifest) MarshalConfigJSON() ([]byte, error) {
	return json.Marshal(m)
}
