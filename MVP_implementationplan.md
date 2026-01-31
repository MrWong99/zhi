# zhi MVP Implementation Plan

## 1. Executive Summary

zhi is a security-first platform for configuration management and provisioning, built in Go. The MVP delivers a working end-to-end flow: load configuration via plugins, validate and transform it, display it in a terminal UI, export it to standard formats, and optionally apply it through an external command.

This plan replaces the earlier "Self-Spawning Micro-Kernel" design with a simpler, proven architecture modeled after HashiCorp Vault: built-in providers are compiled Go types registered at startup, while external plugins are separate binaries discovered from `~/.zhi/plugins/`. The TUI runs in-process using Bubbletea with discrete semantic actions — no bidirectional gRPC streaming for UI.

zhi does **not** manage runtime. It writes, validates, transforms, and stores configuration. It exports to standard formats (JSON, YAML, TOML, dotenv) using template files. An optional "apply" step runs a user-configured external command and streams its stdout/stderr until exit.

**Deferred beyond MVP:** AuthN/AuthZ (Casbin, OIDC), CUE transformation language, Vault storage backend, Kubernetes/Helm provisioning.

## 2. Architectural Principles

### 2.1 Vault-Style Provider Registry (No Self-Spawning)

Built-in providers (like `structuredfile`) are ordinary Go types compiled into the binary. At startup, they are registered in a `Registry` by name. External plugins are separate binaries located in `~/.zhi/plugins/`, launched via hashicorp/go-plugin with the existing handshake (`ZHI_PLUGIN=zhiplugin-v1`, protocol version 1).

The registry resolves a provider name in two steps:

1. **Built-in lookup**: check the in-memory registry for a compiled-in provider.
2. **External lookup**: scan `~/.zhi/plugins/` (and any user-configured directories) for a matching binary.

This avoids the complexity of self-spawning (`os.Executable()` + flag-based mode switching) and aligns with how Vault, Terraform, and other HashiCorp tools handle built-in vs. external backends.

### 2.2 UI Abstraction Layer (Pluggable Frontends)

The core engine exposes a clean Go API that any user interface can consume. A `UIDriver` interface defines the contract between the engine and frontends. The TUI is the **first implementation** of this interface, but the architecture explicitly supports adding other frontends later:

- **TUI** (MVP): Bubbletea-based terminal interface, runs in-process
- **Web UI** (future): HTTP server exposing the engine API to a browser frontend
- **AI Chat** (future): Conversational interface where an LLM drives engine operations
- **Headless/API** (future): REST or gRPC API for programmatic access

The `UIDriver` interface is intentionally minimal:

```go
type UIDriver interface {
    // Run starts the UI and blocks until the user exits or ctx is cancelled.
    Run(ctx context.Context, controller *UIController) error
}
```

The `UIController` is the engine-facing API that all UIs consume. It wraps `*Engine` with UI-relevant operations (load tree, set value, validate, export, apply, list/toggle components). This separation means:

- The engine doesn't know which UI is driving it
- UIs don't reach into engine internals — they go through `UIController`
- Adding a new UI means implementing `UIDriver`, not modifying the engine

For the MVP TUI, all interaction is in-process — no gRPC between UI and engine. The TUI's Bubbletea event loop handles local interactions (typing, navigation). Only semantic actions trigger `UIController` calls:

| Key | Action | UIController Call |
|-----|--------|-------------------|
| Enter | Set value | `controller.SetValue()` |
| `s` | Save tree | `controller.SaveTree()` |
| `v` | Validate | `controller.Validate()` |
| `a` | Apply | `controller.Apply()` |
| `e` | Export | `controller.Export()` |
| `c` | Components | `controller.ListComponents()` |

### 2.3 Component Model (Configuration Bundles)

A **component** is a named group of related configuration values within a tree. Components let users selectively enable or disable entire feature sets without deleting the underlying configuration.

**Core concepts:**

- **Component definition**: A named bundle with a description, a list of config path prefixes, and optional constraints (mandatory, dependencies).
- **Enabled/disabled state**: Users can toggle components on and off. Disabled components' paths are excluded from export and apply operations but remain in the tree for later re-enabling.
- **Mandatory components**: Developers can mark a component as mandatory — it cannot be disabled by users.
- **Dependencies**: Components can declare dependencies on other components. Enabling component A that depends on component B will prompt the user to also enable B (or fail validation). Disabling B when A depends on it will warn/block.
- **Template visibility**: The export/templating system can query component state (`ComponentEnabled`, `EnabledComponents`), allowing templates to conditionally include configuration sections based on which components are active.

