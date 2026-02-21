package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

var setCmd = &cobra.Command{
	Use:   "set <path> <value>",
	Short: "Set a configuration value",
	Long: `Set a single value in the configuration tree at the specified path.

The path must be a valid slash-delimited configuration path where each
segment matches [a-z][a-z0-9._-]*[a-z0-9].

Values are parsed as JSON objects/arrays, booleans, numbers, or strings
(in that order of precedence).`,
	Example: `  zhi set database/host localhost
  zhi set database/port 5432
  zhi set app/debug true
  zhi set app/tags '["web","api"]'
  zhi set database/host myhost --validate`,
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: withEngine,
	RunE:              runSet,
}

var setValidate bool

func init() {
	setCmd.Flags().BoolVar(&setValidate, "validate", false, "validate after setting the value")
	rootCmd.AddCommand(setCmd)
}

func runSet(cmd *cobra.Command, args []string) error {
	path := args[0]
	rawValue := args[1]

	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	ctx := cmd.Context()

	// Parse value: try JSON first, then number, then string.
	val := parseValue(rawValue)

	if err := eng.SetValue(ctx, path, config.Value{Val: val}); err != nil {
		return fmt.Errorf("setting value: %w", err)
	}

	fmt.Fprintf(w, "Set %s = %v\n", path, val)

	if setValidate {
		tree, err := eng.LoadTree(ctx)
		if err != nil {
			return fmt.Errorf("loading tree for validation: %w", err)
		}

		results, err := eng.Validate(ctx, tree)
		if err != nil {
			return fmt.Errorf("validation error: %w", err)
		}

		hasBlocking := false
		for _, r := range results {
			fmt.Fprintf(w, "  %s: %s\n", r.Severity, r.Message)
			if r.Severity == config.Blocking {
				hasBlocking = true
			}
		}
		if hasBlocking {
			return fmt.Errorf("validation failed with blocking errors")
		}
	}

	return nil
}

// parseValue attempts to parse the raw string into an appropriate Go type.
// It tries JSON object/array, boolean, integer, float, and falls back to string.
func parseValue(raw string) any {
	// Try JSON object or array.
	if len(raw) > 1 && (raw[0] == '{' || raw[0] == '[') {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			return v
		}
	}

	// Try boolean.
	if raw == "true" {
		return true
	}
	if raw == "false" {
		return false
	}

	// Try integer. Return int (not int64) for consistency with yaml.v3
	// which unmarshals integers as Go int.
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if i >= math.MinInt && i <= math.MaxInt {
			return int(i)
		}
		return i
	}

	// Try float.
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}

	// Default to string.
	return raw
}
