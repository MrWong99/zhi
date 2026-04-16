package doctor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/lockfile"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// stubConfigPlugin is the smallest config.Plugin needed for registry
// tests here; the checks only ever look at the registered name.
type stubConfigPlugin struct{}

func (stubConfigPlugin) List(context.Context) ([]string, error) { return nil, nil }
func (stubConfigPlugin) Get(context.Context, string) (config.Value, bool, error) {
	return config.Value{}, false, nil
}
func (stubConfigPlugin) Set(context.Context, string, config.Value) error { return nil }
func (stubConfigPlugin) Validate(context.Context, string, config.TreeReader) ([]config.ValidationResult, error) {
	return nil, nil
}

// stubConfigFactory produces a stubConfigPlugin for any options.
func stubConfigFactory(string, map[string]any) (config.Plugin, error) {
	return stubConfigPlugin{}, nil
}

// newRegistryWith returns a fresh Registry with the given built-in config
// factories registered. The doctor checks never call the factories, but
// registration makes the names appear in ListConfig().
func newRegistryWith(t *testing.T, configNames ...string) *core.Registry {
	t.Helper()
	r := core.NewRegistry()
	for _, n := range configNames {
		if err := r.RegisterConfig(n, stubConfigFactory); err != nil {
			t.Fatalf("RegisterConfig(%q): %v", n, err)
		}
	}
	return r
}

// writeFile is a small convenience for tests that create manifest and
// lockfile fixtures in a tempdir.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func statusesByCheckID(t *testing.T, results []Result, wantID string) []Status {
	t.Helper()
	var out []Status
	for _, r := range results {
		if r.CheckID != wantID {
			t.Fatalf("unexpected CheckID: got %q want %q", r.CheckID, wantID)
		}
		out = append(out, r.Status)
	}
	return out
}

func TestPluginsReferencedInstalled(t *testing.T) {
	t.Parallel()

	check := pluginsReferencedInstalledCheck{}

	tests := []struct {
		name     string
		env      *Environment
		want     []Status
		wantMsgs []string // optional message assertions keyed by index
	}{
		{
			name: "nil workspace is skipped",
			env:  &Environment{},
			want: []Status{StatusSkipped},
		},
		{
			name: "registered config provider resolves",
			env: &Environment{
				Registry: newRegistryWith(t, "test-config"),
				Workspace: &core.WorkspaceConfig{
					Config: core.ProviderRef{Provider: "test-config"},
				},
			},
			want: []Status{StatusOK},
		},
		{
			name: "missing config provider errors",
			env: &Environment{
				Registry: newRegistryWith(t),
				Workspace: &core.WorkspaceConfig{
					Config: core.ProviderRef{Provider: "missing"},
				},
			},
			want: []Status{StatusError},
		},
		{
			name: "no providers referenced is skipped",
			env: &Environment{
				Registry:  newRegistryWith(t),
				Workspace: &core.WorkspaceConfig{},
			},
			want: []Status{StatusSkipped},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := check.Run(context.Background(), tc.env)
			gotStatuses := statusesByCheckID(t, got, check.ID())
			if len(gotStatuses) != len(tc.want) {
				t.Fatalf("result count: got %d %v want %d %v", len(gotStatuses), gotStatuses, len(tc.want), tc.want)
			}
			for i, s := range tc.want {
				if gotStatuses[i] != s {
					t.Errorf("result[%d].Status: got %q want %q", i, gotStatuses[i], s)
				}
			}
		})
	}
}

func TestPluginsManifestReadable(t *testing.T) {
	t.Parallel()

	check := pluginsManifestReadableCheck{}

	t.Run("no discovered plugins is skipped", func(t *testing.T) {
		t.Parallel()
		res := check.Run(context.Background(), &Environment{})
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
	})

	t.Run("readable manifest yields OK", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		binPath := filepath.Join(dir, "zhi-config-sample")
		writeFile(t, binPath, "#!/bin/sh\n")
		writeFile(t, filepath.Join(dir, "zhi-plugin.yaml"), `schemaVersion: "1"
name: sample
type: config
version: 1.2.3
`)
		env := &Environment{
			Discovered: []core.PluginInfo{{
				Name: "sample",
				Type: core.PluginTypeConfig,
				Path: binPath,
			}},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 {
			t.Fatalf("want 1 result, got %d", len(res))
		}
		if res[0].Status != StatusOK {
			t.Fatalf("want OK, got %+v", res[0])
		}
		if want := "sample v1.2.3"; res[0].Message != want {
			t.Errorf("Message: got %q want %q", res[0].Message, want)
		}
	})

	t.Run("missing manifest warns", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		env := &Environment{
			Discovered: []core.PluginInfo{{
				Name: "sample",
				Type: core.PluginTypeConfig,
				Path: filepath.Join(dir, "zhi-config-sample"),
			}},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 || res[0].Status != StatusWarning {
			t.Fatalf("want single warning, got %+v", res)
		}
	})

	t.Run("invalid manifest errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Name omitted → Validate() rejects the manifest.
		writeFile(t, filepath.Join(dir, "zhi-plugin.yaml"), `schemaVersion: "1"
type: config
version: 1.2.3
`)
		env := &Environment{
			Discovered: []core.PluginInfo{{
				Name: "sample",
				Type: core.PluginTypeConfig,
				Path: filepath.Join(dir, "zhi-config-sample"),
			}},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 || res[0].Status != StatusError {
			t.Fatalf("want single error, got %+v", res)
		}
	})
}

