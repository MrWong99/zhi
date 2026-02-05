package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidManifest(t *testing.T) {
	data := []byte(`
name: ansible-config
type: config
version: 1.2.0
description: Ansible inventory config provider
author: zhi-project
license: Apache-2.0
homepage: https://github.com/zhi-project/zhi-config-ansible
keywords:
  - ansible
  - inventory
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Name != "ansible-config" {
		t.Errorf("Name = %q, want %q", m.Name, "ansible-config")
	}
	if m.Type != "config" {
		t.Errorf("Type = %q, want %q", m.Type, "config")
	}
	if m.Version != "1.2.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.2.0")
	}
	if m.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", m.SchemaVersion, SchemaVersion)
	}
	if m.Description != "Ansible inventory config provider" {
		t.Errorf("Description = %q", m.Description)
	}
	if len(m.Keywords) != 2 {
		t.Errorf("Keywords = %v, want 2 entries", m.Keywords)
	}
}

func TestParseAllTypes(t *testing.T) {
	for _, typ := range []string{"config", "transform", "store", "ui"} {
		t.Run(typ, func(t *testing.T) {
			data := []byte("name: test-plugin\ntype: " + typ + "\nversion: 0.1.0\n")
			m, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse(%s): %v", typ, err)
			}
			if m.Type != typ {
				t.Errorf("Type = %q, want %q", m.Type, typ)
			}
		})
	}
}

func TestParseWithRuntime(t *testing.T) {
	data := []byte(`
name: vault-store
type: store
version: 2.0.0
runtime:
  type: java
  version: ">=17"
  bundled: true
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Runtime == nil {
		t.Fatal("Runtime is nil")
	}
	if m.Runtime.Type != "java" {
		t.Errorf("Runtime.Type = %q, want %q", m.Runtime.Type, "java")
	}
	if m.Runtime.Version != ">=17" {
		t.Errorf("Runtime.Version = %q, want %q", m.Runtime.Version, ">=17")
	}
	if !m.Runtime.Bundled {
		t.Error("Runtime.Bundled = false, want true")
	}
}

func TestValidateMissingFields(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"missing name", "type: config\nversion: 1.0.0\n"},
		{"missing type", "name: foo\nversion: 1.0.0\n"},
		{"missing version", "name: foo\ntype: config\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestValidateInvalidName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"uppercase", "MyPlugin"},
		{"starts with number", "1plugin"},
		{"special chars", "my_plugin"},
		{"spaces", "my plugin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("name: " + tt.input + "\ntype: config\nversion: 1.0.0\n")
			_, err := Parse(data)
			if err == nil {
				t.Errorf("expected error for name %q", tt.input)
			}
		})
	}
}

func TestValidateInvalidType(t *testing.T) {
	data := []byte("name: test\ntype: invalid\nversion: 1.0.0\n")
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestValidateInvalidVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"no dots", "100"},
		{"only major.minor", "1.0"},
		{"letters", "abc"},
		{"v prefix", "v1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("name: test\ntype: config\nversion: " + tt.version + "\n")
			_, err := Parse(data)
			if err == nil {
				t.Errorf("expected error for version %q", tt.version)
			}
		})
	}
}

func TestValidSemver(t *testing.T) {
	valid := []string{"0.0.1", "1.0.0", "1.2.3", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0+build.123"}
	for _, v := range valid {
		if !IsValidSemver(v) {
			t.Errorf("IsValidSemver(%q) = false, want true", v)
		}
	}
}

func TestBinaryName(t *testing.T) {
	m := &PluginManifest{Name: "ansible-config", Type: "config"}
	if got := m.BinaryName(); got != "zhi-config-ansible-config" {
		t.Errorf("BinaryName = %q, want %q", got, "zhi-config-ansible-config")
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	original := &PluginManifest{
		SchemaVersion: SchemaVersion,
		Name:          "test-plugin",
		Type:          "config",
		Version:       "1.0.0",
		Description:   "A test plugin",
		Author:        "test",
		License:       "MIT",
		Keywords:      []string{"test", "example"},
	}
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.Name != original.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, original.Name)
	}
	if parsed.Version != original.Version {
		t.Errorf("Version = %q, want %q", parsed.Version, original.Version)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zhi-plugin.yaml")
	data := []byte("name: file-test\ntype: store\nversion: 0.1.0\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if m.Name != "file-test" {
		t.Errorf("Name = %q, want %q", m.Name, "file-test")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/zhi-plugin.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestIsValidPluginType(t *testing.T) {
	for _, typ := range []string{"config", "transform", "store", "ui"} {
		if !IsValidPluginType(typ) {
			t.Errorf("IsValidPluginType(%q) = false", typ)
		}
	}
	if IsValidPluginType("invalid") {
		t.Error("IsValidPluginType(\"invalid\") = true")
	}
}

func TestIsValidName(t *testing.T) {
	valid := []string{"a", "ab", "my-plugin", "plugin123", "a1"}
	for _, n := range valid {
		if !IsValidName(n) {
			t.Errorf("IsValidName(%q) = false, want true", n)
		}
	}
	invalid := []string{"", "1abc", "My-Plugin", "my_plugin"}
	for _, n := range invalid {
		if IsValidName(n) {
			t.Errorf("IsValidName(%q) = true, want false", n)
		}
	}
}
