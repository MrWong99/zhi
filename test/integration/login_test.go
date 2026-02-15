package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// --- mock config provider for integration tests ---

type mockConfig struct {
	tree *config.Tree
}

func newMockConfig() *mockConfig {
	t := config.NewTree()
	_ = t.Set("database/host", &config.Value{Val: "localhost"})
	_ = t.Set("database/port", &config.Value{Val: 5432})
	return &mockConfig{tree: t}
}

func (m *mockConfig) List(_ context.Context) ([]string, error) {
	return m.tree.List(), nil
}

func (m *mockConfig) Get(_ context.Context, path string) (config.Value, bool, error) {
	v, ok := m.tree.Get(path)
	return v, ok, nil
}

func (m *mockConfig) Set(_ context.Context, path string, v config.Value) error {
	return m.tree.Set(path, &v)
}

func (m *mockConfig) Validate(_ context.Context, _ string, _ config.TreeReader) ([]config.ValidationResult, error) {
	return nil, nil
}

// --- mock no-auth store for TestIntegration_NoAuthStore ---

type noAuthStore struct {
	trees map[string]map[string]config.Value
}

func newNoAuthStore() *noAuthStore {
	return &noAuthStore{trees: make(map[string]map[string]config.Value)}
}

func (m *noAuthStore) Capabilities(context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{Auth: false}, nil
}

func (m *noAuthStore) AuthMethods(context.Context) ([]store.AuthMethod, error) { return nil, nil }
func (m *noAuthStore) Login(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, nil
}
func (m *noAuthStore) ListTrees(context.Context) ([]string, error) {
	keys := make([]string, 0, len(m.trees))
	for k := range m.trees {
		keys = append(keys, k)
	}
	return keys, nil
}
func (m *noAuthStore) DeleteTree(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}
func (m *noAuthStore) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
	vals := m.trees[id]
	if vals == nil {
		return nil, nil
	}
	result := make(map[string]config.Value)
	for _, p := range paths {
		if v, ok := vals[p]; ok {
			result[p] = v
		}
	}
	return result, nil
}
func (m *noAuthStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	if m.trees[id] == nil {
		m.trees[id] = make(map[string]config.Value)
	}
	for k, v := range values {
		m.trees[id][k] = v
	}
	return nil
}
func (m *noAuthStore) DeleteValues(_ context.Context, id string, paths []string) error {
	vals := m.trees[id]
	if vals == nil {
		return nil
	}
	for _, p := range paths {
		delete(vals, p)
	}
	return nil
}
func (m *noAuthStore) ListTreeVersions(context.Context, string) ([]string, error) { return nil, nil }
func (m *noAuthStore) GetTreeVersion(context.Context, string, string, []string) (map[string]config.Value, error) {
	return nil, nil
}
func (m *noAuthStore) RollbackTree(context.Context, string, string) error      { return nil }
func (m *noAuthStore) DeleteTreeVersion(context.Context, string, string) error { return nil }
func (m *noAuthStore) ListValueVersions(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (m *noAuthStore) GetValueVersion(context.Context, string, string, string) (config.Value, bool, error) {
	return config.Value{}, false, nil
}
func (m *noAuthStore) RollbackValue(context.Context, string, string, string) error      { return nil }
func (m *noAuthStore) DeleteValueVersion(context.Context, string, string, string) error { return nil }
func (m *noAuthStore) InitEncryption(context.Context, []byte) error                     { return nil }
func (m *noAuthStore) RotateEncryption(context.Context, []byte, []byte) error           { return nil }
func (m *noAuthStore) GrantAccess(context.Context, string, string, []store.Permission) error {
	return nil
}
func (m *noAuthStore) RevokeAccess(context.Context, string, string, []string) error { return nil }
func (m *noAuthStore) ListAccess(context.Context, string) (map[string][]store.Permission, error) {
	return nil, nil
}

// --- setup helpers ---

// setupVaultController creates an Engine and UIController backed by a
// real Vault server. It uses the DefaultRegistry (which includes the
// vault store factory) and a mock config provider.
func setupVaultController(t *testing.T, addr, token string) (*ui.UIController, *core.Engine) {
	t.Helper()

	reg := core.DefaultRegistry()

	mc := newMockConfig()
	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
		Store: core.ProviderRef{
			Provider: "vault",
			Options: map[string]any{
				"address": addr,
				"token":   token,
				"prefix":  "zhi-integration-test",
			},
		},
		Dir: t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctrl := ui.NewUIController(eng)
	return ctrl, eng
}

// setupNoAuthController creates an Engine and UIController backed by
// the in-memory no-auth store.
func setupNoAuthController(t *testing.T) (*ui.UIController, *core.Engine) {
	t.Helper()

	reg := core.NewRegistry()

	mc := newMockConfig()
	ms := newNoAuthStore()

	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterStore("mock", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
		Store:   core.ProviderRef{Provider: "mock"},
		Dir:     t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctrl := ui.NewUIController(eng)
	return ctrl, eng
}

// --- integration tests ---

func TestIntegration_VaultUserpassLogin(t *testing.T) {
	addr, rootToken := startVaultDev(t)

	// Set up userpass auth backend with a test user.
	enableUserpass(t, addr, rootToken)
	createUser(t, addr, rootToken, "testuser", "testpass")

	ctrl, _ := setupVaultController(t, addr, rootToken)
	ctx := context.Background()

	// Verify auth methods include userpass.
	methods, err := ctrl.StoreAuthMethods(ctx)
	if err != nil {
		t.Fatalf("StoreAuthMethods: %v", err)
	}
	if !hasAuthMethod(methods, "userpass") {
		t.Fatalf("expected userpass in auth methods, got %v", methodTypes(methods))
	}

	// Login with userpass.
	session, err := ctrl.StoreLogin(ctx, "userpass", map[string]string{
		"username": "testuser",
		"password": "testpass",
	})
	if err != nil {
		t.Fatalf("StoreLogin: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated, got %q", session.Status)
	}
	if session.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	// Load tree (verifies store operations work post-auth).
	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree after login: %v", err)
	}
	if len(tree.List()) == 0 {
		t.Error("expected non-empty tree after load")
	}

	// Save tree.
	if err := ctrl.SaveTree(ctx); err != nil {
		t.Fatalf("SaveTree after login: %v", err)
	}

	// Verify auth status is still authenticated.
	status, err := ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	if status.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated status, got %q", status.Status)
	}

	// Logout.
	if err := ctrl.StoreLogout(ctx); err != nil {
		t.Fatalf("StoreLogout: %v", err)
	}
	status, err = ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus after logout: %v", err)
	}
	if status.Status != zhiui.StoreSessionUnauthenticated {
		t.Fatalf("expected unauthenticated after logout, got %q", status.Status)
	}
}

