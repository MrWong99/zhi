package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// mockStorePlugin records which methods are called and returns configured values.
type mockStorePlugin struct {
	caps          *store.Capabilities
	authMethods   []store.AuthMethod
	credential    *store.Credential
	challenge     *store.InteractiveChallenge
	trees         []string
	values        map[string]config.Value
	versions      []string
	valVersion    config.Value
	valVersionOk  bool
	access        map[string][]store.Permission
	err           error
	calledMethods map[string]bool
}

func newMockStore() *mockStorePlugin {
	return &mockStorePlugin{
		caps:          &store.Capabilities{},
		values:        map[string]config.Value{"a": {Val: "v"}},
		calledMethods: make(map[string]bool),
	}
}

func (m *mockStorePlugin) record(name string) {
	m.calledMethods[name] = true
}

func (m *mockStorePlugin) Capabilities(_ context.Context) (*store.Capabilities, error) {
	m.record("Capabilities")
	return m.caps, m.err
}
func (m *mockStorePlugin) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	m.record("AuthMethods")
	return m.authMethods, m.err
}
func (m *mockStorePlugin) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	m.record("Login")
	return m.credential, m.err
}
func (m *mockStorePlugin) LoginInteractive(_ context.Context, _ string, _ map[string]string) (*store.InteractiveChallenge, error) {
	m.record("LoginInteractive")
	return m.challenge, m.err
}
func (m *mockStorePlugin) LoginInteractiveCallback(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	m.record("LoginInteractiveCallback")
	return m.credential, m.err
}
func (m *mockStorePlugin) ListTrees(_ context.Context) ([]string, error) {
	m.record("ListTrees")
	return m.trees, m.err
}
func (m *mockStorePlugin) DeleteTree(_ context.Context, _ string) error {
	m.record("DeleteTree")
	return m.err
}
func (m *mockStorePlugin) GetValues(_ context.Context, _ string, _ []string) (map[string]config.Value, error) {
	m.record("GetValues")
	return m.values, m.err
}
func (m *mockStorePlugin) PutValues(_ context.Context, _ string, _ map[string]config.Value, _ *store.PutOptions) error {
	m.record("PutValues")
	return m.err
}
func (m *mockStorePlugin) DeleteValues(_ context.Context, _ string, _ []string) error {
	m.record("DeleteValues")
	return m.err
}
func (m *mockStorePlugin) ListTreeVersions(_ context.Context, _ string) ([]string, error) {
	m.record("ListTreeVersions")
	return m.versions, m.err
}
func (m *mockStorePlugin) GetTreeVersion(_ context.Context, _ string, _ string, _ []string) (map[string]config.Value, error) {
	m.record("GetTreeVersion")
	return m.values, m.err
}
func (m *mockStorePlugin) RollbackTree(_ context.Context, _ string, _ string) error {
	m.record("RollbackTree")
	return m.err
}
func (m *mockStorePlugin) DeleteTreeVersion(_ context.Context, _ string, _ string) error {
	m.record("DeleteTreeVersion")
	return m.err
}
func (m *mockStorePlugin) ListValueVersions(_ context.Context, _ string, _ string) ([]string, error) {
	m.record("ListValueVersions")
	return m.versions, m.err
}
func (m *mockStorePlugin) GetValueVersion(_ context.Context, _ string, _ string, _ string) (config.Value, bool, error) {
	m.record("GetValueVersion")
	return m.valVersion, m.valVersionOk, m.err
}
func (m *mockStorePlugin) RollbackValue(_ context.Context, _ string, _ string, _ string) error {
	m.record("RollbackValue")
	return m.err
}
func (m *mockStorePlugin) DeleteValueVersion(_ context.Context, _ string, _ string, _ string) error {
	m.record("DeleteValueVersion")
	return m.err
}
func (m *mockStorePlugin) InitEncryption(_ context.Context, _ []byte) error {
	m.record("InitEncryption")
	return m.err
}
func (m *mockStorePlugin) RotateEncryption(_ context.Context, _, _ []byte) error {
	m.record("RotateEncryption")
	return m.err
}
func (m *mockStorePlugin) GrantAccess(_ context.Context, _ string, _ string, _ []store.Permission) error {
	m.record("GrantAccess")
	return m.err
}
func (m *mockStorePlugin) RevokeAccess(_ context.Context, _ string, _ string, _ []string) error {
	m.record("RevokeAccess")
	return m.err
}
func (m *mockStorePlugin) ListAccess(_ context.Context, _ string) (map[string][]store.Permission, error) {
	m.record("ListAccess")
	return m.access, m.err
}

