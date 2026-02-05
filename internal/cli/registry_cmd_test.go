package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/MrWong99/zhi/pkg/sharing/registry"
)

func TestRegistryLoginLogoutFlow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	store, err := registry.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Login.
	if err := store.Login("ghcr.io", "testuser", "testtoken"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Verify credentials persist.
	store2, err := registry.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	u, p, ok := store2.GetCredentials("ghcr.io")
	if !ok || u != "testuser" || p != "testtoken" {
		t.Errorf("creds = (%q, %q, %v), want (testuser, testtoken, true)", u, p, ok)
	}

	// Logout.
	if err := store2.Logout("ghcr.io"); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	_, _, ok = store2.GetCredentials("ghcr.io")
	if ok {
		t.Error("expected credentials removed after logout")
	}
}

func TestRegistryList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	store, err := registry.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := store.Login("ghcr.io", "u1", "p1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Login("docker.io", "u2", "p2"); err != nil {
		t.Fatal(err)
	}

	hosts := store.ListHosts()
	sort.Strings(hosts)
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0] != "docker.io" || hosts[1] != "ghcr.io" {
		t.Errorf("hosts = %v", hosts)
	}
}

func TestRegistryStoreFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	store, err := registry.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := store.Login("test.io", "u", "p"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %o, want 600", perm)
	}
}
