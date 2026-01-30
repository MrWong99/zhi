# Step 1: Core Engine, Provider Registry, and Component Model

## Overview

Build the central orchestration layer that connects providers (config, transform, store) into a coherent lifecycle. This step creates the provider registry, the core engine, the workspace configuration format, and the **component model** that bundles configuration values into toggleable groups.

## Relevant Existing Files

- `pkg/zhiplugin/plugin.go` — shared handshake definition (`ZHI_PLUGIN=zhiplugin-v1`)
- `pkg/zhiplugin/config/plugin.go` — config Plugin interface (`List`, `Get`, `Set`, `Validate`)
- `pkg/zhiplugin/config/config.go` — `Tree`, `TreeReader`, `Value`, `ValidationResult` types
- `pkg/zhiplugin/transform/plugin.go` — transform Plugin interface (`BeforeDisplay`, `AfterSave`, `ValidatePolicy`)
- `pkg/zhiplugin/transform/transform.go` — `ValidatePolicy` enum
- `pkg/zhiplugin/store/plugin.go` — store Plugin interface (`Save`, `Load`, `Delete`, `ListTrees`, etc.)
- `pkg/zhiplugin/store/store.go` — `EncryptionStatus` enum
- `pkg/providers/config/structuredfile/structuredfile.go` — existing built-in config provider
- `internal/app/zhi/zhi.go` — empty, to be replaced/repurposed

## Implementation Plan

### 1.1 Provider Registry (`internal/core/registry.go`)

A registry that maps provider names to factory functions for each plugin type. The registry is populated at startup with all compiled-in providers.

**Components:**

- `ConfigFactory` — `func() (config.Plugin, error)` type alias for config provider constructors
- `TransformFactory` — `func() (transform.Plugin, error)` type alias for transform provider constructors
- `StoreFactory` — `func() (store.Plugin, error)` type alias for store provider constructors
- `Registry` struct containing three maps: `config`, `transform`, `store`, each mapping `string` name to the corresponding factory
- `NewRegistry()` constructor
- `RegisterConfig(name, factory)`, `RegisterTransform(name, factory)`, `RegisterStore(name, factory)` — register built-in providers
- `ConfigProvider(name) (config.Plugin, error)` — resolve and instantiate a config provider by name
- `TransformProvider(name) (transform.Plugin, error)` — same for transform
- `StoreProvider(name) (store.Plugin, error)` — same for store
- `ListConfig() []string`, `ListTransform() []string`, `ListStore() []string` — enumerate registered names

