package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "init",
		RunE: runInit,
	}
	return cmd
}

func TestInitCreatesWorkspace(t *testing.T) {
	dir := t.TempDir()

	workspace = dir
	initForce = false
	initConfigProvider = "structuredfile"
	initStoreProvider = "zhi-store-json"

	cmd := newInitCmd()
	cmd.SetOut(new(strings.Builder))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// Check zhi.yaml exists.
	zhiYaml := filepath.Join(dir, "zhi.yaml")
	if _, err := os.Stat(zhiYaml); os.IsNotExist(err) {
		t.Error("zhi.yaml was not created")
	}

	// Check config/app.yaml exists.
	configFile := filepath.Join(dir, "config", "app.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config/app.yaml was not created")
	}

	// Check templates/docker-compose.yml.tmpl exists.
	tmplFile := filepath.Join(dir, "templates", "docker-compose.yml.tmpl")
	if _, err := os.Stat(tmplFile); os.IsNotExist(err) {
		t.Error("templates/docker-compose.yml.tmpl was not created")
	}

	// Check .zhi/store/ directory exists.
	storeDir := filepath.Join(dir, ".zhi", "store")
	info, err := os.Stat(storeDir)
	if os.IsNotExist(err) {
		t.Error(".zhi/store/ was not created")
	} else if !info.IsDir() {
		t.Error(".zhi/store/ is not a directory")
	}

	// Check .zhi/components.json exists and has correct content.
	compFile := filepath.Join(dir, ".zhi", "components.json")
	data, err := os.ReadFile(compFile)
	if err != nil {
		t.Fatalf("reading components.json: %v", err)
	}
	var state map[string]bool
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("parsing components.json: %v", err)
	}
	if !state["pokedex-web"] {
		t.Error("pokedex-web component should be enabled")
	}
	if !state["pokemon-api"] {
		t.Error("pokemon-api component should be enabled")
	}
	if state["trainer-info"] {
		t.Error("trainer-info component should be disabled")
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()

	zhiYaml := filepath.Join(dir, "zhi.yaml")
	if err := os.WriteFile(zhiYaml, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	workspace = dir
	initForce = false

	cmd := newInitCmd()
	cmd.SetOut(new(strings.Builder))

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when zhi.yaml exists without --force")
	}
}

func TestInitForceOverwrite(t *testing.T) {
	dir := t.TempDir()

	zhiYaml := filepath.Join(dir, "zhi.yaml")
	if err := os.WriteFile(zhiYaml, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	workspace = dir
	initForce = true
	initConfigProvider = "structuredfile"
	initStoreProvider = "zhi-store-json"

	cmd := newInitCmd()
	cmd.SetOut(new(strings.Builder))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runInit with --force: %v", err)
	}

	data, err := os.ReadFile(zhiYaml)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "existing" {
		t.Error("zhi.yaml was not overwritten")
	}
}

func TestInitBareCreatesMinimalWorkspace(t *testing.T) {
	dir := t.TempDir()

	workspace = dir
	initForce = false
	initBare = true
	initConfigProvider = "structuredfile"
	initStoreProvider = "zhi-store-json"
	defer func() { initBare = false }()

	cmd := newInitCmd()
	cmd.SetOut(new(strings.Builder))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runInit --bare: %v", err)
	}

	// Check zhi.yaml exists and contains config + store sections.
	data, err := os.ReadFile(filepath.Join(dir, "zhi.yaml"))
	if err != nil {
		t.Fatal("zhi.yaml was not created")
	}
	content := string(data)
	if !strings.Contains(content, "provider: structuredfile") {
		t.Error("zhi.yaml should contain config provider")
	}
	if !strings.Contains(content, "provider: zhi-store-json") {
		t.Error("zhi.yaml should contain store provider")
	}
	if !strings.Contains(content, "directory: ./.zhi/store") {
		t.Error("zhi.yaml should contain store directory")
	}

	// Should NOT contain Pokedex demo content.
	if strings.Contains(content, "pokedex") {
		t.Error("bare zhi.yaml should not contain Pokedex content")
	}
	if strings.Contains(content, "components:") {
		t.Error("bare zhi.yaml should not contain components section")
	}
	if strings.Contains(content, "export:") {
		t.Error("bare zhi.yaml should not contain export section")
	}
	if strings.Contains(content, "apply:") {
		t.Error("bare zhi.yaml should not contain apply section")
	}

	// Check config/ directory exists.
	info, err := os.Stat(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal("config/ directory was not created")
	}
	if !info.IsDir() {
		t.Error("config should be a directory")
	}

	// Check .zhi/store/ exists.
	if _, err := os.Stat(filepath.Join(dir, ".zhi", "store")); os.IsNotExist(err) {
		t.Error(".zhi/store/ was not created")
	}

	// Should NOT have demo files.
	if _, err := os.Stat(filepath.Join(dir, "config", "app.yaml")); !os.IsNotExist(err) {
		t.Error("bare init should not create config/app.yaml")
	}
	if _, err := os.Stat(filepath.Join(dir, "templates")); !os.IsNotExist(err) {
		t.Error("bare init should not create templates/")
	}
	if _, err := os.Stat(filepath.Join(dir, "app-data")); !os.IsNotExist(err) {
		t.Error("bare init should not create app-data/")
	}
	if _, err := os.Stat(filepath.Join(dir, ".zhi", "components.json")); !os.IsNotExist(err) {
		t.Error("bare init should not create .zhi/components.json")
	}
}

func TestInitBareWithCustomProviders(t *testing.T) {
	dir := t.TempDir()

	workspace = dir
	initForce = false
	initBare = true
	initConfigProvider = "custom-config"
	initStoreProvider = "vault"
	defer func() {
		initBare = false
		initConfigProvider = "structuredfile"
		initStoreProvider = "zhi-store-json"
	}()

	cmd := newInitCmd()
	cmd.SetOut(new(strings.Builder))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runInit --bare custom: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "zhi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "provider: custom-config") {
		t.Error("zhi.yaml should contain custom config provider")
	}
	if !strings.Contains(content, "provider: vault") {
		t.Error("zhi.yaml should contain vault store provider")
	}
}

func TestInitBareForce(t *testing.T) {
	dir := t.TempDir()

	// Create an existing zhi.yaml.
	zhiYaml := filepath.Join(dir, "zhi.yaml")
	if err := os.WriteFile(zhiYaml, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	workspace = dir
	initForce = true
	initBare = true
	initConfigProvider = "structuredfile"
	initStoreProvider = "zhi-store-json"
	defer func() {
		initBare = false
		initForce = false
	}()

	cmd := newInitCmd()
	cmd.SetOut(new(strings.Builder))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runInit --bare --force: %v", err)
	}

	data, err := os.ReadFile(zhiYaml)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "existing" {
		t.Error("zhi.yaml was not overwritten")
	}
}

func TestInitComponentDefinitions(t *testing.T) {
	dir := t.TempDir()

	workspace = dir
	initForce = false
	initConfigProvider = "structuredfile"
	initStoreProvider = "zhi-store-json"

	cmd := newInitCmd()
	cmd.SetOut(new(strings.Builder))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "zhi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "name: pokedex-web") {
		t.Error("zhi.yaml should contain pokedex-web component")
	}
	if !strings.Contains(content, "name: pokemon-api") {
		t.Error("zhi.yaml should contain pokemon-api component")
	}
	if !strings.Contains(content, "name: trainer-info") {
		t.Error("zhi.yaml should contain trainer-info component")
	}
}
