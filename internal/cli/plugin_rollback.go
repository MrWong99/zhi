package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
)

var pluginRollbackCmd = &cobra.Command{
	Use:   "rollback <name>",
	Short: "Restore a plugin to its previous version",
	Long: `Roll back a plugin to the version that was installed before the most
recent update. The previous version is restored from the backup created
during the last update.

Only the most recent previous version is kept. To install an older
version, use 'zhi plugin install <name>@<version> --force'.`,
	Example: `  zhi plugin rollback ansible-config`,
	Args:    cobra.ExactArgs(1),
	RunE:    runPluginRollback,
}

func init() {
	pluginCmd.AddCommand(pluginRollbackCmd)
}

func runPluginRollback(cmd *cobra.Command, args []string) error {
	name := args[0]
	w := cmd.OutOrStdout()

	metaDir := metadata.DefaultMetadataDir()
	metaStore := metadata.NewStore(metaDir)

	meta, err := metaStore.Load(name)
	if err != nil {
		return fmt.Errorf("loading plugin metadata: %w", err)
	}
	if meta == nil {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	pluginDir := core.DefaultPluginDir()
	binaryName := fmt.Sprintf("zhi-%s-%s", meta.Type, meta.Name)

	currentVersion := meta.Version

	restoredVersion, err := metadata.RestoreBackup(pluginDir, binaryName)
	if err != nil {
		fmt.Fprintf(w, "No backup available for %s.\n", name)
		fmt.Fprintf(w, "To install a specific version: zhi plugin install %s@<version> --force\n", name)
		return fmt.Errorf("rolling back %s: %w", name, err)
	}

	// Update metadata to reflect the rolled-back version.
	meta.Version = restoredVersion
	if err := metaStore.Save(meta); err != nil {
		return fmt.Errorf("updating metadata after rollback: %w", err)
	}

	fmt.Fprintf(w, "Rolled back %s: v%s -> v%s\n", name, currentVersion, restoredVersion)
	return nil
}