func TestPluginsVersionCompatible(t *testing.T) {
	t.Parallel()

	check := pluginsVersionCompatibleCheck{}

	t.Run("no discovered plugins is skipped", func(t *testing.T) {
		t.Parallel()
		res := check.Run(context.Background(), &Environment{})
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
	})

	type caseDef struct {
		name       string
		minVer     string
		zhiVersion string
		want       Status
	}
	cases := []caseDef{
		{name: "compatible version is OK", minVer: "1.0.0", zhiVersion: "1.7.0", want: StatusOK},
		{name: "too old zhi errors", minVer: "1.5.0", zhiVersion: "0.5.0", want: StatusError},
		{name: "dev build warns", minVer: "1.0.0", zhiVersion: "dev", want: StatusWarning},
		{name: "no minimum is OK", minVer: "", zhiVersion: "1.7.0", want: StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			body := `schemaVersion: "1"
name: sample
type: config
version: 1.0.0
`
			if tc.minVer != "" {
				body += "minZhiVersion: " + tc.minVer + "\n"
			}
			writeFile(t, filepath.Join(dir, "zhi-plugin.yaml"), body)
			env := &Environment{
				ZhiVersion: tc.zhiVersion,
				Discovered: []core.PluginInfo{{
					Name: "sample",
					Type: core.PluginTypeConfig,
					Path: filepath.Join(dir, "zhi-config-sample"),
				}},
			}
			res := check.Run(context.Background(), env)
			if len(res) != 1 {
				t.Fatalf("want 1 result, got %d: %+v", len(res), res)
			}
			if res[0].Status != tc.want {
				t.Fatalf("Status: got %q want %q (%+v)", res[0].Status, tc.want, res[0])
			}
		})
	}

	t.Run("manifest absent is silent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		env := &Environment{
			ZhiVersion: "1.0.0",
			Discovered: []core.PluginInfo{{
				Name: "sample",
				Type: core.PluginTypeConfig,
				Path: filepath.Join(dir, "zhi-config-sample"),
			}},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 0 {
			t.Fatalf("want no results when manifest missing (previous check warns), got %+v", res)
		}
	})
}

