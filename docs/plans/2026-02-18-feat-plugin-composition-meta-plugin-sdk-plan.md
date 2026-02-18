---
title: "feat: Plugin Composition via Meta-Plugin + SDK Helpers"
type: feat
status: in-progress
date: 2026-02-18
deepened: 2026-02-18
---

# Plugin Composition via Meta-Plugin + SDK Helpers

## Enhancement Summary

**Deepened on:** 2026-02-18
**Research agents used:** best-practices-researcher, framework-docs-researcher,
architecture-strategist, security-sentinel, performance-oracle,
pattern-recognition-specialist, code-simplicity-reviewer, spec-flow-analyzer

### Key Improvements

1. **Scope reduction** — removed FailoverPlugin, InterceptPlugin, and
   OverlayPlugin (YAGNI). Deferred discovery extraction. Reduced from 7
   phases to 5.
2. **Security hardening** — removed `WithAudit(false)`, added binary path
   validation, environment isolation, and hard-fail integrity checks.
3. **Performance** — MirroredPlugin writes parallelized, MergedPlugin
   List() parallelized, circuit-breaker pattern documented for future use.
4. **Architecture** — nil-Base protection on DelegatingPlugin, compile-time
   interface assertions, `%w` error wrapping, field naming fix
   (`MountedPlugin.Impl`).
5. **Spec gap coverage** — options forwarding contract, crash recovery
   semantics, child process cleanup via `Setpgid`, logging bridge.

### Considerations Discovered

- Nested go-plugin hosting (plugin as both server and client) is fully
  supported — stdio pipes are independent per child process.
- Signal propagation to children requires `Setpgid: true` on the child
  `exec.Cmd` to prevent signals meant for the parent from killing children.
- UI plugin meta-plugins are out of scope due to gRPC broker complexity
  (bidirectional Controller callbacks).
- The existing `versionedTreePlugin` in the test suite already validates
  the DelegatingPlugin embedding pattern.

## Overview

Enable plugin developers to compose existing zhi plugins into new, more
specialized plugins without rewriting them from scratch — even when the
component plugins are written in different languages. This plan focuses on
two complementary pieces:

1. **SDK Composition Helpers** — library code in `pkg/` that eliminates
   boilerplate when wrapping or merging plugins.
2. **Meta-Plugin Pattern** — documented pattern + examples for building
   composition plugins that launch and orchestrate child plugins as
   sub-processes.

The SDK helpers are the foundation. The meta-plugin pattern is the
documented usage of those helpers to build distributable composition
plugins.

## Motivating Examples

### Example 1: Extended Vault Store

A store plugin wraps `zhi-store-vault` but adds automatic Vault ACL policy
and AppRole credential management. Applications deployed via this zhi stack
use those credentials to access secrets.

### Example 2: Merged Config Plugins

Plugin A provides config for application A, plugin B for application B.
Plugin C combines both under distinct path prefixes so the user sees one
unified tree.

### Example 3: Store with Backup

A store writes to a primary Vault store and mirrors every write to a
secondary JSON file store as a backup.

### Example 4: Config with Secret Injection

A config plugin reads its base tree from `structuredfile` but overlays
secret values from a second config plugin that fetches them from an
external secrets manager at runtime.

## Architecture

```
Engine  ──gRPC──>  Meta-Plugin (e.g., vault-managed-store)
                      │
                      ├──launches──>  Child Plugin A (zhi-store-vault)
                      │                    └── gRPC (stdio)
                      │
                      └──launches──>  Child Plugin B (custom-policy-mgr)
                                           └── gRPC (stdio)
```

The meta-plugin:
1. Receives calls from the engine via the normal gRPC interface.
2. Launches child plugins as sub-processes using `pkg/zhiplugin/launch`.
3. Delegates calls to the appropriate child via SDK helpers.
4. Adds its own logic before/after delegation.
5. Returns results to the engine.

The engine sees a standard plugin — no engine changes required.

### Research Insights: Nested go-plugin Hosting