func TestStoreDelegatingPlugin_AllMethodsDelegate(t *testing.T) {
	t.Parallel()
	mock := newMockStore()
	dp := store.NewDelegatingPlugin(mock)
	ctx := context.Background()

	// Call every method through the delegator.
	_, _ = dp.Capabilities(ctx)
	_, _ = dp.AuthMethods(ctx)
	_, _ = dp.Login(ctx, "token", nil)
	_, _ = dp.LoginInteractive(ctx, "oidc", nil)
	_, _ = dp.LoginInteractiveCallback(ctx, "ch1", nil)
	_, _ = dp.ListTrees(ctx)
	_ = dp.DeleteTree(ctx, "t1")
	_, _ = dp.GetValues(ctx, "t1", []string{"a"})
	_ = dp.PutValues(ctx, "t1", nil, nil)
	_ = dp.DeleteValues(ctx, "t1", []string{"a"})
	_, _ = dp.ListTreeVersions(ctx, "t1")
	_, _ = dp.GetTreeVersion(ctx, "t1", "v1", nil)
	_ = dp.RollbackTree(ctx, "t1", "v1")
	_ = dp.DeleteTreeVersion(ctx, "t1", "v1")
	_, _ = dp.ListValueVersions(ctx, "t1", "a")
	_, _, _ = dp.GetValueVersion(ctx, "t1", "a", "v1")
	_ = dp.RollbackValue(ctx, "t1", "a", "v1")
	_ = dp.DeleteValueVersion(ctx, "t1", "a", "v1")
	_ = dp.InitEncryption(ctx, []byte("pass"))
	_ = dp.RotateEncryption(ctx, []byte("old"), []byte("new"))
	_ = dp.GrantAccess(ctx, "t1", "user", nil)
	_ = dp.RevokeAccess(ctx, "t1", "user", nil)
	_, _ = dp.ListAccess(ctx, "t1")

	// Verify all 23 methods were called on the mock.
	expected := []string{
		"Capabilities", "AuthMethods", "Login", "LoginInteractive",
		"LoginInteractiveCallback", "ListTrees", "DeleteTree",
		"GetValues", "PutValues", "DeleteValues",
		"ListTreeVersions", "GetTreeVersion", "RollbackTree", "DeleteTreeVersion",
		"ListValueVersions", "GetValueVersion", "RollbackValue", "DeleteValueVersion",
		"InitEncryption", "RotateEncryption",
		"GrantAccess", "RevokeAccess", "ListAccess",
	}
	for _, name := range expected {
		if !mock.calledMethods[name] {
			t.Errorf("method %s was not delegated to base", name)
		}
	}
}

func TestStoreDelegatingPlugin_ErrorPropagation(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("base error")
	mock := newMockStore()
	mock.err = wantErr
	dp := store.NewDelegatingPlugin(mock)
	ctx := context.Background()

	_, err := dp.Capabilities(ctx)
	if !errors.Is(err, wantErr) {
		t.Errorf("Capabilities error = %v, want %v", err, wantErr)
	}

	err = dp.PutValues(ctx, "t1", nil, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("PutValues error = %v, want %v", err, wantErr)
	}
}

func TestStoreDelegatingPlugin_EmbedOverride(t *testing.T) {
	t.Parallel()
	mock := newMockStore()

	type customStore struct {
		store.DelegatingPlugin
	}

	cs := &customStore{DelegatingPlugin: store.NewDelegatingPlugin(mock)}

	// The embedded delegator forwards Capabilities to the mock.
	caps, err := cs.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps == nil {
		t.Fatal("Capabilities returned nil")
	}
	if !mock.calledMethods["Capabilities"] {
		t.Error("Capabilities was not delegated to base")
	}
}

func TestStoreNewDelegatingPlugin_PanicsOnNil(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil base")
		}
	}()
	store.NewDelegatingPlugin(nil)
}
