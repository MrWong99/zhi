# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

zhi is a security-first platform for configuration management and provisioning, built in Go. It uses an extensible plugin system over gRPC (via hashicorp/go-plugin) with four plugin types: **config**, **transform**, **store**, and **ui**. Plugins run as separate processes communicating over stdio with gRPC transport.

The platform follows a Vault-style architecture: built-in providers are compiled Go types registered at startup, while external plugins are separate binaries discovered from `~/.zhi/plugins/`.

## Build & Development Commands

```bash
make build              # Build zhi binary to bin/
make build-examples     # Build example plugins to bin/examples/
make build-all          # Build everything
make test               # Run all tests (race detection enabled)
make test-short         # Skip long-running tests
make test-cover         # Tests with coverage summary
make check              # Full CI suite: fmt + vet + lint + test
make proto              # Regenerate Go code from .proto files
make proto-check        # Verify generated proto is up-to-date
make lint               # Run golangci-lint
make fmt                # Format with gofmt -s
make tools              # Install dev tools (protoc-gen-go, golangci-lint, etc.)
make deps               # Download dependencies
```

Run a single test: `go test -race -count=1 -run TestName ./path/to/package/...`

## Architecture

### Core Engine

The engine (`internal/core/engine.go`) orchestrates the full lifecycle: load configuration from config plugins, compose via components, transform, validate, store, export to templates, and optionally apply via external commands.

### Provider Registry

The registry (`internal/core/registry.go`) maps provider names to factory functions. Built-in providers (like `structuredfile`) are registered at startup. External plugins are resolved lazily from `~/.zhi/plugins/` when not found in built-ins.

### Component Model

Components (`internal/core/component.go`) are named groups of configuration paths that users can enable/disable. They support dependencies, mandatory flags, and filtering of the tree for exports. Defined in `zhi.yaml`.

### Plugin System

All plugins share a handshake (`pkg/zhiplugin/plugin.go`): magic cookie `ZHI_PLUGIN` with value `zhiplugin-v1`, protocol version 1.

**Config plugins** (`pkg/zhiplugin/config/`) implement the `Plugin` interface:
- `List(ctx) ([]string, error)` -- return all managed paths
- `Get(ctx, path) (Value, bool, error)` -- retrieve a value
- `Set(ctx, path, Value) error` -- store a value
- `Validate(ctx, path, TreeReader) ([]ValidationResult, error)` -- validate with full tree context

**Transform plugins** (`pkg/zhiplugin/transform/`) implement the `Plugin` interface:
- `BeforeDisplay(ctx, *Tree) error` -- mutate tree before UI display
- `AfterSave(ctx, *Tree) error` -- mutate tree after user saves
- `ValidatePolicy(ctx) (ValidatePolicy, error)` -- control validation timing relative to transforms

**Store plugins** (`pkg/zhiplugin/store/`) implement the `Plugin` interface:
- `Save(ctx, id, TreeReader) error` -- persist a configuration tree
- `Load(ctx, id) (*Tree, bool, error)` -- retrieve the latest tree version
- `Delete(ctx, id) error` -- remove a tree and all its versions
- `ListTrees(ctx) ([]string, error)` -- list all stored tree IDs
- `SupportsVersioning(ctx) (bool, error)` -- report versioning capability
- `ListVersions(ctx, id) ([]string, error)` -- list versions (newest first)
- `LoadVersion(ctx, id, version) (*Tree, bool, error)` -- load a specific version
- `DeleteVersion(ctx, id, version) error` -- permanently remove a single version
- `EncryptionStatus(ctx) (EncryptionStatus, error)` -- report encryption state (None, Supported, Active)
- `InitEncryption(ctx, passphrase) error` -- initialize encryption
- `RotateEncryption(ctx, old, new) error` -- rotate encryption keys

**UI plugins** (`pkg/zhiplugin/ui/`) implement the `Plugin` interface:
- `Run(ctx, Controller) error` -- start the UI, block until exit
- `Capabilities(ctx) (Capabilities, error)` -- report capabilities (e.g., RequiresTTY)

UI plugins use bidirectional gRPC: the host calls the plugin to start the UI, and the plugin calls back into the host via a `Controller` interface for core operations.

### Configuration Tree Model

`config.Tree` is a flat key-value store with slash-delimited paths (e.g. `database/host`, `app/tls/cert.pem`). Path segments must match `[a-z][a-z0-9._-]*[a-z0-9]`. `Tree` implements `TreeReader` (read-only) and also exposes `GetPtr` for mutable access and `Delete` for removal. `Value` holds the data (`Val any`), optional `Metadata`, and local `Validators` (closures that never cross the gRPC wire).

Validation results carry a `Severity`: Info, Warning, or Blocking.

### UI Abstraction Layer

The `UIDriver` interface (`internal/ui/driver.go`) decouples the engine from UI frontends. The `UIController` wraps the engine with UI-relevant operations. The TUI (`internal/ui/tui/`) is the first implementation using Bubbletea. The architecture supports adding Web, AI Chat, or headless API frontends without engine changes.

### Export System

Template-based rendering (`internal/core/export.go`) using `text/template` with Sprig functions. Supports built-in formats (JSON, YAML, TOML, dotenv) and custom templates. Component-aware: only enabled components' paths are included by default.

### gRPC Layer

Proto definitions: `api/proto/zhiplugin/v1/`. Generated Go stubs go to `pkg/zhiplugin/{plugin}/proto/`. After editing `.proto` files, run `make proto` and commit the generated `*.pb.go` files. Configuration values are JSON-encoded for wire transfer.

Each plugin type has `grpc_client.go` (host-side) and `grpc_server.go` (plugin-side) implementing the translation between the Go interface and protobuf messages.

## Key Directories

- `cmd/zhi/` -- CLI entry point
- `internal/core/` -- engine, registry, components, export, apply, plugin discovery
- `internal/cli/` -- CLI subcommands (Cobra)
- `internal/ui/` -- UI abstraction layer and TUI implementation
- `pkg/zhiplugin/` -- public plugin framework (config, transform, store, ui)
- `pkg/providers/` -- built-in provider implementations
- `api/proto/zhiplugin/v1/` -- protobuf service definitions
- `examples/` -- working plugin examples
- `docs/user-guide/` -- end-user documentation
- `docs/plugin-development/` -- plugin developer documentation
- `test/` -- integration/E2E tests

## Workspace Configuration

A workspace is defined by `zhi.yaml` containing: config provider, transform providers, store provider, component definitions, export templates, apply commands, and plugin directories. See `docs/user-guide/workspace-configuration.md` for the full reference.

## Prerequisites

Go 1.24+, protoc, protoc-gen-go, protoc-gen-go-grpc, golangci-lint. Install Go tools with `make tools`.