The hashicorp/go-plugin library fully supports a process acting as both a
gRPC server (for its parent) and a gRPC client (for its children). Key
findings from framework documentation research:

- **Stdio pipes are independent per child process.** The meta-plugin's own
  stdin/stdout (used for its gRPC connection to the engine) do not conflict
  with children's stdio pipes. Each `exec.Command` gets its own pipe pair.
- **Magic cookie environment inheritance works correctly.** The handshake
  (`ZHI_PLUGIN=zhiplugin-v1`) set by the engine is inherited by children
  via the parent's environment. Children validate the same cookie.
- **Signal propagation concern:** By default, signals sent to the parent
  process group also reach children. Use `Setpgid: true` on child
  `exec.Cmd.SysProcAttr` to put children in a separate process group,
  so the meta-plugin controls their lifecycle explicitly.
- **Orphaned child detection:** If the meta-plugin crashes, children detect
  parent death via stdin pipe closure (go-plugin built-in behavior) and
  self-terminate.
- **UI meta-plugins are out of scope** for this plan. The UI plugin uses
  bidirectional gRPC via the go-plugin broker (Controller callbacks),
  which adds complexity that does not justify the effort for composition.

## Package Design

### Current state

The following functionality lives in `internal/core/` and is inaccessible
to external plugins:

| Internal symbol | Purpose |
|---|---|
| `LaunchConfig/Transform/Store/UI()` | Start a child plugin process |
| `auditPluginBinary()` | SHA-256 audit + permission checks |
| `Discover()`, `PluginInfo`, `DiscoveryConfig` | Find plugins on disk |
| `DefaultPluginDir()` | `~/.zhi/plugins/` |

### Proposed public packages

Move the launcher to a public package so meta-plugins can reuse it. The
security audit code comes along — meta-plugins launching children should
use the same binary integrity checks.

Discovery extraction is **deferred** (see "Deferred Work" section). The
meta-plugin pattern does not require a public discovery API in Phase 1 —
meta-plugins know their child binaries from configuration, not from
scanning the filesystem.

#### `pkg/zhiplugin/launch` (new)

Extracted from `internal/core/launcher.go`. Provides typed launcher
functions:

```go
package launch

// LaunchConfig launches a config plugin binary and returns the plugin
// interface and a cleanup function that kills the child process.
func LaunchConfig(binary string, opts ...Option) (config.Plugin, func(), error)
func LaunchTransform(binary string, opts ...Option) (transform.Plugin, func(), error)
func LaunchStore(binary string, opts ...Option) (store.Plugin, func(), error)

// Option configures plugin launching.
type Option func(*launchConfig)

// WithLogger bridges the child's hclog output to the parent's logger.
// Uses hclog.NewInterceptLogger to translate between go-plugin's hclog
// and the meta-plugin's structured logger (slog).
func WithLogger(l *slog.Logger) Option

// WithIsolatedEnv restricts the child process environment to only the
// variables needed for go-plugin handshake and user-specified extras.
// Prevents leaking parent secrets or credentials to children.
func WithIsolatedEnv(env map[string]string) Option
```

`internal/core/launcher.go` becomes a thin wrapper calling into this
package, so existing engine behavior is preserved.

**Security hardening (from security review):**

- **No `WithAudit(false)`** — integrity verification is always on when a
  digest store is available. Allowing callers to disable tamper detection
  is a security anti-pattern.
- **Binary path validation** — `LaunchConfig/Store/Transform` resolve
  symlinks via `filepath.EvalSymlinks` and reject paths containing `..`
  or pointing outside `DefaultPluginDir()` and the current workspace.
- **Integrity check is a hard failure** — if binary verification fails,
  launch returns an error (not a warning log). The `auditPluginBinary`
  function uses `return err` rather than `log.Warn`.
- **Environment isolation** — `WithIsolatedEnv` constructs a minimal
  `exec.Cmd.Env` containing only `ZHI_PLUGIN`, `PATH`, and user-specified
  vars. Without this option, children inherit the parent's full env
  (current behavior, preserved for backward compat).

**Child lifecycle management (from framework research):**

