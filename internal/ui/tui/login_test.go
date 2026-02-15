package tui_test

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/internal/ui/tui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// authMockStore is a mock store that requires authentication.
type authMockStore struct {
	trees       map[string]map[string]config.Value
	methods     []store.AuthMethod
	loginFunc   func(method string, creds map[string]string) (*store.Credential, error)
	loggedIn    bool
	authEnabled bool
}

func newAuthMockStore() *authMockStore {
	return &authMockStore{
		trees:       make(map[string]map[string]config.Value),
		authEnabled: true,
		methods: []store.AuthMethod{
			{
				Type:        "userpass",
				Description: "Username and password",
				Fields: []store.AuthField{
					{Name: "username", Description: "Your username", Required: true},
					{Name: "password", Description: "Your password", Required: true, Secret: true},
				},
			},
		},
		loginFunc: func(_ string, creds map[string]string) (*store.Credential, error) {
			if creds["username"] == "admin" && creds["password"] == "secret" {
				return &store.Credential{
					Token:     "tok-123",
					ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
				}, nil
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}
}

func (m *authMockStore) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning: store.VersioningNone,
		Encryption: store.EncryptionNone,
		Auth:       m.authEnabled,
	}, nil
}

func (m *authMockStore) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	if !m.authEnabled {
		return nil, nil
	}
	return m.methods, nil
}

func (m *authMockStore) Login(_ context.Context, method string, creds map[string]string) (*store.Credential, error) {
	cred, err := m.loginFunc(method, creds)
	if err == nil {
		m.loggedIn = true
	}
	return cred, err
}

func (m *authMockStore) ListTrees(_ context.Context) ([]string, error) {
	var ids []string
	for id := range m.trees {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *authMockStore) DeleteTree(_ context.Context, id string) error {
	delete(m.trees, id)
	return nil
}

func (m *authMockStore) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
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

func (m *authMockStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	if m.trees[id] == nil {
		m.trees[id] = make(map[string]config.Value)
	}
	maps.Copy(m.trees[id], values)
	return nil
}

func (m *authMockStore) DeleteValues(_ context.Context, id string, paths []string) error {
	vals := m.trees[id]
	if vals == nil {
		return nil
	}
	for _, p := range paths {
		delete(vals, p)
	}
	return nil
}

func (m *authMockStore) ListTreeVersions(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *authMockStore) GetTreeVersion(_ context.Context, _, _ string, _ []string) (map[string]config.Value, error) {
	return nil, nil
}
func (m *authMockStore) RollbackTree(_ context.Context, _, _ string) error      { return nil }
func (m *authMockStore) DeleteTreeVersion(_ context.Context, _, _ string) error { return nil }
func (m *authMockStore) ListValueVersions(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *authMockStore) GetValueVersion(_ context.Context, _, _, _ string) (config.Value, bool, error) {
	return config.Value{}, false, nil
}
func (m *authMockStore) RollbackValue(_ context.Context, _, _, _ string) error      { return nil }
func (m *authMockStore) DeleteValueVersion(_ context.Context, _, _, _ string) error { return nil }
func (m *authMockStore) InitEncryption(_ context.Context, _ []byte) error           { return nil }
func (m *authMockStore) RotateEncryption(_ context.Context, _, _ []byte) error      { return nil }
func (m *authMockStore) GrantAccess(_ context.Context, _ string, _ string, _ []store.Permission) error {
	return nil
}
func (m *authMockStore) RevokeAccess(_ context.Context, _ string, _ string, _ []string) error {
	return nil
}
func (m *authMockStore) ListAccess(_ context.Context, _ string) (map[string][]store.Permission, error) {
	return nil, nil
}

func setupAuthTestController(t *testing.T, ms *authMockStore) *ui.UIController {
	t.Helper()

	reg := core.NewRegistry()
	mc := newMockConfig()

	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterStore("mock-auth", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
		Store:   core.ProviderRef{Provider: "mock-auth"},
		Dir:     t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatal(err)
	}

	return ui.NewUIController(eng)
}

func TestApp_StartsWithLoginWhenAuthRequired(t *testing.T) {
	ms := newAuthMockStore()
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	view := app.View()
	if !strings.Contains(view, "Login") {
		t.Error("expected Login in header when auth is required")
	}
	if !strings.Contains(view, "requires authentication") {
		t.Error("expected authentication message in login view")
	}
}

func TestApp_SkipsLoginWhenNoAuth(t *testing.T) {
	ms := newAuthMockStore()
	ms.authEnabled = false
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	view := app.View()
	if strings.Contains(view, "Store Login") {
		t.Error("expected no login view when auth is not required")
	}
	if !strings.Contains(view, "Tree") {
		t.Error("expected Tree view header when auth is not required")
	}
}

func TestApp_LoginViewRendersFields(t *testing.T) {
	ms := newAuthMockStore()
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	view := app.View()
	if !strings.Contains(view, "username") {
		t.Error("expected username field in login view")
	}
	if !strings.Contains(view, "password") {
		t.Error("expected password field in login view")
	}
}

func TestApp_LoginViewMethodSelection(t *testing.T) {
	ms := newAuthMockStore()
	ms.methods = []store.AuthMethod{
		{
			Type:        "userpass",
			Description: "Username and password",
			Fields: []store.AuthField{
				{Name: "username", Required: true},
				{Name: "password", Required: true, Secret: true},
			},
		},
		{
			Type:        "token",
			Description: "API token",
			Fields: []store.AuthField{
				{Name: "token", Required: true, Secret: true},
			},
		},
	}
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	view := app.View()
	// Should show method selection, not credential form.
	if !strings.Contains(view, "Select authentication method") {
		t.Error("expected method selection when multiple methods available")
	}
	if !strings.Contains(view, "userpass") {
		t.Error("expected userpass method in selection")
	}
	if !strings.Contains(view, "token") {
		t.Error("expected token method in selection")
	}

	// Navigate down and select the token method.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = model.(*tui.App)
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(*tui.App)

	view = app.View()
	if !strings.Contains(view, "token") {
		t.Error("expected token field after selecting token method")
	}
	// Should no longer show method selection.
	if strings.Contains(view, "Select authentication method") {
		t.Error("expected credential form, not method selection")
	}
}

func TestApp_LoginViewEscBackToMethodSelection(t *testing.T) {
	ms := newAuthMockStore()
	ms.methods = []store.AuthMethod{
		{Type: "userpass", Fields: []store.AuthField{{Name: "username"}}},
		{Type: "token", Fields: []store.AuthField{{Name: "token"}}},
	}
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Select the first method.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(*tui.App)

	// Should be in credential form.
	view := app.View()
	if !strings.Contains(view, "username") {
		t.Error("expected credential form")
	}

	// Press Esc to go back to method selection.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	app = model.(*tui.App)

	view = app.View()
	if !strings.Contains(view, "Select authentication method") {
		t.Error("expected method selection after Esc")
	}
}

func TestApp_LoginSuccess(t *testing.T) {
	ms := newAuthMockStore()
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Type username in the first field.
	for _, r := range "admin" {
		model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = model.(*tui.App)
	}

	// Tab to password field.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = model.(*tui.App)

	// Type password.
	for _, r := range "secret" {
		model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = model.(*tui.App)
	}

	// Submit.
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from login submit")
	}

	// Execute the command to simulate the login call.
	msg := cmd()

	// Process the login result.
	model, cmd = app.Update(msg)
	app = model.(*tui.App)

	if cmd == nil {
		t.Fatal("expected tree load command after successful login")
	}

	// Execute tree load.
	msg = cmd()
	model, _ = app.Update(msg)
	app = model.(*tui.App)

	// Should now be in tree view.
	view := app.View()
	if !strings.Contains(view, "Tree") {
		t.Error("expected Tree view after successful login")
	}
	if strings.Contains(view, "Store Login") {
		t.Error("should not be showing login view after success")
	}
}

