package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/ui"

	// Register the TUI driver.
	_ "github.com/MrWong99/zhi/internal/ui/tui"
)

var editCmd = &cobra.Command{
	Use:   "edit [tree-id]",
	Short: "Launch the interactive configuration editor",
	Long: `Launch the interactive TUI for browsing and editing the configuration tree.

The editor supports tree navigation, value editing, component management,
validation, export preview, and apply command execution.

Examples:
  zhi edit                    # launch the TUI editor
  zhi edit --ui tui           # explicitly select the TUI driver
  zhi edit myconfig           # edit a specific stored tree`,
	Args:              cobra.MaximumNArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runEdit,
}

var editUI string

func init() {
	editCmd.Flags().StringVar(&editUI, "ui", "tui", "UI driver to use")
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	driver, err := ui.Get(editUI)
	if err != nil {
		available := ui.List()
		return fmt.Errorf("%w\navailable UI drivers: %v", err, available)
	}

	controller := ui.NewUIController(eng)
	ctx := cmd.Context()

	return driver.Run(ctx, controller)
}
