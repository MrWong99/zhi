# Store Plugin

The store plugin API lets a zhi plugin persist, retrieve, and delete
configuration trees. Store plugins act as the storage layer for the
configuration system, optionally supporting versioning and encryption
at rest. Each plugin communicates with the host over gRPC using
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).

## Package layout

```
pkg/zhiplugin/
  plugin.go              shared Handshake (all plugin types)
  store/
    store.go             EncryptionStatus type and constants
    plugin.go            Plugin interface, PluginMap, GRPCPlugin wiring
    grpc_client.go       host-side gRPC client
    grpc_server.go       plugin-side gRPC server
    proto/               generated protobuf / gRPC stubs
```

## Core types

### EncryptionStatus

Reports the encryption state of a store plugin.

```go
type EncryptionStatus int

const (
    EncryptionNone      EncryptionStatus = iota
    EncryptionSupported
    EncryptionActive
)
```

| Value                | Meaning                                          |
|----------------------|--------------------------------------------------|
| `EncryptionNone`     | store does not support encryption at rest         |
| `EncryptionSupported`| store supports encryption but it is not yet initialized |
| `EncryptionActive`   | encryption is initialized and data is encrypted   |

### What gets stored

Store plugins persist entire configuration trees. Each tree is
identified by a string ID (e.g. `"production"`, `"staging"`).

Only `Val` and `Metadata` are stored. The `Validators` field on
`config.Value` is tagged `json:"-"` and is automatically excluded
during serialisation. This is by design — validator closures are
in-process logic that cannot be serialised over gRPC.

### Tree identification

Trees are identified by opaque string IDs. The store plugin decides
how to map IDs to its internal storage (file paths, Vault paths,
database keys, etc.).

## Plugin interface

To create a store plugin, implement `store.Plugin`:

```go
type Plugin interface {
    // Core persistence
    Save(ctx context.Context, id string, tree config.TreeReader) error
    Load(ctx context.Context, id string) (*config.Tree, bool, error)
    Delete(ctx context.Context, id string) error
    ListTrees(ctx context.Context) ([]string, error)

    // Versioning (opt-in)
    SupportsVersioning(ctx context.Context) (bool, error)
    ListVersions(ctx context.Context, id string) ([]string, error)
    LoadVersion(ctx context.Context, id string, version string) (*config.Tree, bool, error)
    DeleteVersion(ctx context.Context, id string, version string) error

    // Encryption at rest
    EncryptionStatus(ctx context.Context) (EncryptionStatus, error)
    InitEncryption(ctx context.Context, passphrase []byte) error
    RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error
}
```

### Core persistence

| Method      | Purpose                                                      |
|-------------|--------------------------------------------------------------|
| `Save`      | persist a configuration tree under the given ID              |
| `Load`      | retrieve the latest version of a tree; bool reports existence|
| `Delete`    | remove a tree and all its versions                           |
| `ListTrees` | return all stored tree IDs                                   |

`Save` accepts `config.TreeReader` (read-only interface), which is
sufficient for serialisation. If the store supports versioning, each
`Save` creates a new version. If not, `Save` overwrites the previous
state.

`Load` returns a `*config.Tree` reconstructed from storage. The bool
is `false` when the tree ID does not exist — matching the `(value,
found, error)` pattern used by `config.Plugin.Get`.

### Versioning

| Method              | Purpose                                            |
|---------------------|----------------------------------------------------|
| `SupportsVersioning`| report whether this store keeps older versions     |
| `ListVersions`      | return version IDs for a tree, ordered newest first|
| `LoadVersion`       | retrieve a specific version of a tree              |
| `DeleteVersion`     | permanently remove a single version                |

Versioning is opt-in. Plugins that do not support versioning should
return `(false, nil)` from `SupportsVersioning` and return descriptive
errors from `ListVersions`, `LoadVersion`, and `DeleteVersion`.

Version identifiers are opaque strings. Implementations can use
timestamps, sequential numbers, UUIDs, or any other scheme. The only
requirement is that `ListVersions` returns them newest first.

### Encryption at rest

| Method             | Purpose                                             |
|--------------------|-----------------------------------------------------|
| `EncryptionStatus` | report the current encryption state                 |
| `InitEncryption`   | initialize encryption with a passphrase             |
| `RotateEncryption` | re-encrypt all data with a new passphrase           |

Encryption is optional. Plugins that do not support encryption should
return `EncryptionNone` from `EncryptionStatus` and return errors from
`InitEncryption` and `RotateEncryption`.

Passphrases are transmitted as raw bytes. Since go-plugin communicates
over stdio (local process boundary, not a network socket), this is
acceptable.

## Wiring a plugin binary

A plugin binary needs a `main` function that calls `plugin.Serve`:

```go
package main

import (
    "github.com/hashicorp/go-plugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]plugin.Plugin{
            "store": &store.GRPCPlugin{Impl: &myStore{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

See the [zhi-store-memory example](../../examples/zhi-store-memory/) for a
complete, runnable plugin.

## How storage works over gRPC

### Save

1. The host calls `GRPCClient.Save(id, tree)`.
2. The client serialises the `TreeReader` into `TreeEntry` proto
   messages using `config.TreeToProto`. Each entry contains the path,
   JSON-encoded value, and JSON-encoded metadata. Validators are
   excluded automatically.
3. The plugin-side server receives the entries, reconstructs a
   `*config.Tree` via `config.TreeFromProto`, and passes it as a
   `TreeReader` to the implementation's `Save` method.

### Load / LoadVersion

1. The host calls `GRPCClient.Load(id)` or `LoadVersion(id, version)`.
2. The plugin returns a `*config.Tree`.
3. The server serialises it back to `TreeEntry` messages.
4. The client reconstructs a `*config.Tree` and returns it to the host.

### Delete / DeleteVersion

1. The host calls `GRPCClient.Delete(id)` or `DeleteVersion(id, version)`.
2. The plugin removes the data from its backend.
3. An empty response is returned.

This reuses the same `TreeEntry` proto type and serialisation helpers
(`TreeToProto`, `TreeFromProto`) as config and transform plugins.

## Backend mapping examples

| Backend           | ID mapping                                       | Versioning         |
|-------------------|--------------------------------------------------|--------------------|
| Vault KV v2       | mount path + ID as secret path                   | native (versions)  |
| Local filesystem  | directory per ID, JSON/YAML file per version     | filename-based     |
| PostgreSQL        | table row keyed by ID                            | row per version    |
| S3                | bucket prefix + ID as key prefix                 | S3 versioning      |
