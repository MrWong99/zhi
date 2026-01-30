package core

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestParseFlatName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType PluginType
		wantName string
		wantOK   bool
	}{
		{"config plugin", "zhi-config-pokedex", PluginTypeConfig, "pokedex", true},
		{"transform plugin", "zhi-transform-evolve", PluginTypeTransform, "evolve", true},
		{"store plugin", "zhi-store-vault", PluginTypeStore, "vault", true},
		{"store with dash", "zhi-store-json-store", PluginTypeStore, "json-store", true},
		{"no prefix", "pokedex", "", "", false},
		{"zhi only", "zhi-", "", "", false},
		{"unknown type", "zhi-unknown-foo", "", "", false},
		{"missing name", "zhi-config-", "", "", false},
		{"just zhi prefix no type", "zhi-foo", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt, pn, ok := parseFlatName(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseFlatName(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if pt != tt.wantType {
				t.Errorf("parseFlatName(%q) type = %v, want %v", tt.input, pt, tt.wantType)
			}
			if pn != tt.wantName {
				t.Errorf("parseFlatName(%q) name = %v, want %v", tt.input, pn, tt.wantName)
			}
		})
	}
}

func TestDiscoverFlatLayout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: executable permissions not supported")
	}

	dir := t.TempDir()

	// Create executable plugin binaries.
	createExecutable(t, filepath.Join(dir, "zhi-config-pokedex"))
	createExecutable(t, filepath.Join(dir, "zhi-transform-evolve"))
	createExecutable(t, filepath.Join(dir, "zhi-store-vault"))

	// Create a non-executable file (should be skipped).
	if err := os.WriteFile(filepath.Join(dir, "zhi-config-skipped"), []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a file without the zhi- prefix (should be skipped).
	createExecutable(t, filepath.Join(dir, "other-binary"))

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(plugins) != 3 {
		t.Fatalf("got %d plugins, want 3: %v", len(plugins), plugins)
	}

	// Check each plugin was found.
	assertPluginFound(t, plugins, "pokedex", PluginTypeConfig)
	assertPluginFound(t, plugins, "evolve", PluginTypeTransform)
	assertPluginFound(t, plugins, "vault", PluginTypeStore)
}

func TestDiscoverSubdirectoryLayout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: executable permissions not supported")
	}

	dir := t.TempDir()

	// Create type subdirectories.
	for _, subdir := range []string{"config", "transform", "store"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	createExecutable(t, filepath.Join(dir, "config", "pokedex"))
	createExecutable(t, filepath.Join(dir, "transform", "evolve"))
	createExecutable(t, filepath.Join(dir, "store", "vault"))

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(plugins) != 3 {
		t.Fatalf("got %d plugins, want 3: %v", len(plugins), plugins)
	}

	assertPluginFound(t, plugins, "pokedex", PluginTypeConfig)
	assertPluginFound(t, plugins, "evolve", PluginTypeTransform)
	assertPluginFound(t, plugins, "vault", PluginTypeStore)
}

func TestDiscoverFlatTakesPrecedence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: executable permissions not supported")
	}

	dir := t.TempDir()

	// Create both flat and subdirectory entries for the same plugin.
	flatPath := filepath.Join(dir, "zhi-config-pokedex")
	createExecutable(t, flatPath)

	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	createExecutable(t, filepath.Join(dir, "config", "pokedex"))

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Should only have one entry (flat takes precedence).
	count := 0
	for _, p := range plugins {
		if p.Name == "pokedex" && p.Type == PluginTypeConfig {
			count++
			if p.Path != flatPath {
				t.Errorf("expected flat path %s, got %s", flatPath, p.Path)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 pokedex config plugin, got %d", count)
	}
}

func TestDiscoverEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("got %d plugins from empty dir, want 0", len(plugins))
	}
}

func TestDiscoverNonexistentDirectory(t *testing.T) {
	plugins, err := Discover(DiscoveryConfig{Directories: []string{"/nonexistent/path"}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins from nonexistent dir, want 0", len(plugins))
	}
}

func TestDiscoverMultipleDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: executable permissions not supported")
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	createExecutable(t, filepath.Join(dir1, "zhi-config-alpha"))
	createExecutable(t, filepath.Join(dir2, "zhi-store-beta"))

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir1, dir2}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2: %v", len(plugins), plugins)
	}

	assertPluginFound(t, plugins, "alpha", PluginTypeConfig)
	assertPluginFound(t, plugins, "beta", PluginTypeStore)
}

func TestDiscoverSkipsNonExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: executable permissions not supported")
	}

	dir := t.TempDir()

	// Create a non-executable file with valid name.
	if err := os.WriteFile(filepath.Join(dir, "zhi-config-noexec"), []byte("#!/bin/sh"), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0 (non-executable should be skipped)", len(plugins))
	}
}

func TestDiscoverSkipsDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: executable permissions not supported")
	}

	dir := t.TempDir()

	// Create a directory named like a plugin.
	if err := os.MkdirAll(filepath.Join(dir, "zhi-config-isdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// The directory itself should not be treated as a plugin.
	for _, p := range plugins {
		if p.Name == "isdir" {
			t.Error("directory named like a plugin should not be discovered")
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", home},
		{"~/plugins", filepath.Join(home, "plugins")},
		{"~other", "~other"}, // tilde not followed by / should not expand
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPluginInfoString(t *testing.T) {
	p := PluginInfo{Name: "pokedex", Type: PluginTypeConfig, Path: "/usr/local/bin/zhi-config-pokedex"}
	got := p.String()
	want := "config/pokedex (/usr/local/bin/zhi-config-pokedex)"
	if got != want {
		t.Errorf("PluginInfo.String() = %q, want %q", got, want)
	}
}

func TestDiscoverDuplicateAcrossDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows: executable permissions not supported")
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Same plugin in both directories; first directory wins.
	createExecutable(t, filepath.Join(dir1, "zhi-config-pokedex"))
	createExecutable(t, filepath.Join(dir2, "zhi-config-pokedex"))

	plugins, err := Discover(DiscoveryConfig{Directories: []string{dir1, dir2}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	count := 0
	for _, p := range plugins {
		if p.Name == "pokedex" && p.Type == PluginTypeConfig {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 pokedex plugin (dedup), got %d", count)
	}
}

// --- helpers ---

func createExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertPluginFound(t *testing.T, plugins []PluginInfo, name string, pt PluginType) {
	t.Helper()
	found := slices.ContainsFunc(plugins, func(p PluginInfo) bool {
		return p.Name == name && p.Type == pt
	})
	if !found {
		t.Errorf("plugin %s/%s not found in %v", pt, name, plugins)
	}
}
