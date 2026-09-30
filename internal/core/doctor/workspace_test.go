package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
)

func newEnv(t *testing.T, ws *core.WorkspaceConfig) *Environment {
	t.Helper()
	return &Environment{
		Workspace: ws,
		Registry:  core.NewRegistry(),
	}
}

// resultOrFail returns the single Result in results or fails the test.
func resultOrFail(t *testing.T, results []Result) Result {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("expected exactly one result, got %d: %+v", len(results), results)
	}
	return results[0]
}

func TestWorkspaceExists(t *testing.T) {
	ctx := context.Background()
	check := workspaceExistsCheck{}

	t.Run("ok when workspace loaded", func(t *testing.T) {
		env := newEnv(t, &core.WorkspaceConfig{Dir: "/path/to/ws"})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q", res.Status, StatusOK)
		}
		if !strings.Contains(res.Message, "/path/to/ws") {
			t.Errorf("message missing workspace dir: %q", res.Message)
		}
	})

	t.Run("error when workspace err set", func(t *testing.T) {
		env := &Environment{WorkspaceErr: errors.New("no zhi.yaml found")}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusError {
			t.Fatalf("status = %q, want %q", res.Status, StatusError)
		}
		if res.Detail != "no zhi.yaml found" {
			t.Errorf("detail = %q, want the underlying error", res.Detail)
		}
		if res.Hint == "" {
			t.Error("expected a hint")
		}
	})
}

func TestWorkspaceParses(t *testing.T) {
	ctx := context.Background()
	check := workspaceParsesCheck{}

	t.Run("ok when workspace loaded", func(t *testing.T) {
		env := newEnv(t, &core.WorkspaceConfig{Version: "1"})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q", res.Status, StatusOK)
		}
	})

	t.Run("error when parse error present", func(t *testing.T) {
		env := &Environment{WorkspaceErr: errors.New("parsing zhi.yaml: yaml: line 3: did not find expected key")}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusError {
			t.Fatalf("status = %q, want %q", res.Status, StatusError)
		}
		if !strings.Contains(res.Detail, "parsing") {
			t.Errorf("detail = %q, expected to contain parse error", res.Detail)
		}
	})

	t.Run("skipped when non-parse error", func(t *testing.T) {
		env := &Environment{WorkspaceErr: errors.New("no zhi.yaml or zhi.json found in /tmp")}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusSkipped {
			t.Fatalf("status = %q, want %q", res.Status, StatusSkipped)
		}
	})
}

func TestWorkspaceValidates(t *testing.T) {
	ctx := context.Background()
	check := workspaceValidatesCheck{}

	t.Run("skipped when workspace nil", func(t *testing.T) {
		env := &Environment{}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusSkipped {
			t.Fatalf("status = %q, want %q", res.Status, StatusSkipped)
		}
	})

	t.Run("error when validation fails", func(t *testing.T) {
		// Version "" and empty config provider → ValidateWorkspace
		// returns "unsupported workspace version" + "config provider is required".
		env := newEnv(t, &core.WorkspaceConfig{})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusError {
			t.Fatalf("status = %q, want %q", res.Status, StatusError)
		}
		if !strings.Contains(res.Detail, "unsupported workspace version") {
			t.Errorf("detail missing version error: %q", res.Detail)
		}
		if res.Hint == "" {
			t.Error("expected a hint")
		}
	})

	t.Run("ok when validation passes", func(t *testing.T) {
		reg := core.DefaultRegistry()
		env := &Environment{
			Workspace: &core.WorkspaceConfig{
				Version: "1",
				Config:  core.ProviderRef{Provider: "structuredfile"},
			},
			Registry: reg,
		}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q; detail=%q", res.Status, StatusOK, res.Detail)
		}
	})
}

