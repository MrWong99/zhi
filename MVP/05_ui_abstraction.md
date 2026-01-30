# Step 5: UI Abstraction Layer

## Overview

Define the **UI abstraction layer** that decouples the core engine from any specific user interface. This step introduces the `UIDriver` interface, the `UIController` API surface, and a driver registry — establishing the contract that all future frontends (TUI, Web UI, AI Chat, headless API) must implement. No concrete UI is built here; that happens in Step 6.

## Relevant Existing Files

- `internal/core/engine.go` — core engine (the UIController wraps this)
- `internal/core/component.go` — component manager (UIController exposes component operations)
- `internal/core/export.go` — export system (UIController delegates export calls)
- `internal/core/apply.go` — apply runner (UIController manages the output channel)
- `internal/cli/root.go` — CLI root (the `zhi edit` command will resolve and launch a UIDriver)
- `pkg/zhiplugin/config/config.go` — `Tree`, `Value`, `ValidationResult`, `Severity` types
- `pkg/ui/.keep` — placeholder directory (superseded by `internal/ui/`)

## Implementation Plan

### 5.1 UIDriver Interface (`internal/ui/driver.go`)

Define the interface contract between the engine and any UI frontend.

**Components:**

- `UIDriver` interface:
  ```go
  type UIDriver interface {
      // Run starts the UI and blocks until the user exits or ctx is cancelled.
      // The UIController provides all operations the UI can perform.
      Run(ctx context.Context, controller *UIController) error
  }
  ```

- `UIController` struct — the engine-facing API that all UIs consume:
  ```go
  type UIController struct {
      engine     *core.Engine
      tree       *config.Tree  // cached, reloaded on demand
  }
  ```

- `UIController` methods:
  - **Tree operations:**
    - `LoadTree(ctx) (*config.Tree, error)` — load/reload the full tree
    - `FilteredTree(ctx) (*config.Tree, error)` — tree filtered to enabled components
    - `GetValue(ctx, path) (*config.Value, bool, error)` — get a single value
    - `SetValue(ctx, path, value) error` — set a single value
    - `Validate(ctx) ([]config.ValidationResult, error)` — validate all paths + component dependencies
    - `SaveTree(ctx, id) error` — save tree + component state to store
  - **Component operations:**
    - `ListComponents() []core.ComponentState` — list all components with status
    - `EnableComponent(name) error` — enable a component (auto-enables dependencies)
    - `DisableComponent(name) error` — disable a component (with guard rails)
    - `IsComponentEnabled(name) bool` — check component status
  - **Export operations:**
    - `Export(ctx, config core.ExportConfig) (*core.ExportResult, error)` — export a single template
    - `ExportAll(ctx) ([]*core.ExportResult, error)` — export all workspace templates
    - `ExportPreview(ctx, config core.ExportConfig) (string, error)` — preview without writing
  - **Apply operations:**
    - `Apply(ctx, output chan<- core.ApplyOutput) (*core.ApplyResult, error)` — run apply command
    - `ApplyConfig() *core.ApplyConfig` — get the configured apply settings
  - **Metadata:**
    - `WorkspaceName() string` — workspace identifier for display
    - `ProviderInfo() map[string]string` — provider names for status display

- `NewUIController(engine *core.Engine) *UIController` — constructor

**Design rationale:** The `UIController` exists so that UI implementations don't reach directly into `*Engine`. This gives us a stable surface to add caching, rate limiting, or event notifications later without changing UI code. It also makes testing UIs easier — mock the controller, not the engine.

### 5.2 UI Registry (`internal/ui/registry.go`)

A simple registry of available UI drivers, allowing the CLI to select which frontend to launch.

**Components:**

- `var drivers map[string]UIDriverFactory` — maps driver names to factory functions
- `UIDriverFactory` — `func() UIDriver`
- `Register(name string, factory UIDriverFactory)` — register a UI driver
- `Get(name string) (UIDriver, error)` — retrieve a driver by name
- `List() []string` — list available driver names
- Default registration: `"tui"` → Bubbletea TUI (registered in Step 6)

This registry is intentionally simple for the MVP. Future UI implementations register themselves via `init()` functions or explicit registration at startup.

### 5.3 CLI: `zhi edit` Command (`internal/cli/edit.go`)

Launch the interactive UI. This command lives in the CLI layer but depends on the UI abstraction to resolve which frontend to run.

**Usage:**

```
zhi edit [tree-id]
```

**Behavior:**

1. Load workspace config and build engine
2. Create `UIController` wrapping the engine
3. Resolve the UI driver from the registry (default: `"tui"`)
4. Call `driver.Run(ctx, controller)`
5. On exit, print any final status message

**Flags:**

- `--tree` — specify which tree ID to load (if store has multiple)
- `--ui` — select UI driver (default: `"tui"`). In MVP, only `"tui"` is available. Future: `"web"`, `"chat"`, etc.

### 5.4 Tests

- `internal/ui/driver_test.go` — test UIController methods with mock engine:
  - LoadTree returns correct tree
  - FilteredTree respects component state
  - SetValue delegates to engine
  - Enable/DisableComponent delegates to component manager
  - Validate includes component dependency results
  - ExportPreview returns rendered string without writing
  - Apply delegates to engine and streams output
  - Metadata methods return correct workspace/provider info
- `internal/ui/registry_test.go` — test UI driver registration and retrieval:
  - Register a driver and retrieve it by name
  - Get unknown name returns error
  - List returns all registered names
  - Register duplicate name overwrites
- `internal/cli/edit_test.go` — test `zhi edit` command setup:
  - Default `--ui` resolves to `"tui"`
  - Unknown `--ui` value prints available drivers and exits with error

Testing approach: For UIController tests, use mock implementations of `*core.Engine` and `*core.ComponentManager`. The UIDriver interface is trivial to mock — a test driver that records calls and returns canned responses.

## Verification Criteria

1. `UIDriver` interface is defined in `internal/ui/driver.go`
2. `UIController` provides all tree, component, export, apply, and metadata operations without exposing engine internals
3. `UIController` methods delegate correctly to `*Engine` and `*ComponentManager`
4. UI driver registry works: `Register`, `Get`, `List` behave correctly
5. `Get` for an unregistered name returns a clear error
6. `zhi edit --ui unknown` prints available drivers and exits
7. `zhi edit` resolves to the default driver (`"tui"`) — actual TUI launch is tested in Step 6
8. No concrete UI code in this step — only interfaces, controller, and registry
9. No gRPC — the UIController calls engine methods directly
10. All tests pass with `go test -race ./internal/ui/... ./internal/cli/...`
