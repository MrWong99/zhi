package cli

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// mockConfig is a config.Plugin backed by an in-memory tree for testing.
type mockConfig struct {
	tree *config.Tree
}

func newMockConfig() *mockConfig {
	t := config.NewTree()
	_ = t.Set("database/host", &config.Value{Val: "localhost"})
	_ = t.Set("database/port", &config.Value{Val: 5432})
	_ = t.Set("app/name", &config.Value{Val: "myapp"})
	return &mockConfig{tree: t}
}

func (m *mockConfig) List(context.Context) ([]string, error) {
	return m.tree.List(), nil
}

func (m *mockConfig) Get(_ context.Context, path string) (config.Value, bool, error) {
	v, ok := m.tree.Get(path)
	return v, ok, nil
}

func (m *mockConfig) Set(_ context.Context, path string, v config.Value) error {
	return m.tree.Set(path, &v)
}

func (m *mockConfig) Validate(_ context.Context, path string, tree config.TreeReader) ([]config.ValidationResult, error) {
	if path == "database/host" {
		v, ok := tree.Get(path)
		if ok && v.Val == "localhost" {
			return []config.ValidationResult{
				{Severity: config.Warning, Message: "using localhost"},
			}, nil
		}
	}
	if path == "database/port" {
		v, ok := tree.Get(path)
		if ok {
			port, isInt := v.Val.(int)
			if isInt && (port < 1024 || port > 65535) {
				return []config.ValidationResult{
					{Severity: config.Blocking, Message: "port must be between 1024 and 65535"},
				}, nil
			}
		}
	}
	return nil, nil
}

// mockTransform is a transform.Plugin for testing.
type mockTransform struct{}

func (m *mockTransform) BeforeDisplay(_ context.Context, _ *config.Tree) error { return nil }
func (m *mockTransform) AfterSave(_ context.Context, _ *config.Tree) error     { return nil }
func (m *mockTransform) ValidatePolicy(context.Context) (transform.ValidatePolicy, error) {
	return transform.ValidateBeforeTransform, nil
}

// mockStore is a store.Plugin backed by an in-memory map for testing.
type mockStore struct {
	trees map[string]*config.Tree
}

func newMockStore() *mockStore {
	return &mockStore{trees: make(map[string]*config.Tree)}
}

func (m *mockStore) Save(_ context.Context, id string, tree config.TreeReader) error {
	t := config.NewTree()
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if ok {
			_ = t.Set(path, &v)
		}
	}
	m.trees[id] = t
	return nil
}

func (m *mockStore) Load(_ context.Context, id string) (*config.Tree, bool, error) {
	t, ok := m.trees[id]
	if !ok {
		return nil, false, nil
	}
	return t, true, nil
}

func (m *mockStore) Delete(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}

func (m *mockStore) ListTrees(context.Context) ([]string, error) {
	var ids []string
	for id := range m.trees {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (m *mockStore) SupportsVersioning(context.Context) (bool, error) { return false, nil }
func (m *mockStore) ListVersions(context.Context, string) ([]string, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *mockStore) LoadVersion(context.Context, string, string) (*config.Tree, bool, error) {
	return nil, false, fmt.Errorf("versioning not supported")
}
func (m *mockStore) DeleteVersion(context.Context, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) EncryptionStatus(context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionNone, nil
}
func (m *mockStore) InitEncryption(context.Context, []byte) error           { return nil }
func (m *mockStore) RotateEncryption(context.Context, []byte, []byte) error { return nil }

// setupTestEngine creates a test engine with mock providers and the given components.
func setupTestEngine(t *testing.T, components []core.ComponentDef) *core.Engine {
	t.Helper()

	mc := newMockConfig()
	mt := &mockTransform{}
	ms := newMockStore()

	reg := core.NewRegistry()
	_ = reg.RegisterConfig("mock", func(map[string]any) (config.Plugin, error) {
		return mc, nil
	})
	_ = reg.RegisterTransform("mock", func(map[string]any) (transform.Plugin, error) {
		return mt, nil
	})
	_ = reg.RegisterStore("mock", func(map[string]any) (store.Plugin, error) {
		return ms, nil
	})

	ws := &core.WorkspaceConfig{
		Version:    "1",
		Config:     core.ProviderRef{Provider: "mock"},
		Transform:  []core.ProviderRef{{Provider: "mock"}},
		Store:      core.ProviderRef{Provider: "mock"},
		Components: components,
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng
}

// executeCommand executes a cobra command with the given args and returns stdout output.
func executeCommand(cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// newTestRootCmd creates a fresh root command for testing to avoid global state issues.
func newTestRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "zhi",
		Short:         "Security-first configuration management and provisioning",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&workspace, "workspace", ".", "workspace directory")
	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	return cmd
}

// setEngineContext sets engine and registry in the command context.
func setEngineContext(cmd *cobra.Command, eng *core.Engine, reg *core.Registry) {
	ctx := context.WithValue(context.Background(), engineKey, eng)
	ctx = context.WithValue(ctx, registryKey, reg)
	cmd.SetContext(ctx)
}
