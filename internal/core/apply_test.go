package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplySuccessfulCommand(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "echo hello"},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	// Collect output lines.
	var lines []ApplyOutput
	for line := range output {
		lines = append(lines, line)
	}

	if len(lines) != 1 {
		t.Fatalf("got %d output lines, want 1", len(lines))
	}
	if lines[0].Line != "hello" {
		t.Errorf("output = %q, want %q", lines[0].Line, "hello")
	}
	if lines[0].Stream != "stdout" {
		t.Errorf("stream = %q, want %q", lines[0].Stream, "stdout")
	}
	if lines[0].Time.IsZero() {
		t.Error("output timestamp is zero")
	}
}

func TestApplyNonZeroExitCode(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "exit 42"},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Drain output.
	for range output {
	}

	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v (non-zero exit is not a Go error)", result.Err)
	}
}

func TestApplyContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "sleep 60"},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)

	done := make(chan struct{})
	var result *ApplyResult
	var applyErr error
	go func() {
		defer close(done)
		result, applyErr = Apply(ctx, cfg, output)
	}()

	// Give the process a moment to start, then cancel.
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Drain output channel.
	for range output {
	}

	<-done

	if applyErr != nil {
		t.Fatalf("Apply returned error: %v", applyErr)
	}

	// The process should have been terminated.
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code after cancellation")
	}
}

func TestApplyStdoutAndStderrSeparation(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: `echo stdout_line && echo stderr_line >&2`},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Collect output.
	var stdoutLines, stderrLines []string
	for line := range output {
		switch line.Stream {
		case "stdout":
			stdoutLines = append(stdoutLines, line.Line)
		case "stderr":
			stderrLines = append(stderrLines, line.Line)
		}
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	if len(stdoutLines) != 1 || stdoutLines[0] != "stdout_line" {
		t.Errorf("stdout lines = %v, want [stdout_line]", stdoutLines)
	}
	if len(stderrLines) != 1 || stderrLines[0] != "stderr_line" {
		t.Errorf("stderr lines = %v, want [stderr_line]", stderrLines)
	}
}

func TestApplyEnvironmentVariables(t *testing.T) {
	wsDir := t.TempDir()
	cfg := ApplyRunConfig{
		Target: ApplyTargetConfig{
			Command: `echo "WS=$ZHI_WORKSPACE" && echo "EN=$ZHI_ENABLED_COMPONENTS" && echo "DIS=$ZHI_DISABLED_COMPONENTS" && echo "CUSTOM=$MY_VAR"`,
			Env:     map[string]string{"MY_VAR": "from_workspace"},
		},
		WorkspaceDir:       wsDir,
		TimeoutOverride:    -1,
		EnabledComponents:  []string{"database", "auth"},
		DisabledComponents: []string{"extras"},
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	// Check environment was set.
	found := map[string]bool{
		"WS=" + wsDir:           false,
		"EN=auth,database":      false,
		"DIS=extras":            false,
		"CUSTOM=from_workspace": false,
	}
	for _, line := range lines {
		for expected := range found {
			if line == expected {
				found[expected] = true
			}
		}
	}
	for expected, ok := range found {
		if !ok {
			t.Errorf("expected output line %q not found in %v", expected, lines)
		}
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := ApplyRunConfig{
		Target: ApplyTargetConfig{
			Command: `echo "$MY_VAR"`,
			Env:     map[string]string{"MY_VAR": "workspace_val"},
		},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
		EnvOverrides:    map[string]string{"MY_VAR": "override_val"},
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	// The override should take effect (last wins in env list).
	if len(lines) != 1 || lines[0] != "override_val" {
		t.Errorf("output = %v, want [override_val]", lines)
	}
}

func TestApplyWorkingDirectory(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "pwd"},
		WorkspaceDir:    dir,
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	if len(lines) != 1 || lines[0] != dir {
		t.Errorf("working directory = %v, want %s", lines, dir)
	}
}

func TestApplyCustomWorkdir(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "pwd", Workdir: "."},
		WorkspaceDir:    dir,
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	if len(lines) != 1 || lines[0] != dir {
		t.Errorf("working directory = %v, want %s", lines, dir)
	}
}

func TestApplyTimeout(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "sleep 60", Timeout: 1},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	start := time.Now()
	result, err := Apply(context.Background(), cfg, output)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Drain output.
	for range output {
	}

	// Should have finished quickly due to timeout.
	if elapsed > 10*time.Second {
		t.Errorf("elapsed = %v, expected less than 10s with 1s timeout", elapsed)
	}

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code after timeout")
	}
}