The registry does **not** handle external plugins at this stage (that's Step 7). It only manages built-in Go types.

**Built-in registrations at startup:**

- Config: `"structuredfile"` → `structuredfile.New()` (wrap existing `pkg/providers/config/structuredfile`)
- Transform: (none in MVP initially)
- Store: (none built-in initially; json-store is an example external plugin)

### 1.2 Workspace Configuration (`internal/core/workspace.go`)

A workspace is a directory containing a `zhi.yaml` (or `zhi.json`) file that declares which providers to use and how they are configured.

**Workspace config format (`zhi.yaml`):**

```yaml
version: "1"

config:
  provider: structuredfile
  options:
    directory: ./config

transform: []
  # - provider: my-transform
  #   options: {}

store:
  provider: json-store
  options:
    directory: ./.zhi/store

components:
  - name: database
    description: "PostgreSQL database configuration"
    paths:
      - database/
    mandatory: true

  - name: redis
    description: "Redis cache layer"
    paths:
      - redis/
    dependencies:
      - database

  - name: monitoring
    description: "Prometheus and Grafana monitoring stack"
    paths:
      - monitoring/

export:
  templates:
    - name: docker-compose
      template: ./templates/docker-compose.yml.tmpl
      output: ./docker-compose.override.yml

apply:
  command: "docker compose up -d"
  workdir: "."
```

**Components:**

- `WorkspaceConfig` struct with fields for each section (including `Components []ComponentDef`)
- `LoadWorkspace(dir string) (*WorkspaceConfig, error)` — find and parse `zhi.yaml` in the given directory (or walk up to find it)
- Validation: ensure referenced providers exist in the registry, template files exist on disk, component names are unique, component dependencies form a DAG (no cycles), component path prefixes are valid

### 1.3 Component Model (`internal/core/component.go`)

The component system manages named groups of configuration paths that can be toggled on/off by users.

**Data types:**

- `ComponentDef` struct — a component definition from `zhi.yaml`:
  - `Name string` — unique identifier (e.g., `"database"`, `"redis"`)
  - `Description string` — human-readable description
  - `Paths []string` — list of config path prefixes (e.g., `"database/"`, `"redis/"`)
  - `Mandatory bool` — if true, the component cannot be disabled
  - `Dependencies []string` — names of components this one depends on

- `ComponentState` struct — runtime state of a component:
  - `Name string`
  - `Enabled bool`
  - `Mandatory bool` — copied from definition for convenience
  - `Dependencies []string` — copied from definition

- `ComponentManager` struct — manages component definitions and state:
  - `definitions []ComponentDef` — from workspace config
  - `state map[string]bool` — enabled/disabled per component name

**Operations:**

- `NewComponentManager(defs []ComponentDef) (*ComponentManager, error)` — validate definitions (unique names, no cycles in dependencies, mandatory components start enabled), return manager
- `Enable(name string) error` — enable a component; automatically enables transitive dependencies; returns error if component name is unknown
- `Disable(name string) error` — disable a component; returns error if mandatory or if other enabled components depend on it
- `IsEnabled(name string) bool` — check if a component is currently enabled
- `ListComponents() []ComponentState` — return all components with their current state
- `EnabledPaths() []string` — return all path prefixes belonging to enabled components
- `PathBelongsToComponent(path string) (string, bool)` — given a config path, return which component it belongs to (if any)
- `FilterTree(tree *config.Tree) *config.Tree` — return a new tree containing only paths that belong to enabled components (or paths not claimed by any component)
- `ValidateDependencies() []error` — check that all dependency constraints are satisfied
- `LoadState(state map[string]bool)` — restore component state (from store or state file)
- `SaveState() map[string]bool` — export current state for persistence

**Dependency rules:**

- Enabling component A that depends on B automatically enables B (and B's dependencies, transitively)
- Disabling component B when enabled component A depends on B returns an error: `"cannot disable 'database': required by enabled component 'redis'"`
- Mandatory components cannot be disabled: `"cannot disable 'database': component is mandatory"`
- Circular dependencies are detected at `NewComponentManager()` time using topological sort

**Path ownership:**

- A config path belongs to a component if it starts with any of the component's path prefixes
- Paths not claimed by any component are always included (they are "unmanaged")
- A path can only belong to one component — overlapping prefixes are rejected at workspace validation time

### 1.4 Core Engine (`internal/core/engine.go`)

The engine is the central orchestrator. It holds references to the resolved providers and exposes high-level operations.

**Components:**

- `Engine` struct holding:
  - `registry *Registry`
  - `workspace *WorkspaceConfig`
  - `configPlugin config.Plugin`
  - `transformPlugins []transform.Plugin`
  - `storePlugin store.Plugin`
  - `components *ComponentManager`
- `NewEngine(registry, workspace) (*Engine, error)` — resolve all providers from the workspace config using the registry, initialize `ComponentManager` from workspace component definitions, store references
- `LoadTree(ctx) (*config.Tree, error)` — call `configPlugin.List()` then `configPlugin.Get()` for each path, assemble a `Tree`
- `TransformForDisplay(ctx, tree) error` — iterate `transformPlugins`, call `BeforeDisplay()` on each
- `TransformForSave(ctx, tree) error` — iterate `transformPlugins`, call `AfterSave()` on each
- `Validate(ctx, tree) ([]config.ValidationResult, error)` — call `configPlugin.Validate()` for each path, then also validate component dependency constraints via `components.ValidateDependencies()`
- `SetValue(ctx, path, value) error` — call `configPlugin.Set()`
- `SaveTree(ctx, id, tree) error` — if storePlugin is configured, call `storePlugin.Save()`, also persist component state
- `LoadStoredTree(ctx, id) (*config.Tree, bool, error)` — call `storePlugin.Load()`, also restore component state
- `ListTrees(ctx) ([]string, error)` — call `storePlugin.ListTrees()`
- `Components() *ComponentManager` — expose component manager for UI and CLI access
- `FilteredTree(ctx) (*config.Tree, error)` — load tree and filter to only enabled components' paths (plus unmanaged paths)

### 1.5 Built-in Provider Adapter (`internal/core/builtin.go`)

The existing `structuredfile.Plugin` in `pkg/providers/config/structuredfile/` needs a thin adapter or constructor that conforms to the `ConfigFactory` signature.

**Components:**

- `NewStructuredFileProvider(options map[string]any) (config.Plugin, error)` — reads the `directory` option, calls into `structuredfile` package to construct the provider
- Register this factory in the default registry setup

### 1.6 Default Registry Setup (`internal/core/defaults.go`)

A function that creates a registry pre-populated with all built-in providers.

```go
func DefaultRegistry() *Registry {
    r := NewRegistry()
    r.RegisterConfig("structuredfile", NewStructuredFileProvider)
    return r
}
```

### 1.7 Tests

- `internal/core/registry_test.go` — register/resolve/list providers, error on unknown name
- `internal/core/workspace_test.go` — parse valid workspace config, reject invalid configs, validate component definitions (unique names, no cycles, valid path prefixes)
- `internal/core/component_test.go` — component model tests:
  - Enable/disable components
  - Mandatory components cannot be disabled
  - Enabling a component auto-enables dependencies
  - Disabling a component fails if dependents are enabled
  - Circular dependency detection at construction time
  - `FilterTree()` returns only enabled components' paths plus unmanaged paths
  - `PathBelongsToComponent()` resolves path ownership correctly
  - Overlapping path prefixes are rejected
  - State save/load round-trip
- `internal/core/engine_test.go` — integration test using mock providers: load tree, set value, validate (including component validation), save
- Use the existing test patterns from `pkg/zhiplugin/config/config_test.go` and `pkg/providers/config/structuredfile/structuredfile_test.go` as reference

## Verification Criteria

1. `Registry` can register and resolve config, transform, and store providers by name
2. `Registry.ListConfig()` returns `["structuredfile"]` after default setup
3. `LoadWorkspace()` parses a valid `zhi.yaml` and returns a structured config including component definitions
4. `Engine` can load a tree from `structuredfile` provider using test fixture files from `pkg/providers/config/structuredfile/testdata/`
5. `Engine.Validate()` returns validation results from the config provider and component dependency checks
6. `Engine.SetValue()` calls through to the config provider
7. `ComponentManager` correctly handles enable/disable with dependency resolution
8. `ComponentManager.FilterTree()` produces a tree with only enabled components' paths
9. Circular component dependencies are rejected at workspace load time
10. Mandatory components cannot be disabled
11. All tests pass with `go test -race ./internal/core/...`
12. No changes to any files in `pkg/zhiplugin/` or `pkg/providers/`
