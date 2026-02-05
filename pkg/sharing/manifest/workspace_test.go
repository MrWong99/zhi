package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWorkspaceValid(t *testing.T) {
	data := []byte(`
name: kubernetes-cluster
version: 1.0.0
description: Configure and provision a Kubernetes cluster
author: zhi-project
license: Apache-2.0
dependencies:
  - ref: oci://ghcr.io/zhi-project/zhi-config-structured:v1.0.0
    type: config
  - ref: oci://ghcr.io/zhi-project/zhi-store-vault:v2.0.0
    type: store
    optional: true
tools:
  - name: kubectl
    version: ">=1.28"
  - name: helm
    version: ">=3.12"
keywords:
  - kubernetes
  - cluster
`)
	m, err := ParseWorkspace(data)
	if err != nil {
		t.Fatalf("ParseWorkspace: %v", err)
	}
	if m.Name != "kubernetes-cluster" {
		t.Errorf("Name = %q, want %q", m.Name, "kubernetes-cluster")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if len(m.Dependencies) != 2 {
		t.Errorf("Dependencies = %d, want 2", len(m.Dependencies))
	}
	if m.Dependencies[0].Ref != "oci://ghcr.io/zhi-project/zhi-config-structured:v1.0.0" {
		t.Errorf("Dependencies[0].Ref = %q", m.Dependencies[0].Ref)
	}
	if m.Dependencies[1].Optional != true {
		t.Error("Dependencies[1].Optional = false, want true")
	}
	if len(m.Tools) != 2 {
		t.Errorf("Tools = %d, want 2", len(m.Tools))
	}
	if m.Tools[0].Name != "kubectl" {
		t.Errorf("Tools[0].Name = %q", m.Tools[0].Name)
	}
	if len(m.Keywords) != 2 {
		t.Errorf("Keywords = %v, want 2 entries", m.Keywords)
	}
}

func TestParseWorkspaceMinimal(t *testing.T) {
	data := []byte(`
name: simple-workspace
version: 0.1.0
`)
	m, err := ParseWorkspace(data)
	if err != nil {
		t.Fatalf("ParseWorkspace: %v", err)
	}
	if m.Name != "simple-workspace" {
		t.Errorf("Name = %q", m.Name)
	}
	if len(m.Dependencies) != 0 {
		t.Errorf("Dependencies = %d, want 0", len(m.Dependencies))
	}
}

func TestWorkspaceValidateMissingName(t *testing.T) {
	data := []byte(`version: 1.0.0`)
	_, err := ParseWorkspace(data)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestWorkspaceValidateMissingVersion(t *testing.T) {
	data := []byte(`name: test-ws`)
	_, err := ParseWorkspace(data)
	if err == nil {
		t.Error("expected error for missing version")
	}
}

func TestWorkspaceValidateInvalidName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"uppercase", "MyWorkspace"},
		{"starts with number", "1workspace"},
		{"special chars", "my_workspace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("name: " + tt.input + "\nversion: 1.0.0\n")
			_, err := ParseWorkspace(data)
			if err == nil {
				t.Errorf("expected error for name %q", tt.input)
			}
		})
	}
}

func TestWorkspaceValidateInvalidVersion(t *testing.T) {
	data := []byte("name: test\nversion: notvalid\n")
	_, err := ParseWorkspace(data)
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestWorkspaceValidateInvalidDependencyType(t *testing.T) {
	data := []byte(`
name: test
version: 1.0.0
dependencies:
  - ref: oci://example.com/plugin:v1.0
    type: invalid
`)
	_, err := ParseWorkspace(data)
	if err == nil {
		t.Error("expected error for invalid dependency type")
	}
}

func TestWorkspaceValidateMissingDependencyRef(t *testing.T) {
	data := []byte(`
name: test
version: 1.0.0
dependencies:
  - type: config
`)
	_, err := ParseWorkspace(data)
	if err == nil {
		t.Error("expected error for missing dependency ref")
	}
}

func TestWorkspaceValidateMissingToolName(t *testing.T) {
	data := []byte(`
name: test
version: 1.0.0
tools:
  - version: ">=1.0"
`)
	_, err := ParseWorkspace(data)
	if err == nil {
		t.Error("expected error for missing tool name")
	}
}

func TestWorkspaceMarshalRoundTrip(t *testing.T) {
	original := &WorkspaceManifest{
		Name:        "test-workspace",
		Version:     "1.0.0",
		Description: "A test workspace",
		Author:      "test",
		Dependencies: []PluginDependency{
			{Ref: "oci://example.com/plugin:v1.0", Type: "config"},
		},
		Tools: []ToolRequirement{
			{Name: "kubectl", Version: ">=1.28"},
		},
	}
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := ParseWorkspace(data)
	if err != nil {
		t.Fatalf("ParseWorkspace: %v", err)
	}
	if parsed.Name != original.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, original.Name)
	}
	if len(parsed.Dependencies) != 1 {
		t.Errorf("Dependencies = %d, want 1", len(parsed.Dependencies))
	}
}

func TestLoadWorkspaceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zhi-workspace.yaml")
	data := []byte("name: file-test\nversion: 0.1.0\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadWorkspaceFile(path)
	if err != nil {
		t.Fatalf("LoadWorkspaceFile: %v", err)
	}
	if m.Name != "file-test" {
		t.Errorf("Name = %q, want %q", m.Name, "file-test")
	}
}

func TestLoadWorkspaceFileNotFound(t *testing.T) {
	_, err := LoadWorkspaceFile("/nonexistent/zhi-workspace.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestWorkspaceMarshalConfigJSON(t *testing.T) {
	m := &WorkspaceManifest{
		Name:    "test-ws",
		Version: "1.0.0",
		Dependencies: []PluginDependency{
			{Ref: "oci://example.com/plugin:v1.0"},
		},
	}
	data, err := m.MarshalConfigJSON()
	if err != nil {
		t.Fatalf("MarshalConfigJSON: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}
