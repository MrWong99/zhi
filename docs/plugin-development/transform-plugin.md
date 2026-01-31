# Transform Plugin

The transform plugin API lets a zhi plugin mutate configuration values
before they are displayed in the UI and/or after the UI stores updates.
Each transform plugin receives the full, mutable configuration tree and
can control whether config-plugin validations run before or after the
transformation.

## Package layout

```
pkg/zhiplugin/
  plugin.go              shared Handshake (all plugin types)
  transform/
    transform.go         ValidatePolicy type and constants
    plugin.go            Plugin interface, PluginMap, GRPCPlugin wiring
    grpc_client.go       host-side gRPC client
    grpc_server.go       plugin-side gRPC server
    proto/               generated protobuf / gRPC stubs
```

## Core types

### ValidatePolicy

Controls when config-plugin validations are executed relative to a
transformation.

```go
type ValidatePolicy int

const (
    ValidateBeforeTransform ValidatePolicy = iota
    ValidateAfterTransform
    ValidateBoth
)
```

| Value                     | Meaning                                          |
|---------------------------|--------------------------------------------------|
| `ValidateBeforeTransform` | run config validations first, then transform     |
| `ValidateAfterTransform`  | transform first, then run config validations     |
| `ValidateBoth`            | run validations both before and after             |

### Mutable tree access

Transform plugins receive a `*config.Tree` (pointer), giving full
read-write access to the configuration tree. The key methods are:

| Method      | Signature                              | Purpose                                 |
|-------------|----------------------------------------|-----------------------------------------|
| `GetPtr`    | `(path string) (*Value, bool)`         | mutable pointer to an existing value    |
| `Get`       | `(path string) (Value, bool)`          | read-only copy of a value               |
| `Set`       | `(path string, v *Value) error`        | add or replace a value (validates path) |
| `Delete`    | `(path string)`                        | remove a value from the tree            |
| `List`      | `() []string`                          | all paths in the tree                   |

Use `GetPtr` to mutate values in place, `Set` to add new entries, and
`Delete` to remove entries. `List` enumerates all paths.

## Plugin interface

To create a transform plugin, implement `transform.Plugin`:

```go
type Plugin interface {
    BeforeDisplay(ctx context.Context, tree *config.Tree) error
    AfterSave(ctx context.Context, tree *config.Tree) error
    ValidatePolicy(ctx context.Context) (ValidatePolicy, error)
}
```

| Method            | Purpose                                                    |
|-------------------|------------------------------------------------------------|
| `BeforeDisplay`   | transform values before they are shown in the UI           |
| `AfterSave`       | transform values after the UI stores/applies updates       |
| `ValidatePolicy`  | report when config validations run relative to transforms  |

Plugins implement both `BeforeDisplay` and `AfterSave` but can no-op
either by returning `nil` immediately.

## Wiring a plugin binary

A plugin binary needs a `main` function that calls `plugin.Serve`:

```go
package main

import (
    "context"

    "github.com/hashicorp/go-plugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin/config"
    "github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

type myTransform struct{}

func (t *myTransform) BeforeDisplay(_ context.Context, tree *config.Tree) error {
    // Example: mask a secret before the UI displays it.
    if v, ok := tree.GetPtr("app/secret"); ok {
        v.Val = "********"
    }
    return nil
}

func (t *myTransform) AfterSave(_ context.Context, tree *config.Tree) error {
    // No transformation needed after save.
    return nil
}

func (t *myTransform) ValidatePolicy(_ context.Context) (transform.ValidatePolicy, error) {
    return transform.ValidateAfterTransform, nil
}

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]plugin.Plugin{
            "transform": &transform.GRPCPlugin{Impl: &myTransform{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

## How transforms work over gRPC

1. The host snapshots the full merged tree (all config plugins' values).
2. The host calls `BeforeDisplay` (or `AfterSave`) with the snapshot.
3. The gRPC client serialises the tree as JSON-encoded `TreeEntry`
   messages (reusing the same proto type as config plugins).
4. The plugin-side server reconstructs a local `*config.Tree` and passes
   it to the implementation.
5. The plugin mutates the tree in place (via `GetPtr`, `Set`, `Delete`).
6. The server serialises the modified tree back and returns it.
7. The host applies the returned tree, replacing values that changed,
   adding new paths, and removing deleted paths.

## Validation timing

The host queries `ValidatePolicy` to decide when to run config-plugin
validations relative to the transformation:

```
ValidateBeforeTransform:
  config validations → transform

ValidateAfterTransform:
  transform → config validations

ValidateBoth:
  config validations → transform → config validations
```

This lets transform plugins that depend on valid input (e.g. type
coercion) request pre-validation, while plugins that produce values
needing validation (e.g. computed defaults) request post-validation.
