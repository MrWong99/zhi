package cli

import (
	"bytes"
	"context"
	"fmt"
	"maps"
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
	trees map[string]map[string]config.Value
}

func newMockStore() *mockStore {
	return &mockStore{trees: make(map[string]map[string]config.Value)}
}

func (m *mockStore) Capabilities(context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning: store.VersioningNone,
		Encryption: store.EncryptionNone,
	}, nil
}

func (m *mockStore) AuthMethods(context.Context) ([]store.AuthMethod, error) {
	return nil, fmt.Errorf("auth not supported")
}
func (m *mockStore) Login(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, fmt.Errorf("auth not supported")
}

func (m *mockStore) LoginInteractive(context.Context, string, map[string]string) (*store.InteractiveChallenge, error) {
	return nil, fmt.Errorf("interactive login not supported")
}

func (m *mockStore) LoginInteractiveCallback(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, fmt.Errorf("interactive login not supported")
}

func (m *mockStore) ListTrees(context.Context) ([]string, error) {
	var ids []string
	for id := range m.trees {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (m *mockStore) DeleteTree(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}

func (m *mockStore) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
	vals, ok := m.trees[id]
	if !ok {
		return nil, nil
	}
	result := make(map[string]config.Value)
	for _, p := range paths {
		if v, exists := vals[p]; exists {
			result[p] = v
		}
	}
	return result, nil
}

func (m *mockStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	if m.trees[id] == nil {
		m.trees[id] = make(map[string]config.Value)
	}
	maps.Copy(m.trees[id], values)
	return nil
}

func (m *mockStore) DeleteValues(_ context.Context, id string, paths []string) error {
	vals := m.trees[id]
	if vals == nil {
		return nil
	}
	for _, p := range paths {
		delete(vals, p)
	}
	return nil
}

func (m *mockStore) ListTreeVersions(context.Context, string) ([]string, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *mockStore) GetTreeVersion(context.Context, string, string, []string) (map[string]config.Value, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *mockStore) RollbackTree(context.Context, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) DeleteTreeVersion(context.Context, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) ListValueVersions(context.Context, string, string) ([]string, error) {
	return nil, fmt.Errorf("versioning not supported")
}
func (m *mockStore) GetValueVersion(context.Context, string, string, string) (config.Value, bool, error) {
	return config.Value{}, false, fmt.Errorf("versioning not supported")
}
func (m *mockStore) RollbackValue(context.Context, string, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) DeleteValueVersion(context.Context, string, string, string) error {
	return fmt.Errorf("versioning not supported")
}
func (m *mockStore) InitEncryption(context.Context, []byte) error           { return nil }
func (m *mockStore) RotateEncryption(context.Context, []byte, []byte) error { return nil }
func (m *mockStore) GrantAccess(context.Context, string, string, []store.Permission) error {
	return fmt.Errorf("access control not supported")
}
func (m *mockStore) RevokeAccess(context.Context, string, string, []string) error {
	return fmt.Errorf("access control not supported")
}
func (m *mockStore) ListAccess(context.Context, string) (map[string][]store.Permission, error) {
	return nil, fmt.Errorf("access control not supported")
}

// setupTestEngine creates a test engine with mock providers and the given components.
func setupTestEngine(t *testing.T, components []core.ComponentDef) *core.Engine {
	t.Helper()

	mc := newMockConfig()
	mt := &mockTransform{}
	ms := newMockStore()

	reg := core.NewRegistry()
	_ = reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	})
	_ = reg.RegisterTransform("mock", func(string, map[string]any) (transform.Plugin, error) {
		return mt, nil
	})
	_ = reg.RegisterStore("mock", func(string, map[string]any) (store.Plugin, error) {
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
