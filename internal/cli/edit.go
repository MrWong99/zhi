package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/ui"
)

var editCmd = &cobra.Command{
	Use:   "edit [tree-id]",
	Short: "Launch the interactive configuration editor",
	Long: `Launch an interactive UI for editing configuration.

The edit command loads the workspace configuration, creates a UI controller,
and launches the selected UI driver (default: tui).

Examples:
  zhi edit                  # launch default TUI editor
  zhi edit --ui tui         # explicitly select the TUI driver
  zhi edit my-config        # load a specific tree from the store`,
	Args:              cobra.MaximumNArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runEdit,
}

var editUIDriver string

func init() {
	editCmd.Flags().StringVar(&editUIDriver, "ui", "tui", "UI driver to use (default: tui)")
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	// Resolve UI driver.
	driver, err := ui.Get(editUIDriver)
	if err != nil {
		available := ui.List()
		if len(available) > 0 {
			return fmt.Errorf("%w\navailable UI drivers: %s", err, strings.Join(available, ", "))
		}
		return fmt.Errorf("%w\nno UI drivers are registered; install a UI plugin or rebuild with TUI support", err)
	}

	// Create UIController wrapping the engine.
	controller := ui.NewUIController(eng)

	// Launch the UI driver.
	ctx := cmd.Context()
	return driver.Run(ctx, controller)
}
