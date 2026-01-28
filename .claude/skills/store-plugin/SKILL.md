---
name: store-plugin
description: Implement a store plugin/provider in pkg/providers/store/. Use when creating a new store provider, implementing the store.Plugin interface, or working on persistence, versioning, or encryption features.
---

# Store Plugin Implementation Guide

## Interface (`pkg/zhiplugin/store/plugin.go`)

```go
type Plugin interface {
    // Core persistence
    Save(ctx context.Context, id string, tree config.TreeReader) error
    Load(ctx context.Context, id string) (*config.Tree, bool, error)
    Delete(ctx context.Context, id string) error
    ListTrees(ctx context.Context) ([]string, error)

    // Versioning (return error if unsupported)
    SupportsVersioning(ctx context.Context) (bool, error)
    ListVersions(ctx context.Context, id string) ([]string, error)            // Newest first
    LoadVersion(ctx context.Context, id string, version string) (*config.Tree, bool, error)
    DeleteVersion(ctx context.Context, id string, version string) error

    // Encryption (return error if unsupported)
    EncryptionStatus(ctx context.Context) (EncryptionStatus, error)
    InitEncryption(ctx context.Context, passphrase []byte) error
    RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error
}
```

## EncryptionStatus (`pkg/zhiplugin/store/store.go`)

```go
const (
    EncryptionNone      EncryptionStatus = iota // 0: no encryption support
    EncryptionSupported                         // 1: supports but not initialized
    EncryptionActive                            // 2: initialized and active
)
```

## Registration Boilerplate

```go
package main

import (
    goplugin "github.com/hashicorp/go-plugin"
    "github.com/itsluketwist/zhi/pkg/zhiplugin"
    "github.com/itsluketwist/zhi/pkg/zhiplugin/store"
)

func main() {
    goplugin.Serve(&goplugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake, // ZHI_PLUGIN=zhiplugin-v1, protocol v1
        Plugins: map[string]goplugin.Plugin{
            "store": &store.GRPCPlugin{Impl: &MyStore{}},
        },
        GRPCServer: goplugin.DefaultGRPCServer,
    })
}
```

## Implementation Patterns

- **Save copies data:** Read from `TreeReader` with `tree.List()` and `tree.Get(path)`, then store copies. Never hold references to the passed-in tree.
- **Load returns new Tree:** Build a fresh `config.NewTree()`, populate with `Set()`, return it. The `bool` indicates existence.
- **Thread safety:** Protect shared state with `sync.RWMutex`.
- **Unsupported features:** Return descriptive errors for versioning/encryption methods when not supported. `SupportsVersioning` returns `(false, nil)`, `EncryptionStatus` returns `(EncryptionNone, nil)`.
- **Versioning:** If supported, `ListVersions` must return versions ordered newest first. Version IDs are opaque strings (timestamps, UUIDs, etc.).
- **Encryption:** `InitEncryption` sets up encryption with a passphrase. `RotateEncryption` re-encrypts all data with a new passphrase. Handle the `EncryptionSupported` vs `EncryptionActive` state transition.
- **Wire format:** Tree entries are `(path, value_json, metadata_json)` tuples. Passphrases are raw bytes.

## Reference Files

- Interface & types: `pkg/zhiplugin/store/plugin.go`, `pkg/zhiplugin/store/store.go`
- gRPC layer: `pkg/zhiplugin/store/grpc_client.go`, `pkg/zhiplugin/store/grpc_server.go`
- Proto: `api/proto/zhiplugin/v1/store.proto`
- Example: `examples/memory-store/main.go`
