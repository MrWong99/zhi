package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

func setupComponentTestEngine(t *testing.T) *core.Engine {
	t.Helper()
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}},
		{Name: "monitoring", Paths: []string{"monitoring/"}},
	}
	return setupTestEngine(t, components)
}

func TestComponentEnable(t *testing.T) {
	eng := setupComponentTestEngine(t)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".zhi"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng.SetTestWorkspaceDir(dir)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "enable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentEnable,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"monitoring"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	componentJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("component enable: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Enabled 'monitoring'") {
		t.Errorf("expected enable confirmation, got: %s", output)
	}

	if !eng.Components().IsEnabled("monitoring") {
		t.Error("monitoring should be enabled")
	}
}

func TestComponentEnableWithDependencies(t *testing.T) {
	eng := setupComponentTestEngine(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".zhi"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng.SetTestWorkspaceDir(dir)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "enable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentEnable,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"redis"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	componentJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("component enable redis: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Enabled 'redis'") {
		t.Errorf("expected enable confirmation for redis, got: %s", output)
	}

	if !eng.Components().IsEnabled("redis") {
		t.Error("redis should be enabled")
	}
}

func TestComponentDisable(t *testing.T) {
	eng := setupComponentTestEngine(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".zhi"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng.SetTestWorkspaceDir(dir)

	_ = eng.Components().Enable("monitoring")

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "disable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentDisable,
	}
	cmd.Flags().BoolVar(&componentForce, "force", false, "cascade disable")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"monitoring"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	componentJSON = false
	componentForce = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("component disable: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Disabled 'monitoring'") {
		t.Errorf("expected disable confirmation, got: %s", output)
	}

	if eng.Components().IsEnabled("monitoring") {
		t.Error("monitoring should be disabled")
	}
}

func TestComponentDisableMandatory(t *testing.T) {
	eng := setupComponentTestEngine(t)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "disable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentDisable,
	}
	cmd.Flags().BoolVar(&componentForce, "force", false, "cascade disable")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database"})

	componentJSON = false
	componentForce = false

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when disabling mandatory component")
	}
	if !strings.Contains(err.Error(), "mandatory") {
		t.Errorf("expected 'mandatory' in error, got: %v", err)
	}
}

func TestComponentDisableWithDependents(t *testing.T) {
	eng := setupComponentTestEngine(t)

	_ = eng.Components().Enable("redis")

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "disable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentDisable,
	}
	cmd.Flags().BoolVar(&componentForce, "force", false, "cascade disable")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database"})

	componentJSON = false
	componentForce = false

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when disabling component with dependents")
	}
}

func TestComponentDisableForce(t *testing.T) {
	eng := setupComponentTestEngine(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".zhi"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng.SetTestWorkspaceDir(dir)

	_ = eng.Components().Enable("redis")
	_ = eng.Components().Enable("monitoring")

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "disable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentDisable,
	}
	cmd.Flags().BoolVar(&componentForce, "force", false, "cascade disable")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"monitoring"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	componentJSON = false
	componentForce = true
	defer func() { componentForce = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("component disable --force: %v", err)
	}

	if eng.Components().IsEnabled("monitoring") {
		t.Error("monitoring should be disabled after force disable")
	}
}

func TestComponentInfo(t *testing.T) {
	eng := setupComponentTestEngine(t)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "info",
		Args: cobra.ExactArgs(1),
		RunE: runComponentInfo,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"redis"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	componentJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("component info: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "redis") {
		t.Errorf("expected 'redis' in info output, got: %s", output)
	}
	if !strings.Contains(output, "database") {
		t.Errorf("expected dependency 'database' in info output, got: %s", output)
	}
}

func TestComponentInfoJSON(t *testing.T) {
	eng := setupComponentTestEngine(t)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "info",
		Args: cobra.ExactArgs(1),
		RunE: runComponentInfo,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	componentJSON = true
	defer func() { componentJSON = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("component info JSON: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"name": "database"`) {
		t.Errorf("expected JSON output with database, got: %s", output)
	}
}

func TestComponentInfoUnknown(t *testing.T) {
	eng := setupComponentTestEngine(t)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "info",
		Args: cobra.ExactArgs(1),
		RunE: runComponentInfo,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	componentJSON = false

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown component")
	}
}

func TestComponentEnableJSON(t *testing.T) {
	eng := setupComponentTestEngine(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".zhi"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng.SetTestWorkspaceDir(dir)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "enable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentEnable,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"monitoring"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	componentJSON = true
	defer func() { componentJSON = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("component enable JSON: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "monitoring") {
		t.Errorf("expected 'monitoring' in JSON output, got: %s", output)
	}
}

func TestComponentDisableForceCascade(t *testing.T) {
	eng := setupComponentTestEngine(t)

	_ = eng.Components().Enable("redis")

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "disable",
		Args: cobra.ExactArgs(1),
		RunE: runComponentDisable,
	}
	cmd.Flags().BoolVar(&componentForce, "force", false, "cascade disable")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database"})

	componentJSON = false
	componentForce = true
	defer func() { componentForce = false }()

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when force-disabling mandatory component")
	}
	if !strings.Contains(err.Error(), "mandatory") {
		t.Errorf("expected 'mandatory' in error, got: %v", err)
	}
}
