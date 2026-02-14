# Phase 5: Integration Testing and Vault Validation

## Goal

End-to-end validation that the full login flow works with the Vault store plugin across both UIs. Ensure the generalized approach works correctly with a real store backend.

### Test Scenarios

1. **Vault + WebUI + userpass** -- Start Vault dev server, configure workspace with Vault store, launch WebUI, login with userpass, save/load tree.
2. **Vault + TUI + userpass** -- Same but with TUI.
3. **Vault + WebUI + ldap** -- If LDAP backend is available (can use Vault dev mode with LDAP mock).
4. **No-auth store** -- Use a store plugin that returns `Auth: false` (e.g., memory store). Verify login is skipped entirely.
5. **Mid-session expiry** -- Login, then revoke the Vault token externally. Verify the UI handles the auth error and prompts for re-login.
6. **Token with expiry** -- Login with a short-TTL token. Verify the session manager detects expiry.

### Files to Create/Modify

- **`test/integration/login_test.go`** (or similar) -- Integration tests that spin up a Vault dev server and test the full flow programmatically through the controller layer.
- Extend example plugin tests if applicable.

### Vault-Specific Validation

Verify that the Vault store's existing `Login()` method works correctly through the new controller API for:
- `userpass` -- username + password
- `ldap` -- username + password (same field shape, different Vault backend)
- `kubernetes` -- role + jwt
- `token` -- direct token auth
- `approle` -- role_id + secret_id

The `oidc` method requires browser-based interactive flow and is handled separately in [Phase 6](phase6-browser-auth.md).

## Acceptance Criteria

- All integration test scenarios pass.
- `make check` passes.
- No regressions in existing tests.
- Error messages from Vault are surfaced clearly in both UIs.

## Dependencies

- Phases 1-4 must be complete.
