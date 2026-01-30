package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

func TestListProviders(t *testing.T) {
	reg := core.DefaultRegistry()
	ctx := context.WithValue(context.Background(), registryKey, reg)

	cmd := &cobra.Command{
		Use:  "providers",
		RunE: runListProviders,
	}
	cmd.SetContext(ctx)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Reset global state.
	listJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runListProviders: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "structuredfile") {
		t.Errorf("expected 'structuredfile' in output, got: %s", output)
	}
	if !strings.Contains(output, "Config providers:") {
		t.Errorf("expected 'Config providers:' in output, got: %s", output)
	}
}

func TestListProvidersJSON(t *testing.T) {
	reg := core.DefaultRegistry()
	ctx := context.WithValue(context.Background(), registryKey, reg)

	cmd := &cobra.Command{
		Use:  "providers",
		RunE: runListProviders,
	}
	cmd.SetContext(ctx)

	listJSON = true
	defer func() { listJSON = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runListProviders JSON: %v", err)
	}
}

func TestListComponents(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}},
		{Name: "monitoring", Paths: []string{"monitoring/"}},
	}
	eng := setupTestEngine(t, components)

	ctx := context.WithValue(context.Background(), engineKey, eng)
	ctx = context.WithValue(ctx, registryKey, core.DefaultRegistry())

	cmd := &cobra.Command{
		Use:  "components",
		RunE: runListComponents,
	}
	cmd.SetContext(ctx)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	listJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runListComponents: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "database") {
		t.Errorf("expected 'database' in output, got: %s", output)
	}
	if !strings.Contains(output, "enabled") {
		t.Errorf("expected 'enabled' in output, got: %s", output)
	}
	if !strings.Contains(output, "redis") {
		t.Errorf("expected 'redis' in output, got: %s", output)
	}
}

func TestListComponentsJSON(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "components",
		RunE: runListComponents,
	}
	cmd.SetContext(ctx)

	listJSON = true
	defer func() { listJSON = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runListComponents JSON: %v", err)
	}
}

func TestListPaths(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "paths",
		RunE: runListPaths,
	}
	cmd.SetContext(ctx)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	listJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runListPaths: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "database/host") {
		t.Errorf("expected 'database/host' in output, got: %s", output)
	}
}

func TestListNoComponents(t *testing.T) {
	eng := setupTestEngine(t, nil)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "components",
		RunE: runListComponents,
	}
	cmd.SetContext(ctx)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	listJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runListComponents: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No components defined.") {
		t.Errorf("expected 'No components defined.' in output, got: %s", output)
	}
}
