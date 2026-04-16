package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/core/doctor"
)

// writeMinimalWorkspace writes a zhi.yaml that declares the built-in
// structuredfile config provider and nothing else, which is enough for
// workspace checks to pass.
func writeMinimalWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := []byte(`version: "1"
config:
  provider: structuredfile
  options:
    directory: structuredfile
`)
	if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), cfg, 0o600); err != nil {
		t.Fatalf("write zhi.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "structuredfile"), 0o700); err != nil {
		t.Fatalf("mkdir structuredfile: %v", err)
	}
	return dir
}

func TestDoctorCmd_TextOutput(t *testing.T) {
	dir := writeMinimalWorkspace(t)

	var buf bytes.Buffer
	resetDoctorFlags(t)
	workspace = dir
	doctorNoColor = true
	doctorChecks = []string{"workspace"}

	doctorCmd.SetOut(&buf)
	doctorCmd.SetContext(context.Background())
	if err := runDoctor(doctorCmd, nil); err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Workspace") {
		t.Errorf("output missing Workspace header: %q", out)
	}
	if !strings.Contains(out, "[OK]") {
		t.Errorf("output missing OK marker: %q", out)
	}
	if !strings.Contains(out, "Summary:") {
		t.Errorf("output missing summary: %q", out)
	}
}

func TestDoctorCmd_JSONOutput(t *testing.T) {
	dir := writeMinimalWorkspace(t)

	var buf bytes.Buffer
	resetDoctorFlags(t)
	workspace = dir
	doctorJSON = true
	doctorChecks = []string{"workspace"}

	doctorCmd.SetOut(&buf)
	doctorCmd.SetContext(context.Background())
	if err := runDoctor(doctorCmd, nil); err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	var payload struct {
		Groups  []doctor.CategoryGroup `json:"groups"`
		Summary struct {
			OK       int `json:"ok"`
			Errors   int `json:"errors"`
			Total    int `json:"total"`
			ExitCode int `json:"exit_code"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	if len(payload.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(payload.Groups))
	}
	if payload.Groups[0].Category != doctor.CategoryWorkspace {
		t.Errorf("unexpected category: %s", payload.Groups[0].Category)
	}
	if payload.Summary.ExitCode != 0 {
		t.Errorf("want exit 0, got %d", payload.Summary.ExitCode)
	}
	if payload.Summary.OK == 0 {
		t.Errorf("want at least one OK, got %d", payload.Summary.OK)
	}
}

func TestDoctorCmd_ExitCodeOnError(t *testing.T) {
	// No workspace anywhere — `workspace.exists` should fail with an
	// error, which the runner surfaces via doctorExitError.
	dir := t.TempDir()

	resetDoctorFlags(t)
	workspace = dir
	doctorNoColor = true
	doctorChecks = []string{"workspace"}

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	doctorCmd.SetContext(context.Background())
	err := runDoctor(doctorCmd, nil)
	if err == nil {
		t.Fatal("expected an error because workspace.exists should fail")
	}
	// runDoctor returns a pointer receiver implementation.
	var de *doctorExitError
	if !errors.As(err, &de) {
		t.Fatalf("want *doctorExitError, got %T: %v", err, err)
	}
	if ExitCode(err) != 1 {
		t.Errorf("want exit code 1, got %d", ExitCode(err))
	}
}

func TestDoctorCmd_InvalidCategory(t *testing.T) {
	dir := writeMinimalWorkspace(t)

	resetDoctorFlags(t)
	workspace = dir
	doctorChecks = []string{"nonsense"}

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	doctorCmd.SetContext(context.Background())
	err := runDoctor(doctorCmd, nil)
	if err == nil {
		t.Fatal("expected category parse error")
	}
	if !strings.Contains(err.Error(), "nonsense") {
		t.Errorf("unexpected error: %v", err)
	}
}

// resetDoctorFlags returns the package-level doctor flags to their
// defaults between subtests so tests stay independent.
func resetDoctorFlags(t *testing.T) {
	t.Helper()
	doctorChecks = nil
	doctorJSON = false
	doctorQuiet = false
	doctorFix = false
	doctorDeep = false
	doctorTimeout = 0
	doctorNoColor = false
	doctorUpdates = false
	workspace = ""
	t.Cleanup(func() {
		doctorChecks = nil
		doctorJSON = false
		doctorQuiet = false
		doctorFix = false
		doctorDeep = false
		doctorTimeout = 0
		doctorNoColor = false
		doctorUpdates = false
		workspace = ""
	})
}
