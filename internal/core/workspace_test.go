package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func TestLoadWorkspaceYAML(t *testing.T) {
	dir := t.TempDir()
	content := `
version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
transform: []
store:
  provider: zhi-store-json
  options:
    directory: ./.zhi/store
components:
  - name: database
    description: "PostgreSQL database configuration"
    paths:
      - database/
    mandatory: true
  - name: redis
    description: "Redis cache layer"
    paths:
      - redis/
    dependencies:
      - database
  - name: monitoring
    description: "Monitoring stack"
    paths:
      - monitoring/
export:
  templates: []
apply:
  command: "echo done"
  workdir: "."
`
	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	if ws.Version != "1" {
		t.Errorf("Version = %q, want %q", ws.Version, "1")
	}
	if ws.Config.Provider != "structuredfile" {
		t.Errorf("Config.Provider = %q, want %q", ws.Config.Provider, "structuredfile")
	}
	if ws.Config.Options["directory"] != "./config" {
		t.Errorf("Config.Options[directory] = %v, want %q", ws.Config.Options["directory"], "./config")
	}
	if ws.Store.Provider != "zhi-store-json" {
		t.Errorf("Store.Provider = %q, want %q", ws.Store.Provider, "zhi-store-json")
	}
	if len(ws.Components) != 3 {
		t.Fatalf("len(Components) = %d, want 3", len(ws.Components))
	}
	if ws.Components[0].Name != "database" || !ws.Components[0].Mandatory {
		t.Errorf("Components[0] = %+v, want database+mandatory", ws.Components[0])
	}
	if ws.Apply.Command != "echo done" {
		t.Errorf("Apply.Command = %q, want %q", ws.Apply.Command, "echo done")
	}
	if ws.Dir != dir {
		t.Errorf("Dir = %q, want %q", ws.Dir, dir)
	}
}

func TestLoadWorkspaceJSON(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "version": "1",
  "config": {
    "provider": "structuredfile",
    "options": {"directory": "./config"}
  },
  "components": [
    {"name": "app", "paths": ["app/"], "mandatory": true}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "zhi.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if ws.Config.Provider != "structuredfile" {
		t.Errorf("Config.Provider = %q, want %q", ws.Config.Provider, "structuredfile")
	}
	if len(ws.Components) != 1 {
		t.Fatalf("len(Components) = %d, want 1", len(ws.Components))
	}
}

func TestLoadWorkspaceWalkUp(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `version: "1"
config:
  provider: structuredfile
`
	if err := os.WriteFile(filepath.Join(root, "zhi.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := LoadWorkspace(subdir)
	if err != nil {
		t.Fatalf("LoadWorkspace from subdir: %v", err)
	}
	if ws.Dir != root {
		t.Errorf("Dir = %q, want %q", ws.Dir, root)
	}
}

func TestLoadWorkspaceNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadWorkspace(dir)
	if err == nil {
		t.Error("expected error when no workspace config found")
	}
}

func TestLoadWorkspaceInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte("{{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadWorkspace(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidateWorkspaceValid(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry()
	_ = reg.RegisterConfig("test-config", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})
	_ = reg.RegisterStore("test-store", func(string, map[string]any) (store.Plugin, error) {
		return &stubStore{}, nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "test-config"},
		Store:   ProviderRef{Provider: "test-store"},
		Components: []ComponentDef{
			{Name: "app", Paths: []string{"app/"}, Mandatory: true},
		},
		Dir: dir,
	}

	if err := ValidateWorkspace(ws, reg); err != nil {
		t.Errorf("ValidateWorkspace: %v", err)
	}
}

func TestValidateWorkspaceInvalidVersion(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("test", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	ws := &WorkspaceConfig{
		Version: "99",
		Config:  ProviderRef{Provider: "test"},
		Dir:     t.TempDir(),
	}

	if err := ValidateWorkspace(ws, reg); err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestValidateWorkspaceMissingProvider(t *testing.T) {
	reg := NewRegistry()
	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "nonexistent"},
		Dir:     t.TempDir(),
	}

	if err := ValidateWorkspace(ws, reg); err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestValidateWorkspaceEmptyConfigProvider(t *testing.T) {
	reg := NewRegistry()
	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{},
		Dir:     t.TempDir(),
	}

	if err := ValidateWorkspace(ws, reg); err == nil {
		t.Error("expected error for empty config provider")
	}
}

func TestValidateWorkspaceBadComponents(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("test", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "test"},
		Components: []ComponentDef{
			{Name: "a", Paths: []string{"a/"}, Dependencies: []string{"b"}},
			{Name: "b", Paths: []string{"b/"}, Dependencies: []string{"a"}},
		},
		Dir: t.TempDir(),
	}

	if err := ValidateWorkspace(ws, reg); err == nil {
		t.Error("expected error for circular component dependencies")
	}
}

func TestValidateWorkspaceMissingTemplate(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("test", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	ws := &WorkspaceConfig{
		Version: "1",
		Config:  ProviderRef{Provider: "test"},
		Export: ExportConfig{
			Templates: []ExportTemplate{
				{Name: "tmpl", Template: "nonexistent.tmpl", Output: "out"},
			},
		},
		Dir: t.TempDir(),
	}

	if err := ValidateWorkspace(ws, reg); err == nil {
		t.Error("expected error for missing template file")
	}
}

func TestValidateWorkspaceIterate(t *testing.T) {
	reg := NewRegistry()
	_ = reg.RegisterConfig("test", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "t.tmpl"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		tmpl    ExportTemplate
		wantErr bool
	}{
		{
			name:    "valid iterate",
			tmpl:    ExportTemplate{Name: "g", Template: "t.tmpl", Iterate: "groups", OutputPattern: "./gv/{{ .Key }}.yml"},
			wantErr: false,
		},
		{
			name:    "iterate without output-pattern",
			tmpl:    ExportTemplate{Name: "g", Template: "t.tmpl", Iterate: "groups"},
			wantErr: true,
		},
		{
			name:    "output-pattern without iterate",
			tmpl:    ExportTemplate{Name: "g", Template: "t.tmpl", OutputPattern: "./gv/{{ .Key }}.yml"},
			wantErr: true,
		},
		{
			name:    "iterate with output (conflict)",
			tmpl:    ExportTemplate{Name: "g", Template: "t.tmpl", Iterate: "groups", OutputPattern: "./gv/{{ .Key }}.yml", Output: "out.yml"},
			wantErr: true,
		},
		{
			name:    "output-pattern with path traversal",
			tmpl:    ExportTemplate{Name: "g", Template: "t.tmpl", Iterate: "groups", OutputPattern: "../escape/{{ .Key }}.yml"},
			wantErr: true,
		},
		{
			name:    "output-pattern invalid template syntax",
			tmpl:    ExportTemplate{Name: "g", Template: "t.tmpl", Iterate: "groups", OutputPattern: "{{ .Key"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &WorkspaceConfig{
				Version: "1",
				Config:  ProviderRef{Provider: "test"},
				Export:  ExportConfig{Templates: []ExportTemplate{tt.tmpl}},
				Dir:     dir,
			}
			err := ValidateWorkspace(ws, reg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
