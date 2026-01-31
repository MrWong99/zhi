---
name: transform-plugin
description: Implement a transform plugin/provider in pkg/providers/transform/. Use when creating a new transform provider, implementing the transform.Plugin interface, or working on tree mutation logic.
---

# Transform Plugin Implementation Guide

## Interface (`pkg/zhiplugin/transform/plugin.go`)

```go
type Plugin interface {
    BeforeDisplay(ctx context.Context, tree *config.Tree) error
    AfterSave(ctx context.Context, tree *config.Tree) error
    ValidatePolicy(ctx context.Context) (ValidatePolicy, error)
}
```

## ValidatePolicy (`pkg/zhiplugin/transform/transform.go`)

```go
const (
    ValidateBeforeTransform ValidatePolicy = iota // 0: validate base values first
    ValidateAfterTransform                        // 1: validate transformed values
    ValidateBoth                                  // 2: validate before and after
)
```

Controls when config-plugin validations run relative to transformations.

## Tree Mutation (`pkg/zhiplugin/config/config.go`)

The `*config.Tree` parameter is fully mutable:

```go
tree.GetPtr(path) (*Value, bool)  // Mutable pointer for in-place edits
tree.Get(path) (Value, bool)      // Read-only copy
tree.Set(path, *Value) error      // Add or overwrite a path
tree.Delete(path)                 // Remove a path
tree.List() []string              // Enumerate all paths
```

**Key pattern:** Use `GetPtr()` to mutate existing values in place, `Set()` to add new paths, `Delete()` to remove paths.

The gRPC layer serializes the full tree, applies transforms, and uses `applyTree()` to sync changes back: paths removed in the transform are deleted from the original, and all transformed values overwrite originals.

## Registration Boilerplate

```go
package main

import (
    goplugin "github.com/hashicorp/go-plugin"
    "github.com/itsluketwist/zhi/pkg/zhiplugin"
    "github.com/itsluketwist/zhi/pkg/zhiplugin/config"
    "github.com/itsluketwist/zhi/pkg/zhiplugin/transform"
)

func main() {
    goplugin.Serve(&goplugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake, // ZHI_PLUGIN=zhiplugin-v1, protocol v1
        Plugins: map[string]goplugin.Plugin{
            "transform": &transform.GRPCPlugin{Impl: &MyTransform{}},
        },
        GRPCServer: goplugin.DefaultGRPCServer,
    })
}
```

## Implementation Patterns

- **BeforeDisplay:** Add computed/derived values, format for display, enrich data. Runs before the UI sees the tree.
- **AfterSave:** Reverse display transformations, normalize stored values, enforce invariants. Runs after the user saves.
- **Symmetry:** If BeforeDisplay adds/modifies paths, AfterSave should typically undo those changes so the base tree stays clean.
- **Thread safety:** Transforms receive a tree copy per call, but protect any shared plugin state with mutexes.
- **Wire format:** Tree entries are serialized as `(path, value_json, metadata_json)` tuples. Values must be JSON-compatible.

## Reference Files

- Interface & types: `pkg/zhiplugin/transform/plugin.go`, `pkg/zhiplugin/transform/transform.go`
- gRPC layer: `pkg/zhiplugin/transform/grpc_client.go`, `pkg/zhiplugin/transform/grpc_server.go`
- Proto: `api/proto/zhiplugin/v1/transform.proto`
- Example: `examples/zhi-transform-pokedex/main.go`
