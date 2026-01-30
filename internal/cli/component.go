package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var componentCmd = &cobra.Command{
	Use:   "component",
	Short: "Manage components",
	Long:  "Manage component state: enable, disable, list, and inspect components.",
}

var (
	componentJSON  bool
	componentForce bool
)

func init() {
	componentCmd.PersistentFlags().BoolVar(&componentJSON, "json", false, "output as JSON")
	componentCmd.AddCommand(componentListCmd)
	componentCmd.AddCommand(componentEnableCmd)
	componentCmd.AddCommand(componentDisableCmd)
	componentCmd.AddCommand(componentInfoCmd)
	rootCmd.AddCommand(componentCmd)
}

// --- component list ---

var componentListCmd = &cobra.Command{
	Use:               "list",
	Short:             "List all components with status",
	PersistentPreRunE: withEngine,
	RunE: func(cmd *cobra.Command, args []string) error {
		listJSON = componentJSON
		return runListComponents(cmd, args)
	},
}

// --- component enable ---

var componentEnableCmd = &cobra.Command{
	Use:               "enable <name>",
	Short:             "Enable a component and its dependencies",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runComponentEnable,
}

func runComponentEnable(cmd *cobra.Command, args []string) error {
	name := args[0]

	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	cm := eng.Components()

	enabled, err := cm.EnableWithReport(name)
	if err != nil {
		return fmt.Errorf("Error: %s", err)
	}

	// Save state.
	ws := eng.WorkspaceDir()
	if ws != "" {
		if err := saveComponentState(ws, cm.SaveState()); err != nil {
			return fmt.Errorf("saving component state: %w", err)
		}
	}

	if componentJSON {
		out := map[string]any{
			"enabled": enabled,
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(enabled) == 1 {
		fmt.Fprintf(w, "Enabled '%s'\n", name)
	} else {
		// The last entry is the target; others are auto-enabled dependencies.
		deps := enabled[:len(enabled)-1]
		fmt.Fprintf(w, "Enabled '%s' (also enabled dependency: %s)\n", name, quotedJoin(deps))
	}
	return nil
}

// --- component disable ---

var componentDisableCmd = &cobra.Command{
	Use:               "disable <name>",
	Short:             "Disable a component",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runComponentDisable,
}

func init() {
	componentDisableCmd.Flags().BoolVar(&componentForce, "force", false, "cascade disable to dependent components")
}

func runComponentDisable(cmd *cobra.Command, args []string) error {
	name := args[0]

	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	cm := eng.Components()

	if componentForce {
		disabled, err := cm.DisableCascade(name)
		if err != nil {
			return fmt.Errorf("Error: %s", err)
		}

		ws := eng.WorkspaceDir()
		if ws != "" {
			if err := saveComponentState(ws, cm.SaveState()); err != nil {
				return fmt.Errorf("saving component state: %w", err)
			}
		}

		if componentJSON {
			out := map[string]any{"disabled": disabled}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(w, string(data))
			return nil
		}

		if len(disabled) == 1 {
			fmt.Fprintf(w, "Disabled '%s'\n", name)
		} else {
			fmt.Fprintf(w, "Disabled '%s' (also disabled: %s)\n", name, quotedJoin(disabled[1:]))
		}
		return nil
	}

	if err := cm.Disable(name); err != nil {
		return fmt.Errorf("Error: %s", err)
	}

	ws := eng.WorkspaceDir()
	if ws != "" {
		if err := saveComponentState(ws, cm.SaveState()); err != nil {
			return fmt.Errorf("saving component state: %w", err)
		}
	}

	if componentJSON {
		out := map[string]any{"disabled": []string{name}}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	fmt.Fprintf(w, "Disabled '%s'\n", name)
	return nil
}

// --- component info ---

var componentInfoCmd = &cobra.Command{
	Use:               "info <name>",
	Short:             "Show detailed info about a component",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runComponentInfo,
}

type componentInfoOutput struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	Mandatory    bool     `json:"mandatory"`
	Paths        []string `json:"paths"`
	Dependencies []string `json:"dependencies"`
	Dependents   []string `json:"dependents"`
}

func runComponentInfo(cmd *cobra.Command, args []string) error {
	name := args[0]

	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	cm := eng.Components()

	def, ok := cm.Definition(name)
	if !ok {
		return fmt.Errorf("unknown component: %q", name)
	}

	status := "disabled"
	if cm.IsEnabled(name) {
		status = "enabled"
	}

	dependents := cm.Dependents(name)

	if componentJSON {
		out := componentInfoOutput{
			Name:         def.Name,
			Description:  def.Description,
			Status:       status,
			Mandatory:    def.Mandatory,
			Paths:        def.Paths,
			Dependencies: def.Dependencies,
			Dependents:   dependents,
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	fmt.Fprintf(w, "Name:         %s\n", def.Name)
	fmt.Fprintf(w, "Description:  %s\n", def.Description)
	fmt.Fprintf(w, "Status:       %s\n", status)
	fmt.Fprintf(w, "Mandatory:    %v\n", def.Mandatory)
	fmt.Fprintf(w, "Paths:        %s\n", strings.Join(def.Paths, ", "))
	if len(def.Dependencies) > 0 {
		fmt.Fprintf(w, "Dependencies: %s\n", strings.Join(def.Dependencies, ", "))
	} else {
		fmt.Fprintf(w, "Dependencies: -\n")
	}
	if len(dependents) > 0 {
		fmt.Fprintf(w, "Dependents:   %s\n", strings.Join(dependents, ", "))
	} else {
		fmt.Fprintf(w, "Dependents:   -\n")
	}

	return nil
}

// quotedJoin joins strings with quotes and commas.
func quotedJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "'" + item + "'"
	}
	return strings.Join(quoted, ", ")
}