func TestWorkspacePluginDirsAccessible(t *testing.T) {
	ctx := context.Background()
	check := workspacePluginDirsCheck{}

	t.Run("skipped when workspace nil", func(t *testing.T) {
		env := &Environment{}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusSkipped {
			t.Fatalf("status = %q, want %q", res.Status, StatusSkipped)
		}
	})

	t.Run("existing dir reports ok", func(t *testing.T) {
		existing := t.TempDir()
		env := newEnv(t, &core.WorkspaceConfig{
			Plugins: core.PluginsConfig{Directories: []string{existing}},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q; message=%q", res.Status, StatusOK, res.Message)
		}
		if res.Message != existing {
			t.Errorf("message = %q, want %q", res.Message, existing)
		}
	})

	t.Run("missing dir reports warning", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		env := newEnv(t, &core.WorkspaceConfig{
			Plugins: core.PluginsConfig{Directories: []string{missing}},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusWarning {
			t.Fatalf("status = %q, want %q", res.Status, StatusWarning)
		}
		if res.Hint == "" {
			t.Error("expected a hint for missing dir")
		}
	})

	t.Run("non-dir path reports error", func(t *testing.T) {
		tmp := t.TempDir()
		filePath := filepath.Join(tmp, "not-a-dir")
		if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
			t.Fatalf("writing file: %v", err)
		}
		env := newEnv(t, &core.WorkspaceConfig{
			Plugins: core.PluginsConfig{Directories: []string{filePath}},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusError {
			t.Fatalf("status = %q, want %q", res.Status, StatusError)
		}
	})

	t.Run("mixed dirs produce one result per directory", func(t *testing.T) {
		existing := t.TempDir()
		missing := filepath.Join(t.TempDir(), "gone")
		env := newEnv(t, &core.WorkspaceConfig{
			Plugins: core.PluginsConfig{Directories: []string{existing, missing}},
		})
		results := check.Run(ctx, env)
		if len(results) != 2 {
			t.Fatalf("results = %d, want 2", len(results))
		}
		if results[0].Status != StatusOK {
			t.Errorf("first result status = %q, want %q", results[0].Status, StatusOK)
		}
		if results[1].Status != StatusWarning {
			t.Errorf("second result status = %q, want %q", results[1].Status, StatusWarning)
		}
	})

	t.Run("empty list yields single ok", func(t *testing.T) {
		// Force PluginDirectories() to return an empty slice by making
		// DefaultPluginDir() empty. On Linux an empty HOME and empty
		// XDG variables cause UserHomeDir to error.
		t.Setenv("HOME", "")
		env := newEnv(t, &core.WorkspaceConfig{
			Plugins: core.PluginsConfig{Directories: nil},
		})
		if len(env.Workspace.PluginDirectories()) != 0 {
			t.Skip("DefaultPluginDir resolved a non-empty fallback; cannot reach empty-list branch on this host")
		}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q", res.Status, StatusOK)
		}
		if !strings.Contains(res.Message, "no plugin directories") {
			t.Errorf("message = %q, want mention of empty list", res.Message)
		}
	})

	t.Run("tilde is expanded", func(t *testing.T) {
		home := t.TempDir()
		sub := "my-plugins"
		if err := os.MkdirAll(filepath.Join(home, sub), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv("HOME", home)
		env := newEnv(t, &core.WorkspaceConfig{
			Plugins: core.PluginsConfig{Directories: []string{"~/" + sub}},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q; message=%q", res.Status, StatusOK, res.Message)
		}
	})
}

func TestWorkspaceFileStorePath(t *testing.T) {
	ctx := context.Background()
	check := workspaceFileStoreCheck{}

	t.Run("skipped when workspace nil", func(t *testing.T) {
		env := &Environment{}
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusSkipped {
			t.Fatalf("status = %q, want %q", res.Status, StatusSkipped)
		}
	})

	t.Run("skipped for vault provider", func(t *testing.T) {
		env := newEnv(t, &core.WorkspaceConfig{
			Dir:   t.TempDir(),
			Store: core.ProviderRef{Provider: "vault"},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusSkipped {
			t.Fatalf("status = %q, want %q", res.Status, StatusSkipped)
		}
	})

	t.Run("skipped when store provider empty", func(t *testing.T) {
		env := newEnv(t, &core.WorkspaceConfig{Dir: t.TempDir()})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusSkipped {
			t.Fatalf("status = %q, want %q", res.Status, StatusSkipped)
		}
	})

	t.Run("jsonfile default dir present reports ok", func(t *testing.T) {
		wsDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(wsDir, core.DefaultStoreDir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		env := newEnv(t, &core.WorkspaceConfig{
			Dir:   wsDir,
			Store: core.ProviderRef{Provider: "jsonfile"},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q; detail=%q", res.Status, StatusOK, res.Detail)
		}
	})

	t.Run("jsonfile missing dir reports warning", func(t *testing.T) {
		wsDir := t.TempDir()
		env := newEnv(t, &core.WorkspaceConfig{
			Dir:   wsDir,
			Store: core.ProviderRef{Provider: "jsonfile"},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusWarning {
			t.Fatalf("status = %q, want %q", res.Status, StatusWarning)
		}
		if res.Hint == "" {
			t.Error("expected a hint mentioning on-demand creation")
		}
	})

	t.Run("jsonfile with directory option override", func(t *testing.T) {
		wsDir := t.TempDir()
		custom := "custom-store"
		if err := os.MkdirAll(filepath.Join(wsDir, custom), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		env := newEnv(t, &core.WorkspaceConfig{
			Dir: wsDir,
			Store: core.ProviderRef{
				Provider: "jsonfile",
				Options:  map[string]any{"directory": custom},
			},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusOK {
			t.Fatalf("status = %q, want %q; message=%q", res.Status, StatusOK, res.Message)
		}
		if !strings.Contains(res.Message, custom) {
			t.Errorf("message = %q, expected to contain custom dir %q", res.Message, custom)
		}
	})

	t.Run("jsonfile path is a file not a dir reports error", func(t *testing.T) {
		wsDir := t.TempDir()
		// Create the default store path as a regular file.
		storePath := filepath.Join(wsDir, core.DefaultStoreDir)
		if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(storePath, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		env := newEnv(t, &core.WorkspaceConfig{
			Dir:   wsDir,
			Store: core.ProviderRef{Provider: "jsonfile"},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusError {
			t.Fatalf("status = %q, want %q", res.Status, StatusError)
		}
	})

	t.Run("structuredfile default dir missing reports warning", func(t *testing.T) {
		wsDir := t.TempDir()
		env := newEnv(t, &core.WorkspaceConfig{
			Dir:   wsDir,
			Store: core.ProviderRef{Provider: "structuredfile"},
		})
		res := resultOrFail(t, check.Run(ctx, env))
		if res.Status != StatusWarning {
			t.Fatalf("status = %q, want %q", res.Status, StatusWarning)
		}
	})
}
