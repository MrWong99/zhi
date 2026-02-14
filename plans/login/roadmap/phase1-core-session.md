# Phase 1: Core Session Manager

## Goal

Implement the `SessionManager` in `internal/core/` that manages authentication state for the store plugin. This is the foundation that all UI implementations depend on.

## Scope

Refer to the [Session Manager section of design.md](../design.md#session-manager) for the full type definitions and state machine.

### Files to Create

- **`internal/core/session.go`** -- `SessionManager` struct, `Session`, `SessionStatus` types, all methods (`AuthRequired`, `AuthMethods`, `Login`, `Status`, `Logout`, `HandleAuthError`).
- **`internal/core/session_test.go`** -- Unit tests covering:
  - State transitions: `Unauthenticated -> Authenticated -> Expired -> Unauthenticated`
  - `AuthRequired` returns false when store has `Auth: false`
  - `Login` success and failure paths
  - `HandleAuthError` detects `ErrAuthRequired` and transitions state
  - Expiry detection in `Status()`
  - Concurrent access safety (test with `go test -race`)

### Files to Modify

- **`internal/core/engine.go`** -- Add `session *SessionManager` field to `Engine`. Initialize it in the engine constructor when a store plugin is present. Add `Session() *SessionManager` accessor method.

## Acceptance Criteria

- `SessionManager` correctly transitions between all states.
- All operations are safe for concurrent use (mutex-protected).
- `HandleAuthError` correctly identifies `store.ErrAuthRequired` in error chains.
- Expiry is lazily checked -- no background goroutines in the session manager.
- Unit tests pass with `-race -count=1`.

## Dependencies

None -- this is the first phase.

## Estimated Changes

~200 lines of implementation + ~300 lines of tests.
