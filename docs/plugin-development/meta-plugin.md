# Meta-Plugin SDK

A meta-plugin is a regular zhi plugin that internally launches and
orchestrates one or more child plugins. From the host's perspective
a meta-plugin looks like any other plugin -- it implements the same
gRPC interface. Internally, it uses the SDK helpers to launch child
processes and compose their behaviour.

The meta-plugin SDK provides three building blocks:

1. **Launch** -- start child plugin processes with security auditing
2. **Delegate** -- forward calls to a base plugin, overriding only what you need
3. **Compose** -- combine multiple plugins with structural patterns (merging, mirroring)

## Package layout

```
pkg/zhiplugin/
  launch/
    launch.go         LaunchConfig, LaunchTransform, LaunchStore
    options.go        Option, WithLogger, WithIsolatedEnv, WithAuditMode
    audit.go          AuditBinary, PluginNameFromPath, binary validation
  config/
    delegate.go       DelegatingPlugin (config)
    compose.go        MergedPlugin, MountedPlugin
  transform/
    delegate.go       DelegatingPlugin (transform)
  store/
    delegate.go       DelegatingPlugin (store)
    compose.go        MirroredPlugin
```

## Launch package

`pkg/zhiplugin/launch` provides functions for launching external plugin
binaries as child processes. It extracts the launch logic from the
internal engine so that meta-plugins can reuse the same mechanisms.

### Launch functions

Each function returns the typed plugin interface, a cleanup function
that kills the child process, and an error:

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin/launch"

// Launch a config plugin binary.
cfg, cleanup, err := launch.LaunchConfig("/path/to/zhi-config-foo", opts...)
defer cleanup()

// Launch a transform plugin binary.
tfm, cleanup, err := launch.LaunchTransform("/path/to/zhi-transform-bar", opts...)
defer cleanup()

// Launch a store plugin binary.
st, cleanup, err := launch.LaunchStore("/path/to/zhi-store-baz", opts...)
defer cleanup()
```

The caller **must** call the cleanup function when done to kill the
child process. Use `defer cleanup()` immediately after a successful
launch.

### Options

| Option | Purpose |
|--------|---------|
| `WithLogger(l)` | structured logger for audit results and lifecycle events |
| `WithIsolatedEnv(env)` | restrict child environment to handshake variables + extras |
| `WithAuditMode(mode)` | control how integrity check failures are handled |

#### WithIsolatedEnv

By default, child processes inherit the parent's full environment.
`WithIsolatedEnv` restricts the child to only `ZHI_PLUGIN`, `PATH`,
and the user-supplied map. This prevents leaking secrets or credentials
to children:

```go
launch.LaunchStore(bin,
    launch.WithIsolatedEnv(map[string]string{
        "VAULT_ADDR":  "https://vault.example.com",
        "VAULT_TOKEN": os.Getenv("VAULT_TOKEN"),
    }),
)
```

#### AuditMode

| Mode | Behaviour |
|------|-----------|
| `AuditHardFail` (default) | return an error if binary integrity check fails |
| `AuditWarnOnly` | log a warning but continue launching |

The public API defaults to `AuditHardFail`. The engine uses
`AuditWarnOnly` for backward compatibility.

### Binary security checks

Before launching a plugin, the launch package:

1. **Resolves symlinks** and rejects paths containing `..` components
2. **Computes SHA-256** of the binary and logs it
3. **Warns** if the binary is world-writable
4. **Verifies integrity** against the stored digest from installation
   time (for plugins installed via `zhi plugin install`)

If the binary was not installed via the sharing system (e.g.,
locally-built plugins), the integrity check is skipped.

### Process isolation

Child processes are launched in a separate process group
(`Setpgid: true` on Unix) so that signals sent to the parent do not
propagate directly to children. This ensures clean shutdown via the
cleanup function rather than accidental signal forwarding.

## Delegate pattern

Each plugin type provides a `DelegatingPlugin` struct that implements
the full interface by forwarding every call to a `Base` plugin. Embed
this struct and override only the methods you need:

### config.DelegatingPlugin

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin/config"

type auditingConfig struct {
    config.DelegatingPlugin
    log *slog.Logger
}

func (a *auditingConfig) Set(ctx context.Context, path string, v config.Value) error {
    a.log.Info("config value changed", "path", path)
    return a.Base.Set(ctx, path, v)
}

// List, Get, Validate are all forwarded to Base automatically.
```

