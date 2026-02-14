# Phase 6: Browser-Based Authentication (OIDC)

## Goal

Support authentication methods that require browser interaction, primarily OIDC. Vault's OIDC auth method redirects the user to an external identity provider (e.g., Okta, Auth0, Keycloak) for authentication and receives a token via a callback URL after the user authenticates.

This phase extends the generalized login system from Phases 1-4 to handle interactive, redirect-based auth flows in both the WebUI and TUI.

## Background: How Vault OIDC Works

1. Client sends `POST /v1/auth/oidc/oidc/auth_url` with `role` and a `redirect_uri`.
2. Vault returns an `auth_url` (the IdP's authorization endpoint with a `state` and `nonce`).
3. User is redirected to (or opens) the `auth_url` in their browser.
4. User authenticates at the IdP and is redirected back to the `redirect_uri` with `code` and `state` query parameters.
5. Client sends `PUT /v1/auth/oidc/oidc/callback` with `code`, `state`, and `nonce` to Vault.
6. Vault exchanges the code for tokens at the IdP, validates them, and returns a Vault token.

The critical difference from credential-based methods: **the user must leave the application, authenticate in a browser, and return** -- the client needs a way to receive the callback.

## Protocol Extension: Interactive Auth Methods

### `AuthMethod` Changes

Add an `Interactive` field to `store.AuthMethod`:

```go
type AuthMethod struct {
    Type        string
    Description string
    Fields      []AuthField
    Interactive bool // true if this method requires browser/external interaction
}
```

And the corresponding proto field in `store.proto`:

```protobuf
message AuthMethodMsg {
  string type = 1;
  string description = 2;
  repeated AuthFieldMsg fields = 3;
  bool interactive = 4;
}
```

UIs use `Interactive` to decide how to render the auth method:
- `Interactive: false` -- render a credential form (existing behavior).
- `Interactive: true` -- render a "Login with <method>" button that initiates the browser flow.

Methods without `Interactive` default to form-based login (backward compatible).

### Store Plugin Protocol: `LoginInteractive`

A new method on the `store.Plugin` interface handles the multi-step interactive flow:

```go
// LoginInteractive starts an interactive authentication flow.
// The store returns an InteractiveChallenge containing a URL to open
// and metadata needed to complete the flow.
LoginInteractive(ctx context.Context, method string, params map[string]string) (*InteractiveChallenge, error)

// LoginInteractiveCallback completes the interactive auth flow with
// the callback parameters received from the identity provider.
LoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*Credential, error)
```

```go
type InteractiveChallenge struct {
    ChallengeID string // opaque identifier for this auth attempt
    AuthURL     string // URL the user must visit in their browser
    ExpiresAt   string // RFC3339 timestamp after which the challenge expires
}
```

The corresponding gRPC RPCs go into `store.proto`.

### Controller API

Two new methods on `ui.Controller`:

```go
// StoreLoginInteractive initiates a browser-based login flow.
// Returns a challenge with a URL to open.
StoreLoginInteractive(ctx context.Context, method string, params map[string]string) (*StoreInteractiveChallenge, error)

// StoreLoginInteractiveCallback completes the flow after the user
// authenticates at the IdP. The callbackParams are the query parameters
// from the redirect callback.
StoreLoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*StoreSession, error)
```

---

## Approach A: Local Callback Server

The core (or the UI) starts a temporary HTTP server on localhost to receive the IdP callback.

### How It Works

1. UI calls `StoreLoginInteractive("oidc", {"role": "my-role"})`.
2. The `SessionManager` starts a local HTTP listener on a random port (e.g., `http://localhost:38291/oidc/callback`).
3. It calls `store.LoginInteractive()` with `redirect_uri` set to the local listener's URL.
4. Store plugin contacts Vault, which returns the IdP `auth_url`.
5. `InteractiveChallenge` is returned to the UI with the `auth_url`.
6. **WebUI:** redirects the browser to `auth_url` (or opens in a new tab). **TUI:** calls `xdg-open` / `open` to launch the browser.
7. User authenticates at the IdP.
8. IdP redirects back to `http://localhost:38291/oidc/callback?code=X&state=Y`.
9. The local callback server receives the request, extracts the parameters, and calls `store.LoginInteractiveCallback(challengeID, params)`.
10. Store plugin calls Vault's callback endpoint and returns a `Credential`.
11. Session transitions to `Authenticated`.
12. The local callback server responds with an HTML page saying "Login successful. You may close this tab." and shuts down.
13. **WebUI:** JavaScript on the login page polls `GET /auth/status` and detects the transition, then redirects to `/tree`. **TUI:** a channel or callback triggers the transition from the login view to tree view.

### Callback Server Implementation

Located in `internal/core/authcallback/` (or as part of the `SessionManager`):

```go
type CallbackServer struct {
    listener net.Listener
    result   chan CallbackResult
}

type CallbackResult struct {
    Params map[string]string // query parameters from the IdP callback
    Err    error
}

func NewCallbackServer() (*CallbackServer, error)      // binds to localhost:0
func (s *CallbackServer) URL() string                   // returns the callback URL
func (s *CallbackServer) Wait(ctx context.Context) (CallbackResult, error) // blocks until callback
func (s *CallbackServer) Close()                        // shuts down the listener
```

The server only accepts a single request, responds, and shuts down. It binds to `127.0.0.1` only (not `0.0.0.0`) to prevent external access.

### Pros

- **Works for all UIs.** WebUI, TUI, and any future UI can use the same mechanism. The browser opens the IdP page, the callback lands on localhost regardless of which UI initiated the flow.
- **Self-contained.** The callback server is managed by the core, not the UI. UIs only need to open a browser and wait for the session to become authenticated.
- **Standard OAuth2 pattern.** This is how `vault login -method=oidc` works in the Vault CLI -- it starts a local listener.
- **No polling required.** The callback server directly receives the response and completes the flow synchronously.

### Cons

- **Port allocation.** The callback URL must be known before calling the IdP, but the port is random. Some IdPs require pre-registered redirect URIs, which conflicts with random ports. Vault handles this by using a configurable `allowed_redirect_uris` list in the OIDC role config that includes `http://localhost:8250/oidc/callback` (Vault's default).
  - **Mitigation:** Use a well-known port (e.g., `8250` like Vault does). Fall back to a random port if `8250` is taken. The Vault OIDC role must have the matching redirect URI configured.
- **Firewall/container issues.** In some environments (e.g., remote dev containers, cloud workstations), `localhost` is not reachable from the user's browser.
  - **Mitigation:** Document the requirement. For WebUI running on a remote host, Approach B (below) is more appropriate.
- **Ephemeral listener security.** The callback server is a temporary HTTP endpoint on localhost. An attacker on the same machine could try to send a crafted callback.
  - **Mitigation:** Validate the `state` parameter matches the one in the challenge. The callback server only accepts one request and immediately shuts down. Bind to `127.0.0.1` only.

### Implementation Scope

- **`internal/core/authcallback/server.go`** -- `CallbackServer` implementation.
- **`internal/core/authcallback/server_test.go`** -- Tests.
- **`internal/core/session.go`** -- Extend `SessionManager` to manage the callback server lifecycle during interactive logins.
- **`pkg/zhiplugin/store/plugin.go`** -- Add `LoginInteractive` and `LoginInteractiveCallback` to the `Plugin` interface.
- **`pkg/zhiplugin/store/grpc_client.go` / `grpc_server.go`** -- gRPC implementations.
- **`api/proto/zhiplugin/v1/store.proto`** -- New RPCs and messages.
- **`pkg/providers/store/vault/vault.go`** -- Implement `LoginInteractive` (calls Vault's `auth/oidc/oidc/auth_url`) and `LoginInteractiveCallback` (calls Vault's `auth/oidc/oidc/callback`).

---

## Approach B: Polling-Based Flow

Instead of a local callback server, the UI initiates the OIDC flow and then polls the store plugin for completion.

### How It Works

1. UI calls `StoreLoginInteractive("oidc", {"role": "my-role"})`.
2. `SessionManager` calls `store.LoginInteractive()`. The store plugin generates a `state`/`nonce`, stores them internally, and returns an `InteractiveChallenge` with the `auth_url`.
3. UI opens the `auth_url` in the browser.
4. User authenticates at the IdP.
5. **The redirect URI points back to the store plugin's own callback endpoint** (e.g., Vault's built-in OIDC callback handler at `<vault_addr>/v1/auth/oidc/oidc/callback`). This means Vault handles the callback directly.
6. Meanwhile, the UI polls `StoreLoginInteractiveCallback(challengeID, nil)` periodically.
7. When Vault has received the callback and completed token exchange, the next poll returns the `Credential`.
8. Session transitions to `Authenticated`.

### Vault-Specific Detail

Vault actually does support this model natively. When you configure OIDC with a redirect URI pointing back to Vault itself (e.g., `https://vault.example.com:8200/v1/auth/oidc/oidc/callback`), Vault handles the callback. The client can then poll for completion.

However, Vault's standard API doesn't have a built-in polling endpoint for this -- the Vault CLI uses the local callback approach. So this approach would require:
- Either a custom polling mechanism in the store plugin (tracking challenge state internally), or
- Using Vault's redirect-to-self and intercepting the resulting token through the UI plugin.

### Pros

- **Works in remote environments.** No localhost listener needed. The WebUI running on a remote server can use its own URL as the redirect target (see variant below).
- **No port allocation.** No local listener means no port conflicts.
- **Simpler for WebUI.** The WebUI can use its own `/oidc/callback` route as the redirect URI. The browser redirects back to the WebUI, which receives the code/state and completes the flow server-side.

### Cons

- **Polling adds latency.** The UI must poll periodically (e.g., every 1-2 seconds) until the flow completes. This adds a delay between the user completing auth at the IdP and the UI reflecting it.
- **Timeout management.** Must handle the case where the user never completes the flow. Need a timeout (e.g., 5 minutes) after which the challenge expires and the UI shows an error.
- **Store plugin complexity.** The store plugin must track pending challenges internally (in-memory map keyed by challenge ID), which adds state management.
- **Doesn't generalize as well for TUI.** The TUI can open a browser, but the callback can't go back to the TUI. If the redirect goes to Vault, the polling mechanism works, but it adds complexity to the store plugin that may not exist today.
- **Vault API limitations.** Vault's OIDC flow is designed around the local callback pattern. Making polling work requires custom state tracking in the store plugin, since Vault doesn't natively expose a "check if this challenge completed" API.

### Implementation Scope

- **`pkg/zhiplugin/store/plugin.go`** -- Add `LoginInteractive` and `LoginInteractiveCallback` (same as Approach A).
- **`pkg/zhiplugin/store/store.go`** -- Add `InteractiveChallenge` type (same as Approach A).
- **`pkg/providers/store/vault/vault.go`** -- Implement `LoginInteractive` and `LoginInteractiveCallback` with internal challenge tracking. Vault's `LoginInteractiveCallback` would need to either poll Vault internally or track state from a callback that Vault redirects to.
- No `internal/core/authcallback/` needed.

---

## Comparison

| Aspect | Approach A (Local Callback) | Approach B (Polling) |
|--------|---------------------------|---------------------|
| Works for TUI | Yes (opens browser, localhost catches callback) | Partially (opens browser, but polling adds latency and store complexity) |
| Works for WebUI (local) | Yes | Yes |
| Works for WebUI (remote) | No (browser can't reach remote localhost) | Yes (WebUI can use its own URL) |
| Implementation complexity | Medium (callback server is ~100 lines) | High (challenge tracking in store plugin, polling logic in UI, timeout management) |
| Latency | Instant (callback is synchronous) | 1-2 second poll interval |
| Vault compatibility | Native (same as `vault login -method=oidc`) | Requires custom state tracking |
| Port allocation issues | Yes (mitigated with well-known port) | None |
| Security surface | Small (localhost-only, single-use, state-validated) | Smaller (no listener) |

## Recommendation

**Approach A** for the initial implementation, with a documented limitation that it requires the user's browser to reach `localhost`. This matches Vault's own CLI behavior and is simpler to implement correctly.

**Approach B** can be added later as an alternative for remote WebUI deployments. The `StoreLoginInteractive` / `StoreLoginInteractiveCallback` protocol supports both approaches -- the difference is in who receives the callback (local server vs. store/UI).

A hybrid is also possible: the `SessionManager` could detect which approach to use based on context. If the UI reports `RequiresTTY: true` or the WebUI is running locally, use Approach A. If the WebUI is running remotely, use Approach B (with the WebUI's own `/oidc/callback` route as the redirect URI).

## Dependencies

- Phases 1-5 must be complete.
- Vault OIDC auth backend must be configured with appropriate `allowed_redirect_uris`.

## Files to Create (Approach A)

- `internal/core/authcallback/server.go`
- `internal/core/authcallback/server_test.go`

## Files to Modify

- `pkg/zhiplugin/store/store.go` -- `AuthMethod.Interactive`, `InteractiveChallenge`
- `pkg/zhiplugin/store/plugin.go` -- `LoginInteractive`, `LoginInteractiveCallback`
- `api/proto/zhiplugin/v1/store.proto` -- new RPCs and messages
- `pkg/zhiplugin/store/grpc_client.go` / `grpc_server.go` -- gRPC implementations
- `pkg/providers/store/vault/vault.go` -- Vault OIDC implementation
- `internal/core/session.go` -- interactive login flow with callback server
- `pkg/zhiplugin/ui/plugin.go` -- `StoreLoginInteractive`, `StoreLoginInteractiveCallback` on Controller
- `api/proto/zhiplugin/v1/ui.proto` -- new controller RPCs
- UI implementations (TUI: open browser; WebUI: redirect or new tab + poll `/auth/status`)
