# 06 — Implementation Phases

This document breaks the WASM plugin support into concrete, shippable
phases. Each phase delivers usable functionality and can be released
independently.

## Phase 1: Foundation — Config Plugins

**Goal:** Ship the minimum viable WASM plugin support with config plugins
as the proving ground.

### Deliverables

1. **wazero dependency**
   - Add `github.com/tetratelabs/wazero` to `go.mod`.
   - Verify CI builds pass with `CGO_ENABLED=0` across all targets.
   - Confirm binary size increase is acceptable (~5.5 MB).

2. **Core WASM runtime (`pkg/zhiplugin/wasmloader/`)**
   - `runtime.go` — wazero runtime creation, compiled module cache.
   - `memory.go` — Shared memory protocol: `malloc`/`free` wrappers,
     `host_set_output`/`host_set_error` host functions, string/JSON
     read/write helpers.
   - `options.go` — `Option` type: `WithLogger`, `WithCapabilities`,
     `WithTimeout`.
   - `loader.go` — `LoadConfig()` function: compile module, instantiate
     host functions, instantiate WASI (if needed), instantiate module,
     return adapter.

3. **Config adapter (`pkg/zhiplugin/wasmloader/adapter_config.go`)**
   - Struct implementing `config.Plugin` by calling WASM exports
     (`zhi_config_list`, `zhi_config_get`, `zhi_config_set`,
     `zhi_config_validate`).
   - JSON serialization/deserialization matching the gRPC layer's
     `convert.go` patterns.

4. **Config host functions (`pkg/zhiplugin/wasmloader/host_config.go`)**
   - `zhi_v1` host module with `host_set_output`, `host_set_error`,
     `host_log`.
   - ABI version check (`zhi_abi_version` export).

5. **Discovery changes (`internal/core/discovery.go`)**
   - `PluginFormat` type and `PluginInfo.Format` field.
   - `.wasm` file detection in `discoverFlat()` and
     `discoverSubdirectory()`.
   - Native-over-WASM precedence with warning log.

6. **Registry changes (`internal/core/registry.go`)**
   - `launchExternalConfig` dispatches to `LoadWASMConfig` for WASM
     format.
   - `ProviderInfo.Source` shows "wasm" for WASM plugins.

7. **Go PDK — config (`pkg/zhiplugin/wasmpdk/`)**
   - `pdk.go` — Core ABI: `setOutput`, `setError`, `readString`, `Run`.
   - `config.go` — `ConfigPlugin` interface, `RegisterConfig`, WASM
     exports via `//go:wasmexport`.
   - `types.go` — `Value`, `TreeReader`, `ValidationResult`.

8. **Example: WASM config plugin**
   - Port the existing `examples/zhi-config-pokedex/` to WASM as
     `examples/zhi-config-pokedex-wasm/`.
   - Identical functionality, built with `GOOS=wasip1 GOARCH=wasm`.
   - Test that it works interchangeably with the native version.

9. **Tests**
   - Unit tests for the wasmloader package.
   - Unit tests for the PDK package (standard Go tests, not WASM).
   - Integration test: compile WASM example → load via wasmloader →
     exercise all 4 config methods.
   - Discovery test: mixed native + WASM plugins in a directory.

10. **Documentation**
    - Update `docs/plugin-development/` with a WASM plugin guide.
    - Update `docs/user-guide/workspace-configuration.md` with WASM
      capability syntax.

### What ships

Users can write config plugins in Go, compile to `.wasm`, drop into
`~/.zhi/plugins/`, and use them in workspaces. The engine treats them
identically to native config plugins.

---

## Phase 2: Transform Plugins

**Goal:** Extend WASM support to transform plugins.

### Deliverables

1. **Transform adapter (`pkg/zhiplugin/wasmloader/adapter_transform.go`)**
   - Implements `transform.Plugin` by calling `zhi_transform_before_display`,
     `zhi_transform_after_save`, `zhi_transform_validate_policy`.
   - Handles full-tree JSON serialization (input) and deserialization
     (output) for the mutable tree pattern.

