# TUI Login Implementation

## Overview

The TUI (`internal/ui/tui/`) uses Bubbletea. The login flow is implemented as a new view that is shown before the main tree view when store authentication is required.

## Login View

### File: `internal/ui/tui/login.go`

A new Bubbletea model `loginModel` handles:

1. **Method selection** -- If the store supports multiple auth methods, show a list for the user to pick from. If only one method is available, skip to the credential form.
2. **Credential form** -- Dynamically generated input fields based on `StoreAuthMethod.Fields`. Fields with `Secret: true` use `EchoNone` mode (password masking). Required fields are marked.
3. **Submission** -- On Enter, calls `StoreLogin()` with the collected credentials.
4. **Error display** -- Failed login shows the error message and allows retry.
5. **Success transition** -- On successful login, transitions to the main tree view.

### UX Flow

```
┌─────────────────────────────────────┐
│  zhi - Store Login                  │
│                                     │
│  The store requires authentication. │
│                                     │
│  Method: [userpass ▼]               │
│                                     │
│  Username: [_______________]        │
│  Password: [***************]        │
│                                     │
│  [Login]     [Quit]                 │
│                                     │
│  ✗ login failed: invalid creds      │
└─────────────────────────────────────┘
```

### Key Bindings

- `Tab` / `Shift+Tab` -- navigate between fields
- `Enter` -- submit when on last field or Login button
- `Esc` -- return to method selection (if multiple methods) or quit
- `Ctrl+C` -- quit

## Integration with App Model

The main app model (`internal/ui/tui/app.go`) gains an `appStateLogin` state:

```go
const (
    appStateLogin    appState = iota  // new
    appStateTree
    appStateEdit
    // ...
)
```

On startup, after creating the controller, the app checks `StoreAuthStatus()`:
- If status is `none` -- skip login, go directly to tree view.
- If status is `unauthenticated` or `expired` -- show login view.
- If status is `authenticated` -- skip login, go to tree view.

### Mid-Session Re-Authentication

When any controller operation returns `store.ErrAuthRequired`:
1. The app transitions back to `appStateLogin`.
2. The login view shows a message like "Session expired. Please log in again."
3. After re-login, the app returns to whichever view the user was on.

## Accessibility

- All fields have labels visible in the terminal.
- Error messages use color (red) but also include a text prefix ("Error:") for non-color terminals.
- Tab order follows visual order top-to-bottom.
