package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/client"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/update"
)

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update one or all plugins to latest versions",
	Long: `Check for and install plugin updates from the marketplace.

Without arguments, checks for available updates. With a plugin name,
updates that specific plugin. Use --all to update all plugins at once.

Pinned plugins are skipped during --all updates.`,
	Example: `  zhi plugin update --check
  zhi plugin update --check --changelog
  zhi plugin update ansible-config
  zhi plugin update --all
  zhi plugin update --all --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginUpdate,
}

var (
	pluginUpdateCheck     bool
	pluginUpdateAll       bool
	pluginUpdateJSON      bool
	pluginUpdateChangelog bool
)

func init() {
	pluginUpdateCmd.Flags().BoolVar(&pluginUpdateCheck, "check", false, "only check for updates, don't install")
	pluginUpdateCmd.Flags().BoolVar(&pluginUpdateAll, "all", false, "update all plugins with available updates")
	pluginUpdateCmd.Flags().BoolVar(&pluginUpdateJSON, "json", false, "output as JSON")
	pluginUpdateCmd.Flags().BoolVar(&pluginUpdateChangelog, "changelog", false, "show changelogs for available updates")

	pluginCmd.AddCommand(pluginUpdateCmd)
}

func runPluginUpdate(cmd *cobra.Command, args []string) error {
	metaDir := metadata.DefaultMetadataDir()
	metaStore := metadata.NewStore(metaDir)

	mc, err := newMarketplaceClient()
	if err != nil {
		return fmt.Errorf("connecting to marketplace: %w", err)
	}

	cache := update.NewCache(update.DefaultCachePath(), update.DefaultCheckInterval)
	checker := update.NewChecker(metaStore, mc, cache)

	// If a specific name is given, update that plugin.
	if len(args) == 1 && !pluginUpdateCheck {
		return updateSinglePlugin(cmd, args[0], metaStore, checker)
	}

	// Check mode (default when no args and not --all).
	if pluginUpdateCheck || (len(args) == 0 && !pluginUpdateAll) {
		return checkUpdates(cmd, checker)
	}

	// --all mode: update everything.
	if pluginUpdateAll {
		return updateAllPlugins(cmd, metaStore, checker)
	}

	return cmd.Help()
}

func checkUpdates(cmd *cobra.Command, checker *update.Checker) error {
	w := cmd.OutOrStdout()

	fmt.Fprintln(w, "Checking for updates...")

	result, err := checker.CheckAll(cmd.Context(), false)
	if err != nil {
		return fmt.Errorf("checking updates: %w", err)
	}

	if pluginUpdateJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(result.Updates) == 0 {
		fmt.Fprintln(w, "\nAll plugins are up to date.")
		return nil
	}

	fmt.Fprintln(w)
	headers := []string{"PLUGIN", "INSTALLED", "AVAILABLE", "SEVERITY"}
	var rows [][]string
	for _, u := range result.Updates {
		severity := string(u.Scope)
		if u.HasAdvisory {
			severity = "security"
		}
		if u.Pinned {
			severity += " (pinned)"
		}
		rows = append(rows, []string{u.Name, u.Installed, u.Available, severity})
	}
	fprintTable(w, headers, rows)

	if pluginUpdateChangelog {
		for _, u := range result.Updates {
			if u.Changelog != "" {
				fmt.Fprintf(w, "\n%s %s -> %s:\n  %s\n", u.Name, u.Installed, u.Available, u.Changelog)
			}
		}
	}

	unpinned := 0
	for _, u := range result.Updates {
		if !u.Pinned {
			unpinned++
		}
	}
	if unpinned > 0 {
		fmt.Fprintf(w, "\n%d update(s) available. Run 'zhi plugin update --all' to install.\n", unpinned)
	}

	return nil
}

func updateSinglePlugin(cmd *cobra.Command, name string, metaStore *metadata.Store, checker *update.Checker) error {
	w := cmd.OutOrStdout()

	meta, err := metaStore.Load(name)
	if err != nil {
		return fmt.Errorf("loading plugin metadata: %w", err)
	}
	if meta == nil {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	info, err := checker.CheckPlugin(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}
	if info == nil {
		fmt.Fprintf(w, "%s is already up to date (v%s)\n", name, meta.Version)
		return nil
	}

	fmt.Fprintf(w, "Updating %s: v%s -> v%s\n", name, info.Installed, info.Available)

	// Back up the current binary before updating.
	pluginDir := core.DefaultPluginDir()
	binaryName := fmt.Sprintf("zhi-%s-%s", meta.Type, meta.Name)
	if err := metadata.BackupBinary(pluginDir, binaryName, meta.Version); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "  Warning: could not back up current version: %v\n", err)
	}

	// Install the new version via OCI pull.
	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	// Build the OCI reference for the new version.
	ref := buildUpdateRef(meta.Ref, info.Available)
	result, err := ociClient.PullPlugin(cmd.Context(), ref, client.PullOptions{Force: true})
	if err != nil {
		return fmt.Errorf("updating plugin: %w", err)
	}

	fmt.Fprintf(w, "Updated %s to v%s (%s)\n", result.Manifest.Name, result.Manifest.Version, result.Platform)
	return nil
}

func updateAllPlugins(cmd *cobra.Command, metaStore *metadata.Store, checker *update.Checker) error {
	w := cmd.OutOrStdout()
	errW := cmd.ErrOrStderr()

	fmt.Fprintln(w, "Checking for updates...")

	result, err := checker.CheckAll(cmd.Context(), false)
	if err != nil {
		return fmt.Errorf("checking updates: %w", err)
	}

	if len(result.Updates) == 0 {
		fmt.Fprintln(w, "All plugins are up to date.")
		return nil
	}

	updated := 0
	skipped := 0
	failed := 0
	for _, u := range result.Updates {
		if u.Pinned {
			fmt.Fprintf(w, "Skipping %s (pinned at v%s)\n", u.Name, u.Installed)
			skipped++
			continue
		}

		fmt.Fprintf(w, "Updating %s: v%s -> v%s\n", u.Name, u.Installed, u.Available)

		meta, err := metaStore.Load(u.Name)
		if err != nil || meta == nil {
			fmt.Fprintf(errW, "  Error: could not load metadata for %s\n", u.Name)
			failed++
			continue
		}

		pluginDir := core.DefaultPluginDir()
		binaryName := fmt.Sprintf("zhi-%s-%s", meta.Type, meta.Name)
		if err := metadata.BackupBinary(pluginDir, binaryName, meta.Version); err != nil {
			fmt.Fprintf(errW, "  Warning: could not back up current version: %v\n", err)
		}

		ociClient, err := newSharingClient()
		if err != nil {
			fmt.Fprintf(errW, "  Error: %v\n", err)
			failed++
			continue
		}

		ref := buildUpdateRef(meta.Ref, u.Available)
		_, err = ociClient.PullPlugin(cmd.Context(), ref, client.PullOptions{Force: true})
		if err != nil {
			fmt.Fprintf(errW, "  Error updating %s: %v\n", u.Name, err)
			failed++
			continue
		}

		updated++
		fmt.Fprintf(w, "  Updated %s to v%s\n", u.Name, u.Available)
	}

	fmt.Fprintf(w, "\n%d plugin(s) updated", updated)
	if skipped > 0 {
		fmt.Fprintf(w, ", %d skipped (pinned)", skipped)
	}
	if failed > 0 {
		fmt.Fprintf(w, ", %d failed", failed)
	}
	fmt.Fprintln(w, ".")
	if failed > 0 {
		return fmt.Errorf("%d plugin update(s) failed", failed)
	}
	return nil
}

// buildUpdateRef constructs an OCI reference for a specific version
// based on an existing reference. It replaces the tag with the new version.
func buildUpdateRef(existingRef, newVersion string) string {
	ref := existingRef

	// Handle digest references (host/repo@sha256:...) first.
	// The @ always precedes the digest, so strip everything from @ onwards.
	if atIdx := strings.LastIndex(ref, "@"); atIdx >= 0 {
		return ref[:atIdx] + ":v" + newVersion
	}

	// Handle tag references. Find the last colon that's after the last slash
	// to avoid matching port numbers or the oci:// scheme colon.
	if slashIdx := strings.LastIndex(ref, "/"); slashIdx >= 0 {
		tail := ref[slashIdx:]
		if colonIdx := strings.LastIndex(tail, ":"); colonIdx >= 0 {
			return ref[:slashIdx+colonIdx] + ":v" + newVersion
		}
	}

	// No tag or digest found, append version.
	return ref + ":v" + newVersion
}
