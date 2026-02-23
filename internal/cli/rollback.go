package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Restore files from before the last export",
	Long: `Rollback restores exported files to their state before the last 'zhi export'
or 'zhi apply' (which runs export as a pre-step).

Only one level of rollback is maintained. After a successful rollback, the
snapshot is deleted and no further rollback is possible until the next export.

Examples:
  zhi rollback                 # restore files from last export`,
	PersistentPreRunE: withEngine,
	RunE:              runRollback,
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	manifest, err := core.Rollback(eng.WorkspaceDir())
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Rolled back to snapshot from %s\n", manifest.Timestamp.Format(time.RFC3339))
	for _, entry := range manifest.Files {
		if entry.WasNew {
			fmt.Fprintf(w, "  Removed: %s (was created by export)\n", entry.OriginalPath)
		} else {
			fmt.Fprintf(w, "  Restored: %s\n", entry.OriginalPath)
		}
	}
	return nil
}
