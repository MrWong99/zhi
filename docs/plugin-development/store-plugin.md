# Store Plugin

The store plugin API lets a zhi plugin persist, retrieve, and delete
configuration values within named trees. Store plugins act as the
storage layer for the configuration system, with granular value-level
operations and optional support for versioning (tree-level or
value-level), encryption at rest, authentication, and access control.
Each plugin communicates with the host over gRPC using
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).

## Package layout

```
pkg/zhiplugin/
  plugin.go              shared Handshake (all plugin types)
  store/
    store.go             types: EncryptionStatus, VersioningMode, Capabilities,
                         AuthMethod, AuthField, Credential, PutOptions,
                         Action, Permission
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

### VersioningMode

Describes how a store plugin versions data.

```go
type VersioningMode int

const (
    VersioningNone  VersioningMode = iota
    VersioningTree
    VersioningValue
)
```

| Value             | Meaning                                                          |
|-------------------|------------------------------------------------------------------|
| `VersioningNone`  | store does not support versioning                                |
| `VersioningTree`  | entire tree is versioned atomically; rollbacks restore the whole tree |
| `VersioningValue` | individual values are versioned independently with their own history |

### Capabilities

Returned by the `Capabilities` method to report what a store supports.

```go
type Capabilities struct {
    Versioning    VersioningMode
    Encryption    EncryptionStatus
    Auth          bool
    AccessControl bool
}
```

### AuthMethod and AuthField

Describe authentication methods and their credential fields.

```go
type AuthMethod struct {
    Type        string      // e.g. "userpass", "token", "oidc"
    Description string
    Fields      []AuthField
}

type AuthField struct {
    Name        string // e.g. "username", "password", "token"
    Description string
    Required    bool
    Secret      bool   // true for passwords and tokens
}
```

### Credential

Returned after a successful `Login`.

```go
type Credential struct {
    Token     string
    ExpiresAt string            // optional RFC3339 timestamp
    Metadata  map[string]string // optional session metadata
}
```

### PutOptions

Controls optional behaviour of `PutValues`, including compare-and-swap
(CAS) semantics.

```go
type PutOptions struct {
    CASVersion  string            // tree-level CAS (VersioningTree)
    CASVersions map[string]string // per-value CAS (VersioningValue)
}
```

When `CASVersion` is non-empty the write fails if the current tree
version differs. When `CASVersions` maps a path to an expected version,
the write for that path fails if it does not match. `opts` may be `nil`
when CAS is not needed.

### Action and Permission

Used by the access control methods.

```go
type Action int

const (
    ActionRead Action = iota
    ActionWrite
    ActionDelete
)

