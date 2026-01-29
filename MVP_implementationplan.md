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

### 2.2 TUI In-Process, Discrete Actions Only

The TUI is part of the core binary, built with Bubbletea. All local interaction (typing, navigation, scrolling) stays within the Bubbletea event loop. Only semantic actions trigger calls into the core engine:

| Key | Action | Core Call |
|-----|--------|-----------|
| Enter | Set value | `config.Plugin.Set()` |
| `s` | Save tree | `store.Plugin.Save()` |
| `v` | Validate | `config.Plugin.Validate()` |
| `a` | Apply | Run provisioning command |
| `e` | Export | Write rendered template to disk |

There is no gRPC between TUI and core — the TUI calls Go interfaces directly since both live in the same process.

### 2.3 Config-Focused, No Runtime Management

zhi manages **configuration artifacts**, not running services. The workflow:

1. **Load** configuration from config plugins (structured files, environment, etc.)
2. **Transform** configuration via transform plugins (enrichment, renaming, defaults)
3. **Validate** configuration against constraints
4. **Store** configuration to a store plugin (JSON files, encrypted store, etc.)
5. **Export** configuration to standard formats using Go templates
6. **Apply** (optional): run a user-configured shell command (e.g., `docker compose up`, `kubectl apply`, `ansible-playbook`) and stream its output

The "apply" command is configured per-tree or globally. zhi invokes it as a subprocess and pipes stdout/stderr to the TUI. This avoids embedding Docker/K8s SDKs while supporting any provisioning tool.

## 3. Repository Structure

The MVP extends the existing codebase without reorganizing what already works.

```
cmd/zhi/                          # CLI entry point (exists, currently empty)
internal/
  core/                           # NEW: engine, registry, export, apply, discovery
    engine.go                     #   Orchestrates load → transform → validate → store
    registry.go                   #   Built-in provider registry
    export.go                     #   Template-based export (JSON, YAML, TOML, dotenv)
    apply.go                      #   External command runner with output streaming
    discovery.go                  #   External plugin discovery (~/.zhi/plugins/)
  cli/                            # NEW: CLI subcommands (cobra or plain flag)
    root.go
    init.go                       #   Initialize a new zhi workspace
    edit.go                       #   Launch TUI for a tree
    get.go                        #   Retrieve a single config value
    set.go                        #   Set a single config value
    export.go                     #   Export tree to file
    apply.go                      #   Run apply command
    validate.go                   #   Validate a tree
    list.go                       #   List trees/providers
  ui/                             # NEW: Bubbletea TUI
    app.go                        #   Root Bubbletea model
    tree_view.go                  #   Tree browser/editor
    value_editor.go               #   Single-value editor
    validation_view.go            #   Validation results display
    apply_view.go                 #   Apply command output streaming
    export_view.go                #   Export format selection
pkg/zhiplugin/                    # EXISTS: public plugin framework (unchanged)
  config/                         #   Config plugin interface + gRPC
  transform/                      #   Transform plugin interface + gRPC
  store/                          #   Store plugin interface + gRPC
  plugin.go                       #   Shared handshake
pkg/providers/                    # EXISTS: built-in providers
  config/structuredfile/          #   EXISTS: structured file config provider
  transform/                      #   FUTURE: built-in transform providers
  store/                          #   FUTURE: built-in store providers (json-store promoted)
pkg/ui/                           # EXISTS: placeholder (may merge with internal/ui/)
api/proto/zhiplugin/v1/           # EXISTS: protobuf definitions
examples/                         # EXISTS: example plugins
test/                             # EXISTS: placeholder for integration tests
```

### Key decisions:

- `internal/core/` is new — this is where the MVP engine logic lives.
- `internal/cli/` is new — CLI subcommands.
- `internal/ui/` is new — Bubbletea TUI.
- `pkg/zhiplugin/` and `pkg/providers/` remain untouched — the plugin API is stable.
- `internal/app/zhi/` (currently empty) will be removed or repurposed.

## 4. Component Overview

### 4.1 Provider Registry (`internal/core/registry.go`)

A typed map of provider names to factory functions. At startup, the binary registers all compiled-in providers. The registry exposes a unified interface so the engine doesn't need to know whether a provider is built-in or external.

```
Registry
  ├── config providers:    {"structuredfile": factory}
  ├── transform providers: {}
  └── store providers:     {"json-store": factory}
```

External plugins are resolved lazily: when the registry can't find a built-in match, it calls the discovery module to scan plugin directories.

### 4.2 Core Engine (`internal/core/engine.go`)

The engine orchestrates the full lifecycle:

