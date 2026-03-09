package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

var applyCmd = &cobra.Command{
	Use:   "apply [target]",
	Short: "Run the configured apply command",
	Long: `Execute the apply command defined in the workspace configuration.

The apply system runs an external command (e.g. docker compose, kubectl, ansible)
after optionally exporting configuration files.

Examples:
  zhi apply                    # run the default apply target
  zhi apply destroy            # run the named "destroy" target
  zhi apply --dry-run          # print command without executing
  zhi apply --no-export        # skip pre-export step`,
	Args:              cobra.MaximumNArgs(1),
	PersistentPreRunE: withEngine,
	RunE:              runApply,
}

var (
	applyDryRun   bool
	applyNoExport bool
	applyTimeout  int
	applyEnvFlags []string
)

func init() {
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "print the command that would be executed without running it")
	applyCmd.Flags().BoolVar(&applyNoExport, "no-export", false, "skip the pre-export step even if configured")
	applyCmd.Flags().IntVar(&applyTimeout, "timeout", -1, "override the configured timeout (seconds, 0 = no timeout)")
	applyCmd.Flags().StringArrayVar(&applyEnvFlags, "env", nil, "add/override environment variables (KEY=VALUE, repeatable)")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	w := cmd.OutOrStdout()

	// Determine target name.
	var targetName string
	if len(args) > 0 {
		targetName = args[0]
	}

	// Build run config from workspace.
	runCfg, err := eng.BuildApplyRunConfig(targetName)
	if err != nil {
		return err
	}

	// Apply CLI overrides.
	if applyNoExport {
		runCfg.SkipExport = true
	}
	if applyTimeout >= 0 {
		runCfg.TimeoutOverride = applyTimeout
	}

	// Parse --env flags.
	if len(applyEnvFlags) > 0 {
		if runCfg.EnvOverrides == nil {
			runCfg.EnvOverrides = make(map[string]string)
		}
		for _, envStr := range applyEnvFlags {
			k, v, ok := strings.Cut(envStr, "=")
			if !ok {
				return fmt.Errorf("invalid --env value %q (expected KEY=VALUE)", envStr)
			}
			runCfg.EnvOverrides[k] = v
		}
	}

	// Dry run: just print what would happen.
	if applyDryRun {
		fmt.Fprintf(w, "Would run: %s\n", runCfg.Target.Command)
		workdir := eng.WorkspaceDir()
		if runCfg.Target.Workdir != "" {
			if filepath.IsAbs(runCfg.Target.Workdir) {
				workdir = runCfg.Target.Workdir
			} else {
				workdir = filepath.Join(eng.WorkspaceDir(), runCfg.Target.Workdir)
			}
		}
		fmt.Fprintf(w, "Workdir: %s\n", workdir)
		if runCfg.Target.PreExport && !runCfg.SkipExport {
			fmt.Fprintf(w, "Pre-export: enabled\n")
		}
		if len(runCfg.Target.PreCheck) > 0 {
			fmt.Fprintf(w, "Pre-checks:\n")
			for i, check := range runCfg.Target.PreCheck {
				fmt.Fprintf(w, "  [%d/%d] %s\n", i+1, len(runCfg.Target.PreCheck), check)
			}
		}
		if len(runCfg.Target.Env) > 0 {
			fmt.Fprintf(w, "Environment:\n")
			for k, v := range runCfg.Target.Env {
				fmt.Fprintf(w, "  %s=%s\n", k, v)
			}
		}
		if len(runCfg.EnvOverrides) > 0 {
			fmt.Fprintf(w, "Overrides:\n")
			for k, v := range runCfg.EnvOverrides {
				fmt.Fprintf(w, "  %s=%s\n", k, v)
			}
		}
		return nil
	}

	// Pre-export step.
	if runCfg.Target.PreExport && !runCfg.SkipExport {
		ws := eng.Workspace()
		if ws != nil && len(ws.Export.Templates) > 0 {
			td, err := core.PrepareTreeData(ctx, eng, false, "")
			if err != nil {
				return fmt.Errorf("preparing export data: %w", err)
			}

			configs := core.ExpandTemplates(ws.Export.Templates, ws.Dir, false)
			results, err := core.ExportAll(ctx, td, configs)
			if err != nil {
				return fmt.Errorf("pre-export: %w", err)
			}
			for _, r := range results {
				if r.OutputPath != "" && r.OutputPath != "-" {
					fmt.Fprintf(w, "Exporting: %s ... done\n", filepath.Base(r.OutputPath))
				}
			}
		}
	}

	// Run pre-checks.
	if len(runCfg.Target.PreCheck) > 0 {
		preCheckOutput := make(chan core.ApplyOutput, 100)
		preCheckDone := make(chan error, 1)
		go func() {
			defer close(preCheckOutput)
			preCheckDone <- core.RunPreChecks(ctx, runCfg, preCheckOutput)
		}()
		for line := range preCheckOutput {
			if line.Stream == "stderr" {
				fmt.Fprintln(cmd.ErrOrStderr(), line.Line)
			} else {
				fmt.Fprintln(w, line.Line)
			}
		}
		if err := <-preCheckDone; err != nil {
			return fmt.Errorf("pre-check: %w", err)
		}
	}

	// Run the apply command.
	fmt.Fprintf(w, "Running: %s\n", runCfg.Target.Command)

	output := make(chan core.ApplyOutput, 100)

	// Run apply in a goroutine and collect result.
	type applyResultErr struct {
		result *core.ApplyResult
		err    error
	}
	resultCh := make(chan applyResultErr, 1)
	go func() {
		r, err := core.Apply(ctx, runCfg, output)
		resultCh <- applyResultErr{result: r, err: err}
	}()

	// Stream output to terminal.
	for line := range output {
		if line.Stream == "stderr" {
			fmt.Fprintln(cmd.ErrOrStderr(), line.Line)
		} else {
			fmt.Fprintln(w, line.Line)
		}
	}

	// Get result.
	res := <-resultCh
	if res.err != nil {
		return fmt.Errorf("apply: %w", res.err)
	}

	fmt.Fprintf(w, "\nApply completed (exit code %d)\n", res.result.ExitCode)

	if res.result.ExitCode != 0 {
		os.Exit(res.result.ExitCode)
	}

	return nil
}
