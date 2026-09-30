package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

// mockConfigPlugin is a minimal config.Plugin used by doctor tests. Each
// field can be overridden per-test so we can exercise error branches.
type mockConfigPlugin struct {
	values       map[string]config.Value
	listErr      error
	getErr       error
	validateFunc func(path string) []config.ValidationResult
}

func newMockConfigPlugin(vals map[string]config.Value) *mockConfigPlugin {
	if vals == nil {
		vals = make(map[string]config.Value)
	}
	return &mockConfigPlugin{values: vals}
}

func (m *mockConfigPlugin) List(context.Context) ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	paths := make([]string, 0, len(m.values))
	for p := range m.values {
		paths = append(paths, p)
	}
	return paths, nil
}

func (m *mockConfigPlugin) Get(_ context.Context, path string) (config.Value, bool, error) {
	if m.getErr != nil {
		return config.Value{}, false, m.getErr
	}
	v, ok := m.values[path]
	return v, ok, nil
}

func (m *mockConfigPlugin) Set(_ context.Context, path string, v config.Value) error {
	m.values[path] = v
	return nil
}

func (m *mockConfigPlugin) Validate(_ context.Context, path string, _ config.TreeReader) ([]config.ValidationResult, error) {
	if m.validateFunc != nil {
		return m.validateFunc(path), nil
	}
	return nil, nil
}

// newTestEngine builds a core.Engine backed by mc under the provider name
// "mock". A zhi.yaml is NOT written to disk — most tests don't need one;
// callers that do write one pass the dir back in via ws.Dir.
func newTestEngine(t *testing.T, mc *mockConfigPlugin, comps []core.ComponentDef, dir string) *core.Engine {
	t.Helper()

	reg := core.NewRegistry()
	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatalf("RegisterConfig: %v", err)
	}

	if dir == "" {
		dir = t.TempDir()
	}
	ws := &core.WorkspaceConfig{
		Version:    "1",
		Config:     core.ProviderRef{Provider: "mock"},
		Components: comps,
		Dir:        dir,
	}
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(eng.Close)
	return eng
}

func findResult(rs []Result, status Status) (Result, bool) {
	for _, r := range rs {
		if r.Status == status {
			return r, true
		}
	}
	return Result{}, false
}

func TestConfigValidatesCheck(t *testing.T) {
	tests := []struct {
		name       string
		engineNil  bool
		validateFn func(path string) []config.ValidationResult
		wantStatus Status
		wantMsgSub string
	}{
		{
			name:       "engine nil skips",
			engineNil:  true,
			wantStatus: StatusSkipped,
			wantMsgSub: "engine not initialized",
		},
		{
			name: "blocking finding yields error",
			validateFn: func(path string) []config.ValidationResult {
				if path == "database/host" {
					return []config.ValidationResult{
						{Severity: config.Blocking, Message: "host required"},
					}
				}
				return nil
			},
			wantStatus: StatusError,
			wantMsgSub: "1 blocking",
		},
		{
			name: "warning only yields warning",
			validateFn: func(path string) []config.ValidationResult {
				if path == "app/name" {
					return []config.ValidationResult{
						{Severity: config.Warning, Message: "too short"},
					}
				}
				return nil
			},
			wantStatus: StatusWarning,
			wantMsgSub: "1 warning",
		},
		{
			name:       "clean tree yields ok",
			validateFn: nil,
			wantStatus: StatusOK,
			wantMsgSub: "no validation findings",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := &Environment{}
			if !tc.engineNil {
				mc := newMockConfigPlugin(map[string]config.Value{
					"database/host": {Val: "localhost"},
					"app/name":      {Val: "myapp"},
				})
				mc.validateFunc = tc.validateFn
				env.Engine = newTestEngine(t, mc, nil, "")
				env.Workspace = env.Engine.Workspace()
			}

			results := configValidatesCheck{}.Run(context.Background(), env)
			if len(results) == 0 {
				t.Fatalf("expected at least one result")
			}
			r, ok := findResult(results, tc.wantStatus)
			if !ok {
				t.Fatalf("no result with status %s: %+v", tc.wantStatus, results)
			}
			if !strings.Contains(r.Message, tc.wantMsgSub) {
				t.Errorf("message %q does not contain %q", r.Message, tc.wantMsgSub)
			}
		})
	}
}

