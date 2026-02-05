// Package lockfile implements the zhi-plugins.lock file format for
// reproducible plugin installations. A lock file pins exact OCI digests
// for each plugin dependency, enabling deterministic setups in CI/CD.
package lockfile

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Version is the current lock file format version.
const Version = 1

// LockFile represents the contents of a zhi-plugins.lock file.
type LockFile struct {
	// Version is the lock file format version.
	Version int `yaml:"version" json:"version"`
	// GeneratedAt is the timestamp when the lock file was generated.
	GeneratedAt time.Time `yaml:"generatedAt" json:"generatedAt"`
	// ZhiVersion is the version of zhi that generated the lock file.
	ZhiVersion string `yaml:"zhiVersion" json:"zhiVersion"`
	// Plugins lists the locked plugin entries.
	Plugins []LockedPlugin `yaml:"plugins" json:"plugins"`
}

// LockedPlugin is a single pinned plugin entry in the lock file.
type LockedPlugin struct {
	// Name is the plugin's short name.
	Name string `yaml:"name" json:"name"`
	// Type is the plugin type (config, transform, store, ui).
	Type string `yaml:"type" json:"type"`
	// Ref is the original OCI reference.
	Ref string `yaml:"ref" json:"ref"`
	// Digest is the pinned OCI manifest digest.
	Digest string `yaml:"digest" json:"digest"`
	// Platforms maps platform strings to per-platform binary digests.
	Platforms map[string]string `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	// Signed indicates whether the artifact had a valid signature.
	Signed bool `yaml:"signed" json:"signed"`
	// SigningIdentity is the signer identity (if signed).
	SigningIdentity string `yaml:"signingIdentity,omitempty" json:"signingIdentity,omitempty"`
}

// LoadFile reads and parses a lock file from the given path.
func LoadFile(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading lock file: %w", err)
	}
	return Parse(data)
}

// Parse unmarshals YAML data into a LockFile and validates it.
func Parse(data []byte) (*LockFile, error) {
	var lf LockFile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing lock file YAML: %w", err)
	}
	if err := lf.Validate(); err != nil {
		return nil, err
	}
	return &lf, nil
}

// Validate checks that the lock file has valid contents.
func (lf *LockFile) Validate() error {
	var errs []string

	if lf.Version == 0 {
		errs = append(errs, "version is required")
	} else if lf.Version != Version {
		errs = append(errs, fmt.Sprintf("unsupported lock file version %d (expected %d)", lf.Version, Version))
	}

	for i, p := range lf.Plugins {
		if p.Name == "" {
			errs = append(errs, fmt.Sprintf("plugins[%d]: name is required", i))
		}
		if p.Ref == "" {
			errs = append(errs, fmt.Sprintf("plugins[%d]: ref is required", i))
		}
		if p.Digest == "" {
			errs = append(errs, fmt.Sprintf("plugins[%d]: digest is required", i))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid lock file: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Marshal serialises the lock file as YAML.
func (lf *LockFile) Marshal() ([]byte, error) {
	return yaml.Marshal(lf)
}

// SaveFile writes the lock file to the given path.
func (lf *LockFile) SaveFile(path string) error {
	data, err := lf.Marshal()
	if err != nil {
		return fmt.Errorf("marshalling lock file: %w", err)
	}

	header := []byte("# zhi-plugins.lock (auto-generated, committed to version control)\n")
	content := append(header, data...)

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}
	return nil
}

// FindPlugin looks up a plugin by name in the lock file. Returns nil if not found.
func (lf *LockFile) FindPlugin(name string) *LockedPlugin {
	for i := range lf.Plugins {
		if lf.Plugins[i].Name == name {
			return &lf.Plugins[i]
		}
	}
	return nil
}

// VerifyDigest checks that a plugin's actual digest matches the locked digest.
func (lp *LockedPlugin) VerifyDigest(actualDigest string) error {
	if lp.Digest != actualDigest {
		return fmt.Errorf("digest mismatch for %s: locked %s, got %s", lp.Name, lp.Digest, actualDigest)
	}
	return nil
}
