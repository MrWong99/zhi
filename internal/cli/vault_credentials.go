package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

var vaultCredentialsCmd = &cobra.Command{
	Use:   "vault-credentials",
	Short: "Manage Vault credentials for deployed applications",
	Long:  `Generate, refresh, and bootstrap Vault credentials managed by the zhi-store-vault-manager plugin.`,
}

var (
	refreshApp            string
	refreshOutput         string
	refreshExportTemplate string
)

var vaultCredentialsRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Generate fresh credentials for deployed applications",
	Long: `Generates fresh Vault credentials (AppRole wrapped secret_ids or tokens)
without requiring a full export/apply cycle. Useful for runtime scenarios
like application restarts.`,
	Example: `  zhi vault-credentials refresh
  zhi vault-credentials refresh --app web-api --output env
  zhi vault-credentials refresh --export-template docker-compose`,
	PersistentPreRunE: withEngine,
	RunE:              runVaultCredentialsRefresh,
}

var bootstrapDryRun bool

var vaultCredentialsBootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Generate the Vault admin policy for the vault-manager plugin",
	Long: `Generates and optionally applies the Vault ACL policy required by the
zhi-store-vault-manager meta-plugin's admin token. Run this once during
initial setup.`,
	Example: `  zhi vault-credentials bootstrap --dry-run
  zhi vault-credentials bootstrap`,
	PersistentPreRunE: withEngine,
	RunE:              runVaultCredentialsBootstrap,
}

func init() {
	vaultCredentialsRefreshCmd.Flags().StringVar(&refreshApp, "app", "", "Refresh only a specific app's credentials")
	vaultCredentialsRefreshCmd.Flags().StringVar(&refreshOutput, "output", "json", "Output format: json, env, quiet")
	vaultCredentialsRefreshCmd.Flags().StringVar(&refreshExportTemplate, "export-template", "", "Re-run a specific export template with fresh credentials")

	vaultCredentialsBootstrapCmd.Flags().BoolVar(&bootstrapDryRun, "dry-run", false, "Print the policy HCL without applying it")

	vaultCredentialsCmd.AddCommand(vaultCredentialsRefreshCmd)
	vaultCredentialsCmd.AddCommand(vaultCredentialsBootstrapCmd)
	rootCmd.AddCommand(vaultCredentialsCmd)
}

func runVaultCredentialsRefresh(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	w := cmd.OutOrStdout()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		return fmt.Errorf("loading tree: %w", err)
	}

	if err := eng.TransformForDisplay(ctx, tree); err != nil {
		return fmt.Errorf("applying transforms: %w", err)
	}

	// Extract credential values (paths starting with "vault/credentials/").
	const credPrefix = "vault/credentials/"
	creds := make(map[string]any)
	paths := tree.List()
	sort.Strings(paths)
	for _, p := range paths {
		if !strings.HasPrefix(p, credPrefix) {
			continue
		}
		// If --app is specified, filter to that app's credentials.
		if refreshApp != "" {
			// Expected path: vault/credentials/<app>/...
			rest := strings.TrimPrefix(p, credPrefix)
			parts := strings.SplitN(rest, "/", 2)
			if parts[0] != refreshApp {
				continue
			}
		}
		v, ok := tree.Get(p)
		if !ok {
			continue
		}
		creds[p] = v.Val
	}

	if len(creds) == 0 {
		if refreshApp != "" {
			return fmt.Errorf("no credentials found for app %q under %s", refreshApp, credPrefix)
		}
		return fmt.Errorf("no credentials found under %s", credPrefix)
	}

	switch refreshOutput {
	case "json":
		data, err := json.MarshalIndent(creds, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
	case "env":
		for _, p := range paths {
			val, ok := creds[p]
			if !ok {
				continue
			}
			// Convert path to env var name: vault/credentials/web-api/token -> VAULT_CREDENTIALS_WEB_API_TOKEN
			envKey := strings.ToUpper(strings.NewReplacer("/", "_", "-", "_", ".", "_").Replace(p))
			fmt.Fprintf(w, "%s=%v\n", envKey, val)
		}
	case "quiet":
		fmt.Fprintf(w, "%d credential(s) refreshed\n", len(creds))
	default:
		return fmt.Errorf("unsupported output format %q (expected json, env, or quiet)", refreshOutput)
	}

	// Re-run a specific export template with the fresh credentials.
	if refreshExportTemplate != "" {
		if err := runRefreshExportTemplate(ctx, cmd, eng, refreshExportTemplate); err != nil {
			return err
		}
	}

	return nil
}

