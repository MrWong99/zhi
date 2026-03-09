package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// ApplyOutput represents a single line of output from the apply command.
type ApplyOutput struct {
	Line   string
	Stream string // "stdout" or "stderr"
	Time   time.Time
}

// ApplyResult holds the result of an apply command execution.
type ApplyResult struct {
	ExitCode int
	Duration time.Duration
	Err      error
}

// ApplyRunConfig holds the runtime configuration for an apply invocation.
type ApplyRunConfig struct {
	// Target is the resolved target config from workspace.
	Target ApplyTargetConfig
	// WorkspaceDir is the workspace root directory.
	WorkspaceDir string
	// SkipExport disables pre-export even if the target has it enabled.
	SkipExport bool
	// EnvOverrides are additional environment variables (from --env flags).
	EnvOverrides map[string]string
	// TimeoutOverride overrides the target timeout (seconds). -1 means use target config.
	TimeoutOverride int
	// EnabledComponents is a list of enabled component names.
	EnabledComponents []string
	// DisabledComponents is a list of disabled component names.
	DisabledComponents []string
}

// Apply executes an external command as described by the ApplyRunConfig and
// streams its stdout/stderr to the output channel. The channel is closed
// when the command finishes or ctx is cancelled.
//
// Non-zero exit codes are not treated as errors — they are reported in
// ApplyResult.ExitCode. Errors are reserved for failures to start the
// command or other unexpected conditions.
func Apply(ctx context.Context, cfg ApplyRunConfig, output chan<- ApplyOutput) (*ApplyResult, error) {
	defer close(output)

	start := time.Now()

	if cfg.Target.Command == "" {
		return nil, fmt.Errorf("apply command is empty")
	}

	// Determine timeout.
	timeoutSec := cfg.Target.Timeout
	if cfg.TimeoutOverride >= 0 {
		timeoutSec = cfg.TimeoutOverride
	}

	var cancel context.CancelFunc
	if timeoutSec > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	// Parse command using sh -c for shell feature support.
	cmd := exec.CommandContext(ctx, "sh", "-c", cfg.Target.Command)

	// Set working directory.
	workdir := cfg.WorkspaceDir
	if cfg.Target.Workdir != "" {
		if filepath.IsAbs(cfg.Target.Workdir) {
			workdir = cfg.Target.Workdir
		} else {
			workdir = filepath.Join(cfg.WorkspaceDir, cfg.Target.Workdir)
		}
	}
	cmd.Dir = workdir

	// Build environment.
	cmd.Env = buildApplyEnv(cfg)

	// Set up process group so we can signal the entire group on cancellation.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Use Cancel to send SIGTERM to the process group (not just the process)
	// when context is done. WaitDelay gives the process 5 seconds to exit
	// cleanly before Go closes pipes and sends SIGKILL.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second

	// Pipe stdout and stderr.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Start the command.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting command: %w", err)
	}

	// Scan stdout and stderr concurrently.
	done := make(chan struct{}, 2)
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			output <- ApplyOutput{
				Line:   scanner.Text(),
				Stream: "stdout",
				Time:   time.Now(),
			}
		}
		done <- struct{}{}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			output <- ApplyOutput{
				Line:   scanner.Text(),
				Stream: "stderr",
				Time:   time.Now(),
			}
		}
		done <- struct{}{}
	}()

	// Wait for both scanners to finish.
	<-done
	<-done

	// Wait for the command to finish.
	waitErr := cmd.Wait()
	duration := time.Since(start)

	result := &ApplyResult{
		Duration: duration,
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			result.Err = ctx.Err()
			result.ExitCode = -1
		} else {
			result.Err = waitErr
			result.ExitCode = -1
		}
	}

	return result, nil
}

// RunPreChecks executes the pre-check commands sequentially. Each command
// runs via "sh -c" with the same workdir and env as the main apply command.
// Output is streamed to the output channel. On the first non-zero exit,
// RunPreChecks returns an error identifying which command failed.
// Returns nil if all pre-checks pass (or if none are configured).
func RunPreChecks(ctx context.Context, cfg ApplyRunConfig, output chan<- ApplyOutput) error {
	if len(cfg.Target.PreCheck) == 0 {
		return nil
	}

	// Determine working directory.
	workdir := cfg.WorkspaceDir
	if cfg.Target.Workdir != "" {
		if filepath.IsAbs(cfg.Target.Workdir) {
			workdir = cfg.Target.Workdir
		} else {
			workdir = filepath.Join(cfg.WorkspaceDir, cfg.Target.Workdir)
		}
	}

	env := buildApplyEnv(cfg)

	for i, check := range cfg.Target.PreCheck {
		label := fmt.Sprintf("[pre-check %d/%d] %s", i+1, len(cfg.Target.PreCheck), check)
		output <- ApplyOutput{
			Line:   label,
			Stream: "stdout",
			Time:   time.Now(),
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", check)
		cmd.Dir = workdir
		cmd.Env = env

		// Pipe stdout and stderr.
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("pre-check %d: creating stdout pipe: %w", i+1, err)
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("pre-check %d: creating stderr pipe: %w", i+1, err)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("pre-check %d (%s): failed to start: %w", i+1, check, err)
		}

		// Scan stdout and stderr concurrently.
		done := make(chan struct{}, 2)
		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				output <- ApplyOutput{
					Line:   scanner.Text(),
					Stream: "stdout",
					Time:   time.Now(),
				}
			}
			done <- struct{}{}
		}()
		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				output <- ApplyOutput{
					Line:   scanner.Text(),
					Stream: "stderr",
					Time:   time.Now(),
				}
			}
			done <- struct{}{}
		}()

		<-done
		<-done

		if err := cmd.Wait(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return fmt.Errorf("pre-check %d failed (exit code %d): %s", i+1, exitErr.ExitCode(), check)
			}
			return fmt.Errorf("pre-check %d (%s): %w", i+1, check, err)
		}
	}

	return nil
}

// buildApplyEnv constructs the environment for the apply subprocess.
func buildApplyEnv(cfg ApplyRunConfig) []string {
	// Start with current environment.
	env := os.Environ()

	// Add workspace-defined env overrides.
	for k, v := range cfg.Target.Env {
		env = append(env, k+"="+v)
	}

	// Add CLI --env overrides (these take precedence over workspace env).
	for k, v := range cfg.EnvOverrides {
		env = append(env, k+"="+v)
	}

	// Add zhi-specific environment variables.
	env = append(env, "ZHI_WORKSPACE="+cfg.WorkspaceDir)

	sort.Strings(cfg.EnabledComponents)
	sort.Strings(cfg.DisabledComponents)
	env = append(env, "ZHI_ENABLED_COMPONENTS="+strings.Join(cfg.EnabledComponents, ","))
	env = append(env, "ZHI_DISABLED_COMPONENTS="+strings.Join(cfg.DisabledComponents, ","))

	return env
}