### store.DelegatingPlugin

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin/store"

type cachingStore struct {
    store.DelegatingPlugin
    cache map[string]map[string]config.Value
}

func (c *cachingStore) GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error) {
    // Check cache first, fall back to Base.
    // ...
    return c.Base.GetValues(ctx, id, paths)
}

// All other store methods (Capabilities, PutValues, ListTrees, etc.)
// are forwarded to Base automatically.
```

### transform.DelegatingPlugin

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin/transform"

type loggingTransform struct {
    transform.DelegatingPlugin
    log *slog.Logger
}

func (l *loggingTransform) BeforeDisplay(ctx context.Context, tree *config.Tree) error {
    l.log.Info("transforming tree", "paths", len(tree.List()))
    return l.Base.BeforeDisplay(ctx, tree)
}

// AfterSave and ValidatePolicy are forwarded to Base automatically.
```

### Creating a DelegatingPlugin

Use the `NewDelegatingPlugin` constructor for each type. It panics if
`base` is nil:

```go
base := config.NewDelegatingPlugin(somePlugin)
// or
base := store.NewDelegatingPlugin(somePlugin)
// or
base := transform.NewDelegatingPlugin(somePlugin)
```

## Compose patterns

### config.MergedPlugin

Combines multiple config plugins under distinct path prefixes. Each
child's paths are namespaced by its mount point:

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin/config"

merged, err := config.MergedPlugin(
    config.MountedPlugin{Impl: appA, Prefix: "app-a/"},
    config.MountedPlugin{Impl: appB, Prefix: "app-b/"},
)
```

**Behaviour:**

| Operation | Routing |
|-----------|---------|
| `List` | queries all children in parallel, prefixes each path |
| `Get` | routes to child matching the path prefix |
| `Set` | routes to child matching the path prefix |
| `Validate` | routes to child matching the prefix; provides a scoped `TreeReader` |

**Constraints:**

- At least one child is required
- Prefixes must not overlap (neither can be a prefix of another)
- Empty prefixes are rejected
- Nil `Impl` values are rejected

The scoped `TreeReader` passed to `Validate` strips the prefix so that
each child sees paths relative to its own namespace.

### store.MirroredPlugin

Writes to all stores but reads from the primary. Mirror write errors
are logged but do not fail the operation:

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin/store"

composed := store.MirroredPlugin(logger, primary, mirror1, mirror2)
```

**Behaviour:**

| Operation | Routing |
|-----------|---------|
| Reads (`GetValues`, `ListTrees`, versioning reads) | primary only |
| Writes (`PutValues`, `DeleteValues`, `DeleteTree`) | primary first, then mirrors in parallel |
| `Capabilities` | intersection of all stores (most restrictive wins) |
| Auth/Encryption/Access control | primary only |

Mirror writes use `nil` for `PutOptions` (no CAS on mirrors) to avoid
conflicts. If a mirror write fails, the error is logged but the
operation succeeds as long as the primary succeeded.

## Building a meta-plugin

A meta-plugin is a standard plugin binary that uses the SDK to launch
children and compose them. The host treats it like any other plugin.

### Step 1: Find child binaries

Locate sibling binaries relative to your own executable, or look them
up on `PATH`:

```go
selfPath, _ := os.Executable()
dir := filepath.Dir(selfPath)

primaryBin := filepath.Join(dir, "zhi-store-memory")
mirrorBin := filepath.Join(dir, "zhi-store-json")
```

### Step 2: Launch children

```go
primary, cleanupPrimary, err := launch.LaunchStore(primaryBin,
    launch.WithLogger(logger),
)
if err != nil {
    // handle error
}
defer cleanupPrimary()

mirror, cleanupMirror, err := launch.LaunchStore(mirrorBin,
    launch.WithLogger(logger),
)
if err != nil {
    // handle error
}
defer cleanupMirror()
```