- Children are launched with `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}`
  to isolate them in a separate process group.
- The cleanup function calls `client.Kill()` which sends SIGTERM then
  SIGKILL after a timeout (go-plugin built-in).
- If the meta-plugin crashes, children detect stdin closure and
  self-terminate (go-plugin built-in).

**Note:** `LaunchUI` is omitted. UI meta-plugins require the go-plugin
broker for bidirectional Controller callbacks, which adds complexity
beyond the scope of this plan.

#### SDK composition helpers in existing `pkg/zhiplugin/{config,store,transform}` packages

These are new files added to the existing plugin-type packages:

- `pkg/zhiplugin/config/delegate.go` — `DelegatingPlugin`
- `pkg/zhiplugin/config/compose.go` — `MergedPlugin`
- `pkg/zhiplugin/store/delegate.go` — `DelegatingPlugin`
- `pkg/zhiplugin/store/compose.go` — `MirroredPlugin`
- `pkg/zhiplugin/transform/delegate.go` — `DelegatingPlugin`

No new top-level packages needed for these — they belong with their
respective plugin interfaces.

**Removed from scope** (YAGNI — no motivating example in practice):
- `OverlayPlugin` — unresolved `Set()` routing semantics (which layer
  receives writes for overlapping paths?). Can be added later.
- `FailoverPlugin` — no current user has requested failover. If needed
  later, a circuit-breaker pattern is more appropriate than naive retry.
- `InterceptPlugin` — `DelegatingPlugin` with method overrides already
  covers this use case. A separate hook-based API adds complexity without
  clear benefit over embedding + override.

## SDK Helpers: Detailed Design

### 1. Delegating Base Types

Each plugin type gets a `DelegatingPlugin` struct that implements the full
interface by forwarding every call to an embedded `Base` field. Plugin
developers embed it and override only the methods they need.

**Design decisions (from architecture and pattern reviews):**

- Each `DelegatingPlugin` includes a compile-time interface assertion:
  `var _ Plugin = (*DelegatingPlugin)(nil)` to catch method signature
  drift at compile time rather than runtime.
- All delegated methods that return errors wrap them with `%w` for
  compatibility with `store.Is*` error helpers (e.g.,
  `fmt.Errorf("delegated: %w", err)`).
- The `Base` field is validated at construction time. A `NewDelegatingPlugin`
  constructor panics if `Base` is nil, preventing nil-pointer panics deep
  in delegation chains at runtime.
- The existing `versionedTreePlugin` in the test suite already validates
  this embedding + override pattern in practice.

**Config** (`pkg/zhiplugin/config/delegate.go`):

```go
var _ Plugin = (*DelegatingPlugin)(nil)

// DelegatingPlugin forwards all config.Plugin calls to Base.
// Embed this and override individual methods.
type DelegatingPlugin struct {
    Base Plugin
}

// NewDelegatingPlugin creates a DelegatingPlugin. Panics if base is nil.
func NewDelegatingPlugin(base Plugin) DelegatingPlugin {
    if base == nil {
        panic("config.NewDelegatingPlugin: base must not be nil")
    }
    return DelegatingPlugin{Base: base}
}

func (d *DelegatingPlugin) List(ctx context.Context) ([]string, error) {
    return d.Base.List(ctx)
}
func (d *DelegatingPlugin) Get(ctx context.Context, path string) (Value, bool, error) {
    return d.Base.Get(ctx, path)
}
func (d *DelegatingPlugin) Set(ctx context.Context, path string, v Value) error {
    return d.Base.Set(ctx, path, v)
}
func (d *DelegatingPlugin) Validate(ctx context.Context, path string, tree TreeReader) ([]ValidationResult, error) {
    return d.Base.Validate(ctx, path, tree)
}
```

**Store** (`pkg/zhiplugin/store/delegate.go`):

