package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrWong99/zhi/internal/core"
)

// TestApplyCommandExecution verifies that the apply system correctly executes
// a shell command and captures its output.
func TestApplyCommandExecution(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
apply:
  command: "echo hello-from-apply"
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	runCfg, err := eng.BuildApplyRunConfig("")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig: %v", err)
	}

	output := make(chan core.ApplyOutput, 100)
	ctx := context.Background()

	result, err := core.Apply(ctx, runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	// Collect output.
	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "hello-from-apply") {
		t.Errorf("output should contain 'hello-from-apply', got: %s", joined)
	}
}

// TestApplyCommandWithEnvironment verifies that the apply system passes
// environment variables (both from workspace config and overrides).
func TestApplyCommandWithEnvironment(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
apply:
  command: "echo ZHI_WORKSPACE=$ZHI_WORKSPACE && echo CUSTOM=$CUSTOM_VAR"
  env:
    CUSTOM_VAR: from-workspace
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	runCfg, err := eng.BuildApplyRunConfig("")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig: %v", err)
	}
	runCfg.WorkspaceDir = wsDir

	output := make(chan core.ApplyOutput, 100)
	ctx := context.Background()

	result, err := core.Apply(ctx, runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "ZHI_WORKSPACE="+wsDir) {
		t.Errorf("output should contain ZHI_WORKSPACE, got: %s", joined)
	}
	if !strings.Contains(joined, "CUSTOM=from-workspace") {
		t.Errorf("output should contain CUSTOM_VAR from workspace env, got: %s", joined)
	}
}

// TestApplyCommandWithEnvOverrides verifies that CLI env overrides take
// precedence over workspace env.
func TestApplyCommandWithEnvOverrides(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
apply:
  command: "echo MYVAR=$MYVAR"
  env:
    MYVAR: original
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	runCfg, err := eng.BuildApplyRunConfig("")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig: %v", err)
	}
	runCfg.WorkspaceDir = wsDir
	// Override the env var.
	runCfg.EnvOverrides = map[string]string{"MYVAR": "overridden"}

	output := make(chan core.ApplyOutput, 100)
	ctx := context.Background()

	result, err := core.Apply(ctx, runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "MYVAR=overridden") {
		t.Errorf("output should contain overridden MYVAR, got: %s", joined)
	}
}

