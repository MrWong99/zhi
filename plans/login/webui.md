# WebUI Login Implementation

## Overview

The WebUI (`pkg/providers/ui/webui/`) uses server-rendered HTML with Go templates. The login flow adds a login page that gates access to the main UI when store authentication is required.

## Routes

### New Endpoints

```
GET  /login          -- render the login page
POST /login          -- submit credentials
POST /logout         -- clear session and redirect to login
GET  /auth/status    -- JSON endpoint returning current auth status (for HTMX)
```

### Route Registration

Added in `server.go` alongside existing routes. The login routes do **not** require an active store session (they are the mechanism to create one).

## Login Page

### Template: `templates/login.html`

The login page is a full page (not a partial within the main layout) since the user cannot interact with any other feature until authenticated.

**Structure:**

1. **Header** -- workspace name, zhi branding.
2. **Method selector** -- dropdown or tab bar if multiple auth methods. On change, the form fields update dynamically (HTMX swap or full page reload with query param).
3. **Credential form** -- fields rendered from `StoreAuthMethod.Fields`:
   - `Secret: true` fields use `<input type="password">`.
   - `Required: true` fields have `required` attribute.
   - Each field has a `<label>` with the `Description`.
4. **Submit button** -- "Login" button, POST to `/login`.
5. **Error display** -- shown inline when login fails, with the error message from the store.

**CSRF:** The form includes the CSRF token via the existing `csrfField` template function.

### Handler: `handlers.go`

```go
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
    // Get auth methods from controller
    // Get current auth status
    // If already authenticated, redirect to /tree
    // Render login template with methods and any error flash
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    // Parse form: method + credential fields
    // Call controller.StoreLogin(method, credentials)
    // On success: redirect to /tree
    // On failure: re-render login page with error
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
    // Call controller.StoreLogout()
    // Redirect to /login
}
```

## Auth Middleware

A new middleware `requireAuth` wraps routes that need an active store session:

```go
func (s *Server) requireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        status, err := s.controller.StoreAuthStatus(r.Context())
        if err != nil || status.Status == "unauthenticated" || status.Status == "expired" {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        if status.Status == "none" {
            // No auth required, pass through
            next.ServeHTTP(w, r)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

This middleware is applied to all routes **except** `/login`, `/logout`, `/auth/status`, and static assets.

## Mid-Session Expiry

When a store operation fails with `ErrAuthRequired` during an HTMX request:

1. The handler detects the auth error.
2. Returns an HTMX redirect header (`HX-Redirect: /login`) or a full redirect for non-HTMX requests.
3. The login page shows "Session expired. Please log in again."

## Static Assets

The login page uses the existing CSS framework and static asset pipeline (with content hashing). No new JS dependencies are required -- the method selector can use a simple form submission or HTMX `hx-get` to swap the field set.

## Security

- Login form uses POST with CSRF protection (existing middleware).
- Password fields use `type="password"` and `autocomplete="current-password"`.
- Failed login attempts show a generic "Login failed" message by default. The store plugin's error message is shown if it provides one, but credential values are never reflected back.
- The login page sets `Cache-Control: no-store` to prevent browser caching of auth state.
- Logout clears server-side session state; there is no client-side token.