// runRefreshExportTemplate re-renders a single named export template from the
// workspace configuration using the freshly loaded tree, mirroring the
// per-template export path of `zhi export`.
func runRefreshExportTemplate(ctx context.Context, cmd *cobra.Command, eng *core.Engine, name string) error {
	w := cmd.OutOrStdout()

	ws := eng.Workspace()
	if ws == nil {
		return fmt.Errorf("no workspace configuration loaded")
	}

	var selected *core.ExportTemplate
	for i := range ws.Export.Templates {
		if ws.Export.Templates[i].Name == name {
			selected = &ws.Export.Templates[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("unknown export template %q", name)
	}

	td, err := core.PrepareTreeData(ctx, eng, false, selected.Prefix)
	if err != nil {
		return err
	}

	configs := core.ExpandTemplates([]core.ExportTemplate{*selected}, ws.Dir, false)
	results, err := core.ExportAll(ctx, td, configs)
	if err != nil {
		return err
	}

	for _, r := range results {
		if r.OutputPath == "" || r.OutputPath == "-" {
			fmt.Fprintf(w, "--- %s ---\n", r.Name)
			fmt.Fprint(w, r.Content)
			if len(r.Content) > 0 && r.Content[len(r.Content)-1] != '\n' {
				fmt.Fprintln(w)
			}
		} else {
			fmt.Fprintf(w, "Exported: %s -> %s\n", r.Name, r.OutputPath)
		}
	}
	return nil
}

func runVaultCredentialsBootstrap(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	// Read mount and prefix from workspace config, falling back to defaults.
	mount := "secret"
	prefix := "zhi"
	if eng, err := engineFromCmd(cmd); err == nil {
		if ws := eng.Workspace(); ws != nil {
			if v, ok := ws.Store.Options["mount"].(string); ok && v != "" {
				mount = v
			}
			if v, ok := ws.Store.Options["prefix"].(string); ok && v != "" {
				prefix = v
			}
		}
	}

	policy := bootstrapPolicyHCLForCLI(mount, prefix)

	if bootstrapDryRun {
		fmt.Fprintln(w, "# Vault ACL policy for zhi-store-vault-manager")
		fmt.Fprintf(w, "# Mount: %s, Prefix: %s\n\n", mount, prefix)
		fmt.Fprint(w, policy)
		return nil
	}

	fmt.Fprintln(w, "# Vault ACL policy for zhi-store-vault-manager")
	fmt.Fprintf(w, "# Mount: %s, Prefix: %s\n\n", mount, prefix)
	fmt.Fprint(w, policy)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "To apply this policy, run:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  vault policy write zhi-vault-manager - <<'EOF'")
	fmt.Fprint(w, policy)
	fmt.Fprintln(w, "EOF")

	return nil
}

// bootstrapPolicyHCLForCLI generates the Vault ACL policy HCL required by the
// zhi-store-vault-manager meta-plugin's admin token. This is intentionally
// duplicated here (rather than importing from the example plugin's package main)
// so the CLI remains self-contained.
func bootstrapPolicyHCLForCLI(mount, prefix string) string {
	var b strings.Builder
	paths := []struct {
		path string
		caps []string
	}{
		{mount + "/data/" + prefix + "/*", []string{"read", "create", "update", "delete", "list"}},
		{mount + "/metadata/" + prefix + "/*", []string{"read", "list", "delete"}},
		{"sys/policies/acl/zhi-*", []string{"read", "create", "update", "delete", "list"}},
		{"auth/approle/role/zhi-*", []string{"read", "create", "update", "delete", "list"}},
		{"auth/approle/role/zhi-*/secret-id", []string{"create", "update"}},
		{"auth/token/create-orphan", []string{"create", "update"}},
	}
	for i, p := range paths {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "path %q {\n", p.path)
		quoted := make([]string, len(p.caps))
		for j, c := range p.caps {
			quoted[j] = fmt.Sprintf("%q", c)
		}
		fmt.Fprintf(&b, "  capabilities = [%s]\n", strings.Join(quoted, ", "))
		b.WriteString("}\n")
	}
	return b.String()
}