// TestApplyNonZeroExitCode verifies that non-zero exit codes are captured
// (not treated as errors).
func TestApplyNonZeroExitCode(t *testing.T) {
	runCfg := core.ApplyRunConfig{
		Target: core.ApplyTargetConfig{
			Command: "exit 42",
		},
		WorkspaceDir: t.TempDir(),
	}

	output := make(chan core.ApplyOutput, 100)
	ctx := context.Background()

	result, err := core.Apply(ctx, runCfg, output)
	if err != nil {
		t.Fatalf("Apply should not error for non-zero exit: %v", err)
	}
	// Drain output.
	for range output {
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

// TestApplyTimeout verifies that the apply system respects context cancellation
// to implement timeouts.
func TestApplyTimeout(t *testing.T) {
	runCfg := core.ApplyRunConfig{
		Target: core.ApplyTargetConfig{
			Command: "sleep 60",
		},
		WorkspaceDir: t.TempDir(),
	}

	output := make(chan core.ApplyOutput, 100)
	// Use a context with a 2-second timeout to simulate a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := core.Apply(ctx, runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// Drain output.
	for range output {
	}

	// The command should have been killed due to context cancellation.
	if result.ExitCode == 0 && result.Err == nil {
		t.Error("expected non-zero exit code or error due to timeout")
	}
	if result.Err != nil {
		t.Logf("got expected error from timeout: %v", result.Err)
	}
}

// TestApplyEmptyCommandFails verifies that an empty command is rejected.
func TestApplyEmptyCommandFails(t *testing.T) {
	runCfg := core.ApplyRunConfig{
		Target:       core.ApplyTargetConfig{Command: ""},
		WorkspaceDir: t.TempDir(),
	}

	output := make(chan core.ApplyOutput, 100)
	ctx := context.Background()

	_, err := core.Apply(ctx, runCfg, output)
	if err == nil {
		t.Fatal("Apply should error for empty command")
	}
}

// TestApplyNamedTargets verifies that named apply targets work correctly.
func TestApplyNamedTargets(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
apply:
  targets:
    default:
      command: "echo deploy"
    destroy:
      command: "echo teardown"
    test:
      command: "echo testing"
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	// Test default target.
	runCfg, err := eng.BuildApplyRunConfig("")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig(default): %v", err)
	}
	if runCfg.Target.Command != "echo deploy" {
		t.Errorf("default target command = %q, want 'echo deploy'", runCfg.Target.Command)
	}

	// Test destroy target.
	runCfg, err = eng.BuildApplyRunConfig("destroy")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig(destroy): %v", err)
	}
	if runCfg.Target.Command != "echo teardown" {
		t.Errorf("destroy target command = %q, want 'echo teardown'", runCfg.Target.Command)
	}

	// Test unknown target.
	_, err = eng.BuildApplyRunConfig("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

// TestApplyWithPreExport verifies that the pre-export step works before
// the apply command runs: templates are rendered and written to disk.
func TestApplyWithPreExport(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
export:
  templates:
    - name: env-file
      format: dotenv
      output: ./generated/.env
apply:
  command: "cat ./generated/.env"
  pre-export: true
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: pre-export-app
  port:
    val: 9090
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	ctx := context.Background()

	// Do the pre-export manually (as the CLI would).
	td, err := core.PrepareTreeData(ctx, eng, true, "")
	if err != nil {
		t.Fatalf("PrepareTreeData: %v", err)
	}

	envFilePath := filepath.Join(wsDir, "generated", ".env")
	exportCfg := core.ExportRunConfig{
		Format:     "dotenv",
		OutputPath: envFilePath,
	}
	_, err = core.Export(ctx, td, exportCfg)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Verify the file was created.
	data, err := os.ReadFile(envFilePath)
	if err != nil {
		t.Fatalf("reading generated .env: %v", err)
	}
	if !strings.Contains(string(data), "APP_NAME=pre-export-app") {
		t.Errorf(".env should contain APP_NAME=pre-export-app, got: %s", string(data))
	}

	// Now run the apply command that reads the file.
	runCfg, err := eng.BuildApplyRunConfig("")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig: %v", err)
	}
	runCfg.WorkspaceDir = wsDir

	output := make(chan core.ApplyOutput, 100)
	result, err := core.Apply(ctx, runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "APP_NAME=pre-export-app") {
		t.Errorf("apply output should contain pre-exported content, got: %s", joined)
	}
}

// TestApplyWorkdirConfiguration verifies that the apply system uses the
// configured working directory.
func TestApplyWorkdirConfiguration(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
apply:
  command: "pwd"
  workdir: ./subdir
`,
		map[string]string{
			"app.yaml": `app:
  key:
    val: value
`,
		},
		nil,
	)

	// Create the subdir.
	subDir := filepath.Join(wsDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	runCfg, err := eng.BuildApplyRunConfig("")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig: %v", err)
	}
	runCfg.WorkspaceDir = wsDir

	output := make(chan core.ApplyOutput, 100)
	result, err := core.Apply(context.Background(), runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}
	joined := strings.Join(lines, "\n")
	// The output of pwd should be the subdir path.
	if !strings.Contains(joined, "subdir") {
		t.Errorf("apply should run in subdir, got pwd output: %s", joined)
	}
}

// TestApplyComponentEnvVars verifies that ZHI_ENABLED_COMPONENTS and
// ZHI_DISABLED_COMPONENTS are set correctly.
func TestApplyComponentEnvVars(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: database
    paths:
      - database/
    mandatory: true
  - name: cache
    paths:
      - cache/
apply:
  command: "echo ENABLED=$ZHI_ENABLED_COMPONENTS DISABLED=$ZHI_DISABLED_COMPONENTS"
`,
		map[string]string{
			"database.yaml": `database:
  host:
    val: localhost
`,
			"cache.yaml": `cache:
  host:
    val: redis
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	eng.SetTestWorkspaceDir(wsDir)

	runCfg, err := eng.BuildApplyRunConfig("")
	if err != nil {
		t.Fatalf("BuildApplyRunConfig: %v", err)
	}
	runCfg.WorkspaceDir = wsDir

	output := make(chan core.ApplyOutput, 100)
	result, err := core.Apply(context.Background(), runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	var lines []string
	for line := range output {
		lines = append(lines, line.Line)
	}
	joined := strings.Join(lines, "\n")
	// database is mandatory (enabled), cache is not (disabled).
	if !strings.Contains(joined, "ENABLED=database") {
		t.Errorf("should have database in enabled components, got: %s", joined)
	}
	if !strings.Contains(joined, "DISABLED=cache") {
		t.Errorf("should have cache in disabled components, got: %s", joined)
	}
}

// TestApplyStderrCapture verifies that stderr output from apply commands
// is captured separately.
func TestApplyStderrCapture(t *testing.T) {
	runCfg := core.ApplyRunConfig{
		Target: core.ApplyTargetConfig{
			Command: "echo stdout-line && echo stderr-line >&2",
		},
		WorkspaceDir: t.TempDir(),
	}

	output := make(chan core.ApplyOutput, 100)
	result, err := core.Apply(context.Background(), runCfg, output)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	var stdoutLines, stderrLines []string
	for line := range output {
		switch line.Stream {
		case "stdout":
			stdoutLines = append(stdoutLines, line.Line)
		case "stderr":
			stderrLines = append(stderrLines, line.Line)
		}
	}

	stdoutJoined := strings.Join(stdoutLines, "\n")
	stderrJoined := strings.Join(stderrLines, "\n")

	if !strings.Contains(stdoutJoined, "stdout-line") {
		t.Errorf("stdout should contain 'stdout-line', got: %s", stdoutJoined)
	}
	if !strings.Contains(stderrJoined, "stderr-line") {
		t.Errorf("stderr should contain 'stderr-line', got: %s", stderrJoined)
	}
}