```go
var _ Plugin = (*DelegatingPlugin)(nil)

// DelegatingPlugin forwards all store.Plugin calls to Base.
// Embed this and override individual methods.
type DelegatingPlugin struct {
    Base Plugin
}

// NewDelegatingPlugin creates a DelegatingPlugin. Panics if base is nil.
func NewDelegatingPlugin(base Plugin) DelegatingPlugin {
    if base == nil {
        panic("store.NewDelegatingPlugin: base must not be nil")
    }
    return DelegatingPlugin{Base: base}
}

func (d *DelegatingPlugin) Capabilities(ctx context.Context) (*Capabilities, error) {
    return d.Base.Capabilities(ctx)
}
func (d *DelegatingPlugin) AuthMethods(ctx context.Context) ([]AuthMethod, error) {
    return d.Base.AuthMethods(ctx)
}
// ... all 27 methods delegate to d.Base ...
```

**Transform** (`pkg/zhiplugin/transform/delegate.go`):

```go
var _ Plugin = (*DelegatingPlugin)(nil)

type DelegatingPlugin struct {
    Base Plugin
}

func NewDelegatingPlugin(base Plugin) DelegatingPlugin {
    if base == nil {
        panic("transform.NewDelegatingPlugin: base must not be nil")
    }
    return DelegatingPlugin{Base: base}
}

func (d *DelegatingPlugin) BeforeDisplay(ctx context.Context, tree *config.Tree) error {
    return d.Base.BeforeDisplay(ctx, tree)
}
func (d *DelegatingPlugin) AfterSave(ctx context.Context, tree *config.Tree) error {
    return d.Base.AfterSave(ctx, tree)
}
func (d *DelegatingPlugin) ValidatePolicy(ctx context.Context) (ValidatePolicy, error) {
    return d.Base.ValidatePolicy(ctx)
}
```

**Usage:**

```go
type vaultManagedStore struct {
    store.DelegatingPlugin
}

func newVaultManagedStore(vault store.Plugin) *vaultManagedStore {
    return &vaultManagedStore{
        DelegatingPlugin: store.NewDelegatingPlugin(vault),
    }
}

// Override only PutValues and GrantAccess:
func (s *vaultManagedStore) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
    if err := s.Base.PutValues(ctx, id, values, opts); err != nil {
        return fmt.Errorf("vault put: %w", err)
    }
    return s.syncPolicies(ctx, id, values)
}
```

### 2. Config Composition Helpers

`pkg/zhiplugin/config/compose.go`:

```go
// MountedPlugin associates a config plugin with a path prefix.
type MountedPlugin struct {
    Impl   Plugin // the underlying config plugin
    Prefix string // e.g., "app-a/"
}

// MergedPlugin combines multiple config plugins under distinct path
// prefixes. Each child's paths are prefixed with its mount point.
// Returns an error if any prefixes overlap.
func MergedPlugin(children ...MountedPlugin) (Plugin, error)
```

**Design notes (from pattern and performance reviews):**

- The field is named `Impl` (not `Plugin`) to avoid confusion with the
  `Plugin` interface type in the same package.
- `MergedPlugin` returns an error (not just `Plugin`) so that prefix
  overlap validation can fail at construction time rather than at runtime.
- `List()` calls each child's `List()` in parallel using an `errgroup`,
  then prefixes the results. This matters when children are remote plugin
  processes where each `List()` incurs a gRPC round-trip.
- `Validate()` constructs a filtered `TreeReader` scoped to the child's
  prefix before delegating, so children see only their own subtree.

### 3. Store Composition Helpers

`pkg/zhiplugin/store/compose.go`:

```go
// MirroredPlugin writes to all stores but reads from the primary (first).
// Mirror write errors are logged but do not fail the operation unless the
// primary write fails.
func MirroredPlugin(primary Plugin, mirrors ...Plugin) Plugin
```

**Performance (from performance review):**

- Mirror writes are dispatched **in parallel** using an `errgroup`. The
  primary write completes first; mirror writes happen concurrently after
  the primary succeeds. This prevents write latency from scaling linearly
  with the number of mirrors.
- Read operations (`GetValues`, `ListTrees`, versioning reads) delegate
  to the primary only. No reads hit mirrors.
- `Capabilities()` returns the **intersection** of all stores'
  capabilities (the most restrictive). A mirror that doesn't support
  encryption means the composed store doesn't advertise encryption.
