# Step 1: Core Engine and Provider Registry

## Overview

Build the central orchestration layer that connects providers (config, transform, store) into a coherent lifecycle. This step creates the provider registry, the core engine, and the workspace configuration format.

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

The registry does **not** handle external plugins at this stage (that's Step 6). It only manages built-in Go types.

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

- `WorkspaceConfig` struct with fields for each section
- `LoadWorkspace(dir string) (*WorkspaceConfig, error)` — find and parse `zhi.yaml` in the given directory (or walk up to find it)
- Validation: ensure referenced providers exist in the registry, template files exist on disk

### 1.3 Core Engine (`internal/core/engine.go`)

The engine is the central orchestrator. It holds references to the resolved providers and exposes high-level operations.

**Components:**

- `Engine` struct holding:
  - `registry *Registry`
  - `workspace *WorkspaceConfig`
  - `configPlugin config.Plugin`
  - `transformPlugins []transform.Plugin`
  - `storePlugin store.Plugin`
- `NewEngine(registry, workspace) (*Engine, error)` — resolve all providers from the workspace config using the registry, store references
- `LoadTree(ctx) (*config.Tree, error)` — call `configPlugin.List()` then `configPlugin.Get()` for each path, assemble a `Tree`
- `TransformForDisplay(ctx, tree) error` — iterate `transformPlugins`, call `BeforeDisplay()` on each
- `TransformForSave(ctx, tree) error` — iterate `transformPlugins`, call `AfterSave()` on each
- `Validate(ctx, tree) ([]config.ValidationResult, error)` — call `configPlugin.Validate()` for each path
- `SetValue(ctx, path, value) error` — call `configPlugin.Set()`
- `SaveTree(ctx, id, tree) error` — if storePlugin is configured, call `storePlugin.Save()`
- `LoadStoredTree(ctx, id) (*config.Tree, bool, error)` — call `storePlugin.Load()`
- `ListTrees(ctx) ([]string, error)` — call `storePlugin.ListTrees()`

### 1.4 Built-in Provider Adapter (`internal/core/builtin.go`)

The existing `structuredfile.Plugin` in `pkg/providers/config/structuredfile/` needs a thin adapter or constructor that conforms to the `ConfigFactory` signature.

**Components:**

- `NewStructuredFileProvider(options map[string]any) (config.Plugin, error)` — reads the `directory` option, calls into `structuredfile` package to construct the provider
- Register this factory in the default registry setup

### 1.5 Default Registry Setup (`internal/core/defaults.go`)

A function that creates a registry pre-populated with all built-in providers.

```go
func DefaultRegistry() *Registry {
    r := NewRegistry()
    r.RegisterConfig("structuredfile", NewStructuredFileProvider)
    return r
}
```

### 1.6 Tests

- `internal/core/registry_test.go` — register/resolve/list providers, error on unknown name
- `internal/core/workspace_test.go` — parse valid workspace config, reject invalid configs
- `internal/core/engine_test.go` — integration test using mock providers: load tree, set value, validate, save
- Use the existing test patterns from `pkg/zhiplugin/config/config_test.go` and `pkg/providers/config/structuredfile/structuredfile_test.go` as reference

## Verification Criteria

1. `Registry` can register and resolve config, transform, and store providers by name
2. `Registry.ListConfig()` returns `["structuredfile"]` after default setup
3. `LoadWorkspace()` parses a valid `zhi.yaml` and returns a structured config
4. `Engine` can load a tree from `structuredfile` provider using test fixture files from `pkg/providers/config/structuredfile/testdata/`
5. `Engine.Validate()` returns validation results from the config provider
6. `Engine.SetValue()` calls through to the config provider
7. All tests pass with `go test -race ./internal/core/...`
8. No changes to any files in `pkg/zhiplugin/` or `pkg/providers/`
