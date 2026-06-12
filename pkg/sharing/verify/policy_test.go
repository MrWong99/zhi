package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.RequireSignatures {
		t.Error("default policy should not require signatures")
	}
	if len(p.TrustedPublishers) != 0 {
		t.Error("default policy should have no trusted publishers")
	}
	if len(p.AllowedRegistries) != 0 {
		t.Error("default policy should have no allowed registries")
	}
	if len(p.BlockedPlugins) != 0 {
		t.Error("default policy should have no blocked plugins")
	}
}

func TestParsePolicyWrapped(t *testing.T) {
	data := []byte(`
sharing:
  verification:
    requireSignatures: true
    trustedPublishers:
      - zhi-project
      - myorg
    allowedRegistries:
      - ghcr.io
      - harbor.internal:5000
    blockedPlugins:
      - untrusted/bad-plugin
`)
	p, err := ParsePolicy(data)
	if err != nil {
		t.Fatalf("ParsePolicy: %v", err)
	}
	if !p.RequireSignatures {
		t.Error("expected requireSignatures true")
	}
	if len(p.TrustedPublishers) != 2 {
		t.Errorf("expected 2 trusted publishers, got %d", len(p.TrustedPublishers))
	}
	if len(p.AllowedRegistries) != 2 {
		t.Errorf("expected 2 allowed registries, got %d", len(p.AllowedRegistries))
	}
	if len(p.BlockedPlugins) != 1 {
		t.Errorf("expected 1 blocked plugin, got %d", len(p.BlockedPlugins))
	}
}

func TestParsePolicyBare(t *testing.T) {
	data := []byte(`
requireSignatures: true
trustedPublishers:
  - zhi-project
`)
	p, err := ParsePolicy(data)
	if err != nil {
		t.Fatalf("ParsePolicy: %v", err)
	}
	if !p.RequireSignatures {
		t.Error("expected requireSignatures true")
	}
	if len(p.TrustedPublishers) != 1 {
		t.Errorf("expected 1 trusted publisher, got %d", len(p.TrustedPublishers))
	}
}

func TestParsePolicyEmpty(t *testing.T) {
	data := []byte(`{}`)
	p, err := ParsePolicy(data)
	if err != nil {
		t.Fatalf("ParsePolicy: %v", err)
	}
	if p.RequireSignatures {
		t.Error("expected requireSignatures false for empty policy")
	}
}

func TestLoadPolicyFileNotExist(t *testing.T) {
	p, err := LoadPolicyFile("/nonexistent/policy.yaml")
	if err != nil {
		t.Fatalf("LoadPolicyFile: %v", err)
	}
	// Should return default policy.
	if p.RequireSignatures {
		t.Error("expected default policy for missing file")
	}
}

func TestLoadPolicyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	data := []byte(`
sharing:
  verification:
    requireSignatures: true
    trustedPublishers:
      - zhi-project
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicyFile(path)
	if err != nil {
		t.Fatalf("LoadPolicyFile: %v", err)
	}
	if !p.RequireSignatures {
		t.Error("expected requireSignatures true")
	}
}

func TestEffectiveLevel(t *testing.T) {
	p := DefaultPolicy()
	if p.EffectiveLevel() != LevelSigned {
		t.Errorf("default effective level = %v, want LevelSigned", p.EffectiveLevel())
	}

	p.RequireSignatures = true
	if p.EffectiveLevel() != LevelStrict {
		t.Errorf("strict effective level = %v, want LevelStrict", p.EffectiveLevel())
	}
}

func TestIsTrustedPublisher(t *testing.T) {
	p := &Policy{TrustedPublishers: []string{"zhi-project", "MyOrg"}}
	if !p.IsTrustedPublisher("zhi-project") {
		t.Error("expected zhi-project to be trusted")
	}
	if !p.IsTrustedPublisher("myorg") {
		t.Error("expected case-insensitive match for MyOrg")
	}
	if p.IsTrustedPublisher("unknown") {
		t.Error("expected unknown to not be trusted")
	}
}

func TestIsRegistryAllowed(t *testing.T) {
	// Empty list allows all.
	p := &Policy{}
	if !p.IsRegistryAllowed("oci://docker.io/org/plugin") {
		t.Error("expected all registries allowed when list is empty")
	}

	// With restrictions.
	p = &Policy{AllowedRegistries: []string{"ghcr.io"}}
	if !p.IsRegistryAllowed("oci://ghcr.io/org/plugin") {
		t.Error("expected ghcr.io to be allowed")
	}
	if p.IsRegistryAllowed("oci://docker.io/org/plugin") {
		t.Error("expected docker.io to not be allowed")
	}
}

func TestIsBlocked(t *testing.T) {
	p := &Policy{BlockedPlugins: []string{"untrusted/bad-plugin"}}

	tests := []struct {
		ref     string
		blocked bool
	}{
		{"oci://ghcr.io/untrusted/bad-plugin:v1.0", true},
		{"oci://ghcr.io/trusted/good-plugin:v1.0", false},
		// Digest references are matched on the repository path.
		{"oci://ghcr.io/untrusted/bad-plugin@sha256:abc123", true},
		// Tag stripping must not eat the digest segment match.
		{"oci://ghcr.io/untrusted/bad-plugin", true},
		// Patterns match whole segments, not substrings.
		{"oci://ghcr.io/untrusted/bad-plugin-fork:v1.0", false},
		{"oci://ghcr.io/very-untrusted/bad-plugin:v1.0", false},
		{"oci://ghcr.io/org/xuntrusted/bad-pluginx:v1.0", false},
		// Registries with a port keep the host segment intact.
		{"oci://localhost:5000/untrusted/bad-plugin:v1.0", true},
	}
	for _, tt := range tests {
		if got := p.IsBlocked(tt.ref); got != tt.blocked {
			t.Errorf("IsBlocked(%q) = %v, want %v", tt.ref, got, tt.blocked)
		}
	}

	// Patterns may carry an oci:// prefix themselves.
	p = &Policy{BlockedPlugins: []string{"oci://untrusted/bad-plugin"}}
	if !p.IsBlocked("oci://ghcr.io/untrusted/bad-plugin:v1.0") {
		t.Error("expected oci://-prefixed pattern to block")
	}

	// Empty patterns never match.
	p = &Policy{BlockedPlugins: []string{""}}
	if p.IsBlocked("oci://ghcr.io/org/plugin:v1.0") {
		t.Error("expected empty pattern to not block")
	}
}

func TestMatchesSegments(t *testing.T) {
	if matchesSegments([]string{"a", "b"}, nil) {
		t.Error("expected empty pattern to not match")
	}
	// First pattern segment matches, a later one does not.
	if matchesSegments([]string{"ghcr.io", "untrusted", "good-plugin"}, []string{"untrusted", "bad-plugin"}) {
		t.Error("expected partial segment run to not match")
	}
	if !matchesSegments([]string{"ghcr.io", "untrusted", "bad-plugin"}, []string{"untrusted", "bad-plugin"}) {
		t.Error("expected contiguous segment run to match")
	}
}
