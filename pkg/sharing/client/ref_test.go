package client

import "testing"

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		host    string
		repo    string
		tag     string
		digest  string
		wantErr bool
	}{
		{
			name:  "full oci reference with tag",
			input: "oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
			host:  "ghcr.io",
			repo:  "zhi-project/zhi-config-ansible",
			tag:   "v1.2.0",
		},
		{
			name:   "oci reference with digest",
			input:  "oci://ghcr.io/zhi-project/zhi-config-ansible@sha256:abc123",
			host:   "ghcr.io",
			repo:   "zhi-project/zhi-config-ansible",
			digest: "sha256:abc123",
		},
		{
			name:  "without scheme",
			input: "ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
			host:  "ghcr.io",
			repo:  "zhi-project/zhi-config-ansible",
			tag:   "v1.2.0",
		},
		{
			name:  "docker hub style",
			input: "docker.io/library/plugin:latest",
			host:  "docker.io",
			repo:  "library/plugin",
			tag:   "latest",
		},
		{
			name:  "no tag or digest",
			input: "ghcr.io/org/repo",
			host:  "ghcr.io",
			repo:  "org/repo",
		},
		{
			name:  "registry with port",
			input: "localhost:5000/myrepo:v1.0.0",
			host:  "localhost:5000",
			repo:  "myrepo",
			tag:   "v1.0.0",
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "oci scheme only",
			input:   "oci://",
			wantErr: true,
		},
		{
			name:    "no slash",
			input:   "ghcr.io",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseReference(%q): %v", tt.input, err)
			}
			if ref.Host != tt.host {
				t.Errorf("Host = %q, want %q", ref.Host, tt.host)
			}
			if ref.Repository != tt.repo {
				t.Errorf("Repository = %q, want %q", ref.Repository, tt.repo)
			}
			if ref.Tag != tt.tag {
				t.Errorf("Tag = %q, want %q", ref.Tag, tt.tag)
			}
			if ref.Digest != tt.digest {
				t.Errorf("Digest = %q, want %q", ref.Digest, tt.digest)
			}
		})
	}
}

func TestIsShortName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ansible-config", true},
		{"ansible-config@1.2.0", true},
		{"vault-store", true},
		{"oci://ghcr.io/org/repo:v1.0", false},
		{"ghcr.io/org/repo:v1.0", false},
		{"ghcr.io/org/repo", false},
		{"localhost:5000/myrepo:v1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsShortName(tt.input)
			if got != tt.want {
				t.Errorf("IsShortName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsedRefReference(t *testing.T) {
	tests := []struct {
		name string
		ref  ParsedRef
		want string
	}{
		{
			name: "with tag",
			ref:  ParsedRef{Host: "ghcr.io", Repository: "org/repo", Tag: "v1.0.0"},
			want: "ghcr.io/org/repo:v1.0.0",
		},
		{
			name: "with digest",
			ref:  ParsedRef{Host: "ghcr.io", Repository: "org/repo", Digest: "sha256:abc"},
			want: "ghcr.io/org/repo@sha256:abc",
		},
		{
			name: "no tag or digest",
			ref:  ParsedRef{Host: "ghcr.io", Repository: "org/repo"},
			want: "ghcr.io/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.Reference(); got != tt.want {
				t.Errorf("Reference() = %q, want %q", got, tt.want)
			}
		})
	}
}