func TestConfigValidatesCheckLoadError(t *testing.T) {
	mc := newMockConfigPlugin(map[string]config.Value{"a": {Val: "x"}})
	mc.listErr = os.ErrPermission
	env := &Environment{Engine: newTestEngine(t, mc, nil, "")}

	results := configValidatesCheck{}.Run(context.Background(), env)
	if len(results) != 1 || results[0].Status != StatusError {
		t.Fatalf("expected single error result, got %+v", results)
	}
	if !strings.Contains(results[0].Message, "could not load config tree") {
		t.Errorf("unexpected message: %q", results[0].Message)
	}
}

func TestConfigNoDeprecatedCheck(t *testing.T) {
	tests := []struct {
		name       string
		values     map[string]config.Value
		engineNil  bool
		wantStatus Status
		wantName   string
		wantHint   string
	}{
		{
			name:       "engine nil skips",
			engineNil:  true,
			wantStatus: StatusSkipped,
		},
		{
			name: "deprecated field flagged",
			values: map[string]config.Value{
				"legacy/flag": {
					Val: true,
					Metadata: map[string]any{
						labels.LabelCoreDeprecated: "use X instead",
					},
				},
				"app/name": {Val: "myapp"},
			},
			wantStatus: StatusWarning,
			wantName:   "legacy/flag",
			wantHint:   "use X instead",
		},
		{
			name: "empty deprecated metadata ignored",
			values: map[string]config.Value{
				"app/name": {
					Val:      "myapp",
					Metadata: map[string]any{labels.LabelCoreDeprecated: ""},
				},
			},
			wantStatus: StatusOK,
		},
		{
			name: "clean tree yields ok",
			values: map[string]config.Value{
				"app/name": {Val: "myapp"},
			},
			wantStatus: StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := &Environment{}
			if !tc.engineNil {
				mc := newMockConfigPlugin(tc.values)
				env.Engine = newTestEngine(t, mc, nil, "")
			}

			results := configNoDeprecatedCheck{}.Run(context.Background(), env)
			if len(results) == 0 {
				t.Fatalf("expected at least one result")
			}
			r, ok := findResult(results, tc.wantStatus)
			if !ok {
				t.Fatalf("no result with status %s: %+v", tc.wantStatus, results)
			}
			if tc.wantName != "" && r.Name != tc.wantName {
				t.Errorf("name = %q, want %q", r.Name, tc.wantName)
			}
			if tc.wantHint != "" && r.Hint != tc.wantHint {
				t.Errorf("hint = %q, want %q", r.Hint, tc.wantHint)
			}
		})
	}
}

