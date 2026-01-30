package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
)

// testEditDriver is a UIDriver that records whether Run was called.
type testEditDriver struct {
	runCalled bool
}

func (d *testEditDriver) Run(_ context.Context, _ *ui.UIController) error {
	d.runCalled = true
	return nil
}

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "edit [tree-id]",
		Short:         "Launch the interactive configuration editor",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE:          runEdit,
	}
	cmd.Flags().StringVar(&editUIDriver, "ui", "tui", "UI driver to use")
	return cmd
}

func resetEditFlags() {
	editUIDriver = "tui"
}

func TestEditDefaultUI(t *testing.T) {
	resetEditFlags()
	ui.Reset()
	defer ui.Reset()

	driver := &testEditDriver{}
	ui.Register("tui", func() ui.UIDriver {
		return driver
	})

	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newEditCmd()
	setEngineContext(cmd, eng, reg)

	_, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if !driver.runCalled {
		t.Error("expected TUI driver Run to be called")
	}
}

func TestEditExplicitUI(t *testing.T) {
	resetEditFlags()
	ui.Reset()
	defer ui.Reset()

	driver := &testEditDriver{}
	ui.Register("custom", func() ui.UIDriver {
		return driver
	})

	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newEditCmd()
	setEngineContext(cmd, eng, reg)

	_, err := executeCommand(cmd, "--ui", "custom")
	if err != nil {
		t.Fatalf("edit --ui custom: %v", err)
	}
	if !driver.runCalled {
		t.Error("expected custom driver Run to be called")
	}
}

func TestEditUnknownUI(t *testing.T) {
	resetEditFlags()
	ui.Reset()
	defer ui.Reset()

	// Register one driver so the error message can list available options.
	ui.Register("tui", func() ui.UIDriver {
		return &testEditDriver{}
	})

	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newEditCmd()
	setEngineContext(cmd, eng, reg)

	_, err := executeCommand(cmd, "--ui", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown UI driver")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the unknown driver name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "tui") {
		t.Errorf("error should list available drivers, got: %v", err)
	}
}

func TestEditUnknownUINoDrivers(t *testing.T) {
	resetEditFlags()
	ui.Reset()
	defer ui.Reset()

	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newEditCmd()
	setEngineContext(cmd, eng, reg)

	_, err := executeCommand(cmd, "--ui", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown UI driver")
	}
	if !strings.Contains(err.Error(), "no") {
		t.Errorf("error should indicate no drivers registered, got: %v", err)
	}
}
