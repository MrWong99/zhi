# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

zhi is a security-first platform for configuration management and provisioning, built in Go. It uses an extensible plugin system over gRPC (via hashicorp/go-plugin) with two plugin types: **config** and **transform**. Plugins run as separate processes communicating over stdio with gRPC transport.

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

### Plugin System

All plugins share a handshake (`pkg/zhiplugin/plugin.go`): magic cookie `ZHI_PLUGIN` with value `zhiplugin-v1`, protocol version 1.

**Config plugins** (`pkg/zhiplugin/config/`) implement the `Plugin` interface:
- `List(ctx) ([]string, error)` — return all managed paths
- `Get(ctx, path) (Value, bool, error)` — retrieve a value
- `Set(ctx, path, Value) error` — store a value
- `Validate(ctx, path, TreeReader) ([]ValidationResult, error)` — validate with full tree context

**Transform plugins** (`pkg/zhiplugin/transform/`) implement the `Plugin` interface:
- `BeforeDisplay(ctx, *Tree) error` — mutate tree before UI display
- `AfterSave(ctx, *Tree) error` — mutate tree after user saves
- `ValidatePolicy(ctx) (ValidatePolicy, error)` — control validation timing relative to transforms

### Configuration Tree Model

`config.Tree` is a flat key-value store with slash-delimited paths (e.g. `database/host`, `app/tls/cert.pem`). Path segments must match `[a-z][a-z0-9._-]*[a-z0-9]`. `Tree` implements `TreeReader` (read-only) and also exposes `GetPtr` for mutable access and `Delete` for removal. `Value` holds the data (`Val any`), optional `Metadata`, and local `Validators` (closures that never cross the gRPC wire).

Validation results carry a `Severity`: Info, Warning, or Blocking.

### gRPC Layer

Proto definitions: `api/proto/zhiplugin/v1/` (config.proto, transform.proto). Generated Go stubs go to `pkg/zhiplugin/{config,transform}/proto/`. After editing `.proto` files, run `make proto` and commit the generated `*.pb.go` files. Configuration values are JSON-encoded for wire transfer.

Each plugin type has `grpc_client.go` (host-side) and `grpc_server.go` (plugin-side) implementing the translation between the Go interface and protobuf messages.

## Key Directories

- `cmd/zhi/` — CLI entry point
- `pkg/zhiplugin/` — public plugin framework (config, transform)
- `pkg/providers/` — built-in provider implementations
- `api/proto/zhiplugin/v1/` — protobuf service definitions
- `examples/` — working plugin examples
- `test/` — integration/E2E tests

## Prerequisites

Go 1.24+, protoc, protoc-gen-go, protoc-gen-go-grpc, golangci-lint. Install Go tools with `make tools`.
