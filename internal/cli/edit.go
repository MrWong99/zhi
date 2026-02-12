package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/providers/ui/webui"

	// Register the TUI driver.
	_ "github.com/MrWong99/zhi/internal/ui/tui"
)

var editCmd = &cobra.Command{
	Use:   "edit [tree-id]",
	Short: "Launch the interactive configuration editor",
	Long: `Launch the interactive configuration editor.

The editor supports tree navigation, value editing, component management,
validation, export preview, and apply command execution.

The UI provider can be configured in the workspace file (ui.provider) or
overridden with the --ui flag. If neither is set, the default "tui" driver
is used.

Available built-in UI providers:
  tui     Terminal UI using Bubbletea (default, requires TTY)
  webui   Browser-based UI served on localhost

Examples:
  zhi edit                    # launch the default UI (tui)
  zhi edit --ui tui           # explicitly select the TUI driver
  zhi edit --ui webui         # launch the web UI in your browser
  zhi edit myconfig           # edit a specific stored tree`,
	Args:              cobra.MaximumNArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runEdit,
}

var editUI string

func init() {
	editCmd.Flags().StringVar(&editUI, "ui", "", "UI driver to use (overrides workspace config)")
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	reg, err := registryFromCmd(cmd)
	if err != nil {
		return err
	}

	// Register builtin UI drivers (e.g. TUI, webui) into the plugin registry.
	ui.RegisterBuiltins(reg)

	// Determine which UI provider to use. Priority:
	// 1. --ui flag (explicit override)
	// 2. Workspace config (ui.provider)
	// 3. Default: "tui"
	uiName := editUI
	if uiName == "" {
		ws := eng.Workspace()
		if ws != nil && ws.UI.Provider != "" {
			uiName = ws.UI.Provider
		} else {
			uiName = "tui"
		}
	}

	wsDir := eng.WorkspaceDir()
	var options map[string]any
	if ws := eng.Workspace(); ws != nil && ws.UI.Provider == uiName {
		options = ws.UI.Options
	}

	plugin, err := reg.UIProvider(wsDir, uiName, options)
	if err != nil {
		available := reg.ListUI()
		return fmt.Errorf("resolving UI provider %q: %w\navailable UI providers: %v", uiName, err, available)
	}

	controller := ui.NewUIController(eng)
	if mc, err := newMarketplaceClient(); err != nil {
		controller.SetMarketplaceError(err)
	} else {
		controller.SetMarketplace(mc)
	}
	adapter := &ui.ControllerAdapter{Inner: controller}
	ctx := cmd.Context()

	// For the web UI, display the URL and open the browser.
	if w, ok := plugin.(*webui.WebUI); ok {
		return runWebUI(ctx, w, adapter, cmd)
	}

	return plugin.Run(ctx, adapter)
}

// runWebUI starts the web UI server, prints the URL, and blocks until
// the context is cancelled.
func runWebUI(ctx context.Context, w *webui.WebUI, adapter *ui.ControllerAdapter, cmd *cobra.Command) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Run(ctx, adapter)
	}()

	// Wait for the server to be ready and print the URL.
	addr, err := w.Addr(ctx)
	if err != nil {
		// If Addr fails, Run might have already returned with an error.
		select {
		case runErr := <-errCh:
			return runErr
		default:
			return err
		}
	}

	url := "http://" + addr
	fmt.Fprintf(cmd.ErrOrStderr(), "zhi webui: open %s in your browser (press Ctrl+C to stop)\n", url)

	return <-errCh
}
