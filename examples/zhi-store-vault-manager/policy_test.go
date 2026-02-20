package main

import (
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestBuildPolicyHCL(t *testing.T) {
	values := map[string]config.Value{
		"database/host": {
			Val:      "localhost",
			Metadata: map[string]any{"vault.app.web-api": "read"},
		},
		"database/password": {
			Val:      "secret",
			Metadata: map[string]any{"vault.app.web-api": "read,write"},
		},
		"redis/url": {
			Val:      "redis://localhost",
			Metadata: map[string]any{"vault.app.web-api": "read", "vault.app.worker": "read"},
		},
	}

	hcl := buildPolicyHCL("web-api", values, "secret", "zhi", "prod")
	if hcl == "" {
		t.Fatal("expected non-empty HCL")
	}

	// Should contain data paths
	if !containsLine(hcl, `path "secret/data/zhi/prod/database/host"`) {
		t.Error("missing database/host data path")
	}
	if !containsLine(hcl, `path "secret/metadata/zhi/prod/database/host"`) {
		t.Error("missing database/host metadata path")
	}

	// read capability
	if !containsLine(hcl, `capabilities = ["read"]`) {
		t.Error("missing read capabilities")
	}
}

func TestBuildPolicyHCL_NoLabelsForApp(t *testing.T) {
	values := map[string]config.Value{
		"database/host": {
			Val:      "localhost",
			Metadata: map[string]any{"vault.app.other": "read"},
		},
	}
	hcl := buildPolicyHCL("web-api", values, "secret", "zhi", "prod")
	if hcl != "" {
		t.Errorf("expected empty HCL for app with no labels, got: %s", hcl)
	}
}

func TestScanApps(t *testing.T) {
	values := map[string]config.Value{
		"database/host": {
			Val:      "localhost",
			Metadata: map[string]any{"vault.app.web-api": "read", "vault.app.worker": "read,write"},
		},
		"redis/url": {
			Val:      "redis://localhost",
			Metadata: map[string]any{"vault.app.web-api": "read"},
		},
	}

	apps := scanApps(values)
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
	if _, ok := apps["web-api"]; !ok {
		t.Error("missing web-api")
	}
	if _, ok := apps["worker"]; !ok {
		t.Error("missing worker")
	}
}

func TestParseCapabilities(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"read", []string{"read"}},
		{"read,write", []string{"read", "create", "update"}},
		{"read,write,delete", []string{"read", "create", "update", "delete"}},
		{"write", []string{"create", "update"}},
	}
	for _, tt := range tests {
		got := parseCapabilities(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseCapabilities(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func containsLine(s, substr string) bool {
	return len(s) > 0 && contains(s, substr)
}
