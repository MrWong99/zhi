package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

func newDriftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "drift",
		Short:         "Detect configuration drift",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runDrift,
	}
	cmd.Flags().BoolVar(&driftJSON, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&driftWatch, "watch", false, "run continuously")
	cmd.Flags().StringVar(&driftInterval, "interval", "1m", "check interval")
	cmd.Flags().StringVar(&driftOnDrift, "on-drift", "", "shell command on drift")
	return cmd
}

func resetDriftFlags() {
	driftJSON = false
	driftWatch = false
	driftInterval = "1m"
	driftOnDrift = ""
}

// setupDriftEngine creates a test engine with export templates pointing to the
// given temp dir. Returns the engine and a function to write template files.
func setupDriftEngine(t *testing.T, dir string, templates []core.ExportTemplate) *core.Engine {
	t.Helper()

	eng := setupTestEngine(t, nil)
	eng.SetTestWorkspaceDir(dir)

	ws := eng.Workspace()
	ws.Dir = dir
	ws.Export = core.ExportConfig{Templates: templates}

	return eng
}

func TestDriftNoDrift(t *testing.T) {
	resetDriftFlags()
	dir := t.TempDir()

	// Create template.
	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(`name={{ .Get "app/name" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create matching output.
	outPath := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(outPath, []byte("name=myapp"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := setupDriftEngine(t, dir, []core.ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "out.txt"},
	})
	reg := core.NewRegistry()

	cmd := newDriftCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("drift with no drift should succeed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(output, "IN SYNC") {
		t.Errorf("expected IN SYNC in output: %s", output)
	}
}

func TestDriftDetected(t *testing.T) {
	resetDriftFlags()
	dir := t.TempDir()

	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(`name={{ .Get "app/name" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write different content → drift.
	outPath := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(outPath, []byte("name=oldapp"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := setupDriftEngine(t, dir, []core.ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "out.txt"},
	})
	reg := core.NewRegistry()

	cmd := newDriftCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd)
	if err == nil {
		t.Fatal("drift detected should return error for exit code 1")
	}
	if ExitCode(err) != 1 {
		t.Errorf("expected exit code 1, got %d (err: %v)", ExitCode(err), err)
	}
	if !strings.Contains(output, "DRIFTED") {
		t.Errorf("expected DRIFTED in output: %s", output)
	}
	if !strings.Contains(output, "zhi export") {
		t.Errorf("expected remediation hint in output: %s", output)
	}
}

func TestDriftJSON(t *testing.T) {
	resetDriftFlags()
	dir := t.TempDir()

	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(`name={{ .Get "app/name" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(outPath, []byte("name=myapp"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := setupDriftEngine(t, dir, []core.ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "out.txt"},
	})
	reg := core.NewRegistry()

	cmd := newDriftCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--json")
	if err != nil {
		t.Fatalf("drift --json: %v\nOutput: %s", err, output)
	}

	var result driftJSONOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}
	if len(result.InSync) != 1 {
		t.Errorf("expected 1 in_sync entry, got %d", len(result.InSync))
	}
	if len(result.Drifted) != 0 {
		t.Errorf("expected 0 drifted entries, got %d", len(result.Drifted))
	}
}

func TestDriftJSONWithDrift(t *testing.T) {
	resetDriftFlags()
	dir := t.TempDir()

	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(`name={{ .Get "app/name" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(outPath, []byte("name=changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := setupDriftEngine(t, dir, []core.ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "out.txt"},
	})
	reg := core.NewRegistry()

	cmd := newDriftCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd, "--json")
	if err == nil {
		t.Fatal("drift --json with drift should return error")
	}

	var result driftJSONOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}
	if len(result.Drifted) != 1 {
		t.Errorf("expected 1 drifted entry, got %d", len(result.Drifted))
	}
	if result.Drifted[0].Diff == "" {
		t.Error("expected diff in JSON output")
	}
}

func TestDriftNoTemplates(t *testing.T) {
	resetDriftFlags()
	dir := t.TempDir()

	eng := setupDriftEngine(t, dir, nil)
	reg := core.NewRegistry()

	cmd := newDriftCmd()
	setEngineContext(cmd, eng, reg)

	output, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("drift with no templates should succeed: %v", err)
	}
	if !strings.Contains(output, "no export templates") {
		t.Errorf("expected warning about no templates: %s", output)
	}
}

func TestDriftOnDriftRequiresWatch(t *testing.T) {
	resetDriftFlags()
	eng := setupTestEngine(t, nil)
	reg := core.NewRegistry()

	cmd := newDriftCmd()
	setEngineContext(cmd, eng, reg)

	_, err := executeCommand(cmd, "--on-drift", "echo hi")
	if err == nil {
		t.Fatal("--on-drift without --watch should fail")
	}
	if !strings.Contains(err.Error(), "--on-drift requires --watch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDriftWatchCancellation(t *testing.T) {
	resetDriftFlags()
	dir := t.TempDir()

	tmplPath := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(`name={{ .Get "app/name" }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(outPath, []byte("name=myapp"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := setupDriftEngine(t, dir, []core.ExportTemplate{
		{Name: "test", Template: "test.tmpl", Output: "out.txt"},
	})
	reg := core.NewRegistry()

	cmd := newDriftCmd()
	// Use a context that we'll cancel quickly.
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, engineKey, eng)
	ctx = context.WithValue(ctx, registryKey, reg)
	cmd.SetContext(ctx)

	done := make(chan error, 1)
	go func() {
		_, err := executeCommand(cmd, "--watch", "--interval", "100ms")
		done <- err
	}()

	// Let it run one cycle then cancel.
	time.Sleep(250 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("watch mode should exit cleanly on cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch mode did not exit after context cancellation")
	}
}