func TestIntegration_VaultTokenLogin(t *testing.T) {
	addr, rootToken := startVaultDev(t)

	ctrl, _ := setupVaultController(t, addr, rootToken)
	ctx := context.Background()

	// Login with root token.
	session, err := ctrl.StoreLogin(ctx, "token", map[string]string{
		"token": rootToken,
	})
	if err != nil {
		t.Fatalf("StoreLogin: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated, got %q", session.Status)
	}

	// Verify auth methods returns all 6.
	methods, err := ctrl.StoreAuthMethods(ctx)
	if err != nil {
		t.Fatalf("StoreAuthMethods: %v", err)
	}
	expectedMethods := []string{"token", "userpass", "approle", "ldap", "kubernetes", "oidc"}
	for _, m := range expectedMethods {
		if !hasAuthMethod(methods, m) {
			t.Errorf("expected %q in auth methods, got %v", m, methodTypes(methods))
		}
	}
}

func TestIntegration_VaultAppRoleLogin(t *testing.T) {
	addr, rootToken := startVaultDev(t)

	// Set up approle auth backend.
	enableAppRole(t, addr, rootToken)
	roleID, secretID := createAppRole(t, addr, rootToken, "test-role")

	ctrl, _ := setupVaultController(t, addr, rootToken)
	ctx := context.Background()

	// Login with approle credentials.
	session, err := ctrl.StoreLogin(ctx, "approle", map[string]string{
		"role_id":   roleID,
		"secret_id": secretID,
	})
	if err != nil {
		t.Fatalf("StoreLogin: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated, got %q", session.Status)
	}

	// Load and save tree to verify operations work.
	if _, err := ctrl.LoadTree(ctx); err != nil {
		t.Fatalf("LoadTree after approle login: %v", err)
	}
	if err := ctrl.SaveTree(ctx); err != nil {
		t.Fatalf("SaveTree after approle login: %v", err)
	}
}

