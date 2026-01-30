package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func newValue(v any) config.Value {
	return config.Value{Val: v}
}

func TestGetValue(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "get",
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	getRaw = false
	getJSON = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runGet: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "localhost") {
		t.Errorf("expected 'localhost' in output, got: %s", output)
	}
}

func TestGetValueRaw(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "get",
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	getRaw = true
	getJSON = false
	defer func() { getRaw = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runGet raw: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "localhost") {
		t.Errorf("expected 'localhost' in raw output, got: %s", output)
	}
}

func TestGetValueJSON(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "get",
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"database/host"})

	getJSON = true
	getRaw = false
	defer func() { getJSON = false }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runGet JSON: %v", err)
	}
}

func TestGetValueNotFound(t *testing.T) {
	eng := setupTestEngine(t, nil)
	ctx := context.WithValue(context.Background(), engineKey, eng)

	cmd := &cobra.Command{
		Use:  "get",
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent/path"})

	getRaw = false
	getJSON = false

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}
