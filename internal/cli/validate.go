package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the current configuration tree",
	Long: `Validate the current configuration tree and print results grouped by severity.

Runs config provider validation for all paths and checks component dependency
constraints. Results are grouped by severity: blocking errors prevent deployment,
warnings signal potential issues, and info messages are informational.

The exit code is 1 if any blocking errors are found.`,
	Example: `  zhi validate
  zhi validate --path database/host
  zhi validate --json`,
	PersistentPreRunE: withEngine,
	RunE:              runValidate,
}

var (
	validatePath string
	validateJSON bool
)

func init() {
	validateCmd.Flags().StringVar(&validatePath, "path", "", "validate only a specific path")
	validateCmd.Flags().BoolVar(&validateJSON, "json", false, "output validation results as JSON")
	rootCmd.AddCommand(validateCmd)
}

type validationOutput struct {
	Path     string `json:"path"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func runValidate(cmd *cobra.Command, _ []string) error {
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

	// Apply transforms.
	if err := eng.TransformForDisplay(ctx, tree); err != nil {
		return fmt.Errorf("applying transforms: %w", err)
	}

	// Build annotated results with paths.
	type annotatedResult struct {
		Path     string
		Severity config.Severity
		Message  string
	}

	var annotated []annotatedResult

	if validatePath != "" {
		// Validate a specific path only.
		vr, err := eng.ValidatePath(ctx, validatePath, tree)
		if err != nil {
			return err
		}
		for _, r := range vr {
			annotated = append(annotated, annotatedResult{
				Path:     validatePath,
				Severity: r.Severity,
				Message:  r.Message,
			})
		}
		// Also include component dep results.
		for _, depErr := range eng.Components().ValidateDependencies() {
			annotated = append(annotated, annotatedResult{
				Path:     "_components",
				Severity: config.Blocking,
				Message:  depErr.Error(),
			})
		}
	} else {
		// Run per-path to get path association.
		for _, path := range tree.List() {
			vr, err := eng.ValidatePath(ctx, path, tree)
			if err != nil {
				return err
			}
			for _, r := range vr {
				annotated = append(annotated, annotatedResult{
					Path:     path,
					Severity: r.Severity,
					Message:  r.Message,
				})
			}
		}
		// Add component dependency results.
		for _, depErr := range eng.Components().ValidateDependencies() {
			annotated = append(annotated, annotatedResult{
				Path:     "_components",
				Severity: config.Blocking,
				Message:  depErr.Error(),
			})
		}
	}

	// Sort by severity (blocking first).
	sort.Slice(annotated, func(i, j int) bool {
		if annotated[i].Severity != annotated[j].Severity {
			return annotated[i].Severity > annotated[j].Severity
		}
		return annotated[i].Path < annotated[j].Path
	})

	if validateJSON {
		var out []validationOutput
		for _, r := range annotated {
			out = append(out, validationOutput{
				Path:     r.Path,
				Severity: r.Severity.String(),
				Message:  r.Message,
			})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
	} else {
		if len(annotated) > 0 {
			rows := make([][]string, len(annotated))
			for i, r := range annotated {
				rows[i] = []string{strings.ToUpper(r.Severity.String()), r.Path, r.Message}
			}
			fprintTable(w, []string{"SEVERITY", "PATH", "MESSAGE"}, rows)
		}

		blocking := 0
		warnings := 0
		info := 0
		for _, r := range annotated {
			switch r.Severity {
			case config.Blocking:
				blocking++
			case config.Warning:
				warnings++
			case config.Info:
				info++
			}
		}
		fmt.Fprintf(w, "\nValidation: %d blocking, %d warning, %d info\n", blocking, warnings, info)
	}

	// Exit code 1 if any blocking results.
	for _, r := range annotated {
		if r.Severity == config.Blocking {
			return fmt.Errorf("validation failed with blocking errors")
		}
	}

	return nil
}
