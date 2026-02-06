package server

import (
	"os"
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p == nil {
		t.Fatal("DefaultPolicy returned nil")
	}
	if p.RequireSignatures {
		t.Error("default policy should not require signatures")
	}
	if len(p.AllowedPublishers) != 0 {
		t.Error("default policy should have no allowed publishers")
	}
}

func TestParsePolicy(t *testing.T) {
	yaml := `
allowedPublishers:
  - zhi-project
  - myorg
blockedArtifacts:
  - untrusted/bad-plugin
allowedTypes:
  - config
  - transform
requireSignatures: true
sync:
  - ref: "oci://ghcr.io/zhi-project/zhi-config-ansible:v1.*"
    schedule: "0 2 * * *"
retention:
  unusedDays: 90
  maxStorage: 50GB
`
	p, err := ParsePolicy([]byte(yaml))
	if err != nil {
		t.Fatalf("ParsePolicy: %v", err)
	}
	if !p.RequireSignatures {
		t.Error("expected requireSignatures=true")
	}
	if len(p.AllowedPublishers) != 2 {
		t.Errorf("expected 2 allowed publishers, got %d", len(p.AllowedPublishers))
	}
	if len(p.BlockedArtifacts) != 1 {
		t.Errorf("expected 1 blocked artifact, got %d", len(p.BlockedArtifacts))
	}
	if len(p.AllowedTypes) != 2 {
		t.Errorf("expected 2 allowed types, got %d", len(p.AllowedTypes))
	}
	if len(p.Sync) != 1 {
		t.Errorf("expected 1 sync rule, got %d", len(p.Sync))
	}
	if p.Retention.UnusedDays != 90 {
		t.Errorf("expected unusedDays=90, got %d", p.Retention.UnusedDays)
	}
	if p.Retention.MaxStorage != "50GB" {
		t.Errorf("expected maxStorage=50GB, got %s", p.Retention.MaxStorage)
	}
}

func TestParsePolicyInvalid(t *testing.T) {
	_, err := ParsePolicy([]byte(":::invalid"))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestIsPublisherAllowed(t *testing.T) {
	tests := []struct {
		name      string
		allowed   []string
		publisher string
		want      bool
	}{
		{
			name:      "empty list allows all",
			allowed:   nil,
			publisher: "anyone",
			want:      true,
		},
		{
			name:      "publisher in list",
			allowed:   []string{"zhi-project", "myorg"},
			publisher: "myorg",
			want:      true,
		},
		{
			name:      "publisher not in list",
			allowed:   []string{"zhi-project"},
			publisher: "untrusted",
			want:      false,
		},
		{
			name:      "case insensitive",
			allowed:   []string{"Zhi-Project"},
			publisher: "zhi-project",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{AllowedPublishers: tt.allowed}
			got := p.IsPublisherAllowed(tt.publisher)
			if got != tt.want {
				t.Errorf("IsPublisherAllowed(%q) = %v, want %v", tt.publisher, got, tt.want)
			}
		})
	}
}

func TestIsArtifactBlocked(t *testing.T) {
	p := &Policy{BlockedArtifacts: []string{"untrusted/bad-plugin"}}

	if !p.IsArtifactBlocked("untrusted/bad-plugin") {
		t.Error("expected artifact to be blocked")
	}
	if !p.IsArtifactBlocked("oci://untrusted/bad-plugin:v1") {
		t.Error("expected artifact with oci:// prefix to be blocked")
	}
	if p.IsArtifactBlocked("trusted/good-plugin") {
		t.Error("expected artifact to be allowed")
	}
}

func TestIsTypeAllowed(t *testing.T) {
	tests := []struct {
		name       string
		allowed    []string
		pluginType string
		want       bool
	}{
		{
			name:       "empty list allows all",
			allowed:    nil,
			pluginType: "config",
			want:       true,
		},
		{
			name:       "type in list",
			allowed:    []string{"config", "transform"},
			pluginType: "config",
			want:       true,
		},
		{
			name:       "type not in list",
			allowed:    []string{"config"},
			pluginType: "ui",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{AllowedTypes: tt.allowed}
			got := p.IsTypeAllowed(tt.pluginType)
			if got != tt.want {
				t.Errorf("IsTypeAllowed(%q) = %v, want %v", tt.pluginType, got, tt.want)
			}
		})
	}
}

func TestCheckRepository(t *testing.T) {
	p := &Policy{
		AllowedPublishers: []string{"zhi-project"},
		BlockedArtifacts:  []string{"zhi-project/blocked-plugin"},
	}

	if err := p.CheckRepository("zhi-project/good-plugin"); err != nil {
		t.Errorf("expected allowed, got: %v", err)
	}

	if err := p.CheckRepository("zhi-project/blocked-plugin"); err == nil {
		t.Error("expected blocked artifact to be rejected")
	}

	if err := p.CheckRepository("untrusted/plugin"); err == nil {
		t.Error("expected unknown publisher to be rejected")
	}
}

func TestLoadPolicyMissing(t *testing.T) {
	p, err := LoadPolicy("/nonexistent/policy.yaml")
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p == nil {
		t.Fatal("expected default policy, got nil")
	}
	if p.RequireSignatures {
		t.Error("default policy should not require signatures")
	}
}

func TestLoadPolicyFromFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/policy.yaml"
	data := []byte("requireSignatures: true\nallowedPublishers:\n  - myorg\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if !p.RequireSignatures {
		t.Error("expected requireSignatures=true")
	}
	if len(p.AllowedPublishers) != 1 || p.AllowedPublishers[0] != "myorg" {
		t.Errorf("unexpected allowed publishers: %v", p.AllowedPublishers)
	}
}

func TestExtractPublisher(t *testing.T) {
	tests := []struct {
		repo string
		want string
	}{
		{"zhi-project/zhi-config-ansible", "zhi-project"},
		{"myorg/my-plugin", "myorg"},
		{"simple", "simple"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractPublisher(tt.repo)
		if got != tt.want {
			t.Errorf("extractPublisher(%q) = %q, want %q", tt.repo, got, tt.want)
		}
	}
}