// TestPluginsAlive exercises the branches that do not require launching a
// real plugin binary: nil workspace, empty references, and the built-in
// short-circuit. Exercising a true external probe would need a prebuilt
// plugin binary, which is out of scope here.
//
// TODO: add an end-to-end alive probe using one of the bin/examples/
// plugins when they are available at test time.
func TestPluginsAlive(t *testing.T) {
	t.Parallel()

	check := pluginsAliveCheck{}

	t.Run("nil workspace is skipped", func(t *testing.T) {
		t.Parallel()
		res := check.Run(context.Background(), &Environment{Registry: core.NewRegistry()})
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
	})

	t.Run("no references is skipped", func(t *testing.T) {
		t.Parallel()
		env := &Environment{
			Registry:  core.NewRegistry(),
			Workspace: &core.WorkspaceConfig{},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
	})

	t.Run("built-in provider is OK without launch", func(t *testing.T) {
		t.Parallel()
		env := &Environment{
			Registry: newRegistryWith(t, "test-config"),
			Workspace: &core.WorkspaceConfig{
				Config: core.ProviderRef{Provider: "test-config"},
			},
			Timeout: time.Second,
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 {
			t.Fatalf("want 1 result, got %+v", res)
		}
		if res[0].Status != StatusOK {
			t.Errorf("Status: got %q want %q", res[0].Status, StatusOK)
		}
		if res[0].Name != "config/test-config" {
			t.Errorf("Name: got %q", res[0].Name)
		}
	})

	t.Run("external provider without binary errors", func(t *testing.T) {
		t.Parallel()
		env := &Environment{
			Registry: core.NewRegistry(),
			Workspace: &core.WorkspaceConfig{
				Config: core.ProviderRef{Provider: "phantom"},
			},
			Timeout: time.Second,
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 {
			t.Fatalf("want 1 result, got %+v", res)
		}
		if res[0].Status != StatusError {
			t.Errorf("Status: got %q want %q", res[0].Status, StatusError)
		}
	})

	t.Run("UI provider is skipped", func(t *testing.T) {
		t.Parallel()
		env := &Environment{
			Registry: core.NewRegistry(),
			Workspace: &core.WorkspaceConfig{
				UI: core.ProviderRef{Provider: "some-ui"},
			},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
		if res[0].Name != "ui/some-ui" {
			t.Errorf("Name: got %q", res[0].Name)
		}
	})
}

func TestPluginsSignatureVerified(t *testing.T) {
	t.Parallel()

	check := pluginsSignatureVerifiedCheck{}

	t.Run("deep disabled is skipped", func(t *testing.T) {
		t.Parallel()
		res := check.Run(context.Background(), &Environment{Deep: false})
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
		if res[0].Message != "enable with --deep" {
			t.Errorf("Message: got %q", res[0].Message)
		}
	})

	t.Run("deep with no lockfile is skipped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		env := &Environment{
			Deep:      true,
			Workspace: &core.WorkspaceConfig{Dir: dir},
			Discovered: []core.PluginInfo{{
				Name: "sample",
				Type: core.PluginTypeConfig,
				Path: filepath.Join(dir, "zhi-config-sample"),
			}},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
	})

	t.Run("deep with empty discovered is skipped", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		env := &Environment{
			Deep:      true,
			Workspace: &core.WorkspaceConfig{Dir: dir},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 || res[0].Status != StatusSkipped {
			t.Fatalf("want single skipped, got %+v", res)
		}
	})

	t.Run("lockfile without platform digest warns", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		binPath := filepath.Join(dir, "zhi-config-sample")
		writeFile(t, binPath, "not really a binary")
		// Craft a lockfile with a platform entry that intentionally does
		// not match the current runtime, forcing the "missing platform"
		// warning branch.
		otherPlatform := "nonexistent/arch"
		if otherPlatform == runtime.GOOS+"/"+runtime.GOARCH {
			otherPlatform = "xyz/xyz"
		}
		lf := &lockfile.LockFile{
			Version:    lockfile.Version,
			ZhiVersion: "1.0.0",
			Plugins: []lockfile.LockedPlugin{{
				Name:   "sample",
				Type:   string(core.PluginTypeConfig),
				Ref:    "oci://example/sample:v1",
				Digest: "sha256:deadbeef",
				Platforms: map[string]string{
					otherPlatform: "sha256:deadbeef",
				},
			}},
		}
		data, err := lf.Marshal()
		if err != nil {
			t.Fatalf("marshal lockfile: %v", err)
		}
		writeFile(t, filepath.Join(dir, "zhi-plugins.lock"), string(data))

		env := &Environment{
			Deep:      true,
			Workspace: &core.WorkspaceConfig{Dir: dir},
			Discovered: []core.PluginInfo{{
				Name: "sample",
				Type: core.PluginTypeConfig,
				Path: binPath,
			}},
		}
		res := check.Run(context.Background(), env)
		if len(res) != 1 {
			t.Fatalf("want 1 result, got %+v", res)
		}
		if res[0].Status != StatusWarning {
			t.Errorf("Status: got %q want %q", res[0].Status, StatusWarning)
		}
	})
}

func TestManifestPathFor(t *testing.T) {
	t.Parallel()
	p := core.PluginInfo{Path: "/tmp/plugins/zhi-config-foo"}
	if got, want := manifestPathFor(p), "/tmp/plugins/zhi-plugin.yaml"; got != want {
		t.Errorf("manifestPathFor: got %q want %q", got, want)
	}
}

func TestBuiltinSet(t *testing.T) {
	t.Parallel()
	infos := []core.ProviderInfo{
		{Name: "builtin-one", Source: "built-in"},
		{Name: "external-one", Source: "/path/to/bin"},
		{Name: "builtin-two", Source: "built-in"},
	}
	got := builtinSet(infos)
	if !got["builtin-one"] || !got["builtin-two"] {
		t.Errorf("expected both built-in entries, got %v", got)
	}
	if got["external-one"] {
		t.Errorf("did not expect external entry, got %v", got)
	}
}
