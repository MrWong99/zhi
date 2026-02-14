# Login System Design

## Overview

This document describes the generalized authentication flow that allows any UI plugin to authenticate against any store plugin through the core engine. The design introduces a **session manager** in the core that acts as the single source of truth for authentication state, keeping UI plugins unaware of store-specific details beyond a session ID and login status.

## Architecture

```
+----------+     Controller      +-----------+     Session Mgr     +--------+
|  UI      | ─────────────────>  |   Core    | ──────────────────> | Store  |
| (TUI,    |  AuthMethods()      |  Engine   |  AuthMethods()      | Plugin |
|  WebUI,  |  Login(method,creds)|           |  Login(method,creds)| (Vault,|
|  custom) |  AuthStatus()       |  Session  |                     |  etc.) |
|          |  Logout()           |  Manager  |                     |        |
+----------+                     +-----------+                     +--------+
```

### Key Principles

1. **Core owns the session.** The `SessionManager` in `internal/core/` holds authentication state. UIs only see an opaque session ID and a status enum.
2. **Store plugins define auth methods.** Each store advertises its supported auth methods via `AuthMethods()` with self-describing field definitions. The UI renders forms dynamically from these definitions.
3. **Credentials never persist in the core.** The core passes credentials through to the store plugin and discards them. Only the opaque token returned by the store is held in memory for the session lifetime.
4. **Auth errors are typed.** `store.ErrAuthRequired` already exists and is returned by store operations when auth is needed. The core intercepts these and surfaces them to the UI as a session status change.

## Session Manager

The session manager lives in `internal/core/session.go` and is owned by the `Engine`.

### State Machine

```
                     Login(ok)
   Unauthenticated ──────────> Authenticated
         ^                          |
         |    Logout / Expire /     |
         +──── ErrAuthRequired ─────+
```

### Session State

```go
// SessionStatus represents the current authentication state.
type SessionStatus int

const (
    SessionNone           SessionStatus = iota // no auth required by store
    SessionUnauthenticated                     // auth required, not logged in
    SessionAuthenticated                       // logged in
    SessionExpired                             // was authenticated, token expired
)

// Session holds the current authentication state for a store plugin.
type Session struct {
    ID        string            // opaque session identifier (UUID)
    Status    SessionStatus
    ExpiresAt time.Time         // zero value = no expiry
    Metadata  map[string]string // informational (e.g. username, auth method)
}
```

### Manager Interface

```go
// SessionManager manages the authentication lifecycle for the active
// store plugin within a single workspace session.
type SessionManager struct {
    mu      sync.RWMutex
    session *Session
    store   store.Plugin
}

func NewSessionManager(store store.Plugin) *SessionManager

// AuthRequired checks store capabilities and returns whether login is needed.
func (m *SessionManager) AuthRequired(ctx context.Context) (bool, error)

// AuthMethods returns the auth methods supported by the store.
func (m *SessionManager) AuthMethods(ctx context.Context) ([]store.AuthMethod, error)

// Login authenticates with the store using the given method and credentials.
// On success, creates a new session with the returned token.
func (m *SessionManager) Login(ctx context.Context, method string, credentials map[string]string) (*Session, error)

// Status returns the current session status without checking the store.
func (m *SessionManager) Status() *Session

// Logout clears the current session.
func (m *SessionManager) Logout()

// HandleAuthError checks if an error is an auth error and transitions
// the session to Expired if so. Returns true if the error was auth-related.
func (m *SessionManager) HandleAuthError(err error) bool
```

### Expiry Handling

When a store returns a `Credential` with `ExpiresAt`, the session manager tracks the expiry time. Rather than a background goroutine, the session is lazily checked:

- `Status()` checks `ExpiresAt` and transitions to `SessionExpired` if past.
- The Vault store's own token renewal goroutine handles the actual Vault token renewal. The session manager's expiry is a safety net for the UI to know when re-login may be needed.
- Store operations that return `ErrAuthRequired` cause the session to transition to `SessionExpired` via `HandleAuthError()`.

## Controller API Extensions

The `ui.Controller` interface (in `pkg/zhiplugin/ui/plugin.go`) gains four new methods:

```go
// StoreAuthMethods returns the authentication methods supported by the
// configured store plugin. Returns nil if no store is configured or
// authentication is not required.
StoreAuthMethods(ctx context.Context) ([]StoreAuthMethod, error)

// StoreLogin authenticates with the store using the specified method
// and credentials. Returns the session status after the attempt.
StoreLogin(ctx context.Context, method string, credentials map[string]string) (*StoreSession, error)

// StoreAuthStatus returns the current authentication status.
StoreAuthStatus(ctx context.Context) (*StoreSession, error)

// StoreLogout clears the current authentication session.
StoreLogout(ctx context.Context) error
```