- Auth operations delegate to the primary only.

**Error handling:**

- Primary write failure returns the error immediately — mirrors are not
  attempted.
- Mirror write failures are collected and logged (using the logger from
  `WithLogger` if provided). They do not fail the overall operation.
- All errors are wrapped with `%w` for compatibility with `store.Is*`
  helpers.

## Implementation Roadmap

### Phase 1: Extract Launcher to `pkg/zhiplugin/launch`

**Goal:** Make plugin launching available to external code so meta-plugins
can start child processes.

**Steps:**

1. Create `pkg/zhiplugin/launch/launch.go` with the typed launcher
   functions (`LaunchConfig`, `LaunchStore`, `LaunchTransform`).
2. Move `auditPluginBinary()` into `pkg/zhiplugin/launch/audit.go` as an
   unexported helper. Harden it: resolve symlinks, reject `..` in paths,
   make verification failure a hard error (not a log warning).
3. Add `Option` type with `WithLogger(*slog.Logger)` and
   `WithIsolatedEnv(map[string]string)`.
4. Set `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` on all child
   `exec.Cmd` to isolate children in a separate process group.
5. Update `internal/core/launcher.go` to call `pkg/zhiplugin/launch`
   instead of containing its own implementation. Keep the internal
   functions as thin wrappers for backward compatibility.
6. Write unit tests for the launch package (mock plugin binaries using
   test helpers or the existing example plugins in `bin/examples/`).

**Files changed:**
- `pkg/zhiplugin/launch/launch.go` (new)
- `pkg/zhiplugin/launch/audit.go` (new)
- `pkg/zhiplugin/launch/options.go` (new)
- `pkg/zhiplugin/launch/launch_test.go` (new)
- `internal/core/launcher.go` (refactored to delegate)
- `internal/core/launcher_test.go` (updated imports if needed)

**Depends on:** Nothing.

### Phase 2: Delegating Base Types

**Goal:** Eliminate boilerplate for meta-plugins that wrap a single child.

**Steps:**

1. Create `pkg/zhiplugin/config/delegate.go` with `DelegatingPlugin`
   implementing all 4 `config.Plugin` methods via `Base`.
   - Add `var _ Plugin = (*DelegatingPlugin)(nil)` compile-time assertion.
   - Add `NewDelegatingPlugin(base)` constructor that panics on nil.
2. Create `pkg/zhiplugin/transform/delegate.go` with `DelegatingPlugin`
   implementing all 3 `transform.Plugin` methods via `Base`.
   - Same assertions and constructor.
3. Create `pkg/zhiplugin/store/delegate.go` with `DelegatingPlugin`
   implementing all 27 `store.Plugin` methods via `Base`.
   - Same assertions and constructor.
   - All error returns wrap with `%w` for `store.Is*` compatibility.
4. Write comprehensive tests for each delegate type: verify every method
   forwards correctly to a mock base, verify that embedding + overriding
   a single method works as expected, verify nil base panics.

**Files changed:**
- `pkg/zhiplugin/config/delegate.go` (new)
- `pkg/zhiplugin/config/delegate_test.go` (new)
- `pkg/zhiplugin/transform/delegate.go` (new)
- `pkg/zhiplugin/transform/delegate_test.go` (new)
- `pkg/zhiplugin/store/delegate.go` (new)
- `pkg/zhiplugin/store/delegate_test.go` (new)

**Depends on:** Nothing (can run in parallel with Phase 1).

### Phase 3: Composition Helpers

**Goal:** Provide `MergedPlugin` (config) and `MirroredPlugin` (store)
as ready-to-use building blocks.

**Steps:**

1. Create `pkg/zhiplugin/config/compose.go` with `MountedPlugin` and
   `MergedPlugin()`.
2. `MergedPlugin` implementation:
   - Constructor validates prefix uniqueness, returns error on overlap.
   - `List()` calls each child in parallel (`errgroup`), prefixes results.
   - `Get()` routes by prefix to the correct child.
   - `Set()` routes by prefix to the correct child.
   - `Validate()` routes by prefix, constructs a filtered `TreeReader`
     scoped to the child's subtree.