2. **Transform PDK (`pkg/zhiplugin/wasmpdk/transform.go`)**
   - `TransformPlugin` interface, `RegisterTransform`.
   - `Tree` type with `List()`, `Get()`, `Set()`, `Delete()` for
     in-WASM tree manipulation.

3. **`LoadTransform` in wasmloader**
   - Registry integration (`launchExternalTransform` dispatch).

4. **Example: WASM transform plugin**
   - Simple example (e.g., base64 encode/decode, case transform).

5. **Tests**
   - Integration test: compile → load → before_display → verify tree
     mutation → after_save → verify.

### What ships

Users can write transform plugins in Go/WASM. Combined with Phase 1,
config + transform covers the "data pipeline" portion of zhi without
any process spawning.

---

## Phase 3: Security and Capabilities

**Goal:** Implement the capability-based security model.

### Deliverables

1. **Capability configuration**
   - Parse `wasm.capabilities` from `zhi.yaml` workspace config.
   - `Capabilities` struct in wasmloader options.
   - Default capabilities (clock, random: yes; fs, net, env: no).

2. **Filesystem host functions**
   - `host_fs_read`, `host_fs_write`, `host_fs_list`, `host_fs_delete`,
     `host_fs_mkdir`, `host_fs_stat`.
   - Path validation: resolve against allowed directories, reject
     traversal.
   - `FSConfig` integration with wazero's WASI module.

3. **HTTP host function**
   - `host_http_request` with URL allowlisting.
   - Timeout propagation from Go context.

4. **Environment variable scoping**
   - Only granted env vars passed to module config.

5. **Memory limits**
   - `WithMemoryLimitPages` on module config.

6. **Execution timeouts**
   - Context deadline propagation for each method call.

7. **PDK host call wrappers**
   - `wasmpdk.HostFS.Read()`, `.Write()`, etc.
   - `wasmpdk.HostHTTP.Do()`.

8. **Audit logging**
   - Log capability usage at DEBUG level.
   - Log denied capability access at WARN level.

9. **Tests**
   - Test: plugin with no capabilities cannot call fs/net host functions.
   - Test: plugin with scoped fs access can only read allowed paths.
   - Test: plugin with net access can only reach allowed hosts.
   - Test: memory limit enforcement.
   - Test: timeout enforcement.

### What ships

WASM plugins run in a fully sandboxed environment. Administrators can
grant precise capabilities. Denied access is logged and does not crash
the plugin.

---

## Phase 4: Store Plugins

**Goal:** Enable WASM store plugins using the host functions from Phase 3.

### Deliverables

1. **Store adapter (`pkg/zhiplugin/wasmloader/adapter_store.go`)**
   - Implements all 23 methods of `store.Plugin`.
   - Each method maps to a `zhi_store_*` WASM export.

2. **Store PDK (`pkg/zhiplugin/wasmpdk/store.go`)**
   - `StorePlugin` interface (mirrors `store.Plugin`).
   - `RegisterStore`, WASM exports for all 23 methods.
   - Types: `Capabilities`, `AuthMethod`, `Credential`, `PutOptions`,
     `Permission`, etc.

3. **`LoadStore` in wasmloader**
   - Registry integration.

4. **Example: WASM in-memory store**
   - Port `examples/zhi-store-memory/` to WASM.
   - Pure in-memory, no host function calls needed.

5. **Example: WASM file-based store**
   - New example that uses `host_fs_*` functions to persist JSON files.
   - Demonstrates capability grants in `zhi.yaml`.

6. **Tests**
   - Full store interface integration test.
   - Capability-gated filesystem access test.

### What ships

Users can write store plugins in WASM. In-memory stores work with zero
capabilities. File-based stores work with scoped filesystem grants.
Network-backed stores (e.g., Vault) work with HTTP grants.

---

## Phase 5: UI Plugins (Stretch Goal)

**Goal:** Enable WASM UI plugins for non-TTY UIs (HTTP-based).

### Deliverables

1. **Controller host functions**
   - All 25 `host_ctrl_*` host functions from the ABI spec.
   - Each calls through to the `UIController` on the host side.

