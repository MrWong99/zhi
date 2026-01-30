package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:               "get <path>",
	Short:             "Get a configuration value",
	Long:              "Get a single value from the configuration tree at the specified path.",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runGet,
}

var (
	getRaw  bool
	getJSON bool
)

func init() {
	getCmd.Flags().BoolVar(&getRaw, "raw", false, "print just the value, no metadata")
	getCmd.Flags().BoolVar(&getJSON, "json", false, "output as JSON including metadata")
	rootCmd.AddCommand(getCmd)
}

type getOutput struct {
	Path     string         `json:"path"`
	Value    any            `json:"value"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func runGet(cmd *cobra.Command, args []string) error {
	path := args[0]

	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	ctx := cmd.Context()

	tree, err := eng.LoadTree(ctx)
	if err != nil {
		return fmt.Errorf("loading tree: %w", err)
	}

	if err := eng.TransformForDisplay(ctx, tree); err != nil {
		return fmt.Errorf("applying transforms: %w", err)
	}

	v, ok := tree.Get(path)
	if !ok {
		return fmt.Errorf("path not found: %s", path)
	}

	if getJSON {
		out := getOutput{
			Path:     path,
			Value:    v.Val,
			Metadata: v.Metadata,
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if getRaw {
		fmt.Fprintln(w, v.Val)
		return nil
	}

	// Default: print value and metadata.
	fmt.Fprintf(w, "%s: %v\n", path, v.Val)
	if len(v.Metadata) > 0 {
		for k, mv := range v.Metadata {
			fmt.Fprintf(w, "  %s: %v\n", k, mv)
		}
	}
	return nil
}
