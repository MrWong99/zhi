package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/metadata"
)

var pluginPinCmd = &cobra.Command{
	Use:   "pin <name>",
	Short: "Pin a plugin at its current version",
	Long: `Pin a plugin to prevent it from being updated during 'zhi plugin update --all'.

Pinned plugins can still be updated explicitly with 'zhi plugin update <name>'.`,
	Example: `  zhi plugin pin ansible-config`,
	Args:    cobra.ExactArgs(1),
	RunE:    runPluginPin,
}

var pluginUnpinCmd = &cobra.Command{
	Use:   "unpin <name>",
	Short: "Remove version pin from a plugin",
	Long: `Remove the version pin from a plugin, making it eligible for automatic
updates again during 'zhi plugin update --all'.`,
	Example: `  zhi plugin unpin ansible-config`,
	Args:    cobra.ExactArgs(1),
	RunE:    runPluginUnpin,
}

func init() {
	pluginCmd.AddCommand(pluginPinCmd)
	pluginCmd.AddCommand(pluginUnpinCmd)
}

func runPluginPin(cmd *cobra.Command, args []string) error {
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

	if meta.Pinned {
		fmt.Fprintf(w, "%s is already pinned at v%s\n", name, meta.Version)
		return nil
	}

	if err := metaStore.SetPinned(name, true); err != nil {
		return fmt.Errorf("pinning plugin: %w", err)
	}

	fmt.Fprintf(w, "Pinned %s at v%s\n", name, meta.Version)
	return nil
}

func runPluginUnpin(cmd *cobra.Command, args []string) error {
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

	if !meta.Pinned {
		fmt.Fprintf(w, "%s is not pinned\n", name)
		return nil
	}

	if err := metaStore.SetPinned(name, false); err != nil {
		return fmt.Errorf("unpinning plugin: %w", err)
	}

	fmt.Fprintf(w, "Unpinned %s (was at v%s)\n", name, meta.Version)
	return nil
}
