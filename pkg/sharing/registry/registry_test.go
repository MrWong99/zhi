package registry

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNewStoreEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	hosts := s.ListHosts()
	if len(hosts) != 0 {
		t.Errorf("expected no hosts, got %v", hosts)
	}
}

func TestLoginLogout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Login.
	if err := s.Login("ghcr.io", "user", "token123"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Check credentials.
	u, p, ok := s.GetCredentials("ghcr.io")
	if !ok {
		t.Fatal("GetCredentials returned false")
	}
	if u != "user" || p != "token123" {
		t.Errorf("got (%q, %q), want (%q, %q)", u, p, "user", "token123")
	}

	// Check file was written.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Logout.
	if err := s.Logout("ghcr.io"); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	_, _, ok = s.GetCredentials("ghcr.io")
	if ok {
		t.Error("expected credentials to be removed after logout")
	}
}

func TestLogoutNotLoggedIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := s.Logout("ghcr.io"); err == nil {
		t.Error("expected error for logout when not logged in")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Store 1: login.
	s1, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := s1.Login("ghcr.io", "user1", "pass1"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := s1.Login("docker.io", "user2", "pass2"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Store 2: reload from disk.
	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	hosts := s2.ListHosts()
	sort.Strings(hosts)
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %v", hosts)
	}
	if hosts[0] != "docker.io" || hosts[1] != "ghcr.io" {
		t.Errorf("hosts = %v", hosts)
	}

	u, p, ok := s2.GetCredentials("ghcr.io")
	if !ok || u != "user1" || p != "pass1" {
		t.Errorf("ghcr.io creds = (%q, %q, %v)", u, p, ok)
	}
}

func TestGetCredentialsNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, _, ok := s.GetCredentials("ghcr.io")
	if ok {
		t.Error("expected ok=false for unknown host")
	}
}

func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()
	if dir == "" {
		t.Skip("cannot determine home directory")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DefaultConfigDir() = %q, want absolute path", dir)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Skip("cannot determine home directory")
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultConfigPath() = %q, want config.yaml basename", path)
	}
}

func TestDefaultRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Write config with default registry.
	data := []byte("default: ghcr.io\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if got := s.DefaultRegistry(); got != "ghcr.io" {
		t.Errorf("DefaultRegistry() = %q, want %q", got, "ghcr.io")
	}
}

func TestMarketplaceConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	data := []byte(`marketplace:
  url: https://marketplace.example.com
  apiKey: zhk_test123
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if got := s.MarketplaceURL(); got != "https://marketplace.example.com" {
		t.Errorf("MarketplaceURL() = %q", got)
	}
	if got := s.MarketplaceAPIKey(); got != "zhk_test123" {
		t.Errorf("MarketplaceAPIKey() = %q", got)
	}
}

func TestMarketplaceConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if got := s.MarketplaceURL(); got != "" {
		t.Errorf("MarketplaceURL() = %q, want empty", got)
	}
	if got := s.MarketplaceAPIKey(); got != "" {
		t.Errorf("MarketplaceAPIKey() = %q, want empty", got)
	}
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := s.Login("test.io", "u", "p"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want %o", perm, 0o600)
	}
}
