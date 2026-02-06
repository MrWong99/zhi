package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TrustStore manages trusted signing keys stored in ~/.zhi/keys/.
// Each key file is a PEM-encoded public key used to verify signatures
// from a specific publisher.
//
// Directory layout:
//
//	~/.zhi/keys/
//	├── zhi-project.pub      # bundled zhi project key
//	└── custom/
//	    └── myorg.pub         # user-added organisation key
type TrustStore struct {
	dir string
}

// TrustedKey represents a public key loaded from the trust store.
type TrustedKey struct {
	// Name is the publisher or key name (derived from filename without extension).
	Name string
	// Path is the absolute path to the key file.
	Path string
	// Custom indicates the key is in the custom/ subdirectory (user-added).
	Custom bool
}

// NewTrustStore creates a trust store rooted at the given directory.
func NewTrustStore(dir string) *TrustStore {
	return &TrustStore{dir: dir}
}

// DefaultTrustStoreDir returns the default trust store directory (~/.zhi/keys/).
func DefaultTrustStoreDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zhi", "keys")
}

// ListKeys returns all trusted keys, sorted by name. Keys in the
// custom/ subdirectory are marked as Custom.
func (ts *TrustStore) ListKeys() ([]TrustedKey, error) {
	var keys []TrustedKey

	// Read top-level keys.
	topKeys, err := ts.readKeysInDir(ts.dir, false)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading trust store: %w", err)
	}
	keys = append(keys, topKeys...)

	// Read custom/ subdirectory.
	customDir := filepath.Join(ts.dir, "custom")
	customKeys, err := ts.readKeysInDir(customDir, true)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading custom keys: %w", err)
	}
	keys = append(keys, customKeys...)

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})
	return keys, nil
}

// GetKey returns the trusted key with the given name, checking both
// the top-level and custom/ directories.
func (ts *TrustStore) GetKey(name string) (*TrustedKey, error) {
	// Check top-level first.
	path := filepath.Join(ts.dir, name+".pub")
	if _, err := os.Stat(path); err == nil {
		return &TrustedKey{Name: name, Path: path, Custom: false}, nil
	}

	// Check custom/.
	path = filepath.Join(ts.dir, "custom", name+".pub")
	if _, err := os.Stat(path); err == nil {
		return &TrustedKey{Name: name, Path: path, Custom: true}, nil
	}

	return nil, fmt.Errorf("key %q not found in trust store", name)
}

// AddKey writes a public key to the custom/ subdirectory.
func (ts *TrustStore) AddKey(name string, keyData []byte) error {
	customDir := filepath.Join(ts.dir, "custom")
	if err := os.MkdirAll(customDir, 0o700); err != nil {
		return fmt.Errorf("creating custom keys directory: %w", err)
	}
	path := filepath.Join(customDir, name+".pub")
	if err := os.WriteFile(path, keyData, 0o600); err != nil {
		return fmt.Errorf("writing key file: %w", err)
	}
	return nil
}

// RemoveKey removes a key from the custom/ subdirectory. Top-level
// (bundled) keys cannot be removed.
func (ts *TrustStore) RemoveKey(name string) error {
	path := filepath.Join(ts.dir, "custom", name+".pub")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("custom key %q not found", name)
		}
		return fmt.Errorf("removing key: %w", err)
	}
	return nil
}

// readKeysInDir reads .pub files from a directory.
func (ts *TrustStore) readKeysInDir(dir string, custom bool) ([]TrustedKey, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var keys []TrustedKey
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".pub")
		keys = append(keys, TrustedKey{
			Name:   name,
			Path:   filepath.Join(dir, entry.Name()),
			Custom: custom,
		})
	}
	return keys, nil
}
