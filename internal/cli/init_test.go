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

	// Check templates/sample.tmpl exists.
	tmplFile := filepath.Join(dir, "templates", "sample.tmpl")
	if _, err := os.Stat(tmplFile); os.IsNotExist(err) {
		t.Error("templates/sample.tmpl was not created")
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
	if !state["app"] {
		t.Error("app component should be enabled")
	}
	if !state["database"] {
		t.Error("database component should be enabled")
	}
	if state["monitoring"] {
		t.Error("monitoring component should be disabled")
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
	if !strings.Contains(content, "name: app") {
		t.Error("zhi.yaml should contain app component")
	}
	if !strings.Contains(content, "name: database") {
		t.Error("zhi.yaml should contain database component")
	}
	if !strings.Contains(content, "name: monitoring") {
		t.Error("zhi.yaml should contain monitoring component")
	}
}