func TestApplyTimeoutOverride(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "sleep 60", Timeout: 60},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: 1, // override to 1 second
	}

	output := make(chan ApplyOutput, 100)
	start := time.Now()
	result, err := Apply(context.Background(), cfg, output)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Drain output.
	for range output {
	}

	if elapsed > 10*time.Second {
		t.Errorf("elapsed = %v, expected less than 10s with 1s timeout override", elapsed)
	}

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code after timeout")
	}
}

func TestApplyEmptyCommand(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: ""},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	_, err := Apply(context.Background(), cfg, output)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	// Drain output (should be closed by Apply on error path too).
	for range output {
	}
}

func TestApplyDuration(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: "echo fast"},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Drain output.
	for range output {
	}

	if result.Duration <= 0 {
		t.Errorf("duration = %v, expected positive", result.Duration)
	}
}

func TestApplyMultipleOutputLines(t *testing.T) {
	cfg := ApplyRunConfig{
		Target:          ApplyTargetConfig{Command: `echo line1 && echo line2 && echo line3`},
		WorkspaceDir:    t.TempDir(),
		TimeoutOverride: -1,
	}

	output := make(chan ApplyOutput, 100)
	result, err := Apply(context.Background(), cfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	want := []string{"line1", "line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(lines), len(want), lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestApplyResolveTargetSimple(t *testing.T) {
	ac := &ApplyConfig{
		Command:   "echo hello",
		Workdir:   "subdir",
		PreExport: true,
		Env:       map[string]string{"FOO": "bar"},
		Timeout:   60,
	}

	target, err := ac.ResolveTarget("")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}

	if target.Command != "echo hello" {
		t.Errorf("command = %q, want %q", target.Command, "echo hello")
	}
	if target.Workdir != "subdir" {
		t.Errorf("workdir = %q, want %q", target.Workdir, "subdir")
	}
	if !target.PreExport {
		t.Error("pre-export should be true")
	}
	if target.Env["FOO"] != "bar" {
		t.Errorf("env FOO = %q, want %q", target.Env["FOO"], "bar")
	}
	if target.Timeout != 60 {
		t.Errorf("timeout = %d, want 60", target.Timeout)
	}
}

func TestApplyResolveTargetNamed(t *testing.T) {
	ac := &ApplyConfig{
		Targets: map[string]ApplyTargetConfig{
			"default": {Command: "up"},
			"destroy": {Command: "down"},
		},
	}

	// Default target.
	target, err := ac.ResolveTarget("")
	if err != nil {
		t.Fatalf("ResolveTarget default: %v", err)
	}
	if target.Command != "up" {
		t.Errorf("default command = %q, want %q", target.Command, "up")
	}

	// Named target.
	target, err = ac.ResolveTarget("destroy")
	if err != nil {
		t.Fatalf("ResolveTarget destroy: %v", err)
	}
	if target.Command != "down" {
		t.Errorf("destroy command = %q, want %q", target.Command, "down")
	}

	// Unknown target.
	_, err = ac.ResolveTarget("unknown")
	if err == nil {
		t.Error("expected error for unknown target")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error = %q, expected to mention 'unknown'", err.Error())
	}
}

func TestApplyResolveTargetNoConfig(t *testing.T) {
	ac := &ApplyConfig{}

	_, err := ac.ResolveTarget("")
	if err == nil {
		t.Fatal("expected error for empty config")
	}
	if !strings.Contains(err.Error(), "no apply command configured") {
		t.Errorf("error = %q, expected to mention 'no apply command configured'", err.Error())
	}
}

func TestApplyResolveTargetSimpleNonDefault(t *testing.T) {
	ac := &ApplyConfig{
		Command: "echo hello",
	}

	_, err := ac.ResolveTarget("destroy")
	if err == nil {
		t.Fatal("expected error for non-default target with simple config")
	}
}
