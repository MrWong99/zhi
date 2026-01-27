# Configuration Plugin

The configuration plugin API lets a zhi plugin manage a set of typed,
validated configuration values. Each plugin owns a slice of the global
configuration tree and communicates with the host over gRPC using
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).

## Package layout

```
pkg/zhiplugin/
  plugin.go              shared Handshake (all plugin types)
  config/
    config.go            Value, Tree, TreeReader, Severity, validation types
    plugin.go            Plugin interface, PluginMap, GRPCPlugin wiring
    grpc_client.go       host-side gRPC client
    grpc_server.go       plugin-side gRPC server
    proto/               generated protobuf / gRPC stubs
```

## Core types

### Value

A single node in the configuration tree.

```go
type Value struct {
    Val        any            `json:"value"    yaml:"value"`
    Metadata   map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
    Validators []ValidateFunc `json:"-"        yaml:"-"`
}
```

- **Val** can be any type that marshals to JSON and YAML (primitives, slices,
  maps, structs with tags).
- **Metadata** is optional, free-form key-value data (descriptions, display
  hints, etc.).
- **Validators** are local functions that never cross the wire. They are only
  used inside the plugin process.

### Tree and TreeReader

`Tree` is a flat map keyed by slash-delimited paths. It implements the
read-only `TreeReader` interface:

```go
type TreeReader interface {
    Get(path string) (Value, bool)
    List() []string
}
```

`Get` returns a **copy** of the value, so callers cannot mutate tree
internals.

### Path format

Paths use `/` as the hierarchy separator. Each segment must match:

```
[a-z][a-z0-9._-]*[a-z0-9]
```

Single-character segments like `a` are valid. Examples:

```
pokedex/trainer.name
pokedex/region
database/host
app/tls/cert.pem
```

This scheme maps naturally to storage backends:

| Backend      | Mapping                                         |
|--------------|-------------------------------------------------|
| Vault        | use path as-is under the secret mount           |
| K8s Secret   | replace `/` with `-`                            |
| Env variable | uppercase, replace `/` and `.` with `_`         |

### Severity and ValidationResult

Validation produces results with three severity levels:

| Severity   | Meaning                                     |
|------------|---------------------------------------------|
| `Info`     | informational, does not block               |
| `Warning`  | potential problem, does not block           |
| `Blocking` | prevents the configuration from being used  |

```go
type ValidationResult struct {
    Severity Severity       `json:"severity"`
    Message  string         `json:"message"`
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

### ValidateFunc

```go
type ValidateFunc func(value any, tree TreeReader) []ValidationResult
```

Receives the value under validation **and** a read-only view of the full
configuration tree. This enables both single-value checks and cross-value
validation (e.g. "port must be > 1024 when host is localhost"). Return an
empty or nil slice when the value is valid.

## Plugin interface

To create a configuration plugin, implement `config.Plugin`:

```go
type Plugin interface {
    List(ctx context.Context) ([]string, error)
    Get(ctx context.Context, path string) (Value, bool, error)
    Set(ctx context.Context, path string, v Value) error
    Validate(ctx context.Context, path string, tree TreeReader) ([]ValidationResult, error)
}
```

| Method     | Purpose                                                      |
|------------|--------------------------------------------------------------|
| `List`     | return every path this plugin manages                        |
| `Get`      | retrieve a single value by path                              |
| `Set`      | store a value at the given path                              |
| `Validate` | check one value, with full tree access for cross-value rules |

## Wiring a plugin binary

A plugin binary needs a `main` function that calls `plugin.Serve`:

```go
package main

import (
    "github.com/hashicorp/go-plugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]plugin.Plugin{
            "config": &config.GRPCPlugin{Impl: &myPlugin{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

See the [Pokedex example](../examples/pokedex-config/) for a complete,
runnable plugin.

## How validation works over gRPC

1. The host snapshots the full merged tree (all plugins' values).
2. For each plugin, for each of its paths, the host calls `Validate` with the
   path and the snapshot.
3. The gRPC client serialises the tree as JSON-encoded `TreeEntry` messages.
4. The plugin-side server reconstructs a local `Tree` and passes it to the
   implementation's `Validate` method.
5. The plugin runs its `ValidateFunc` closures internally (they never cross
   the wire) and returns the results.

This keeps things simple and works well for configuration data, which is
typically small.