type Permission struct {
    Path    string   // exact path or prefix ending in "/"
    Actions []Action
}
```

### What gets stored

Store plugins persist individual configuration values within named
trees. Each tree is identified by a string ID (e.g. `"production"`,
`"staging"`).

Only `Val` and `Metadata` are stored. The `Validators` field on
`config.Value` is tagged `json:"-"` and is automatically excluded
during serialisation. This is by design -- validator closures are
in-process logic that cannot be serialised over gRPC.

### Tree identification

Trees are identified by opaque string IDs. The store plugin decides
how to map IDs to its internal storage (file paths, Vault paths,
database keys, etc.).

## Plugin interface

To create a store plugin, implement `store.Plugin`:

```go
type Plugin interface {
    // --- Capabilities ---
    Capabilities(ctx context.Context) (*Capabilities, error)

    // --- Authentication ---
    AuthMethods(ctx context.Context) ([]AuthMethod, error)
    Login(ctx context.Context, method string, credentials map[string]string) (*Credential, error)

    // --- Tree management ---
    ListTrees(ctx context.Context) ([]string, error)
    DeleteTree(ctx context.Context, id string) error

    // --- Value operations ---
    GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error)
    PutValues(ctx context.Context, id string, values map[string]config.Value, opts *PutOptions) error
    DeleteValues(ctx context.Context, id string, paths []string) error

    // --- Tree-level versioning (VersioningTree mode) ---
    ListTreeVersions(ctx context.Context, id string) ([]string, error)
    GetTreeVersion(ctx context.Context, id string, version string, paths []string) (map[string]config.Value, error)
    RollbackTree(ctx context.Context, id string, version string) error
    DeleteTreeVersion(ctx context.Context, id string, version string) error

    // --- Value-level versioning (VersioningValue mode) ---
    ListValueVersions(ctx context.Context, id string, path string) ([]string, error)
    GetValueVersion(ctx context.Context, id string, path string, version string) (config.Value, bool, error)
    RollbackValue(ctx context.Context, id string, path string, version string) error
    DeleteValueVersion(ctx context.Context, id string, path string, version string) error

    // --- Encryption ---
    InitEncryption(ctx context.Context, passphrase []byte) error
    RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error

    // --- Access control ---
    GrantAccess(ctx context.Context, id string, user string, permissions []Permission) error
    RevokeAccess(ctx context.Context, id string, user string, paths []string) error
    ListAccess(ctx context.Context, id string) (map[string][]Permission, error)
}
```

### Capabilities

| Method         | Purpose                                                    |
|----------------|------------------------------------------------------------|
| `Capabilities` | report supported features: versioning mode, encryption state, auth, and access control |

The host calls `Capabilities` first to discover which parts of the
interface apply. Methods in unsupported sections should return
descriptive errors (e.g. `"versioning not supported"`).

### Authentication

| Method        | Purpose                                               |
|---------------|-------------------------------------------------------|
| `AuthMethods` | return the authentication methods supported by the store |
| `Login`       | authenticate with a method and credentials; the plugin stores the session internally |

Authentication is optional. Plugins that do not require auth should
return `nil` or an empty slice from `AuthMethods` and an error from
`Login`.

### Tree management

| Method       | Purpose                                              |
|--------------|------------------------------------------------------|
| `ListTrees`  | return all tree IDs the current identity has access to |
| `DeleteTree` | remove an entire tree and all its versions/values    |

### Value operations

| Method         | Purpose                                                        |
|----------------|----------------------------------------------------------------|
| `GetValues`    | retrieve specific values from a tree by path; missing paths are omitted |
| `PutValues`    | create or update specific values; other paths are unaffected   |
| `DeleteValues` | remove specific values from a tree                             |

Value operations are the core of the store API. Unlike the previous
tree-level API, individual paths can be read, written, and deleted
without loading or saving the entire tree. This enables more efficient
backends (e.g. Vault KV, databases) and fine-grained access control.

`PutValues` accepts an optional `*PutOptions` for compare-and-swap
semantics. Pass `nil` when CAS is not needed.

### Tree-level versioning (VersioningTree)

| Method               | Purpose                                              |
|----------------------|------------------------------------------------------|
| `ListTreeVersions`   | return version IDs for a tree, ordered newest first  |
| `GetTreeVersion`     | retrieve specific values from a specific tree version |
| `RollbackTree`       | restore the tree to a previous version (creates a new version) |
| `DeleteTreeVersion`  | permanently remove a single tree version             |

These methods apply when `Capabilities` reports `VersioningTree`. The
entire tree is versioned atomically. Plugins that do not support this
mode should return errors.

Version identifiers are opaque strings. Implementations can use
timestamps, sequential numbers, UUIDs, or any other scheme. The only
requirement is that `ListTreeVersions` returns them newest first.

### Value-level versioning (VersioningValue)

| Method                | Purpose                                                |
|-----------------------|--------------------------------------------------------|
| `ListValueVersions`   | return version IDs for a specific value, newest first |
| `GetValueVersion`     | retrieve a specific version of a single value         |
| `RollbackValue`       | restore a value to a previous version (creates a new version) |
| `DeleteValueVersion`  | permanently remove a single value version             |

These methods apply when `Capabilities` reports `VersioningValue`.
Each value has its own independent version history. Plugins that do
not support this mode should return errors.

`GetValueVersion` follows the `(value, found, error)` pattern --
`found` is `false` when the version does not exist.

### Encryption at rest

| Method             | Purpose                                             |
|--------------------|-----------------------------------------------------|
| `InitEncryption`   | initialize encryption with a passphrase             |
| `RotateEncryption` | re-encrypt all data with a new passphrase           |

Encryption is optional. Plugins that do not support encryption should
report `EncryptionNone` via `Capabilities` and return errors from
these methods.

Passphrases are transmitted as raw bytes. Since go-plugin communicates
over stdio (local process boundary, not a network socket), this is
acceptable.

### Access control

| Method         | Purpose                                                       |
|----------------|---------------------------------------------------------------|
| `GrantAccess`  | grant a user access to specific paths within a tree           |
| `RevokeAccess` | remove a user's access to specific paths within a tree        |
| `ListAccess`   | return current access permissions for a tree, keyed by user   |

Access control is optional. Plugins that do not support it should
report `AccessControl: false` via `Capabilities` and return errors
from these methods.

Permissions specify a path (exact or prefix ending in `"/"`) and a
list of allowed actions (`ActionRead`, `ActionWrite`, `ActionDelete`).

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

### GetValues

1. The host calls `GRPCClient.GetValues(id, paths)`.
2. The plugin retrieves the requested values from its backend.
3. The server serialises each value into `TreeEntry` proto messages
   with JSON-encoded value and metadata.
4. The client reconstructs a `map[string]config.Value` from the
   entries and returns it to the host.

### PutValues

1. The host calls `GRPCClient.PutValues(id, values, opts)`.
2. The client serialises the values map into `TreeEntry` proto
   messages. CAS fields from `PutOptions` are included in the request.
3. The plugin-side server deserialises the entries and passes them to
   the implementation's `PutValues` method.

### DeleteValues / DeleteTree

1. The host calls `GRPCClient.DeleteValues(id, paths)` or
   `DeleteTree(id)`.
2. The plugin removes the data from its backend.
3. An empty response is returned.

### Versioned reads

Tree-version reads (`GetTreeVersion`) and value-version reads
(`GetValueVersion`) follow the same serialisation pattern as
`GetValues` -- values are JSON-encoded into `TreeEntry` messages or
individual value/metadata JSON fields.

Values are always JSON-encoded for wire transfer using `TreeEntry`
proto messages, reusing the same serialisation helpers as config and
transform plugins. Validator closures are excluded automatically.

## Structured error types

The `store` package provides structured error types for common failure
scenarios. Plugin implementations should return these errors (or wrap
them with `fmt.Errorf("...: %w", err)`) so that the engine and UI can
present meaningful feedback without string-matching.

### Error types

| Type                         | When to return                                                |
|------------------------------|---------------------------------------------------------------|
| `*ErrCASConflict`            | compare-and-swap version mismatch during `PutValues`          |
| `*ErrAuthRequired`           | authentication is required or the session has expired         |
| `*ErrAccessDenied`           | insufficient permissions for the requested operation          |
| `*ErrPathNotFound`           | the requested path does not exist in the tree                 |
| `*ErrEncryptionNotInitialized` | an operation requiring encryption was attempted before setup |
| `*ErrNotSupported`           | a feature is not supported by the plugin's capabilities       |

### Creating errors

```go
// CAS conflict with details
return &store.ErrCASConflict{
    Path:     "database/host",
    Expected: "v3",
    Actual:   "v5",
}

