package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

func TestBootstrapPolicyUsesWorkspaceMount(t *testing.T) {
	eng := setupTestEngine(t, nil)
	// Override the workspace store options with a custom mount and prefix.
	ws := eng.Workspace()
	ws.Store.Options = map[string]any{
		"mount":  "kv",
		"prefix": "myapp",
	}

	ctx := context.WithValue(context.Background(), engineKey, eng)
	ctx = context.WithValue(ctx, registryKey, core.NewRegistry())

	cmd := &cobra.Command{
		Use:  "bootstrap",
		RunE: runVaultCredentialsBootstrap,
	}
	cmd.Flags().BoolVar(&bootstrapDryRun, "dry-run", false, "")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--dry-run"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runVaultCredentialsBootstrap: %v", err)
	}

	output := buf.String()

	// Should use workspace mount "kv", not default "secret".
	if strings.Contains(output, "secret/data/") {
		t.Error("policy contains default mount 'secret' instead of workspace mount 'kv'")
	}
	if !strings.Contains(output, "kv/data/myapp/*") {
		t.Errorf("expected 'kv/data/myapp/*' in policy, got:\n%s", output)
	}
	if !strings.Contains(output, "kv/metadata/myapp/*") {
		t.Errorf("expected 'kv/metadata/myapp/*' in policy, got:\n%s", output)
	}
	if !strings.Contains(output, "Mount: kv") {
		t.Errorf("expected 'Mount: kv' in header, got:\n%s", output)
	}
}

func TestBootstrapPolicyDefaultsWithoutWorkspaceOptions(t *testing.T) {
	eng := setupTestEngine(t, nil)
	// No store options set — should fall back to defaults.

	ctx := context.WithValue(context.Background(), engineKey, eng)
	ctx = context.WithValue(ctx, registryKey, core.NewRegistry())

	cmd := &cobra.Command{
		Use:  "bootstrap",
		RunE: runVaultCredentialsBootstrap,
	}
	cmd.Flags().BoolVar(&bootstrapDryRun, "dry-run", false, "")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--dry-run"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runVaultCredentialsBootstrap: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "secret/data/zhi/*") {
		t.Errorf("expected default 'secret/data/zhi/*' in policy, got:\n%s", output)
	}
}

func TestBootstrapPolicyIncludesOrphanTokenPath(t *testing.T) {
	policy := bootstrapPolicyHCLForCLI("secret", "zhi")
	if !strings.Contains(policy, "auth/token/create-orphan") {
		t.Errorf("policy should reference auth/token/create-orphan, got:\n%s", policy)
	}
	if strings.Contains(policy, `"auth/token/create"`) {
		t.Errorf("policy should not reference auth/token/create (non-orphan), got:\n%s", policy)
	}
}
