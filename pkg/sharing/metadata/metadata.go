// Package metadata manages the local installed-plugin metadata store at
// ~/.zhi/metadata/. Each installed plugin has a JSON file tracking its
// version, OCI reference, digest, and installation time.
package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// InstalledPlugin records metadata for a plugin installed from an OCI registry.
type InstalledPlugin struct {
	// Name is the plugin's short name.
	Name string `json:"name"`
	// Type is the plugin type: config, transform, store, ui.
	Type string `json:"type"`
	// Version is the installed semver version.
	Version string `json:"version"`
	// Ref is the full OCI reference the plugin was installed from.
	Ref string `json:"ref"`
	// Digest is the OCI manifest digest (e.g. "sha256:abc123...").
	Digest string `json:"digest"`
	// Platform is the OS/arch string (e.g. "linux/amd64").
	Platform string `json:"platform"`
	// InstalledAt is the installation timestamp.
	InstalledAt time.Time `json:"installedAt"`
	// Publisher is the plugin author or organisation.
	Publisher string `json:"publisher,omitempty"`
}

// Store manages plugin metadata files on disk.
type Store struct {
	dir string
}

// DefaultMetadataDir returns the default metadata directory (~/.zhi/metadata/).
func DefaultMetadataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zhi", "metadata")
}

// NewStore creates a metadata store rooted at the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Save writes plugin metadata to a JSON file named <plugin-name>.json.
func (s *Store) Save(p *InstalledPlugin) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("creating metadata directory: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling metadata: %w", err)
	}
	path := s.pluginPath(p.Name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}
	return nil
}

// Load reads metadata for the named plugin. Returns nil if not found.
func (s *Store) Load(name string) (*InstalledPlugin, error) {
	path := s.pluginPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading metadata: %w", err)
	}
	var p InstalledPlugin
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing metadata: %w", err)
	}
	return &p, nil
}

// Delete removes the metadata file for the named plugin.
func (s *Store) Delete(name string) error {
	path := s.pluginPath(name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("removing metadata: %w", err)
	}
	return nil
}

// List returns metadata for all installed plugins, sorted by name.
func (s *Store) List() ([]*InstalledPlugin, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing metadata directory: %w", err)
	}

	var plugins []*InstalledPlugin
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".json")]
		p, err := s.Load(name)
		if err != nil {
			continue
		}
		if p != nil {
			plugins = append(plugins, p)
		}
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	return plugins, nil
}

// pluginPath returns the file path for a plugin's metadata.
func (s *Store) pluginPath(name string) string {
	return filepath.Join(s.dir, name+".json")
}
