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
	// BinaryDigest is the SHA-256 digest of the installed binary, used
	// to detect post-install tampering at launch time.
	BinaryDigest string `json:"binaryDigest,omitempty"`
	// Signed indicates whether the artifact had a valid signature at install time.
	Signed bool `json:"signed,omitempty"`
	// SigningIdentity is the signer identity (e.g. email, OIDC subject).
	SigningIdentity string `json:"signingIdentity,omitempty"`
	// Pinned prevents automatic updates for this plugin.
	Pinned bool `json:"pinned,omitempty"`
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

// SetPinned sets the pinned state for a plugin.
func (s *Store) SetPinned(name string, pinned bool) error {
	p, err := s.Load(name)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("plugin %q is not installed", name)
	}
	p.Pinned = pinned
	return s.Save(p)
}

// BackupBinary moves the current plugin binary to the backup directory.
// The backup is named <binary>.{version} and stored in a .backup/
// subdirectory of pluginDir. Only the most recent backup is kept.
func BackupBinary(pluginDir, binaryName, version string) error {
	src := filepath.Join(pluginDir, binaryName)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // nothing to back up
	}

	backupDir := filepath.Join(pluginDir, ".backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("creating backup directory: %w", err)
	}

	dst := filepath.Join(backupDir, binaryName+"."+version)
	// Remove existing backup if present (only keep one).
	_ = os.Remove(dst)

	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("backing up binary: %w", err)
	}
	return nil
}

// RestoreBackup moves a backed-up binary back to the plugin directory.
// Returns the version string of the restored backup, or an error if no
// backup exists.
func RestoreBackup(pluginDir, binaryName string) (string, error) {
	backupDir := filepath.Join(pluginDir, ".backup")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no backup found for %s", binaryName)
		}
		return "", fmt.Errorf("reading backup directory: %w", err)
	}

	prefix := binaryName + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			version := name[len(prefix):]
			src := filepath.Join(backupDir, name)
			dst := filepath.Join(pluginDir, binaryName)
			if err := os.Rename(src, dst); err != nil {
				return "", fmt.Errorf("restoring backup: %w", err)
			}
			return version, nil
		}
	}

	return "", fmt.Errorf("no backup found for %s", binaryName)
}

// pluginPath returns the file path for a plugin's metadata.
func (s *Store) pluginPath(name string) string {
	return filepath.Join(s.dir, name+".json")
}
