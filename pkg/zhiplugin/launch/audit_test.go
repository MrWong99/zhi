package launch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBinaryPath(t *testing.T) {
	t.Parallel()

	t.Run("valid absolute path", func(t *testing.T) {
		t.Parallel()
		// Use the test binary itself as a valid path.
		exe, err := os.Executable()
		if err != nil {
			t.Skip("cannot determine executable path")
		}

		resolved, err := validateBinaryPath(exe)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved == "" {
			t.Fatal("expected non-empty resolved path")
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		t.Parallel()
		_, err := validateBinaryPath("/nonexistent/path/to/binary")
		if err == nil {
			t.Fatal("expected error for nonexistent path")
		}
	})

	t.Run("symlink resolves correctly", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}

		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}

		resolved, err := validateBinaryPath(link)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Resolve target through EvalSymlinks too, because on macOS the
		// temp-dir path itself contains a symlink (/var -> /private/var).
		wantTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatal(err)
		}
		if resolved != wantTarget {
			t.Fatalf("expected resolved=%q, got %q", wantTarget, resolved)
		}
	})
}

func TestPluginNameFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"/home/user/.zhi/plugins/zhi-config-ansible", "ansible"},
		{"/home/user/.zhi/plugins/zhi-store-vault", "vault"},
		{"/home/user/.zhi/plugins/zhi-transform-env", "env"},
		{"/home/user/.zhi/plugins/zhi-ui-tui", "tui"},
		{"/home/user/.zhi/plugins/other-binary", ""},
		{"zhi-config-test", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := PluginNameFromPath(tt.path)
			if got != tt.want {
				t.Errorf("PluginNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