3. Create `pkg/zhiplugin/store/compose.go` with `MirroredPlugin()`.
4. `MirroredPlugin` implementation:
   - Read operations delegate to primary only.
   - Write operations write to primary first, then dispatch mirror writes
     **in parallel** via `errgroup`. Mirror errors are logged, not fatal.
   - `Capabilities()` returns intersection of all stores' capabilities.
   - Auth operations delegate to primary only.
5. Write tests for both helpers with mock plugins. Edge cases: empty
   prefix, path not matching any child, mirror write failure, primary
   write failure.

**Files changed:**
- `pkg/zhiplugin/config/compose.go` (new)
- `pkg/zhiplugin/config/compose_test.go` (new)
- `pkg/zhiplugin/store/compose.go` (new)
- `pkg/zhiplugin/store/compose_test.go` (new)

**Depends on:** Phase 2 (MirroredPlugin uses `DelegatingPlugin` for
read-only delegation to the primary).

### Phase 4: Example Meta-Plugin

**Goal:** Demonstrate the full pattern with a working, distributable
example.

**Steps:**

1. Create `examples/meta-store-mirror/` — a meta-plugin that launches
   two store children (memory + JSON file) and mirrors writes.
   - Uses `launch.LaunchStore()` to start children.
   - Uses `store.MirroredPlugin()` to compose them.
   - `main()` is ~20 lines of code.
2. Add `zhi-plugin.yaml` manifest with dependencies declared.
3. Add the example to `Makefile` build targets (`build-examples`).
4. Write an integration test that builds and runs the meta-plugin end to
   end: start meta-plugin, write values, verify both children received
   the data, read values back from primary.

**Files changed:**
- `examples/meta-store-mirror/main.go` (new)
- `examples/meta-store-mirror/zhi-plugin.yaml` (new)
- `Makefile` (add to `build-examples`)

**Depends on:** Phases 1, 2, 3.

### Phase 5: Documentation

**Goal:** Document the composition pattern and SDK helpers for plugin
developers.

**Steps:**

1. Create `docs/plugin-development/composition.md` — comprehensive guide
   covering:
   - When to use composition vs. writing a plugin from scratch.
   - The `DelegatingPlugin` pattern with examples.
   - Config helpers: `MergedPlugin`.
   - Store helpers: `MirroredPlugin`.
   - The `launch` package: starting child plugins from a meta-plugin.
   - Full walkthrough: building the mirror-store example.
   - Options forwarding: how the meta-plugin passes config to children
     (environment variables or temp config files, not the launch package).
   - Crash recovery: what happens when a child dies (stdin closure
     detection, meta-plugin responsibility to restart or fail).
2. Update `docs/plugin-development/overview.md` to reference composition.
3. Update `docs/user-guide/workspace-configuration.md` to document how
   `options` in `zhi.yaml` are passed to meta-plugins (and how
   meta-plugins forward options to children).

**Files changed:**
- `docs/plugin-development/composition.md` (new)
- `docs/plugin-development/overview.md` (updated)
- `docs/user-guide/workspace-configuration.md` (updated)

**Depends on:** Phases 1-4 (documents the completed implementation).

## Acceptance Criteria

### Functional Requirements

- [x] `pkg/zhiplugin/launch` can start config, transform, and store
      plugins and return the typed interface + cleanup function
- [x] Launch package validates binary paths (no `..`, resolves symlinks)
- [x] Launch package runs integrity verification as a hard failure
- [x] `DelegatingPlugin` exists for config, transform, and store;
      embedding + override works correctly
- [x] `NewDelegatingPlugin(nil)` panics for all three types
- [x] `MergedPlugin` correctly prefix-mounts multiple config plugins
- [x] `MergedPlugin` rejects overlapping prefixes at construction
- [x] `MirroredPlugin` writes to all stores in parallel, reads from primary
- [x] Mirror write failures are logged but do not fail the operation
- [x] Example meta-plugin builds, runs, and passes integration tests
- [x] Existing engine behavior is unchanged (internal wrappers delegate
      to new public packages)

