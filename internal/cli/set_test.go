package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestSetValue(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host", "remotehost"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	setValidate = false
	setNoSave = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runSet: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Set database/host") {
		t.Errorf("expected confirmation in output, got: %s", output)
	}
	// Default behavior saves the value.
	if !strings.Contains(output, "(saved)") {
		t.Errorf("expected (saved) in output, got: %s", output)
	}

	// Verify the value was actually set.
	tree, err := eng.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := tree.Get("database/host")
	if !ok || v.Val != "remotehost" {
		t.Errorf("database/host = %v (ok=%v), want remotehost", v.Val, ok)
	}
}

func TestSetValueOverridesStoredValue(t *testing.T) {
	eng := setupTestEngine(t, nil)

	// Step 1: Save the default tree so the store has database/host = "localhost".
	tree, err := eng.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.SaveTree(context.Background(), tree); err != nil {
		t.Fatal(err)
	}

	// Step 2: Run "zhi set database/host remotehost".
	ctx := context.WithValue(context.Background(), engineKey, eng)
	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host", "remotehost"})
	cmd.SetOut(new(bytes.Buffer))

	setValidate = false
	setNoSave = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runSet: %v", err)
	}

	// Step 3: Load tree and assert the new value is "remotehost", not "localhost".
	tree, err = eng.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := tree.Get("database/host")
	if !ok {
		t.Fatal("expected database/host to exist")
	}
	if v.Val != "remotehost" {
		t.Errorf("database/host = %v, want \"remotehost\" (stale store value overwrote the set)", v.Val)
	}
}

func TestSetValueNoSave(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host", "remotehost"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	setValidate = false
	setNoSave = true
	defer func() { setNoSave = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runSet --no-save: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "(in-memory only)") {
		t.Errorf("expected (in-memory only) in output, got: %s", output)
	}
}

func TestSetValueWithValidation(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host", "remotehost"})

	setValidate = true
	setNoSave = false
	defer func() { setValidate = false }()

	// Should succeed since remotehost doesn't trigger the "localhost" warning.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("runSet with validation: %v", err)
	}
}

func TestSetValueWithBlockingValidation(t *testing.T) {
	components := []core.ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	eng := setupTestEngine(t, components)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	// Port 80 triggers a blocking validation error in the mock.
	cmd.SetArgs([]string{"database/port", "80"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	setValidate = true
	setNoSave = false
	defer func() { setValidate = false }()

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for blocking validation")
	}
	if !strings.Contains(err.Error(), "value not saved") {
		t.Errorf("expected 'value not saved' in error, got: %v", err)
	}
}

func TestSetValueNoStore(t *testing.T) {
	// Create engine without a store provider.
	mc := newMockConfig()
	reg := core.NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	})
	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
	}
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host", "remotehost"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	setValidate = false
	setNoSave = false

	// Should succeed with a warning, not error.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("runSet without store should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "not persisted") {
		t.Errorf("expected 'not persisted' warning in output, got: %s", output)
	}
}

func TestSetIntegerValue(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/port", "3306"})

	setValidate = false
	setNoSave = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runSet integer: %v", err)
	}

	tree, err := eng.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := tree.Get("database/port")
	if !ok {
		t.Fatal("expected database/port to exist")
	}
	// parseValue converts "3306" to int (matching yaml.v3 behavior).
	if v.Val != 3306 {
		t.Errorf("database/port = %v (%T), want int(3306)", v.Val, v.Val)
	}
}

func TestSetBooleanValue(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "set",
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"app/debug", "true"})

	setValidate = false
	setNoSave = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runSet boolean: %v", err)
	}

	tree, err := eng.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := tree.Get("app/debug")
	if !ok {
		t.Fatal("expected app/debug to exist")
	}
	if v.Val != true {
		t.Errorf("app/debug = %v, want true", v.Val)
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{"hello", "hello"},
		{"true", true},
		{"false", false},
		{"42", 42},
		{"3.14", 3.14},
		{`{"key":"val"}`, map[string]any{"key": "val"}},
		{`[1,2,3]`, []any{float64(1), float64(2), float64(3)}},
	}

	for _, tt := range tests {
		got := parseValue(tt.input)
		switch w := tt.want.(type) {
		case map[string]any:
			gotMap, ok := got.(map[string]any)
			if !ok {
				t.Errorf("parseValue(%q) = %T, want map", tt.input, got)
				continue
			}
			for k, v := range w {
				if gotMap[k] != v {
					t.Errorf("parseValue(%q)[%s] = %v, want %v", tt.input, k, gotMap[k], v)
				}
			}
		case []any:
			gotSlice, ok := got.([]any)
			if !ok {
				t.Errorf("parseValue(%q) = %T, want slice", tt.input, got)
				continue
			}
			if len(gotSlice) != len(w) {
				t.Errorf("parseValue(%q) len = %d, want %d", tt.input, len(gotSlice), len(w))
			}
		default:
			if got != tt.want {
				t.Errorf("parseValue(%q) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
			}
		}
	}
}
