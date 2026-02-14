package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// authMockStore is a store.Plugin mock that supports authentication.
type authMockStore struct {
	mockStore
	authRequired bool
	methods      []store.AuthMethod
	loginErr     error
	credential   *store.Credential
}

func newAuthMockStore(authRequired bool) *authMockStore {
	return &authMockStore{
		mockStore:    *newMockStore(),
		authRequired: authRequired,
		methods: []store.AuthMethod{
			{
				Type:        "userpass",
				Description: "Username and password",
				Fields: []store.AuthField{
					{Name: "username", Required: true},
					{Name: "password", Required: true, Secret: true},
				},
			},
		},
		credential: &store.Credential{
			Token:    "test-token",
			Metadata: map[string]string{"username": "admin"},
		},
	}
}

func (m *authMockStore) Capabilities(context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Auth: m.authRequired,
	}, nil
}

func (m *authMockStore) AuthMethods(context.Context) ([]store.AuthMethod, error) {
	return m.methods, nil
}

func (m *authMockStore) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	if m.loginErr != nil {
		return nil, m.loginErr
	}
	return m.credential, nil
}

func TestSessionManager_AuthRequired_True(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	required, err := sm.AuthRequired(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !required {
		t.Fatal("expected auth to be required")
	}

	s := sm.Status()
	if s.Status != SessionUnauthenticated {
		t.Fatalf("expected SessionUnauthenticated, got %d", s.Status)
	}
}

func TestSessionManager_AuthRequired_False(t *testing.T) {
	ms := newAuthMockStore(false)
	sm := NewSessionManager(ms)

	required, err := sm.AuthRequired(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if required {
		t.Fatal("expected auth not to be required")
	}

	s := sm.Status()
	if s.Status != SessionNone {
		t.Fatalf("expected SessionNone, got %d", s.Status)
	}
}

func TestSessionManager_LoginSuccess(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	s, err := sm.Login(context.Background(), "userpass", map[string]string{
		"username": "admin",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != SessionAuthenticated {
		t.Fatalf("expected SessionAuthenticated, got %d", s.Status)
	}
	if s.ID == "" {
		t.Fatal("expected session ID to be set")
	}
	if s.Metadata["username"] != "admin" {
		t.Fatalf("expected metadata username=admin, got %q", s.Metadata["username"])
	}
}

func TestSessionManager_LoginFailure(t *testing.T) {
	ms := newAuthMockStore(true)
	ms.loginErr = fmt.Errorf("invalid credentials")
	sm := NewSessionManager(ms)

	_, err := sm.Login(context.Background(), "userpass", map[string]string{
		"username": "admin",
		"password": "wrong",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	s := sm.Status()
	if s.Status != SessionUnauthenticated {
		t.Fatalf("expected SessionUnauthenticated after failed login, got %d", s.Status)
	}
}

func TestSessionManager_LoginWithExpiry(t *testing.T) {
	ms := newAuthMockStore(true)
	expiry := time.Now().Add(time.Hour)
	ms.credential = &store.Credential{
		Token:     "expiring-token",
		ExpiresAt: expiry.Format(time.RFC3339),
	}
	sm := NewSessionManager(ms)

	s, err := sm.Login(context.Background(), "userpass", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ExpiresAt.IsZero() {
		t.Fatal("expected ExpiresAt to be set")
	}

	// Status should still be authenticated since expiry is in the future.
	s = sm.Status()
	if s.Status != SessionAuthenticated {
		t.Fatalf("expected SessionAuthenticated, got %d", s.Status)
	}
}

func TestSessionManager_ExpiryDetection(t *testing.T) {
	ms := newAuthMockStore(true)
	// Set expiry in the past.
	ms.credential = &store.Credential{
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Second).Format(time.RFC3339),
	}
	sm := NewSessionManager(ms)

	_, err := sm.Login(context.Background(), "userpass", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status should lazily detect expiry.
	s := sm.Status()
	if s.Status != SessionExpired {
		t.Fatalf("expected SessionExpired, got %d", s.Status)
	}
}

func TestSessionManager_Logout(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	_, err := sm.Login(context.Background(), "userpass", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sm.Logout()
	s := sm.Status()
	if s.Status != SessionUnauthenticated {
		t.Fatalf("expected SessionUnauthenticated after logout, got %d", s.Status)
	}
	if s.ID != "" {
		t.Fatal("expected session ID to be cleared")
	}
}

func TestSessionManager_HandleAuthError(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	// Login first.
	_, err := sm.Login(context.Background(), "userpass", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-auth error should not affect session.
	handled := sm.HandleAuthError(fmt.Errorf("some other error"))
	if handled {
		t.Fatal("expected non-auth error to return false")
	}
	s := sm.Status()
	if s.Status != SessionAuthenticated {
		t.Fatalf("expected SessionAuthenticated, got %d", s.Status)
	}

	// Auth error should transition to Expired.
	authErr := &store.ErrAuthRequired{Reason: "token expired"}
	handled = sm.HandleAuthError(authErr)
	if !handled {
		t.Fatal("expected auth error to return true")
	}
	s = sm.Status()
	if s.Status != SessionExpired {
		t.Fatalf("expected SessionExpired, got %d", s.Status)
	}
}

func TestSessionManager_HandleAuthError_Wrapped(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	_, err := sm.Login(context.Background(), "userpass", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wrapped auth error should still be detected.
	wrapped := fmt.Errorf("store operation failed: %w", &store.ErrAuthRequired{Reason: "expired"})
	handled := sm.HandleAuthError(wrapped)
	if !handled {
		t.Fatal("expected wrapped auth error to return true")
	}
	s := sm.Status()
	if s.Status != SessionExpired {
		t.Fatalf("expected SessionExpired, got %d", s.Status)
	}
}

func TestSessionManager_FullLifecycle(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	// Start: Unauthenticated.
	s := sm.Status()
	if s.Status != SessionUnauthenticated {
		t.Fatalf("expected SessionUnauthenticated, got %d", s.Status)
	}

	// Login -> Authenticated.
	s, err := sm.Login(context.Background(), "userpass", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != SessionAuthenticated {
		t.Fatalf("expected SessionAuthenticated, got %d", s.Status)
	}

	// HandleAuthError -> Expired.
	sm.HandleAuthError(&store.ErrAuthRequired{Reason: "expired"})
	s = sm.Status()
	if s.Status != SessionExpired {
		t.Fatalf("expected SessionExpired, got %d", s.Status)
	}

	// Logout -> Unauthenticated.
	sm.Logout()
	s = sm.Status()
	if s.Status != SessionUnauthenticated {
		t.Fatalf("expected SessionUnauthenticated, got %d", s.Status)
	}
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			sm.Status()
		}()
		go func() {
			defer wg.Done()
			sm.Login(context.Background(), "userpass", map[string]string{})
		}()
		go func() {
			defer wg.Done()
			sm.Logout()
		}()
	}
	wg.Wait()
}

func TestSessionManager_AuthMethods(t *testing.T) {
	ms := newAuthMockStore(true)
	sm := NewSessionManager(ms)

	methods, err := sm.AuthMethods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if methods[0].Type != "userpass" {
		t.Fatalf("expected userpass method, got %q", methods[0].Type)
	}
}
