package client

import (
	"runtime"
	"testing"
)

func TestCurrentPlatform(t *testing.T) {
	p := CurrentPlatform()
	if p.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", p.OS, runtime.GOOS)
	}
	if p.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", p.Arch, runtime.GOARCH)
	}
}

func TestPlatformString(t *testing.T) {
	p := Platform{OS: "linux", Arch: "amd64"}
	if got := p.String(); got != "linux/amd64" {
		t.Errorf("String() = %q, want %q", got, "linux/amd64")
	}
}

func TestMediaTypeConstants(t *testing.T) {
	// Verify media type constants are consistent with the distribution format spec.
	if MediaTypePluginConfig != "application/vnd.zhi.plugin.config.v1+json" {
		t.Errorf("MediaTypePluginConfig = %q", MediaTypePluginConfig)
	}
	if MediaTypePluginBinary != "application/vnd.zhi.plugin.binary.v1" {
		t.Errorf("MediaTypePluginBinary = %q", MediaTypePluginBinary)
	}
	if MediaTypePluginRuntime != "application/vnd.zhi.plugin.runtime.v1+tar.gz" {
		t.Errorf("MediaTypePluginRuntime = %q", MediaTypePluginRuntime)
	}
}

func TestParseConfigBlob(t *testing.T) {
	data := []byte(`{
		"name": "test-plugin",
		"type": "config",
		"version": "1.0.0",
		"description": "A test plugin"
	}`)
	m, err := parseConfigBlob(data)
	if err != nil {
		t.Fatalf("parseConfigBlob: %v", err)
	}
	if m.Name != "test-plugin" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Type != "config" {
		t.Errorf("Type = %q", m.Type)
	}
}

func TestParseConfigBlobInvalid(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"invalid json", `not json`},
		{"missing name", `{"type":"config","version":"1.0.0"}`},
		{"invalid type", `{"name":"test","type":"bad","version":"1.0.0"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConfigBlob([]byte(tt.data))
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}