### UI-Facing Types

These types live in `pkg/zhiplugin/ui/auth.go` and mirror the store types but are decoupled to allow UI-specific additions:

```go
type StoreAuthMethod struct {
    Type        string
    Description string
    Fields      []StoreAuthField
}

type StoreAuthField struct {
    Name        string
    Description string
    Required    bool
    Secret      bool
}

type StoreSessionStatus string

const (
    StoreSessionNone            StoreSessionStatus = "none"
    StoreSessionUnauthenticated StoreSessionStatus = "unauthenticated"
    StoreSessionAuthenticated   StoreSessionStatus = "authenticated"
    StoreSessionExpired         StoreSessionStatus = "expired"
)

type StoreSession struct {
    SessionID string
    Status    StoreSessionStatus
    ExpiresAt string              // RFC3339 or empty
    Metadata  map[string]string   // informational only
}
```

## gRPC Protocol Extensions

New messages and RPCs are added to `api/proto/zhiplugin/v1/ui.proto` in the `UIControllerService`:

```protobuf
// --- Store Authentication ---
rpc StoreAuthMethods(CtrlStoreAuthMethodsRequest) returns (CtrlStoreAuthMethodsResponse);
rpc StoreLogin(CtrlStoreLoginRequest) returns (CtrlStoreLoginResponse);
rpc StoreAuthStatus(CtrlStoreAuthStatusRequest) returns (CtrlStoreAuthStatusResponse);
rpc StoreLogout(CtrlStoreLogoutRequest) returns (CtrlStoreLogoutResponse);
```

See [proto.md](proto.md) for full message definitions.

## Authentication Flow

### Initial Load

1. Engine starts, loads workspace config, initializes store plugin.
2. `SessionManager` is created with the store plugin reference.
3. Engine calls `SessionManager.AuthRequired()` which calls `store.Capabilities()`.
4. If `Capabilities.Auth == true`, session status is `SessionUnauthenticated`.
5. UI starts, calls `StoreAuthStatus()` to check if login is needed.
6. If unauthenticated, UI calls `StoreAuthMethods()` and renders a login form.

### Login

1. User fills in credentials in the UI.
2. UI calls `StoreLogin(method, credentials)`.
3. `UIController` delegates to `SessionManager.Login()`.
4. `SessionManager` calls `store.Login(method, credentials)`.
5. On success: session transitions to `Authenticated`, credential token is stored in memory.
6. On failure: error is returned to UI, session stays `Unauthenticated`.
7. UI proceeds to load the tree on success.

### Mid-Session Expiry

1. UI calls `SaveTree()` or `LoadTree()`.
2. Store returns `ErrAuthRequired`.
3. Engine's error handling calls `SessionManager.HandleAuthError(err)`.
4. Session transitions to `SessionExpired`.
5. Error propagates to UI with typed auth error.
6. UI detects auth error (via `store.IsAuthRequired()`) and shows re-login prompt.

### Logout

1. User triggers logout in UI.
2. UI calls `StoreLogout()`.
3. `SessionManager.Logout()` clears session state.
4. Session transitions to `SessionUnauthenticated`.

## Security Considerations

- **No credential persistence.** Credentials are passed through to the store plugin and never written to disk or held in core memory. Only the opaque token is retained.
- **Token in memory only.** The session token lives only in the `SessionManager`'s in-memory state. It is not serialized, logged, or exported.
- **Credential masking.** Fields with `Secret: true` must be masked in all UI implementations (password inputs, no echo in TUI).
- **CSRF protection.** WebUI login form POST requests are protected by the existing CSRF double-submit cookie middleware.
- **Transport security.** gRPC channels between host and external UI plugins use the existing hashicorp/go-plugin transport (Unix socket or loopback with mTLS).
- **Rate limiting.** The core does not implement login rate limiting since the store plugin (e.g., Vault) handles this at the backend. The UI should display errors from failed attempts clearly.
- **Session isolation.** Each workspace run has its own `Engine` and `SessionManager`. There is no cross-workspace session leakage.

## Error Handling Strategy

All store operations in the `UIController` should be wrapped to detect auth errors:

```go
func (c *UIController) SaveTree(ctx context.Context) error {
    // ... existing logic ...
    err := c.engine.SaveTree(ctx, c.tree)
    if err != nil {
        c.engine.Session().HandleAuthError(err)
    }
    return err
}
```

UIs should check `store.IsAuthRequired(err)` on any controller error to decide whether to show a re-login prompt rather than a generic error.