Components are defined in `zhi.yaml` and their enabled/disabled state is stored alongside the tree in the store plugin.

**Example:**

```yaml
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
```

In this example, `database` is always enabled. `redis` requires `database` to be enabled. `monitoring` is optional and independent.

### 2.4 Config-Focused, No Runtime Management

zhi manages **configuration artifacts**, not running services. The workflow:

1. **Load** configuration from config plugins (structured files, environment, etc.)
2. **Compose** configuration by selecting which components are active
3. **Transform** configuration via transform plugins (enrichment, renaming, defaults)
4. **Validate** configuration against constraints (including component dependencies)
5. **Store** configuration and component state to a store plugin (JSON files, encrypted store, etc.)
6. **Export** configuration to standard formats using Go templates (component-aware: only enabled components' paths are included, templates can query component state)
7. **Apply** (optional): run a user-configured shell command (e.g., `docker compose up`, `kubectl apply`, `ansible-playbook`) and stream its output

The "apply" command is configured per-tree or globally. zhi invokes it as a subprocess and streams stdout/stderr to whichever UI is active (TUI, Web UI, or CLI). This avoids embedding Docker/K8s SDKs while supporting any provisioning tool.

## 3. Repository Structure

The MVP extends the existing codebase without reorganizing what already works.

```
cmd/zhi/                          # CLI entry point (exists, currently empty)
internal/
  core/                           # NEW: engine, registry, export, apply, components, discovery
    engine.go                     #   Orchestrates load → compose → transform → validate → store
    registry.go                   #   Built-in provider registry
    component.go                  #   Component model, dependency resolution, state management
    export.go                     #   Template-based export (JSON, YAML, TOML, dotenv)
    apply.go                      #   External command runner with output streaming
    discovery.go                  #   External plugin discovery (~/.zhi/plugins/)
  cli/                            # NEW: CLI subcommands (cobra or plain flag)
    root.go
    init.go                       #   Initialize a new zhi workspace
    edit.go                       #   Launch UI (TUI by default) for a tree
    get.go                        #   Retrieve a single config value
    set.go                        #   Set a single config value
    export.go                     #   Export tree to file
    apply.go                      #   Run apply command
    validate.go                   #   Validate a tree
    list.go                       #   List trees/providers/components
    component.go                  #   Enable/disable/list components
  ui/                             # NEW: UI abstraction + implementations
    driver.go                     #   UIDriver interface + UIController
    tui/                          #   Bubbletea TUI implementation
      app.go                      #     Root Bubbletea model (implements UIDriver)
      tree_view.go                #     Tree browser/editor
      value_editor.go             #     Single-value editor
      component_view.go           #     Component toggle/selection view
      validation_view.go          #     Validation results display
      apply_view.go               #     Apply command output streaming
      export_view.go              #     Export format selection
pkg/zhiplugin/                    # EXISTS: public plugin framework (unchanged)
  config/                         #   Config plugin interface + gRPC
  transform/                      #   Transform plugin interface + gRPC
  store/                          #   Store plugin interface + gRPC
  plugin.go                       #   Shared handshake
pkg/providers/                    # EXISTS: built-in providers
  config/structuredfile/          #   EXISTS: structured file config provider
  transform/                      #   FUTURE: built-in transform providers
  store/                          #   FUTURE: built-in store providers (zhi-store-json promoted)
pkg/ui/                           # EXISTS: placeholder (may merge with internal/ui/)
api/proto/zhiplugin/v1/           # EXISTS: protobuf definitions
examples/                         # EXISTS: example plugins
test/                             # EXISTS: placeholder for integration tests
```

### Key decisions:

- `internal/core/` is new — engine logic, component model, export, apply, and plugin discovery.
- `internal/cli/` is new — CLI subcommands including component management.
- `internal/ui/` is new — UI abstraction layer (`driver.go`) with the TUI as the first implementation in `internal/ui/tui/`. This structure allows adding `internal/ui/web/`, `internal/ui/chat/`, etc. later without changing the engine or CLI.
- `pkg/zhiplugin/` and `pkg/providers/` remain untouched — the plugin API is stable.
- `internal/app/zhi/` (currently empty) will be removed or repurposed.

## 4. Component Overview

### 4.1 Provider Registry (`internal/core/registry.go`)

A typed map of provider names to factory functions. At startup, the binary registers all compiled-in providers. The registry exposes a unified interface so the engine doesn't need to know whether a provider is built-in or external.

```
Registry
  ├── config providers:    {"structuredfile": factory}
  ├── transform providers: {}
  └── store providers:     {"zhi-store-json": factory}
```

External plugins are resolved lazily: when the registry can't find a built-in match, it calls the discovery module to scan plugin directories.

### 4.2 Component System (`internal/core/component.go`)

The component system manages named groups of configuration paths that users can enable or disable.

```
ComponentManager
  ├── definitions []ComponentDef    # from zhi.yaml
  ├── state map[string]bool          # enabled/disabled per component
  ├── Enable(name) error             # enable a component (and dependencies)
  ├── Disable(name) error            # disable (blocked if mandatory or depended-upon)
  ├── IsEnabled(name) bool
  ├── ListComponents() []ComponentInfo
  ├── ValidateDependencies() []error # check all dependency constraints
  └── FilterTree(tree) *Tree         # return tree with only enabled components' paths
```

**Component definitions** come from `zhi.yaml`. **Component state** (which components are enabled) is persisted alongside the tree in the store plugin, or in a local `.zhi/components.json` state file.

**Dependency resolution:**

- Enabling a component auto-enables its transitive dependencies
- Disabling a component fails if other enabled components depend on it (with a clear error listing the dependents)
- Circular dependencies are detected at workspace load time and rejected

### 4.3 Core Engine (`internal/core/engine.go`)

The engine orchestrates the full lifecycle:

1. Resolve providers from registry
2. Load configuration tree from config plugin(s)
3. Resolve component state — determine which components are enabled
4. Run transform plugins (BeforeDisplay)
5. Present tree in UI (via UIController) or CLI output
6. On save: run transform plugins (AfterSave), then store tree + component state via store plugin
7. On export: filter tree to enabled components, render Go template with tree data and component state, write to file
8. On apply: execute configured command with component-filtered exported config

### 4.4 Export System (`internal/core/export.go`)

Template-based rendering using `text/template`. Users provide template files that reference tree paths. Built-in format helpers for JSON, YAML, TOML, and dotenv. Templates are **component-aware**: by default, only paths belonging to enabled components are included. Templates can also query component state directly.

```yaml
# template: docker-compose.override.yml.tmpl
services:
  db:
    environment:
      POSTGRES_HOST: {{ .Get "database/host" }}
      POSTGRES_PORT: {{ .Get "database/port" }}
{{ if .ComponentEnabled "redis" }}
  redis:
    environment:
      REDIS_URL: {{ .Get "redis/url" }}
{{ end }}
{{ if .ComponentEnabled "monitoring" }}
  prometheus:
    environment:
      SCRAPE_INTERVAL: {{ .Get "monitoring/scrape-interval" }}
{{ end }}
```

Template functions for component state: `.ComponentEnabled "name"`, `.EnabledComponents` (returns `[]string`), `.DisabledComponents` (returns `[]string`).

### 4.5 Apply System (`internal/core/apply.go`)

Runs a provisioning plugin that might call an external command (`exec.Command`) with the exported config file(s) available. Streams stdout/stderr to a channel that any UI or CLI consumer can read. Returns the exit code. The apply step receives component-filtered exports — only enabled components' configuration is included in the generated files.

### 4.6 CLI (`internal/cli/`)

Subcommand-based CLI providing non-UI access to all operations:

- `zhi init` — scaffold a new workspace with config files, templates, and sample components
- `zhi edit` — launch the interactive UI (TUI by default) to browse/edit a tree
- `zhi get` — retrieve a single config value by path
- `zhi set` — set a single config value by path
- `zhi export` — export a tree to a file using a template (component-aware)
- `zhi apply` — run the configured apply command
- `zhi validate` — validate a tree and print results (including component dependency checks)
- `zhi list` — list available trees, providers, plugins, and components
- `zhi component` — manage components:
  - `zhi component list` — show all components with enabled/disabled status
  - `zhi component enable <name>` — enable a component (and its dependencies)
  - `zhi component disable <name>` — disable a component (if not mandatory or depended-upon)

### 4.7 UI Layer (`internal/ui/`)

The UI layer defines the `UIDriver` interface (`internal/ui/driver.go`) and the `UIController` that all frontends consume. The MVP ships with a single implementation: the Bubbletea TUI in `internal/ui/tui/`.

**UIController** exposes:
- Tree operations: load, get, set, validate, save
- Component operations: list, enable, disable, filter
- Export and apply operations
- Event subscription for streaming output (apply command logs)

**TUI implementation** (`internal/ui/tui/`) — a Bubbletea application with multiple views:

- **Tree view**: browse configuration paths, see values and metadata. Paths belonging to disabled components are visually dimmed or hidden.
- **Component view**: toggle components on/off, see dependency graph, mandatory indicators
- **Value editor**: edit a single value with type-appropriate input
- **Validation view**: display validation results with severity highlighting (including component dependency violations)
- **Apply view**: stream apply command output with scroll-back
- **Export view**: select format and destination, preview output

**Adding a new UI frontend** (future work): implement `UIDriver`, register it, and add a `--ui <name>` flag to `zhi edit`. The engine and CLI remain unchanged.

## 5. Implementation Steps

The implementation is broken into 8 sequential steps, each documented in detail in the `MVP/` directory:

| Step | File | Summary |
|------|------|---------|
| 1 | `MVP/01_core_engine_and_registry.md` | Provider registry, core engine, tree lifecycle, **component model** |
| 2 | `MVP/02_cli_foundation.md` | CLI subcommands, workspace init, basic operations, **component CLI** |
| 3 | `MVP/03_provision_system.md` | Export system with templates, format helpers, **component-aware rendering** |
| 4 | `MVP/04_apply_system.md` | External command runner with output streaming, **component-filtered exports** |
| 5 | `MVP/05_ui_and_tui.md` | **UI abstraction layer** (`UIDriver`), Bubbletea TUI with tree browser, value editor, **component toggle view** |
| 6 | `MVP/06_external_plugin_discovery.md` | Plugin directory scanning, external plugin launch |
| 7 | `MVP/07_security_and_polish.md` | Input sanitization, error handling, documentation, **component dependency validation** |
| 8 | `MVP/08_ci_cd_and_release.md` | GoReleaser, CI pipeline, release automation |

Each step builds on the previous. Steps 1-2 establish the foundation (including component model and CLI). Steps 3-4 add component-aware export and apply capabilities. Step 5 introduces the UI abstraction layer and the TUI as its first implementation. Steps 6-8 handle extensibility, hardening, and release.

## 6. What's Deferred

The following are explicitly out of scope for the MVP:

- **AuthN/AuthZ**: No Casbin, no OIDC, no RBAC. Authentication and authorization will be added in a future release.
- **CUE transformation**: Transform plugins exist but no CUE-based implementation in MVP. Transforms are written in Go.
- **Vault storage**: The store plugin API supports encryption, but no Vault backend in MVP. The zhi-store-json example is the reference implementation.
- **Kubernetes/Helm provisioning**: The apply system supports `kubectl apply` as an external command, but there's no embedded K8s client.
- **Docker Compose SDK**: Same as K8s — use `docker compose` via the new Compose Go SDK.
- **Bidirectional gRPC streaming**: Not needed. TUI is in-process.
- **Polyglot plugins**: The gRPC API supports it in theory, but MVP focuses on Go plugins only.
- **Check-and-Set (CAS)**: Documented in TODOS.md for future work.
- **Tree-level metadata**: Documented in TODOS.md for future work.
- **Web UI frontend**: The `UIDriver` interface is ready, but only the TUI implementation ships in MVP. A Web UI (HTTP server + browser frontend) is planned for a future release.
- **AI Chat frontend**: An LLM-driven conversational interface for configuration management. The `UIDriver` abstraction makes this possible without engine changes.
- **Component conditions**: Components can be enabled/disabled manually in MVP. Conditional activation based on other values (e.g., "enable monitoring if environment=production") is future work.

## 7. Success Criteria

The MVP is complete when:

1. `zhi init` creates a working workspace with a structuredfile config and sample component definitions
2. `zhi edit` opens an interactive UI (TUI) where users can browse, edit, validate configuration, and toggle components
3. `zhi export` renders a template file with tree data to stdout or a file, respecting enabled/disabled component state
4. `zhi apply` runs an external command with component-filtered exported config available
5. `zhi validate` prints validation results including component dependency checks to stdout
6. `zhi list` shows available trees, providers, and components with their status
7. `zhi component enable/disable/list` manages component state from the CLI
8. External plugins in `~/.zhi/plugins/` are discovered and usable
9. All operations work with the existing plugin API (no breaking changes to `pkg/zhiplugin/`)
10. The UI layer uses the `UIDriver` interface — adding a new frontend requires no engine changes
11. `make check` passes (tests, lint, vet, fmt)
12. Binary builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
