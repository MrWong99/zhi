// Package manifest defines the plugin manifest schema (zhi-plugin.yaml) and
// provides parsing and validation for plugin metadata.
package manifest

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the current manifest schema version.
const SchemaVersion = "1"

// Valid plugin types.
const (
	TypeConfig    = "config"
	TypeTransform = "transform"
	TypeStore     = "store"
	TypeUI        = "ui"
)

// validTypes is the set of accepted plugin types.
var validTypes = map[string]bool{
	TypeConfig:    true,
	TypeTransform: true,
	TypeStore:     true,
	TypeUI:        true,
}

// namePattern matches valid plugin names: lowercase letter, then lowercase
// alphanumeric or hyphens. Minimum 2 characters.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// semverPattern matches a basic semver string (major.minor.patch with optional
// pre-release suffix).
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// PluginManifest describes a published plugin's metadata. It is serialised as
// zhi-plugin.yaml in the project root and as the OCI config blob.
type PluginManifest struct {
	// SchemaVersion allows forward-compatible manifest evolution.
	SchemaVersion string `yaml:"schemaVersion,omitempty" json:"schemaVersion,omitempty"`

	// Name is the plugin's short name (e.g. "ansible-config").
	Name string `yaml:"name" json:"name"`

	// Type is one of: config, transform, store, ui.
	Type string `yaml:"type" json:"type"`

	// Version is a semver version string (e.g. "1.2.0").
	Version string `yaml:"version" json:"version"`

	// ZhiProtocolVersion is the zhi plugin protocol version.
	ZhiProtocolVersion string `yaml:"zhiProtocolVersion,omitempty" json:"zhiProtocolVersion,omitempty"`

	// Description is a short summary of what the plugin does.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Author is the plugin publisher's name or organisation.
	Author string `yaml:"author,omitempty" json:"author,omitempty"`

	// License is the SPDX license identifier (e.g. "Apache-2.0").
	License string `yaml:"license,omitempty" json:"license,omitempty"`

	// Homepage is a URL for the plugin's documentation or repository.
	Homepage string `yaml:"homepage,omitempty" json:"homepage,omitempty"`

	// Keywords are search terms for marketplace discovery.
	Keywords []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`

	// Runtime declares non-Go runtime dependencies.
	Runtime *RuntimeRequirement `yaml:"runtime,omitempty" json:"runtime,omitempty"`

	// MinZhiVersion is the minimum zhi version that supports this plugin.
	MinZhiVersion string `yaml:"minZhiVersion,omitempty" json:"minZhiVersion,omitempty"`

	// Binaries maps platform strings to binary paths (for publishing).
	Binaries map[string]string `yaml:"binaries,omitempty" json:"binaries,omitempty"`
}

// RuntimeRequirement declares a non-Go runtime needed by the plugin binary.
type RuntimeRequirement struct {
	// Type is the runtime name: java, python, node, etc.
	Type string `yaml:"type" json:"type"`
	// Version is a semver constraint like ">=17" or "^3.10".
	Version string `yaml:"version" json:"version"`
	// Bundled indicates whether the runtime is included in the OCI artifact.
	Bundled bool `yaml:"bundled" json:"bundled"`
}

// LoadFile reads and parses a manifest from the given file path.
func LoadFile(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	return Parse(data)
}

// Parse unmarshals YAML data into a PluginManifest and validates it.
func Parse(data []byte) (*PluginManifest, error) {
	var m PluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}
	if m.SchemaVersion == "" {
		m.SchemaVersion = SchemaVersion
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks that the manifest has all required fields with valid values.
func (m *PluginManifest) Validate() error {
	var errs []string

	if m.Name == "" {
		errs = append(errs, "name is required")
	} else if !namePattern.MatchString(m.Name) {
		errs = append(errs, fmt.Sprintf("name %q must match [a-z][a-z0-9-]*", m.Name))
	}

	if m.Type == "" {
		errs = append(errs, "type is required")
	} else if !validTypes[m.Type] {
		errs = append(errs, fmt.Sprintf("type %q must be one of: config, transform, store, ui", m.Type))
	}

	if m.Version == "" {
		errs = append(errs, "version is required")
	} else if !semverPattern.MatchString(m.Version) {
		errs = append(errs, fmt.Sprintf("version %q is not valid semver", m.Version))
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid plugin manifest: %s", strings.Join(errs, "; "))
	}
	return nil
}

// BinaryName returns the expected binary name for this plugin following the
// zhi flat naming convention: zhi-<type>-<name>.
func (m *PluginManifest) BinaryName() string {
	return fmt.Sprintf("zhi-%s-%s", m.Type, m.Name)
}

// Marshal serialises the manifest as YAML.
func (m *PluginManifest) Marshal() ([]byte, error) {
	return yaml.Marshal(m)
}

// IsValidPluginType returns whether the given string is a known plugin type.
func IsValidPluginType(t string) bool {
	return validTypes[t]
}

// IsValidName returns whether the given name matches the plugin naming rules.
func IsValidName(name string) bool {
	return namePattern.MatchString(name)
}

// IsValidSemver returns whether the given string is valid semver.
func IsValidSemver(v string) bool {
	return semverPattern.MatchString(v)
}
