# Phase 3: WebUI Login

## Goal

Add a login page and authentication flow to the WebUI so users can authenticate with the store plugin through their browser.

## Scope

Refer to [webui.md](../webui.md) for the full design including routes, template structure, and middleware.

### Files to Create

- **`pkg/providers/ui/webui/templates/login.html`** -- Login page template with method selector and dynamic credential form.
- **`pkg/providers/ui/webui/static/css/login.css`** -- Login-specific styles (if needed beyond existing styles). Alternatively, add login styles to the existing stylesheet.

### Files to Modify

- **`pkg/providers/ui/webui/server.go`** -- Register new routes: `GET /login`, `POST /login`, `POST /logout`, `GET /auth/status`. Apply `requireAuth` middleware to existing routes.
- **`pkg/providers/ui/webui/handlers.go`** -- Add handler methods: `handleLoginPage`, `handleLogin`, `handleLogout`, `handleAuthStatus`.
- **`pkg/providers/ui/webui/middleware.go`** -- Add `requireAuth` middleware that checks `StoreAuthStatus` and redirects to `/login` when needed.
- **`pkg/providers/ui/webui/templates/`** -- Add logout button/link to the main layout when authenticated.

### Auth Middleware

The `requireAuth` middleware wraps all existing routes (except `/login`, `/logout`, `/auth/status`, and static assets). It calls `StoreAuthStatus()` on each request:

- `none` -- pass through (no auth required).
- `authenticated` -- pass through.
- `unauthenticated` / `expired` -- redirect to `/login`.

For HTMX requests (detected by `HX-Request` header), use `HX-Redirect` header instead of HTTP 303.

### Error Handling

When a handler's controller call fails with `ErrAuthRequired`:
1. Call `StoreLogout()` (or let `HandleAuthError` update the session).
2. For HTMX: respond with `HX-Redirect: /login`.
3. For full page: redirect to `/login` with a flash message.

### Tests

- **`pkg/providers/ui/webui/handlers_test.go`** (extend existing) -- Test login page render, successful login, failed login, logout, redirect behavior.
- Test `requireAuth` middleware with mock controller returning different auth statuses.
- Test CSRF token is required on POST `/login`.
- Test that authenticated users are redirected away from `/login`.

## Acceptance Criteria

- Login page renders dynamically based on store auth methods.
- Password fields are masked (`type="password"`).
- Successful login redirects to `/tree`.
- Failed login shows error and allows retry.
- Logout clears session and redirects to `/login`.
- All existing routes require auth when store has `Auth: true`.
- Existing routes work unchanged when store has `Auth: false`.
- CSRF protection on login form.
- `make check` passes.

## Dependencies

- Phase 2 (Controller API Extensions) must be complete.
