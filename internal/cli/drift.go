package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Detect configuration drift in exported files",
	Long: `Compare exported config files on disk against what the workspace would
currently generate. Reports files that have drifted (been modified outside zhi)
and files that are in sync.

Exit codes:
  0  No drift detected (all files in sync)
  1  Drift detected (one or more files differ)
  2  Error during drift check`,
	Example: `  zhi drift                                          # check all exports
  zhi drift --json                                   # JSON output for scripting
  zhi drift --watch --interval 5m                    # watch mode
  zhi drift --watch --interval 5m --on-drift "cmd"   # watch with hook`,
	PersistentPreRunE: withEngine,
	RunE:              runDrift,
}

var (
	driftJSON     bool
	driftWatch    bool
	driftInterval string
	driftOnDrift  string
)

func init() {
	driftCmd.Flags().BoolVar(&driftJSON, "json", false, "output as JSON")
	driftCmd.Flags().BoolVar(&driftWatch, "watch", false, "run continuously, checking at --interval")
	driftCmd.Flags().StringVar(&driftInterval, "interval", "1m", "check interval for watch mode (e.g. 30s, 5m)")
	driftCmd.Flags().StringVar(&driftOnDrift, "on-drift", "", "shell command to run when drift is detected")
	rootCmd.AddCommand(driftCmd)
}

// driftJSONOutput is the JSON output structure for drift results.
type driftJSONOutput struct {
	Drifted  []driftJSONEntry `json:"drifted"`
	InSync   []driftJSONEntry `json:"in_sync"`
	Warnings []string         `json:"warnings,omitempty"`
	Errors   []driftJSONError `json:"errors,omitempty"`
}

type driftJSONEntry struct {
	Name       string `json:"name"`
	OutputPath string `json:"output_path"`
	IsNew      bool   `json:"is_new,omitempty"`
	Diff       string `json:"diff,omitempty"`
}

type driftJSONError struct {
	Name       string `json:"name"`
	OutputPath string `json:"output_path,omitempty"`
	Error      string `json:"error"`
}

func runDrift(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	// Validate flag combinations.
	if !driftWatch && driftOnDrift != "" {
		return fmt.Errorf("--on-drift requires --watch")
	}

	if driftWatch {
		return runDriftWatch(cmd, eng)
	}

	return runDriftOnce(cmd, eng)
}

func runDriftOnce(cmd *cobra.Command, eng *core.Engine) error {
	ctx := cmd.Context()

	result, err := core.CheckDrift(ctx, eng)
	if err != nil {
		return err
	}

	if driftJSON {
		return printDriftJSON(cmd, result)
	}
	return printDriftHuman(cmd, result)
}

func runDriftWatch(cmd *cobra.Command, eng *core.Engine) error {
	ctx := cmd.Context()
	w := cmd.OutOrStdout()

	interval, err := time.ParseDuration(driftInterval)
	if err != nil {
		return fmt.Errorf("invalid --interval %q: %w", driftInterval, err)
	}

	// State tracking for edge-triggered output.
	type driftState int
	const (
		stateUnknown driftState = iota
		stateClean
		stateDrifted
	)
	prevState := stateUnknown

	check := func() {
		result, err := core.CheckDrift(ctx, eng)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "drift check error: %v\n", err)
			return
		}
		if result.HasErrors() {
			for _, e := range result.Errors {
				fmt.Fprintf(cmd.ErrOrStderr(), "drift check error: %s: %v\n", e.Name, e.Err)
			}
		}

		var currentState driftState
		if result.HasDrift() {
			currentState = stateDrifted
		} else {
			currentState = stateClean
		}

		if currentState == prevState {
			return
		}
		prevState = currentState

		if driftJSON {
			// NDJSON: one line per state change event.
			data, err := json.Marshal(toJSONOutput(result))
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "json marshal error: %v\n", err)
				return
			}
			fmt.Fprintln(w, string(data))
		} else {
			if currentState == stateDrifted {
				printDriftHuman(cmd, result)
			} else {
				fmt.Fprintf(w, "[%s] All files in sync\n", time.Now().Format(time.DateTime))
			}
		}

		// Fire on-drift hook on transition to drifted state.
		if currentState == stateDrifted && driftOnDrift != "" {
			hookCmd := exec.CommandContext(ctx, "sh", "-c", driftOnDrift)
			if out, err := hookCmd.CombinedOutput(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "on-drift hook failed: %v (%s)\n", err, strings.TrimSpace(string(out)))
			}
		}
	}

	// Run first check immediately.
	check()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			check()
		}
	}
}