1. Resolve providers from registry
2. Load configuration tree from config plugin(s)
3. Run transform plugins (BeforeDisplay)
4. Present tree in TUI or CLI output
5. On save: run transform plugins (AfterSave), then store via store plugin
6. On export: render Go template with tree data, write to file
7. On apply: execute configured command as subprocess

### 4.3 Export System (`internal/core/export.go`)

Template-based rendering using `text/template`. Users provide template files that reference tree paths. Built-in format helpers for JSON, YAML, TOML, and dotenv. Example:

```yaml
# template: docker-compose.override.yml.tmpl
services:
  db:
    environment:
      POSTGRES_HOST: {{ .Get "database/host" }}
      POSTGRES_PORT: {{ .Get "database/port" }}
```

### 4.4 Apply System (`internal/core/apply.go`)

Runs a provisioning plugin that might call an external command (`exec.Command`) with the exported config file(s) available. Streams stdout/stderr to a channel that the TUI or CLI can consume. Returns the exit code.

### 4.5 CLI (`internal/cli/`)

Subcommand-based CLI providing non-UI access to all operations:

- `zhi init` — scaffold a new workspace with config files and templates
- `zhi edit` — launch TUI to browse/edit a tree
- `zhi get` — retrieve a single config value by path
- `zhi set` — set a single config value by path
- `zhi export` — export a tree to a file using a template
- `zhi apply` — run the configured apply command
- `zhi validate` — validate a tree and print results
- `zhi list` — list available trees, providers, and plugins

### 4.6 TUI (`internal/ui/`)

Bubbletea application with multiple views:

- **Tree view**: browse configuration paths, see values and metadata
- **Value editor**: edit a single value with type-appropriate input
- **Validation view**: display validation results with severity highlighting
- **Apply view**: stream apply command output with scroll-back
- **Export view**: select format and destination, preview output

## 5. Implementation Steps

The implementation is broken into 8 sequential steps, each documented in detail in the `MVP/` directory:

| Step | File | Summary |
|------|------|---------|
| 1 | `MVP/01_core_engine_and_registry.md` | Provider registry, core engine, tree lifecycle |
| 2 | `MVP/02_cli_foundation.md` | CLI subcommands, workspace init, basic operations |
| 3 | `MVP/03_provision_system.md` | Export system with templates and format helpers |
| 4 | `MVP/04_apply_system.md` | External command runner with output streaming |
| 5 | `MVP/05_tui.md` | Bubbletea TUI with tree browser and value editor |
| 6 | `MVP/06_external_plugin_discovery.md` | Plugin directory scanning, external plugin launch |
| 7 | `MVP/07_security_and_polish.md` | Input sanitization, error handling, documentation |
| 8 | `MVP/08_ci_cd_and_release.md` | GoReleaser, CI pipeline, release automation |

Each step builds on the previous. Steps 1-2 establish the foundation. Steps 3-4 add export and apply capabilities. Step 5 adds the TUI. Steps 6-8 handle extensibility, hardening, and release.

## 6. What's Deferred

The following are explicitly out of scope for the MVP:

- **AuthN/AuthZ**: No Casbin, no OIDC, no RBAC. Authentication and authorization will be added in a future release.
- **CUE transformation**: Transform plugins exist but no CUE-based implementation in MVP. Transforms are written in Go.
- **Vault storage**: The store plugin API supports encryption, but no Vault backend in MVP. The json-store example is the reference implementation.
- **Kubernetes/Helm provisioning**: The apply system supports `kubectl apply` as an external command, but there's no embedded K8s client.
- **Docker Compose SDK**: Same as K8s — use `docker compose` via the new Compose Go SDK.
- **Bidirectional gRPC streaming**: Not needed. TUI is in-process.
- **Polyglot plugins**: The gRPC API supports it in theory, but MVP focuses on Go plugins only.
- **Check-and-Set (CAS)**: Documented in TODOS.md for future work.
- **Tree-level metadata**: Documented in TODOS.md for future work.

## 7. Success Criteria

The MVP is complete when:

1. `zhi init` creates a working workspace with a structuredfile config
2. `zhi edit` opens a TUI where users can browse, edit, and validate configuration
3. `zhi export` renders a template file with tree data to stdout or a file
4. `zhi apply` runs an external command with exported config available
5. `zhi validate` prints validation results to stdout
6. `zhi list` shows available trees and providers
7. External plugins in `~/.zhi/plugins/` are discovered and usable
8. All operations work with the existing plugin API (no breaking changes to `pkg/zhiplugin/`)
9. `make check` passes (tests, lint, vet, fmt)
10. Binary builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