### Non-Functional Requirements

- [x] No engine changes — meta-plugins are standard plugins
- [x] All new public API has GoDoc comments
- [x] All new code has unit tests with race detection (`-race`)
- [x] `make check` passes (fmt, vet, lint, test)
- [x] Child processes are launched in separate process groups (`Setpgid`)
- [x] Compile-time interface assertions on all `DelegatingPlugin` types

### Quality Gates

- [ ] Test coverage for new packages ≥ 80%
- [x] `make lint` clean
- [x] Example meta-plugin builds with `make build-examples`
- [ ] Integration test demonstrates end-to-end meta-plugin lifecycle

## Resolved Questions

1. **Should `InterceptPlugin` use typed hooks or `[]any`?**
   **Resolved: removed from scope.** `DelegatingPlugin` with method
   overrides already covers the interception use case. No separate
   hook-based API needed.

2. **Should `MirroredPlugin` mirror errors be fatal or logged?**
   **Resolved: log-and-continue.** Mirror write failures are logged but
   do not fail the operation. A strict mode can be added later if needed.

3. **Should the launch package support passing plugin options?**
   **Resolved: not the launch package's responsibility.** The meta-plugin
   forwards options to children via environment variables or temp config
   files. The launch package only starts binaries. Document this in Phase 5.

4. **Should `DelegatingPlugin` be generated from the interface?**
   **Resolved: write by hand initially.** Add `go generate` only if
   interface churn becomes a problem. The store plugin interface has been
   stable for months.

## Open Questions

1. **Should `MergedPlugin.Validate()` aggregate results from all children
   or only route to the prefix-matched child?**
   Recommendation: route to the matched child only. Cross-child validation
   (e.g., "app-a's database host must match app-b's") is the meta-plugin's
   responsibility, not MergedPlugin's.

2. **How should the meta-plugin detect and handle child process crashes
   mid-operation?**
   The go-plugin client returns an error on the next RPC call after a child
   dies. The meta-plugin can either: (a) propagate the error to the engine,
   or (b) attempt to relaunch the child and retry. Document both strategies
   in Phase 5; recommend (a) for simplicity, with (b) as an advanced
   pattern.

## Deferred Work

These items are intentionally out of scope for this plan. They can be
added in future iterations if demand arises.

| Item | Reason for deferral |
|------|---------------------|
| `pkg/zhiplugin/discovery` extraction | Meta-plugins know their children from config, not filesystem scanning. Discovery can stay internal. |
| `OverlayPlugin` (config) | Unresolved `Set()` routing semantics for overlapping paths. |
| `FailoverPlugin` (store) | No motivating example. Circuit-breaker pattern is more appropriate if needed later. |
| `InterceptPlugin` (store) | `DelegatingPlugin` + method override already covers this. |
| `LaunchUI` | UI plugins use bidirectional gRPC broker; composition adds broker-forwarding complexity. |
| Multi-language SDK (Python/Rust) | The Go SDK is the priority. Other languages can follow the same patterns later. |
| Manifest `dependencies` field | Plugin manifests don't yet have a `dependencies` field in the struct. Add when meta-plugin distribution through the marketplace is implemented. |
| Circular composition detection | A meta-plugin that launches itself would infinite-loop. Detect via an environment variable depth counter. Low priority — plugin developers control their own composition. |

## References

- Current launcher: `internal/core/launcher.go`
- Current discovery: `internal/core/discovery.go`
- Plugin interfaces: `pkg/zhiplugin/{config,transform,store,ui}/plugin.go`
- Plugin handshake: `pkg/zhiplugin/plugin.go`
- Store errors: `pkg/zhiplugin/store/errors.go`
- Existing example plugins: `examples/`
- Existing delegation pattern: `versionedTreePlugin` in store tests
- Manifest format: `pkg/sharing/manifest/`
- hashicorp/go-plugin: `github.com/hashicorp/go-plugin` v1.7.0