func TestIntegration_NoAuthStore(t *testing.T) {
	ctrl, eng := setupNoAuthController(t)
	ctx := context.Background()

	// Verify auth is not required.
	sess := eng.Session()
	required, err := sess.AuthRequired(ctx)
	if err != nil {
		t.Fatalf("AuthRequired: %v", err)
	}
	if required {
		t.Fatal("expected auth not to be required for no-auth store")
	}

	// Auth status should be "none".
	status, err := ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	if status.Status != zhiui.StoreSessionNone {
		t.Fatalf("expected none status, got %q", status.Status)
	}

	// LoadTree and SaveTree should work without login.
	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree without login: %v", err)
	}
	if len(tree.List()) == 0 {
		t.Error("expected non-empty tree")
	}

	if err := ctrl.SaveTree(ctx); err != nil {
		t.Fatalf("SaveTree without login: %v", err)
	}
}

func TestIntegration_MidSessionTokenRevocation(t *testing.T) {
	addr, rootToken := startVaultDev(t)

	enableUserpass(t, addr, rootToken)
	createUser(t, addr, rootToken, "testuser", "testpass")

	ctrl, _ := setupVaultController(t, addr, rootToken)
	ctx := context.Background()

	// Login with userpass.
	session, err := ctrl.StoreLogin(ctx, "userpass", map[string]string{
		"username": "testuser",
		"password": "testpass",
	})
	if err != nil {
		t.Fatalf("StoreLogin: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated, got %q", session.Status)
	}

	// Load tree first (needed for SaveTree).
	if _, err := ctrl.LoadTree(ctx); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	// Revoke the Vault client token externally using the root token.
	// The session.Metadata doesn't expose the vault token, but the
	// store internally uses the token from the login response.
	// We need to find the actual client token to revoke.
	// Since the Vault store sets the client token internally, we
	// revoke all non-root tokens by creating a child token and
	// revoking it. Instead, we'll use the root token to revoke
	// the userpass token via the accessor.

	// Actually, the vault store's Login sets the client token internally.
	// We can revoke it by revoking the parent (userpass token accessor).
	// For simplicity, revoke all tokens except root.
	// The cleanest approach: lookup the session token via vault API.

	// The vault store internally called s.client.setToken(resp.Auth.ClientToken)
	// after login. We can't easily extract that token. Instead, we'll
	// test the error path by directly revoking the root token that was
	// used to initialize the store -- which the store uses as fallback.
	// Actually, the store replaced its token with the userpass login token.
	// Let's just revoke all child tokens of root.
	revokeAllChildTokens(t, addr, rootToken)

	// SaveTree should now fail because the Vault token is revoked.
	err = ctrl.SaveTree(ctx)
	if err == nil {
		t.Fatal("expected error after token revocation, got nil")
	}

	// Vault returns 403 for revoked tokens, which maps to ErrAccessDenied.
	// HandleAuthError only catches ErrAuthRequired (401), not ErrAccessDenied (403).
	// This documents current behavior: the session stays Authenticated because
	// the error is ErrAccessDenied, not ErrAuthRequired.
	if store.IsAccessDenied(err) {
		t.Logf("correctly got ErrAccessDenied: %v", err)
	} else if store.IsAuthRequired(err) {
		t.Logf("got ErrAuthRequired: %v", err)
	} else {
		t.Logf("got unexpected error type: %T: %v", err, err)
	}

	// Re-login should work (with a new valid credential).
	// First we need to re-create the user since the token is revoked.
	// Actually, user still exists -- just the token was revoked.
	session, err = ctrl.StoreLogin(ctx, "userpass", map[string]string{
		"username": "testuser",
		"password": "testpass",
	})
	if err != nil {
		t.Fatalf("Re-login after revocation: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated after re-login, got %q", session.Status)
	}
}

func TestIntegration_TokenWithShortTTL(t *testing.T) {
	addr, rootToken := startVaultDev(t)

	// Create a short-lived token (2 seconds).
	shortToken := createShortTTLToken(t, addr, rootToken, "2s")

	ctrl, _ := setupVaultController(t, addr, rootToken)
	ctx := context.Background()

	// Login with the short-lived token.
	session, err := ctrl.StoreLogin(ctx, "token", map[string]string{
		"token": shortToken,
	})
	if err != nil {
		t.Fatalf("StoreLogin: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated, got %q", session.Status)
	}
	if session.ExpiresAt == "" {
		t.Fatal("expected non-empty ExpiresAt for short-TTL token")
	}

	// Wait for the token to expire.
	time.Sleep(3 * time.Second)

	// SessionManager.Status() lazily detects expiry via timestamp.
	status, err := ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	if status.Status != zhiui.StoreSessionExpired {
		t.Fatalf("expected expired status after TTL, got %q", status.Status)
	}
}

