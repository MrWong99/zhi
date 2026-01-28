---
name: config-plugin
description: Implement a config plugin/provider in pkg/providers/config/. Use when creating a new config provider, implementing the config.Plugin interface, or working on config plugin gRPC integration.
---

# Config Plugin Implementation Guide

## Interface (`pkg/zhiplugin/config/plugin.go`)

```go
type Plugin interface {
    List(ctx context.Context) ([]string, error)
    Get(ctx context.Context, path string) (Value, bool, error)
    Set(ctx context.Context, path string, v Value) error
    Validate(ctx context.Context, path string, tree TreeReader) ([]ValidationResult, error)
}
```

## Key Types (`pkg/zhiplugin/config/config.go`)

```go
type Value struct {
    Val        any               // JSON/YAML-compatible data
    Metadata   map[string]any    // Optional metadata
    Validators []ValidateFunc    // Local only, NEVER crosses gRPC wire
}

type TreeReader interface {
    Get(path string) (Value, bool)  // Returns copy
    List() []string
}

type ValidationResult struct {
    Severity Severity       // Info (0), Warning (1), Blocking (2)
    Message  string
    Metadata map[string]any // Optional, useful for UI hints
}
```

**Path format:** slash-delimited, segments match `[a-z][a-z0-9._-]*[a-z0-9]`. Use `config.ValidatePath()` to check.

## Registration Boilerplate

```go
package main

import (
    goplugin "github.com/hashicorp/go-plugin"
    "github.com/itsluketwist/zhi/pkg/zhiplugin"
    "github.com/itsluketwist/zhi/pkg/zhiplugin/config"
)

func main() {
    goplugin.Serve(&goplugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake, // ZHI_PLUGIN=zhiplugin-v1, protocol v1
        Plugins: map[string]goplugin.Plugin{
            "config": &config.GRPCPlugin{Impl: &MyPlugin{}},
        },
        GRPCServer: goplugin.DefaultGRPCServer,
    })
}
```

## Implementation Patterns

- **Thread safety:** Protect shared state with `sync.RWMutex` (RLock for List/Get/Validate, Lock for Set).
- **Get returns copies:** Return `*v` not `v` when storing pointers internally.
- **Validate with tree context:** Use `tree.Get(otherPath)` for cross-value validation. Return multiple `ValidationResult` entries with appropriate severity.
- **Validators field:** `Value.Validators` are closures and never serialize over gRPC. Only use them for local (in-process) validation. Plugin-side validation goes in the `Validate` method.
- **Wire format:** Values are JSON-encoded (`Val` and `Metadata` separately). Ensure `Val` holds JSON-compatible types.

## Reference Files

- Interface & types: `pkg/zhiplugin/config/plugin.go`, `pkg/zhiplugin/config/config.go`
- gRPC layer: `pkg/zhiplugin/config/grpc_client.go`, `pkg/zhiplugin/config/grpc_server.go`
- Proto: `api/proto/zhiplugin/v1/config.proto`
- Existing provider: `pkg/providers/config/structuredfile/structuredfile.go`
- Example: `examples/pokedex-config/main.go`
