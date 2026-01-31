package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use: "version",
		Run: func(_ *cobra.Command, _ []string) {
			// Directly test the run function's logic.
		},
	}

	// Test via the actual versionCmd.
	buf := new(bytes.Buffer)
	versionCmd.SetOut(buf)
	versionCmd.SetArgs([]string{})

	// Reset to defaults before test.
	SetVersionInfo("1.2.3", "abc1234", "2026-01-29T00:00:00Z")

	if err := versionCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	// The version command writes to stdout (fmt.Println), not cmd.OutOrStdout(),
	// so we test VersionString() directly.
	_ = cmd // avoid unused

	vs := VersionString()
	if !strings.Contains(vs, "1.2.3") {
		t.Errorf("expected version '1.2.3' in output, got: %s", vs)
	}
	if !strings.Contains(vs, "abc1234") {
		t.Errorf("expected commit 'abc1234' in output, got: %s", vs)
	}
	if !strings.Contains(vs, "2026-01-29T00:00:00Z") {
		t.Errorf("expected date in output, got: %s", vs)
	}
}

func TestVersionStringDefaults(t *testing.T) {
	// Reset to defaults.
	appVersion = "dev"
	appCommit = "none"
	appDate = "unknown"

	vs := VersionString()
	expected := "zhi dev (commit: none, built: unknown)"
	if vs != expected {
		t.Errorf("expected %q, got %q", expected, vs)
	}
}

func TestSetVersionInfoEmpty(t *testing.T) {
	// Reset to known values.
	appVersion = "dev"
	appCommit = "none"
	appDate = "unknown"

	// Empty strings should not override.
	SetVersionInfo("", "", "")
	if appVersion != "dev" {
		t.Errorf("expected version 'dev', got %q", appVersion)
	}
	if appCommit != "none" {
		t.Errorf("expected commit 'none', got %q", appCommit)
	}
	if appDate != "unknown" {
		t.Errorf("expected date 'unknown', got %q", appDate)
	}
}
