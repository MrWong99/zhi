// Package registry provides registry configuration and authentication for
// OCI registries used by zhi's plugin sharing system.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds sharing registry settings, typically loaded from
// ~/.zhi/config.yaml.
type Config struct {
	// Default is the default OCI registry host.
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	// Registries maps host names to per-registry configuration.
	Registries map[string]Entry `yaml:"registries,omitempty" json:"registries,omitempty"`
	// Marketplace holds marketplace connection settings.
	Marketplace MarketplaceConfig `yaml:"marketplace,omitempty" json:"marketplace,omitempty"`
}

// MarketplaceConfig holds configuration for connecting to a marketplace server.
type MarketplaceConfig struct {
	// URL is the base URL of the marketplace (e.g. "https://marketplace.zhi.dev").
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
	// APIKey is the API key for authenticated operations (publishing, registration).
	APIKey string `yaml:"apiKey,omitempty" json:"apiKey,omitempty"`
}

// Entry holds authentication and options for a single OCI registry.
type Entry struct {
	// Username for basic authentication.
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	// Password or token for authentication. Never logged.
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	// Insecure allows plain HTTP (for development registries).
	Insecure bool `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

// Store manages registry credentials on disk. It is safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
	cfg  Config
}

// DefaultConfigDir returns the default zhi config directory (~/.zhi/).
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zhi")
}

// DefaultConfigPath returns the default path for the sharing config file.
func DefaultConfigPath() string {
	dir := DefaultConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}

// NewStore creates a credential store backed by the given file path. If the
// file does not exist, it starts with an empty configuration.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		cfg:  Config{Registries: make(map[string]Entry)},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if err := yaml.Unmarshal(data, &s.cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if s.cfg.Registries == nil {
		s.cfg.Registries = make(map[string]Entry)
	}
	return s, nil
}

// Login stores credentials for the given registry host.
func (s *Store) Login(host, username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Registries[host] = Entry{
		Username: username,
		Password: password,
	}
	return s.save()
}

// Logout removes credentials for the given registry host.
func (s *Store) Logout(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cfg.Registries[host]; !ok {
		return fmt.Errorf("not logged in to %s", host)
	}
	delete(s.cfg.Registries, host)
	return s.save()
}

// GetCredentials returns the stored credentials for a registry host.
func (s *Store) GetCredentials(host string) (username, password string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.cfg.Registries[host]
	if !exists {
		return "", "", false
	}
	return entry.Username, entry.Password, true
}

// ListHosts returns all registry hosts that have stored credentials.
func (s *Store) ListHosts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	hosts := make([]string, 0, len(s.cfg.Registries))
	for h := range s.cfg.Registries {
		hosts = append(hosts, h)
	}
	return hosts
}

// DefaultRegistry returns the configured default registry host.
func (s *Store) DefaultRegistry() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Default
}

// MarketplaceURL returns the configured marketplace URL, or empty if not set.
func (s *Store) MarketplaceURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Marketplace.URL
}

// MarketplaceAPIKey returns the configured marketplace API key, or empty if not set.
func (s *Store) MarketplaceAPIKey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Marketplace.APIKey
}

// save writes the current configuration to disk. Must be called with s.mu held.
func (s *Store) save() error {
	data, err := yaml.Marshal(&s.cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