func TestIntegration_VaultLoginWrongPassword(t *testing.T) {
	addr, rootToken := startVaultDev(t)

	enableUserpass(t, addr, rootToken)
	createUser(t, addr, rootToken, "testuser", "testpass")

	ctrl, _ := setupVaultController(t, addr, rootToken)
	ctx := context.Background()

	// Attempt login with wrong password.
	_, err := ctrl.StoreLogin(ctx, "userpass", map[string]string{
		"username": "testuser",
		"password": "wrongpass",
	})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}

	// Session should remain unauthenticated.
	status, err := ctrl.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	if status.Status != zhiui.StoreSessionUnauthenticated {
		t.Fatalf("expected unauthenticated after failed login, got %q", status.Status)
	}
}

func TestIntegration_VaultMultipleAuthMethods(t *testing.T) {
	addr, rootToken := startVaultDev(t)

	enableUserpass(t, addr, rootToken)
	createUser(t, addr, rootToken, "testuser", "testpass")
	enableAppRole(t, addr, rootToken)

	ctrl, _ := setupVaultController(t, addr, rootToken)
	ctx := context.Background()

	// Verify auth methods include all expected types.
	methods, err := ctrl.StoreAuthMethods(ctx)
	if err != nil {
		t.Fatalf("StoreAuthMethods: %v", err)
	}
	for _, expected := range []string{"token", "userpass", "approle", "ldap", "kubernetes", "oidc"} {
		if !hasAuthMethod(methods, expected) {
			t.Errorf("expected %q in auth methods, got %v", expected, methodTypes(methods))
		}
	}

	// Login with userpass.
	session, err := ctrl.StoreLogin(ctx, "userpass", map[string]string{
		"username": "testuser",
		"password": "testpass",
	})
	if err != nil {
		t.Fatalf("StoreLogin userpass: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated via userpass, got %q", session.Status)
	}

	// Logout.
	if err := ctrl.StoreLogout(ctx); err != nil {
		t.Fatalf("StoreLogout: %v", err)
	}

	// Login with token.
	session, err = ctrl.StoreLogin(ctx, "token", map[string]string{
		"token": rootToken,
	})
	if err != nil {
		t.Fatalf("StoreLogin token: %v", err)
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Fatalf("expected authenticated via token, got %q", session.Status)
	}
}

// --- helpers ---

func hasAuthMethod(methods []zhiui.StoreAuthMethod, typ string) bool {
	for _, m := range methods {
		if m.Type == typ {
			return true
		}
	}
	return false
}

func methodTypes(methods []zhiui.StoreAuthMethod) []string {
	types := make([]string, len(methods))
	for i, m := range methods {
		types[i] = m.Type
	}
	return types
}

// revokeAllChildTokens revokes all tokens that are children of the root token.
func revokeAllChildTokens(t *testing.T, addr, rootToken string) {
	t.Helper()

	// List all accessors.
	resp := vaultRequest(t, addr, rootToken, "LIST", "auth/token/accessors", nil)
	data, _ := resp["data"].(map[string]any)
	keys, _ := data["keys"].([]any)

	for _, key := range keys {
		accessor, _ := key.(string)
		if accessor == "" {
			continue
		}

		// Look up each accessor to check if it's the root token.
		lookupResp := vaultRequestNoFail(t, addr, rootToken, http.MethodPost,
			"auth/token/lookup-accessor", map[string]any{"accessor": accessor})
		if lookupResp == nil {
			continue
		}
		lookupData, _ := lookupResp["data"].(map[string]any)
		policies, _ := lookupData["policies"].([]any)

		isRoot := false
		for _, p := range policies {
			if p == "root" {
				isRoot = true
				break
			}
		}

		if !isRoot {
			// Revoke non-root token by accessor.
			vaultRequestNoFail(t, addr, rootToken, http.MethodPost,
				"auth/token/revoke-accessor", map[string]any{"accessor": accessor})
		}
	}
}