func printDriftHuman(cmd *cobra.Command, result *core.DriftCheckResult) error {
	w := cmd.OutOrStdout()

	for _, warn := range result.Warnings {
		fmt.Fprintf(w, "Warning: %s\n", warn)
	}

	// If no templates were checked, just print the warning and exit clean.
	if len(result.Drifted) == 0 && len(result.InSync) == 0 && len(result.Errors) == 0 {
		return nil
	}

	fmt.Fprintln(w, "Drift Detection Report")
	fmt.Fprintln(w)

	if len(result.Drifted) > 0 {
		fmt.Fprintf(w, "DRIFTED (%d):\n", len(result.Drifted))
		for _, d := range result.Drifted {
			fmt.Fprintf(w, "  %s:\n", d.OutputPath)
			// Indent the diff output.
			for line := range strings.SplitSeq(strings.TrimRight(d.Diff, "\n"), "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
		fmt.Fprintln(w)
	}

	if len(result.InSync) > 0 {
		fmt.Fprintf(w, "IN SYNC (%d):\n", len(result.InSync))
		for _, s := range result.InSync {
			fmt.Fprintf(w, "  %s\n", s.OutputPath)
		}
		fmt.Fprintln(w)
	}

	if len(result.Errors) > 0 {
		fmt.Fprintf(w, "ERRORS (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Fprintf(w, "  %s: %v\n", e.Name, e.Err)
		}
		fmt.Fprintln(w)
	}

	if result.HasDrift() {
		fmt.Fprintln(w, "Run `zhi export` to reconcile, or `zhi export --diff` for full details.")
		return &driftDetectedError{}
	}

	if result.HasErrors() {
		return &driftCheckError{}
	}

	return nil
}

func printDriftJSON(cmd *cobra.Command, result *core.DriftCheckResult) error {
	w := cmd.OutOrStdout()
	out := toJSONOutput(result)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, string(data))

	if result.HasDrift() {
		return &driftDetectedError{}
	}
	if result.HasErrors() {
		return &driftCheckError{}
	}
	return nil
}

func toJSONOutput(result *core.DriftCheckResult) driftJSONOutput {
	out := driftJSONOutput{
		Drifted:  make([]driftJSONEntry, 0, len(result.Drifted)),
		InSync:   make([]driftJSONEntry, 0, len(result.InSync)),
		Warnings: result.Warnings,
	}
	for _, d := range result.Drifted {
		out.Drifted = append(out.Drifted, driftJSONEntry{
			Name:       d.Name,
			OutputPath: d.OutputPath,
			IsNew:      d.IsNew,
			Diff:       d.Diff,
		})
	}
	for _, s := range result.InSync {
		out.InSync = append(out.InSync, driftJSONEntry{
			Name:       s.Name,
			OutputPath: s.OutputPath,
		})
	}
	for _, e := range result.Errors {
		out.Errors = append(out.Errors, driftJSONError{
			Name:       e.Name,
			OutputPath: e.OutputPath,
			Error:      e.Err.Error(),
		})
	}
	return out
}

// driftDetectedError signals exit code 1.
type driftDetectedError struct{}

func (e *driftDetectedError) Error() string { return "drift detected" }
func (e *driftDetectedError) ExitCode() int { return 1 }

// driftCheckError signals exit code 2.
type driftCheckError struct{}

func (e *driftCheckError) Error() string { return "drift check encountered errors" }
func (e *driftCheckError) ExitCode() int { return 2 }
