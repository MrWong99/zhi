---
title: "feat: Codebase Improvements — Deferred Architecture Tasks"
type: feat
status: completed
date: 2026-02-21
---

# Codebase Improvements — Deferred Architecture Tasks

## Overview

Three architecture tasks deferred from the main codebase improvements PR (`feat/codebase-improvements`). These are independent refactoring efforts that can each be a separate PR.

## Completed in prior PR

All 6 bug fixes (BUG-1 through BUG-6), 2 plugin promotions (jsonfile, httpapi), `config.Tree` mutex, structured error types, Yaegi security docs, and signature verification TODOs.

---

## Task 1: Extract shared Vault HTTP client

- **Duplicate code:** `pkg/providers/store/vault/client.go` and `examples/zhi-store-vault-manager/adminclient.go` share ~150 lines of HTTP client logic.
- **Target:** `pkg/providers/store/vault/httpclient/` (internal to vault packages)
- **Shared surface:** Token management (`sync.RWMutex`), JSON request/response, namespace header, error parsing
- **Differences to handle:** `WrapInfo` support (admin client only), extra headers (`X-Vault-Wrap-TTL`), response types
- **Design:** Shared base client with configurable response unmarshaling. Both `vault.client` and `vaultmanager.adminClient` wrap the shared client and add their specific response handling.

**Acceptance Criteria:**
- [x] `pkg/providers/store/vault/httpclient/` package exists
- [x] `vault.client` refactored to use shared client
- [x] `vaultmanager.adminClient` refactored to use shared client
- [x] All existing vault and vault-manager tests pass
- [x] `make check` passes

---

## Task 2: Promote zhi-store-vault-manager to `pkg/providers/store/vaultmanager`

Depends on: Task 1 (shared HTTP client extracted).

- **Current:** `examples/zhi-store-vault-manager/` — 800+ lines across 12 files, 29 tests
- **Steps:**
  1. Create `pkg/providers/store/vaultmanager/` package
  2. Extract all types as exported: `Store`, `CredManager`, `AdminClient`, `Config`
  3. Refactor `adminClient` to use shared Vault HTTP client
  4. Move tests, update mock Vault servers
  5. Keep thin wrapper in `examples/zhi-store-vault-manager/main.go`
  6. Do NOT register as built-in default — meta-plugins launch child binaries, which is fundamentally an external plugin pattern
  7. Update `zhi-plugin.yaml` manifest paths if needed

**Acceptance Criteria:**
- [x] `pkg/providers/store/vaultmanager/` package exists
- [x] Uses shared Vault HTTP client
- [x] All 29 tests pass in new location
- [x] Non-wrapped AppRole path works (BUG-3 fix verified)
- [x] No dead code remains
- [x] Example wrapper compiles and works
- [x] `make build-examples` succeeds
- [x] `make test` passes

---

## Task 3: Reduce registry launch boilerplate with generics

- **File:** `internal/core/registry.go:282-404`
- **Problem:** Four `launchExternal{Config,Transform,Store,UI}` methods are nearly identical (~100 lines duplicated).
- **Fix:** Extract a single generic function:
  ```go
  func launchExternal[P any](r *Registry, cache map[string]P, name string, pluginType PluginType, launcher func(string, map[string]any) (P, func(), error), opts map[string]any) (P, error)
  ```
- **Constraint:** The `launcher` parameter abstracts the type-specific `Launch*` call.

**Acceptance Criteria:**
- [x] Single generic `launchExternal[P]` function replaces 4 methods
- [x] All existing registry tests pass
- [x] `make check` passes

---

## Dependency Graph

```
Task 1: Extract shared Vault HTTP client
         │
Task 2: Promote vault-manager (depends on Task 1)

Task 3: Registry generics (independent)
```

Tasks 1+2 are ordered. Task 3 is independent and can be done in parallel.

## References

- Production provider pattern: `pkg/providers/store/vault/` (directory layout, constructor, tests)
- Example wrapper pattern: `examples/zhi-store-vault/main.go` (imports from pkg/providers/ and serves)
- Registry launch boilerplate: `internal/core/registry.go:282-404`
- Plugin release pipeline: `.github/workflows/release-plugins.yml`
