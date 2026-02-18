# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

zhi is a security-first platform for configuration management and provisioning, built in Go 1.26. It uses an extensible plugin system over gRPC (via hashicorp/go-plugin) with four plugin types: **config**, **transform**, **store**, and **ui**. Plugins run as separate processes communicating over stdio with gRPC transport.

The platform follows a Vault-style architecture: built-in providers are compiled Go types registered at startup, while external plugins are separate binaries discovered from `~/.zhi/plugins/`. It includes a plugin marketplace with OCI-based distribution and an enterprise air-gapped mirror.

## Build & Development Commands

```bash
# Build
make build              # Build all cmd/ binaries to bin/
make build-examples     # Build all Go example plugins to bin/examples/
make build-all          # Build zhi + marketplace + mirror + examples
make install            # Install cmd/ binaries to $GOPATH/bin

# Test
make test               # Run all tests (race detection, -count=1)
make test-short         # Skip long-running tests
make test-verbose       # Verbose test output
make test-cover         # Tests with coverage summary (bin/coverage.out)
make test-cover-html    # Generate HTML coverage report
make bench              # Run benchmarks

# Code quality
make check              # Full CI suite: fmt + vet + lint + test
make fmt                # Format with gofmt -s
make vet                # Run go vet
make lint               # Run golangci-lint
make lint-fix           # Run golangci-lint with auto-fix

# Code generation
make proto              # Regenerate Go code from .proto files (downloads protoc automatically)
make proto-check        # Verify generated proto is up-to-date
make generate           # Run all code generation (proto + go generate)
make install-protoc     # Download protoc v33.5 to bin/protoc/ (called by make proto)

# Dependencies & tools
make tools              # Install dev tools (protoc-gen-go, protoc-gen-go-grpc, golangci-lint v2.8.0)
make deps               # Download Go module dependencies
make tidy               # Run go mod tidy

# Release
make snapshot           # Build snapshot release locally (GoReleaser)
make release-dry-run    # GoReleaser without publishing

# Cleanup
make clean              # Remove build artifacts and caches
make clean-protoc       # Remove locally installed protoc
```

Run a single test: `go test -race -count=1 -run TestName ./path/to/package/...`

## Linting

golangci-lint v2 with all default linters except `errcheck` (disabled in `.golangci.yml`). CI uses `golangci/golangci-lint-action@v9`.

## CI Pipeline

GitHub Actions (`.github/workflows/ci.yml`) runs on push/PR to main:

1. **Lint** -- formatting check, `go vet`, golangci-lint
2. **Test** -- `make test` + `make test-cover` on ubuntu and macOS
3. **Build** -- cross-compile zhi, marketplace, and mirror (linux/darwin × amd64/arm64) with `CGO_ENABLED=0`
4. **Proto Check** -- ensures generated `*.pb.go` files are committed
5. **Integration** -- builds all binaries, runs `test/` if test files exist

## Plugin Release Pipeline

GitHub Actions (`.github/workflows/release-plugins.yml`) runs on tag push (`v*`) or manual dispatch:

- Cross-compiles all Go example plugins for linux/darwin × amd64/arm64
- Publishes each as a multi-platform OCI artifact to `ghcr.io/mrwong99/zhi/<plugin-name>:<tag>` using `zhi plugin publish` or `zhi workspace publish`
- Signs artifacts with keyless cosign via Sigstore Fulcio/OIDC (GitHub Actions OIDC identity)
- Each example plugin has a `zhi-plugin.yaml` manifest with short name + type + multi-platform binaries
- Install published plugins: `zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-memory:v0.0.9`

## Architecture

### CLI Binaries

The project builds three binaries from `cmd/`:

- **`cmd/zhi/`** -- main CLI tool for configuration management
- **`cmd/zhi-marketplace/`** -- marketplace/sharing registry server with auth (`auth/`), HTTP API (`server/`), and data storage (`storage/`)
- **`cmd/zhi-mirror/`** -- enterprise air-gapped mirror with OCI storage, import/export for disconnected environments

### Plugin System

All plugins share a handshake (`pkg/zhiplugin/plugin.go`): magic cookie `ZHI_PLUGIN` with value `zhiplugin-v1`, protocol version 1.

Methods for unsupported capabilities should return a descriptive error.

UI plugins use bidirectional gRPC: the host calls the plugin to start the UI, and the plugin calls back into the host via a `Controller` interface. The Controller provides:
- Core operations: `LoadTree`, `SetValue`, `Validate`, `SaveTree`
- Export/Apply: `ExportTemplates`, `Export`, `Apply` (streaming output via callback)
- Components: `ListComponents`, `EnableComponent`, `DisableComponent`, `WorkspaceName`
- Marketplace: `SearchMarketplace`, `GetMarketplaceDetail`, `InstallPlugin`, `UninstallPlugin`, `ListInstalledPlugins`, `CheckUpdates`, `UpdatePlugin`, `RatePlugin`