func TestConfigEnvVarCheck(t *testing.T) {
	t.Run("workspace nil skipped", func(t *testing.T) {
		results := configEnvVarCheck{}.Run(context.Background(), &Environment{})
		if len(results) != 1 || results[0].Status != StatusSkipped {
			t.Fatalf("expected single skipped result, got %+v", results)
		}
	})

	t.Run("literal env var yields warning", func(t *testing.T) {
		dir := t.TempDir()
		yaml := "version: \"1\"\nconfig:\n  provider: mock\n  options:\n    token: \"${FOO}\"\n"
		if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatalf("write zhi.yaml: %v", err)
		}

		mc := newMockConfigPlugin(map[string]config.Value{"app/name": {Val: "myapp"}})
		env := &Environment{
			Engine:    newTestEngine(t, mc, nil, dir),
			Workspace: &core.WorkspaceConfig{Dir: dir},
		}

		results := configEnvVarCheck{}.Run(context.Background(), env)
		if len(results) != 1 || results[0].Status != StatusWarning {
			t.Fatalf("expected single warning, got %+v", results)
		}
		if !strings.Contains(results[0].Message, "literal ${VAR}") {
			t.Errorf("message %q missing summary", results[0].Message)
		}
		if !strings.Contains(results[0].Detail, "${FOO}") {
			t.Errorf("detail %q missing placeholder", results[0].Detail)
		}
		if !strings.Contains(results[0].Hint, "not implemented") {
			t.Errorf("hint missing feature-request pointer: %q", results[0].Hint)
		}
	})

	t.Run("clean workspace yields ok", func(t *testing.T) {
		dir := t.TempDir()
		yaml := "version: \"1\"\nconfig:\n  provider: mock\n"
		if err := os.WriteFile(filepath.Join(dir, "zhi.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatalf("write zhi.yaml: %v", err)
		}
		mc := newMockConfigPlugin(map[string]config.Value{"app/name": {Val: "myapp"}})
		env := &Environment{
			Engine:    newTestEngine(t, mc, nil, dir),
			Workspace: &core.WorkspaceConfig{Dir: dir},
		}

		results := configEnvVarCheck{}.Run(context.Background(), env)
		if len(results) != 1 || results[0].Status != StatusOK {
			t.Fatalf("expected single OK result, got %+v", results)
		}
	})

	t.Run("env var in tree value flagged", func(t *testing.T) {
		dir := t.TempDir()
		// No zhi.yaml — only the tree value carries the placeholder.
		mc := newMockConfigPlugin(map[string]config.Value{
			"secrets/api_key": {Val: "${SECRET_TOKEN}"},
		})
		env := &Environment{
			Engine:    newTestEngine(t, mc, nil, dir),
			Workspace: &core.WorkspaceConfig{Dir: dir},
		}

		results := configEnvVarCheck{}.Run(context.Background(), env)
		if len(results) != 1 || results[0].Status != StatusWarning {
			t.Fatalf("expected warning, got %+v", results)
		}
		if !strings.Contains(results[0].Detail, "secrets/api_key") {
			t.Errorf("detail %q missing tree path", results[0].Detail)
		}
	})
}

func TestConfigNoConflictsCheck(t *testing.T) {
	t.Run("engine nil skips", func(t *testing.T) {
		results := configNoConflictsCheck{}.Run(context.Background(), &Environment{})
		if len(results) != 1 || results[0].Status != StatusSkipped {
			t.Fatalf("expected single skipped result, got %+v", results)
		}
	})

	t.Run("consistent components yields ok", func(t *testing.T) {
		mc := newMockConfigPlugin(map[string]config.Value{"app/name": {Val: "myapp"}})
		comps := []core.ComponentDef{
			{Name: "database", Paths: []string{"database/"}, Mandatory: true},
			{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}, Mandatory: true},
		}
		env := &Environment{Engine: newTestEngine(t, mc, comps, "")}

		results := configNoConflictsCheck{}.Run(context.Background(), env)
		if len(results) != 1 || results[0].Status != StatusOK {
			t.Fatalf("expected single OK result, got %+v", results)
		}
	})

	t.Run("dependency violation yields error", func(t *testing.T) {
		mc := newMockConfigPlugin(map[string]config.Value{"app/name": {Val: "myapp"}})
		// redis depends on database; enable redis but disable database to
		// trigger a violation via ValidateDependencies.
		comps := []core.ComponentDef{
			{Name: "database", Paths: []string{"database/"}},
			{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}, Mandatory: true},
		}
		env := &Environment{Engine: newTestEngine(t, mc, comps, "")}
		// Disable database manually by toggling state via Enable/Disable.
		// Mandatory redis is already enabled; database starts disabled, so
		// ValidateDependencies should already report the violation.

		results := configNoConflictsCheck{}.Run(context.Background(), env)
		if len(results) == 0 {
			t.Fatalf("expected at least one error result")
		}
		r := results[0]
		if r.Status != StatusError {
			t.Fatalf("expected error status, got %s: %+v", r.Status, results)
		}
		if r.Name != "components" {
			t.Errorf("name = %q, want %q", r.Name, "components")
		}
		if !strings.Contains(r.Message, "redis") {
			t.Errorf("message %q missing component name", r.Message)
		}
	})
}
