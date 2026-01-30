package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

func newApplyTestCmd(eng *core.Engine, reg *core.Registry) *cobra.Command {
	root := newTestRootCmd()

	cmd := &cobra.Command{
		Use:   "apply [target]",
		Short: "Run the configured apply command",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runApply,
	}
	cmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "print the command without executing")
	cmd.Flags().BoolVar(&applyNoExport, "no-export", false, "skip the pre-export step")
	cmd.Flags().IntVar(&applyTimeout, "timeout", -1, "override timeout (seconds)")
	cmd.Flags().StringArrayVar(&applyEnvFlags, "env", nil, "add/override env vars (KEY=VALUE)")

	root.AddCommand(cmd)
	setEngineContext(root, eng, reg)

	return root
}

func setupApplyTestEngine(t *testing.T, applyConfig core.ApplyConfig) (*core.Engine, *core.Registry) {
	t.Helper()

	// Reset global flags.
	applyDryRun = false
	applyNoExport = false
	applyTimeout = -1
	applyEnvFlags = nil

	reg := core.NewRegistry()
	mc := newMockConfig()
	mt := &mockTransform{}
	ms := newMockStore()

	_ = reg.RegisterConfig("mock", func(map[string]any) (config.Plugin, error) {
		return mc, nil
	})
	_ = reg.RegisterTransform("mock", func(map[string]any) (transform.Plugin, error) {
		return mt, nil
	})
	_ = reg.RegisterStore("mock", func(map[string]any) (store.Plugin, error) {
		return ms, nil
	})

	ws := &core.WorkspaceConfig{
		Version:   "1",
		Config:    core.ProviderRef{Provider: "mock"},
		Transform: []core.ProviderRef{{Provider: "mock"}},
		Store:     core.ProviderRef{Provider: "mock"},
		Apply:     applyConfig,
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	eng.SetTestWorkspaceDir(t.TempDir())

	return eng, reg
}

func TestApplyCLIDryRun(t *testing.T) {
	eng, reg := setupApplyTestEngine(t, core.ApplyConfig{
		Command:   "docker compose up -d",
		PreExport: true,
		Env:       map[string]string{"FOO": "bar"},
	})

	root := newApplyTestCmd(eng, reg)
	output, err := executeCommand(root, "apply", "--dry-run")
	if err != nil {
		t.Fatalf("apply --dry-run: %v", err)
	}

	if !strings.Contains(output, "Would run: docker compose up -d") {
		t.Errorf("expected 'Would run' in output, got: %s", output)
	}
	if !strings.Contains(output, "Pre-export: enabled") {
		t.Errorf("expected 'Pre-export: enabled' in output, got: %s", output)
	}
}

func TestApplyCLIDryRunNoExport(t *testing.T) {
	eng, reg := setupApplyTestEngine(t, core.ApplyConfig{
		Command:   "echo hello",
		PreExport: true,
	})

	root := newApplyTestCmd(eng, reg)
	output, err := executeCommand(root, "apply", "--dry-run", "--no-export")
	if err != nil {
		t.Fatalf("apply --dry-run --no-export: %v", err)
	}

	if !strings.Contains(output, "Would run: echo hello") {
		t.Errorf("expected 'Would run' in output, got: %s", output)
	}
	if strings.Contains(output, "Pre-export: enabled") {
		t.Errorf("should not show pre-export when --no-export is set, got: %s", output)
	}
}

func TestApplyCLIExecution(t *testing.T) {
	eng, reg := setupApplyTestEngine(t, core.ApplyConfig{
		Command: "echo test_output",
	})

	root := newApplyTestCmd(eng, reg)
	output, err := executeCommand(root, "apply")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if !strings.Contains(output, "Running: echo test_output") {
		t.Errorf("expected 'Running:' in output, got: %s", output)
	}
	if !strings.Contains(output, "test_output") {
		t.Errorf("expected command output in result, got: %s", output)
	}
	if !strings.Contains(output, "Apply completed (exit code 0)") {
		t.Errorf("expected 'Apply completed (exit code 0)' in output, got: %s", output)
	}
}

func TestApplyCLINamedTarget(t *testing.T) {
	eng, reg := setupApplyTestEngine(t, core.ApplyConfig{
		Targets: map[string]core.ApplyTargetConfig{
			"default": {Command: "echo default_target"},
			"destroy": {Command: "echo destroy_target"},
		},
	})

	root := newApplyTestCmd(eng, reg)
	output, err := executeCommand(root, "apply", "destroy", "--dry-run")
	if err != nil {
		t.Fatalf("apply destroy --dry-run: %v", err)
	}

	if !strings.Contains(output, "Would run: echo destroy_target") {
		t.Errorf("expected destroy target command in output, got: %s", output)
	}
}

func TestApplyCLINoConfig(t *testing.T) {
	eng, reg := setupApplyTestEngine(t, core.ApplyConfig{})

	root := newApplyTestCmd(eng, reg)
	_, err := executeCommand(root, "apply")
	if err == nil {
		t.Fatal("expected error when no apply config is set")
	}
	if !strings.Contains(err.Error(), "no apply command configured") {
		t.Errorf("error = %q, expected to mention 'no apply command configured'", err.Error())
	}
}

func TestApplyCLIUnknownTarget(t *testing.T) {
	eng, reg := setupApplyTestEngine(t, core.ApplyConfig{
		Targets: map[string]core.ApplyTargetConfig{
			"default": {Command: "echo hello"},
		},
	})

	root := newApplyTestCmd(eng, reg)
	_, err := executeCommand(root, "apply", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
	if !strings.Contains(err.Error(), "unknown apply target") {
		t.Errorf("error = %q, expected to mention 'unknown apply target'", err.Error())
	}
}