TTY-dependent UIs (like the TUI) must set `RequiresTTY: true` and be registered as built-in plugins, since external gRPC processes lack terminal access.

### Meta-Plugin SDK

A meta-plugin is a regular plugin binary that internally launches and composes child plugins using `pkg/zhiplugin/launch/`. The SDK provides three building blocks:

- **Launch** (`pkg/zhiplugin/launch/`) -- start child plugin processes with binary validation and integrity auditing
- **Delegate** (`{config,transform,store}/delegate.go`) -- forward all interface calls to a base plugin, override selectively
- **Compose** (`config/compose.go`, `store/compose.go`) -- structural patterns: `MergedPlugin` (namespace config plugins by prefix) and `MirroredPlugin` (primary + backup stores)

See `docs/plugin-development/meta-plugin.md` for the full guide and `examples/zhi-store-mirror/` for a working example.

### Configuration Tree Model

`config.Tree` is a flat key-value store with slash-delimited paths (e.g. `database/host`, `app/tls/cert.pem`). Path segments must match `[a-z][a-z0-9._-]*[a-z0-9]`. `Tree` implements `TreeReader` (read-only) and also exposes `GetPtr` for mutable access and `Delete` for removal. `Value` holds the data (`Val any`), optional `Metadata`, and local `Validators` (closures that never cross the gRPC wire).

Validation results carry a `Severity`: Info, Warning, or Blocking.

### UI Abstraction Layer

The `UIDriver` interface (`internal/ui/driver.go`) decouples the engine from UI frontends. The `UIController` wraps the engine with UI-relevant operations. The TUI (`internal/ui/tui/`) is the first implementation using Bubbletea with views for tree browsing, value editing, validation, export, apply, component management, and marketplace. The architecture supports adding Web, AI Chat, or headless API frontends without engine changes.

### Export System

Template-based rendering (`internal/core/export.go`) using `text/template` with Sprig functions. Supports built-in formats (JSON, YAML, TOML, dotenv) and custom templates. Component-aware: only enabled components' paths are included by default.

### gRPC Layer

Proto definitions: `api/proto/zhiplugin/v1/` (config.proto, transform.proto, store.proto, ui.proto). Generated Go stubs go to `pkg/zhiplugin/{plugin}/proto/`. After editing `.proto` files, run `make proto` and commit the generated `*.pb.go` files. Configuration values are JSON-encoded for wire transfer.

Each plugin type has `grpc_client.go` (host-side) and `grpc_server.go` (plugin-side) implementing the translation between the Go interface and protobuf messages.

## Key Directories

- `cmd/zhi/` -- main CLI entry point
- `cmd/zhi-marketplace/` -- marketplace registry server
- `cmd/zhi-mirror/` -- enterprise air-gapped mirror
- `internal/core/` -- engine, registry, components, export, apply, plugin discovery
- `internal/cli/` -- CLI subcommands (Cobra)
- `internal/ui/` -- UI abstraction layer and TUI implementation
- `pkg/zhiplugin/` -- public plugin framework (config, transform, store, ui, labels, launch)
- `pkg/providers/` -- built-in provider implementations (structuredfile config, vault store)
- `pkg/sharing/` -- plugin distribution: manifests, lockfiles, OCI client, verification, marketplace
- `api/proto/zhiplugin/v1/` -- protobuf service definitions
- `examples/` -- working plugin examples (pokedex config, memory/json/vault stores, transform, HTTP API UI, store mirror meta-plugin)
- `docs/user-guide/` -- end-user documentation
- `docs/plugin-development/` -- plugin developer documentation
- `docs/design/` -- design documents (e.g., metadata-labels API)
- `test/` -- integration/E2E tests (placeholder; CI skips if empty)

## Workspace Configuration

A workspace is defined by `zhi.yaml` containing: config provider, transform providers, store provider, component definitions, export templates, apply commands, and plugin directories. See `docs/user-guide/workspace-configuration.md` for the full reference.

## Conventions

- **Testing:** all tests use `-race -count=1`. Coverage outputs to `bin/coverage.out`. Test files live alongside source code as `*_test.go`.
- **Formatting:** `gofmt -s` (enforced in CI). No tabs-vs-spaces debate -- gofmt decides.
- **Proto workflow:** edit `.proto` in `api/proto/zhiplugin/v1/`, run `make proto` (downloads protoc v33.5 automatically), commit the generated `*.pb.go`.
- **Static linking:** CI builds with `CGO_ENABLED=0` for portable binaries.
- **Version injection:** version, commit, and date are injected via `-ldflags` at build time.

## Prerequisites

Go 1.26+, golangci-lint. Install Go tools with `make tools`. Protoc is downloaded automatically by `make proto` (no system install required).
