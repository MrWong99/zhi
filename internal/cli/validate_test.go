package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

func TestValidateNoBlocking(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "validate",
		RunE: runValidate,
	}
	cmd.SetContext(ctx)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	validatePath = ""
	validateJSON = false

	err := cmd.Execute()

	output := buf.String()
	// Should contain the "using localhost" warning.
	if !strings.Contains(output, "using localhost") {
		t.Errorf("expected 'using localhost' warning, got: %s", output)
	}
	// No blocking, so no error.
	if err != nil {
		t.Errorf("expected no error for non-blocking results, got: %v", err)
	}
}

func TestValidateWithBlockingPort(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)

	// Set an invalid port.
	ctx := context.Background()
	ctxWithEng := context.WithValue(ctx, engineKey, eng)

	_ = eng.SetValue(ctx, "database/port", newValue(80))

	cmd := &cobra.Command{
		Use:  "validate",
		RunE: runValidate,
	}
	cmd.SetContext(ctxWithEng)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	validatePath = ""
	validateJSON = false

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for blocking validation")
	}

	output := buf.String()
	if !strings.Contains(output, "port must be between") {
		t.Errorf("expected port validation error, got: %s", output)
	}
}

func TestValidateJSON(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "validate",
		RunE: runValidate,
	}
	cmd.SetContext(ctx)

	validatePath = ""
	validateJSON = true
	defer func() { validateJSON = false }()

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("validate JSON: %v", err)
	}
}

func TestValidateSpecificPath(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "validate",
		RunE: runValidate,
	}
	cmd.SetContext(ctx)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	validatePath = "database/host"
	validateJSON = false
	defer func() { validatePath = "" }()

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("validate specific path: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "using localhost") {
		t.Errorf("expected 'using localhost' warning for specific path, got: %s", output)
	}
}

func TestValidateComponentDependencyViolation(t *testing.T) {
	// Use non-mandatory components so LoadState can actually create a violation.
	// "cache" is not mandatory and "session" depends on "cache".
	components := []core.ComponentDef{
		{Name: "app", Paths: []string{"app/"}, Mandatory: true},
		{Name: "cache", Paths: []string{"cache/"}},
		{Name: "session", Paths: []string{"session/"}, Dependencies: []string{"cache"}},
	}
	eng := setupTestEngine(t, components)

	// Force session enabled but cache disabled to create a dependency violation.
	cm := eng.Components()
	cm.LoadState(map[string]bool{"cache": false, "session": true})

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "validate",
		RunE: runValidate,
	}
	cmd.SetContext(ctx)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	validatePath = ""
	validateJSON = false

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for component dependency violation")
	}

	output := buf.String()
	if !strings.Contains(output, "BLOCKING") {
		t.Errorf("expected BLOCKING in output for dep violation, got: %s", output)
	}
}