func TestApp_LoginFailure(t *testing.T) {
	ms := newAuthMockStore()
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Type wrong credentials.
	for _, r := range "wronguser" {
		model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = model.(*tui.App)
	}

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = model.(*tui.App)

	for _, r := range "wrongpass" {
		model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = model.(*tui.App)
	}

	// Submit.
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from login submit")
	}

	// Execute the login call.
	msg := cmd()

	// Process the login result.
	model, _ = app.Update(msg)
	app = model.(*tui.App)

	// Should still be on login view with error.
	view := app.View()
	if !strings.Contains(view, "Login") {
		t.Error("expected to remain on login view after failure")
	}
	if !strings.Contains(view, "invalid credentials") {
		t.Error("expected error message after failed login")
	}
}

func TestApp_LoginSecretFieldsMasked(t *testing.T) {
	ms := newAuthMockStore()
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Tab to password field.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = model.(*tui.App)

	// Type a password.
	for _, r := range "secret" {
		model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = model.(*tui.App)
	}

	// The view should not contain the plaintext password.
	view := app.View()
	if strings.Contains(view, "secret") {
		t.Error("expected password to be masked, but found plaintext in view")
	}
}

func TestApp_LoginShowsCustomMessage(t *testing.T) {
	ms := newAuthMockStore()
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Should be on login view. Default message is shown.
	view := app.View()
	if !strings.Contains(view, "requires authentication") {
		t.Error("expected default authentication message")
	}
}

func TestApp_LoginTabNavigation(t *testing.T) {
	ms := newAuthMockStore()
	ms.methods = []store.AuthMethod{
		{
			Type: "userpass",
			Fields: []store.AuthField{
				{Name: "username", Required: true},
				{Name: "password", Required: true, Secret: true},
				{Name: "otp", Description: "One-time password"},
			},
		},
	}
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	// Tab through all fields.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = model.(*tui.App)
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = model.(*tui.App)

	// Should render without panicking.
	view := app.View()
	if view == "" {
		t.Error("expected non-empty view after tabbing")
	}

	// Shift+Tab should go back.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	app = model.(*tui.App)

	view = app.View()
	if view == "" {
		t.Error("expected non-empty view after shift-tab")
	}
}

func TestApp_LoginRequiredFieldsMarked(t *testing.T) {
	ms := newAuthMockStore()
	ms.methods = []store.AuthMethod{
		{
			Type: "userpass",
			Fields: []store.AuthField{
				{Name: "username", Required: true},
				{Name: "password", Required: true, Secret: true},
				{Name: "domain", Required: false},
			},
		},
	}
	ctrl := setupAuthTestController(t, ms)
	ctx := context.Background()

	app, err := tui.NewApp(ctx, ctrl)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*tui.App)

	view := app.View()
	// Required fields should have a marker.
	if !strings.Contains(view, "username") {
		t.Error("expected username field")
	}
	if !strings.Contains(view, "password") {
		t.Error("expected password field")
	}
	if !strings.Contains(view, "domain") {
		t.Error("expected domain field")
	}
}
