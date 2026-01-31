<img src="assets/logo.png" alt="zhi logo" width="120" align="right" />

# Contributing to zhi

Thank you for your interest in contributing to **zhi**! This document explains how
to set up a local development environment, run common tasks, and submit changes.

## Prerequisites

| Tool | Purpose | Install |
|------|---------|---------|
| **Go 1.24+** | Build & test | [go.dev/dl](https://go.dev/dl/) |
| **protoc** | Compile `.proto` files | [protobuf releases](https://github.com/protocolbuffers/protobuf/releases) |
| **protoc-gen-go** | Go code generation for protobuf | `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` |
| **protoc-gen-go-grpc** | Go gRPC stub generation | `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` |
| **golangci-lint** *(optional)* | Linter | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |

You can install all Go-based tools at once with:

```sh
make tools
```

## Getting started

```sh
# Clone the repository
git clone https://github.com/MrWong99/zhi.git
cd zhi

# Download dependencies
make deps

# Run the full check suite (format, vet, lint, test)
make check
```

## Makefile reference

Run `make help` to see all available targets. The most important ones are listed
below.

### Build

| Target | Description |
|--------|-------------|
| `make build` | Build the `zhi` binary into `bin/` |
| `make build-examples` | Build all example plugins into `bin/examples/` |
| `make build-all` | Build the main binary and all examples |
| `make install` | Install the `zhi` binary into `$GOPATH/bin` |

### Test

| Target | Description |
|--------|-------------|
| `make test` | Run all tests with race detection |
| `make test-short` | Run tests in short mode (skip long-running tests) |
| `make test-verbose` | Run all tests with verbose output |
| `make test-cover` | Run tests and print a coverage summary |
| `make test-cover-html` | Generate an HTML coverage report in `bin/coverage.html` |
| `make bench` | Run benchmarks |

### Code generation

| Target | Description |
|--------|-------------|
| `make proto` | Regenerate Go code from `.proto` files |
| `make proto-check` | Verify generated proto code is up-to-date (useful in CI) |
| `make generate` | Run `make proto` followed by `go generate ./...` |

### Lint & format

| Target | Description |
|--------|-------------|
| `make fmt` | Format all Go source files with `gofmt -s` |
| `make vet` | Run `go vet` |
| `make lint` | Run `golangci-lint` |
| `make lint-fix` | Run `golangci-lint` with auto-fix |
| `make check` | Run format, vet, lint, and test in sequence |

### Housekeeping

| Target | Description |
|--------|-------------|
| `make tidy` | Run `go mod tidy` |
| `make deps` | Download module dependencies |
| `make tools` | Install required development tools |
| `make clean` | Remove build artifacts and caches |

## Project layout

```
api/proto/              Proto definitions for plugin gRPC services
cmd/zhi/                Main CLI entry point
docs/
  user-guide/           End-user documentation (CLI usage, workspace config, etc.)
  plugin-development/   Plugin developer documentation (APIs, examples, testing)
examples/               Example plugin implementations
internal/
  cli/                  CLI subcommands (Cobra)
  core/                 Engine, registry, components, export, apply, discovery
  ui/                   UI abstraction layer and TUI implementation
pkg/zhiplugin/          Public plugin framework (config, transform, store, ui)
pkg/providers/          Built-in provider implementations
test/                   Integration / end-to-end tests
```

## Working with Protocol Buffers

Proto definitions live under `api/proto/zhiplugin/v1/`. After editing a `.proto`
file, regenerate the Go code:

```sh
make proto
```

This runs `protoc` with `--go_out` and `--go-grpc_out` and places the generated
files alongside the Go packages that consume them:

- `api/proto/zhiplugin/v1/config.proto` -> `pkg/zhiplugin/config/proto/`
- `api/proto/zhiplugin/v1/transform.proto` -> `pkg/zhiplugin/transform/proto/`
- `api/proto/zhiplugin/v1/store.proto` -> `pkg/zhiplugin/store/proto/`
- `api/proto/zhiplugin/v1/ui.proto` -> `pkg/zhiplugin/ui/proto/`

Before committing, verify the generated code is up-to-date:

```sh
make proto-check
```

## Writing a plugin

zhi uses [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) with gRPC
transport. There are four plugin types:

- **Config plugin** -- implements `config.Plugin` (List, Get, Set, Validate)
- **Transform plugin** -- implements `transform.Plugin` (BeforeDisplay, AfterSave, ValidatePolicy)
- **Store plugin** -- implements `store.Plugin` (Save, Load, Delete, ListTrees, versioning, encryption)
- **UI plugin** -- implements `ui.Plugin` (Run, Capabilities) with bidirectional gRPC

Look at the `examples/` directory for working reference implementations:

- `examples/zhi-config-pokedex/` -- a config plugin that manages Pokedex settings
- `examples/zhi-transform-pokedex/` -- a transform plugin that evolves starter Pokemon
- `examples/zhi-store-json/` -- a store plugin persisting trees as JSON files
- `examples/zhi-store-memory/` -- a minimal in-memory store plugin
- `examples/zhi-ui-httpapi/` -- a UI plugin exposing an HTTP/JSON API

Each example includes tests that use `goplugin.TestPluginGRPCConn()` for
in-process gRPC testing without starting a subprocess.

See the [Plugin Development docs](docs/plugin-development/overview.md) for the
full API reference.

## Submitting changes

1. **Fork** the repository and create a feature branch from `main`.
2. Make your changes. Keep commits focused and well-described.
3. Run the full check suite before pushing:
   ```sh
   make check
   ```
4. If you changed `.proto` files, regenerate and commit the generated code:
   ```sh
   make proto
   ```
5. Open a **Pull Request** against `main`. Describe what your change does and
   why.

## Release process

zhi uses [GoReleaser](https://goreleaser.com/) to automate cross-compilation and
release packaging. Pushing a version tag triggers the release workflow
automatically.

**Release checklist:**

1. Ensure `main` is green (all CI checks pass).
2. Create and push a version tag:
   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```
3. GoReleaser automatically builds and creates a **draft** GitHub Release.
4. Review the draft release and edit release notes if needed.
5. Publish the release.

**Local testing:**

```sh
# Build a snapshot release locally (no publish)
make snapshot

# Dry-run release (builds everything, skips publish)
make release-dry-run
```

**Versioning policy:**

- Follow [Semantic Versioning](https://semver.org/) (SemVer).
- Pre-1.0: breaking changes may occur in minor versions.
- Plugin protocol version is tracked separately (`ProtocolVersion` in handshake).

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Keep exported APIs documented with GoDoc comments.
- Write tests for new functionality. Aim to maintain or improve coverage.
- Prefer table-driven tests where appropriate.
- Generated files (`*.pb.go`) should not be edited by hand.

## License

By contributing to zhi you agree that your contributions will be licensed under
the [MIT License](LICENSE).
