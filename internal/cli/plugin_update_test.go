package cli

import "testing"

func TestBuildUpdateRef(t *testing.T) {
	tests := []struct {
		existingRef string
		newVersion  string
		want        string
	}{
		{
			"oci://ghcr.io/zhi-project/zhi-config-ansible:v1.1.0",
			"1.2.0",
			"oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
		},
		{
			"ghcr.io/zhi-project/zhi-config-ansible:v1.1.0",
			"1.2.0",
			"ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
		},
		{
			"ghcr.io/org/plugin@sha256:abc123",
			"2.0.0",
			"ghcr.io/org/plugin:v2.0.0",
		},
		{
			"ghcr.io/org/plugin",
			"1.0.0",
			"ghcr.io/org/plugin:v1.0.0",
		},
	}
	for _, tt := range tests {
		got := buildUpdateRef(tt.existingRef, tt.newVersion)
		if got != tt.want {
			t.Errorf("buildUpdateRef(%q, %q) = %q, want %q", tt.existingRef, tt.newVersion, got, tt.want)
		}
	}
}
