# Phase 4: TUI Login

## Goal

Add a login view to the TUI so users can authenticate with the store plugin from the terminal.

## Scope

Refer to [tui.md](../tui.md) for the full design including the view layout, key bindings, and integration with the app model.

### Files to Create

- **`internal/ui/tui/login.go`** -- `loginModel` Bubbletea model with:
  - Method selection list (when multiple methods available).
  - Dynamic credential form generated from `StoreAuthMethod.Fields`.
  - Password masking for secret fields (`textinput.EchoNone`).
  - Submit handling that calls `StoreLogin()` via the controller.
  - Error display with retry capability.
- **`internal/ui/tui/login_test.go`** -- Unit tests for the login model.

### Files to Modify

- **`internal/ui/tui/app.go`** -- Add `appStateLogin` state. On startup, check `StoreAuthStatus()` and show login view if needed. Handle transition back to login on auth errors from any view.
- **`internal/ui/tui/app.go`** (key handling) -- Route auth errors from controller calls to trigger login view re-display.

### Auth Error Interception

All views that call controller methods (tree, edit, save, etc.) should propagate errors upward. The app model checks errors from controller calls:

```go
if store.IsAuthRequired(err) {
    return m.showLoginView("Session expired. Please log in again.")
}
```

### Tests

- Test login model renders correct fields for a given set of auth methods.
- Test successful login transitions to tree view.
- Test failed login shows error and stays on login view.
- Test method selection works when multiple methods are available.
- Test that secret fields are masked.
- Test that the app starts with login view when auth is required.

## Acceptance Criteria

- Login view shows before tree view when auth is required.
- Method selector appears when multiple auth methods exist.
- Credential fields are generated dynamically from auth method definitions.
- Secret fields are masked in the terminal.
- Successful login transitions to tree view.
- Failed login shows error with retry.
- Mid-session expiry shows login view with expiry message.
- `make check` passes.

## Dependencies

- Phase 2 (Controller API Extensions) must be complete.
- Can be done in parallel with Phase 3 (WebUI Login).
