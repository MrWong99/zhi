# Phase 2: Controller API Extensions

## Goal

Expose store authentication operations through the UI controller layer so that both built-in and external UI plugins can initiate login flows.

## Scope

Refer to [Controller API Extensions in design.md](../design.md#controller-api-extensions) for the method signatures and [proto.md](../proto.md) for the gRPC message definitions.

### Files to Create

- **`pkg/zhiplugin/ui/auth.go`** -- UI-facing auth types: `StoreAuthMethod`, `StoreAuthField`, `StoreSession`, `StoreSessionStatus` constants.

### Files to Modify

- **`pkg/zhiplugin/ui/plugin.go`** -- Add `StoreAuthMethods`, `StoreLogin`, `StoreAuthStatus`, `StoreLogout` to the `Controller` interface.
- **`internal/ui/driver.go`** (`UIController`) -- Implement the four new methods, delegating to `engine.Session()`.
- **`internal/ui/adapter.go`** (`ControllerAdapter`) -- Bridge the new `UIController` methods to the `Controller` interface.
- **`api/proto/zhiplugin/v1/ui.proto`** -- Add the new RPCs and messages to `UIControllerService` (see [proto.md](../proto.md)).
- **`pkg/zhiplugin/ui/controller_client.go`** -- gRPC client implementations for the four new RPCs.
- **`pkg/zhiplugin/ui/controller_server.go`** -- gRPC server implementations for the four new RPCs.
- **`pkg/zhiplugin/ui/proto/`** -- Regenerated via `make proto`.

### Error Propagation in Existing Methods

Wrap existing `UIController` methods (`SaveTree`, `LoadTree`) to call `engine.Session().HandleAuthError(err)` on failures. This ensures the session status is updated when a store operation fails with `ErrAuthRequired`. See [Error Handling Strategy in design.md](../design.md#error-handling-strategy).

### Test Updates

- **`internal/ui/driver_test.go`** (or new file) -- Test the `UIController` auth methods with a mock store plugin.
- **`internal/ui/adapter_test.go`** -- Test that `ControllerAdapter` correctly delegates auth calls.
- Update any existing mock implementations of `Controller` to include the four new methods (return `nil` / `StoreSessionNone` by default).

## Acceptance Criteria

- `Controller` interface includes all four auth methods.
- `UIController` delegates correctly to `SessionManager`.
- `ControllerAdapter` bridges correctly.
- gRPC round-trip works for all four RPCs.
- `make proto-check` passes (generated code committed).
- Existing tests still pass (mocks updated).
- `make check` passes.

## Dependencies

- Phase 1 (Core Session Manager) must be complete.