// Authentication required
return &store.ErrAuthRequired{Reason: "token expired"}

// Access denied
return &store.ErrAccessDenied{Path: "secrets/", Action: "write"}

// Path not found
return &store.ErrPathNotFound{TreeID: "production", Path: "missing/key"}

// Encryption not initialized
return &store.ErrEncryptionNotInitialized{}

// Feature not supported
return &store.ErrNotSupported{Feature: "versioning"}
```

### Checking errors

Each error type has a corresponding `Is*` helper that uses
`errors.As` to match wrapped errors:

```go
err := plugin.PutValues(ctx, id, values, opts)
if store.IsCASConflict(err) {
    // handle version mismatch -- show diff, let user retry
}
if store.IsAuthRequired(err) {
    // prompt for credentials
}
if store.IsAccessDenied(err) {
    // show permission error
}
if store.IsNotSupported(err) {
    // feature unavailable
}
```

### gRPC status code mapping

Structured errors are automatically mapped to gRPC status codes when
crossing the plugin boundary:

| Error type                   | gRPC code              |
|------------------------------|------------------------|
| `ErrCASConflict`             | `codes.Aborted`        |
| `ErrAuthRequired`            | `codes.Unauthenticated`|
| `ErrAccessDenied`            | `codes.PermissionDenied`|
| `ErrPathNotFound`            | `codes.NotFound`       |
| `ErrEncryptionNotInitialized`| `codes.FailedPrecondition`|
| `ErrNotSupported`            | `codes.Unimplemented`  |

The gRPC server (`grpc_server.go`) converts structured errors to status
codes before sending, and the gRPC client (`grpc_client.go`) converts
status codes back to structured errors on receipt. This means plugin
implementations can return the Go error types directly and callers on
the host side can use the `Is*` helpers transparently.

## Backend mapping examples

| Backend           | ID mapping                                       | Versioning           |
|-------------------|--------------------------------------------------|----------------------|
| Vault KV v2       | mount path + ID as secret path                   | tree (native versions) |
| Local filesystem  | directory per ID, JSON/YAML file per version     | tree (filename-based)  |
| PostgreSQL        | table row keyed by ID, column per path           | value (row per version)|
| S3                | bucket prefix + ID as key prefix                 | tree (S3 versioning)   |
| etcd              | key prefix per tree, key per path                | value (revision-based) |