### Step 3: Compose

```go
composed := store.MirroredPlugin(logger, primary, mirror)
```

### Step 4: Serve to host

```go
goplugin.Serve(&goplugin.ServeConfig{
    HandshakeConfig: zhiplugin.Handshake,
    Plugins: map[string]goplugin.Plugin{
        "store": &store.GRPCPlugin{Impl: composed},
    },
    GRPCServer: goplugin.DefaultGRPCServer,
})
```

> **Pitfall: `os.Exit` skips deferred cleanups.** Once a child plugin
> has been launched, do **not** call `os.Exit` (directly or via
> `log.Fatal`) on a later error path — `os.Exit` does not run deferred
> functions, so `cleanupPrimary()`/`cleanupMirror()` never fire and the
> already-running child processes are orphaned (they live in their own
> process group and will not receive the terminal's signals). Put the
> launch-and-serve logic in a helper that returns an error and uses
> `defer`, then exit only from `main`:
>
> ```go
> func main() {
>     if err := run(logger); err != nil {
>         logger.Error("meta-plugin failed", "error", err)
>         os.Exit(1)
>     }
> }
>
> func run(logger hclog.Logger) error {
>     primary, cleanupPrimary, err := launch.LaunchStore(primaryBin, launch.WithLogger(logger))
>     if err != nil {
>         return err
>     }
>     defer cleanupPrimary()
>     // ... launch mirror, compose, Serve ...
>     return nil
> }
> ```

### Plugin manifest

A meta-plugin's `zhi-plugin.yaml` uses the standard format. The `type`
matches the interface the meta-plugin exposes to the host:

```yaml
schemaVersion: "1"
name: mirror
type: store
version: 0.0.1
zhiProtocolVersion: "1"
description: Meta-plugin that mirrors writes to two child store plugins
keywords:
  - store
  - mirror
  - meta-plugin
  - composition
```

### Naming convention

Meta-plugin binaries follow the same naming scheme as regular plugins:
`zhi-<type>-<name>` (e.g. `zhi-store-mirror`).

## Complete examples

### zhi-store-mirror

The [`examples/zhi-store-mirror/`](../../examples/zhi-store-mirror/)
directory contains a working meta-plugin that launches `zhi-store-memory`
as the primary and `zhi-store-json` as the mirror. Build and run it
with:

```sh
make build-examples
```

Both child plugin binaries must be available in the same directory as
the meta-plugin binary (or on `PATH`).

### zhi-store-vault-manager

The [`examples/zhi-store-vault-manager/`](../../examples/zhi-store-vault-manager/)
directory demonstrates a more advanced meta-plugin that wraps
`zhi-store-vault` with automatic Vault credential management. It
uses `DelegatingPlugin` to override `Login`, `GetValues`, and
`PutValues` while forwarding all other store methods to the child.

Key techniques shown:

- **`WithIsolatedEnv`** to prevent leaking the admin token to the child
- **`WithPluginOptions`** to forward connection options to the child
- **`store.ephemeral` label** to inject credentials that are never persisted
- **Policy reconciliation** driven by `vault.app.*` metadata labels

See the [Vault Credential Management](../user-guide/vault-credentials.md)
guide for end-user documentation.

## Use cases

| Pattern | Use case |
|---------|----------|
| **MirroredPlugin** | backup/replication: fast in-memory primary with durable JSON file mirror |
| **MergedPlugin** | multi-tenant config: each tenant's config plugin mounted under a prefix |
| **DelegatingPlugin + Launch** | add logging, caching, or access control around an existing plugin |
| **DelegatingPlugin alone** | override specific methods of any plugin without reimplementing the full interface |
| **DelegatingPlugin + Launch + credential management** | wrap a store with automatic policy reconciliation and credential injection (see vault-manager below) |

## Security considerations

- Use `WithIsolatedEnv` to avoid leaking secrets to child processes
- The default `AuditHardFail` mode rejects tampered binaries; use
  `AuditWarnOnly` only during development
- Child processes run in separate process groups for clean lifecycle
  management
- Binary paths are validated against symlink traversal attacks
