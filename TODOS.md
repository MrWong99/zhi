# Future Work for Store Plugin Ecosystem

This file tracks remaining work needed after the store API restructure.
The core API (`store.Plugin` interface, proto definitions, gRPC client/server,
and engine integration) has been implemented and all tests pass.

## Vault Store Plugin Implementation

### Authentication & Session Management

- [ ] Implement `AuthMethods` returning Vault auth backends (token, userpass,
  OIDC, LDAP, AppRole, Kubernetes, etc.)
- [ ] Implement `Login` that calls `vault.Auth().Login()` and stores the
  resulting client token internally for subsequent API calls
- [ ] Handle token renewal / lease management in a background goroutine
- [ ] Support Vault namespace isolation if applicable

### Value Operations (KV v2)

- [ ] Implement `GetValues` using per-path `vault.KVv2().Get()` calls
  (one Vault secret per tree path, e.g. `secret/data/zhi/<id>/<path>`)
- [ ] Implement `PutValues` using per-path `vault.KVv2().Put()` calls
- [ ] Implement `DeleteValues` using per-path `vault.KVv2().Delete()` (soft
  delete) or `vault.KVv2().DeleteMetadata()` (permanent)
- [ ] Implement `DeleteTree` that lists and removes all secrets under the
  tree prefix

### Check-and-Set (CAS)

- [ ] Wire `PutOptions.CASVersions` to Vault KV v2 CAS: each per-path write
  should include `cas=<expected_version>` to prevent lost updates
- [x] Surface Vault 412 (Precondition Failed) errors as a structured CAS
  conflict error type that the engine/UI can present to the user

### Value-Level Versioning

- [ ] Implement `ListValueVersions` using `vault.KVv2().GetVersionsAsList()`
- [ ] Implement `GetValueVersion` using `vault.KVv2().GetVersion()`
- [ ] Implement `RollbackValue` by reading the target version and writing it
  as a new version
- [ ] Implement `DeleteValueVersion` using `vault.KVv2().DeleteVersions()`

### Encryption

- [ ] Vault handles encryption transparently via its barrier; `InitEncryption`
  and `RotateEncryption` may map to Vault Transit or be no-ops
- [ ] Consider whether encryption passphrase maps to Vault unseal keys or
  Transit key rotation

### Access Control & Policies

- [ ] Implement `GrantAccess` by writing/updating Vault ACL policies that
  grant per-path capabilities (read, create, update, delete)
- [ ] Implement `RevokeAccess` by removing path rules from policies
- [ ] Implement `ListAccess` by reading policies and mapping them back to
  `Permission` structs
- [ ] Auto-generate policy names following a convention (e.g.
  `zhi-<tree_id>-<user>`)
- [ ] Consider Vault identity entities/groups for user representation

### Tree Listing / Discovery

- [ ] Implement `ListTrees` by listing secret mount prefixes or a dedicated
  metadata path; Vault LIST on `secret/metadata/zhi/` returns tree IDs

## Engine & UI Integration

- [x] Add engine methods for tree-level and value-level version operations
  (currently only `SaveTree`/`LoadStoredTree`/`ListTrees` are wired)
- [ ] Expose Capabilities in the UI so users see versioning/encryption/auth
  status
- [ ] Wire authentication flow into the UI (prompt for auth method + credentials
  before accessing a tree)
- [ ] Add CAS conflict resolution UI (show diff, let user choose)
- [ ] Wire version browsing and rollback into the TUI
- [ ] Wire access control management into the TUI (grant/revoke for admin users)

## Documentation

- [x] Update `docs/plugin-development/store-plugin.md` to reflect the new API
  (the current docs still reference the old `Save`/`Load`/`Delete`/
  `SupportsVersioning` interface)
- [ ] Add a Vault-specific plugin development guide with examples
- [x] Document the CAS workflow and error handling patterns
- [ ] Document auth method registration and credential flow

## Error Handling

- [x] Define structured error types for common store failures:
  - CAS conflict (version mismatch)
  - Authentication required / session expired
  - Access denied (insufficient permissions)
  - Path not found
  - Encryption not initialized
- [x] Ensure gRPC status codes map cleanly to these error types so the engine
  and UI can present meaningful feedback

## Testing

- [ ] Add integration tests for the Vault plugin against a real Vault dev
  server (use `t.Skip` for CI environments without Vault)
- [x] Add CAS conflict test cases in the store package test suite
- [x] Add value-level versioning test plugin and tests (analogous to the
  existing `versionedTreePlugin` for tree-level versioning)
- [ ] Add auth flow test cases (mock auth method, login, token expiry)
- [ ] Add access control test cases
