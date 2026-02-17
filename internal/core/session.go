package core

import (
	"context"
	"sync"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/google/uuid"
)

// SessionStatus represents the current authentication state.
type SessionStatus int

const (
	SessionNone            SessionStatus = iota // no auth required by store
	SessionUnauthenticated                      // auth required, not logged in
	SessionAuthenticated                        // logged in
	SessionExpired                              // was authenticated, token expired
)

// Session holds the current authentication state for a store plugin.
type Session struct {
	ID        string // opaque session identifier (UUID)
	Status    SessionStatus
	ExpiresAt time.Time         // zero value = no expiry
	Metadata  map[string]string // informational (e.g. username, auth method)
}

// SessionManager manages the authentication lifecycle for the active
// store plugin within a single workspace session.
type SessionManager struct {
	mu      sync.RWMutex
	session *Session
	store   store.Plugin
}

// NewSessionManager creates a new SessionManager for the given store plugin.
func NewSessionManager(s store.Plugin) *SessionManager {
	return &SessionManager{
		store: s,
		session: &Session{
			Status: SessionUnauthenticated,
		},
	}
}

// AuthRequired checks store capabilities and returns whether login is needed.
func (m *SessionManager) AuthRequired(ctx context.Context) (bool, error) {
	caps, err := m.store.Capabilities(ctx)
	if err != nil {
		return false, err
	}
	if !caps.Auth {
		m.mu.Lock()
		m.session.Status = SessionNone
		m.mu.Unlock()
		return false, nil
	}
	return true, nil
}

// AuthMethods returns the auth methods supported by the store.
func (m *SessionManager) AuthMethods(ctx context.Context) ([]store.AuthMethod, error) {
	return m.store.AuthMethods(ctx)
}

// Login authenticates with the store using the given method and credentials.
// On success, creates a new session with the returned token.
func (m *SessionManager) Login(ctx context.Context, method string, credentials map[string]string) (*Session, error) {
	cred, err := m.store.Login(ctx, method, credentials)
	if err != nil {
		return nil, err
	}

	var expiresAt time.Time
	if cred.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, cred.ExpiresAt)
		if err == nil {
			expiresAt = t
		}
	}

	m.mu.Lock()
	m.session = &Session{
		ID:        uuid.New().String(),
		Status:    SessionAuthenticated,
		ExpiresAt: expiresAt,
		Metadata:  cred.Metadata,
	}
	s := *m.session
	m.mu.Unlock()

	return &s, nil
}

// LoginInteractive starts an interactive (browser-based) authentication
// flow by delegating to the store plugin.
func (m *SessionManager) LoginInteractive(ctx context.Context, method string, params map[string]string) (*store.InteractiveChallenge, error) {
	return m.store.LoginInteractive(ctx, method, params)
}

// LoginInteractiveCallback completes an interactive auth flow with the
// callback parameters from the identity provider. On success, creates a
// new session just like Login does.
func (m *SessionManager) LoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*Session, error) {
	cred, err := m.store.LoginInteractiveCallback(ctx, challengeID, callbackParams)
	if err != nil {
		return nil, err
	}

	var expiresAt time.Time
	if cred.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, cred.ExpiresAt)
		if err == nil {
			expiresAt = t
		}
	}

	m.mu.Lock()
	m.session = &Session{
		ID:        uuid.New().String(),
		Status:    SessionAuthenticated,
		ExpiresAt: expiresAt,
		Metadata:  cred.Metadata,
	}
	s := *m.session
	m.mu.Unlock()

	return &s, nil
}

// Status returns the current session state. It lazily checks expiry:
// if the session has an expiry time that is in the past, it transitions
// to SessionExpired.
func (m *SessionManager) Status() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session.Status == SessionAuthenticated &&
		!m.session.ExpiresAt.IsZero() &&
		time.Now().After(m.session.ExpiresAt) {
		m.session.Status = SessionExpired
	}

	s := *m.session
	return &s
}

// Logout clears the current session, transitioning to Unauthenticated.
func (m *SessionManager) Logout() {
	m.mu.Lock()
	m.session = &Session{
		Status: SessionUnauthenticated,
	}
	m.mu.Unlock()
}

// HandleAuthError checks if an error is an auth error and transitions
// the session to Expired if so. Returns true if the error was auth-related.
func (m *SessionManager) HandleAuthError(err error) bool {
	if !store.IsAuthRequired(err) {
		return false
	}

	m.mu.Lock()
	if m.session.Status == SessionAuthenticated || m.session.Status == SessionExpired {
		m.session.Status = SessionExpired
	}
	m.mu.Unlock()

	return true
}