2. **UI adapter (`pkg/zhiplugin/wasmloader/adapter_ui.go`)**
   - Implements `ui.Plugin` by calling `zhi_ui_run` and
     `zhi_ui_capabilities`.
   - `Capabilities` always returns `RequiresTTY: false`.

3. **Apply streaming (Option A: polled iteration)**
   - `host_ctrl_apply_start`, `host_ctrl_apply_next`,
     `host_ctrl_apply_result` host functions.

4. **UI PDK (`pkg/zhiplugin/wasmpdk/ui.go`)**
   - `UIPlugin` interface, `RegisterUI`.
   - `Controller` wrapper with methods for all host functions.

5. **HTTP listener host functions (optional)**
   - `host_http_listen`, `host_http_accept`, `host_http_respond`.
   - Enables WASM plugins to serve HTTP UIs.

6. **Example: WASM HTTP UI**
   - Port or adapt `examples/zhi-ui-http/` to WASM.

7. **Tests**
   - Integration test exercising the Controller callback chain.

### What ships

HTTP-based UI plugins can be written as WASM modules. TTY-based UIs
remain native-only. This completes WASM support for all four plugin
types.

---

## Phase 6: Distribution and Tooling

**Goal:** Polish the developer and user experience.

### Deliverables

1. **OCI distribution**
   - `zhi-plugin.yaml` supports `format: wasm` with `os: any, arch: any`.
   - `zhi plugin publish` handles WASM artifacts.
   - `zhi plugin install` detects `.wasm` and places correctly.

2. **Plugin scaffolding**
   - `zhi plugin new --format wasm` generates WASM plugin project.

3. **CLI display**
   - `zhi plugin list` shows format column (native/wasm).
   - `zhi plugin info` shows capabilities for WASM plugins.

4. **CI pipeline updates**
   - `release-plugins.yml` builds WASM examples alongside native.
   - WASM artifacts published as `os: any` OCI manifests.

5. **Makefile targets**
   - `make build-wasm-examples`
   - `make test-wasm` (runs WASM integration tests)

---

## Dependency Summary

| Phase | Depends On | New Dependencies |
|-------|------------|------------------|
| 1 | — | `github.com/tetratelabs/wazero` |
| 2 | Phase 1 | None |
| 3 | Phase 1 | None |
| 4 | Phase 1, 3 | None |
| 5 | Phase 1, 3, 4 | None |
| 6 | Phase 1–5 | None |

The only new external dependency is wazero, added in Phase 1.

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| wazero API changes | Low (v1.0+ stable) | Medium | Pin version, update incrementally |
| Go `wasip1` limitations | Medium | Medium | Test early; fall back to TinyGo for edge cases |
| Performance unacceptable for store ops | Low | Medium | Profile early; native plugins remain available |
| ABI design needs revision after Phase 1 | Medium | Low | Version the ABI (`zhi_v1`); Phase 1 is the proving ground |
| `//go:wasmexport` doesn't support needed signatures | Low | High | Test with Go 1.24+ early; use raw WASM exports as fallback |
| wazero filesystem sandbox escape | Known | Low | Don't rely on WASI fs; use custom host functions with path validation |
| Binary size concerns with Go gc WASM | Medium | Low | TinyGo option available; WASM is still smaller than native |

---

## Success Criteria

**Phase 1 is successful if:**
- A WASM config plugin passes all the same integration tests as its
  native equivalent.
- CI builds pass on all platforms (linux/darwin × amd64/arm64) with
  the wazero dependency.
- The WASM plugin is discoverable and usable without any workspace
  configuration changes.

**The full feature is successful if:**
- All four plugin types (config, transform, store, UI) can be
  implemented as WASM plugins.
- WASM plugins are demonstrably more secure than native plugins
  (capability enforcement, no arbitrary syscalls).
- Plugin authors can build and distribute WASM plugins using the PDK
  and OCI pipeline.
- Performance is acceptable for typical plugin workloads (config
  load/save, transform, store get/put).
