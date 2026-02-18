# Approach 4: SDK Composition Helpers

**Status**: Draft
**Date**: 2026-02-18

## Summary

Provide library code in the zhi SDK (and potentially in SDKs for other
languages) that makes it trivial to compose plugins by eliminating the
boilerplate. This approach complements Approach 3 (meta-plugin) by making
it easy to build meta-plugins.

This is not a standalone composition mechanism. It is tooling that reduces
the cost of writing composition plugins in code.

## Problem: Boilerplate in Meta-Plugins

The store interface has 23 methods. A meta-plugin that wants to override
`PutValues` and `GrantAccess` while delegating everything else must still
implement all 23 methods as pass-through stubs. This is tedious and
error-prone.

## Proposed SDK Additions

### 1. Delegating Base Types

```go
// pkg/zhiplugin/store/delegate.go

// DelegatingPlugin wraps a base store.Plugin and delegates all calls to it.
// Embed this struct and override only the methods you need.
type DelegatingPlugin struct {
    Base Plugin
}

func (d *DelegatingPlugin) Capabilities(ctx context.Context) (*Capabilities, error) {
    return d.Base.Capabilities(ctx)
}
func (d *DelegatingPlugin) AuthMethods(ctx context.Context) ([]AuthMethod, error) {
    return d.Base.AuthMethods(ctx)
}
func (d *DelegatingPlugin) Login(ctx context.Context, method string, creds map[string]string) (*Credential, error) {
    return d.Base.Login(ctx, method, creds)
}
// ... all 23 methods delegate to d.Base ...
```

Usage:

```go
type vaultManagedStore struct {
    store.DelegatingPlugin // embed the delegator
}

// Only override what you need:
func (s *vaultManagedStore) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
    if err := s.Base.PutValues(ctx, id, values, opts); err != nil {
        return err
    }
    return s.syncPolicies(ctx, id, values)
}

func (s *vaultManagedStore) GrantAccess(ctx context.Context, id string, user string, perms []store.Permission) error {
    return s.createAppRole(ctx, id, user, perms)
}
```

This reduces a 23-method implementation to just the 2 methods that differ.

### 2. Config Merge Helpers

```go
// pkg/zhiplugin/config/compose.go

// MergedPlugin combines multiple config plugins under distinct path prefixes.
// Each child's paths are prefixed with the specified mount point.
func MergedPlugin(children ...MountedPlugin) Plugin {
    return &mergedPlugin{children: children}
}

// MountedPlugin associates a config plugin with a path prefix.
type MountedPlugin struct {
    Plugin Plugin
    Prefix string  // e.g., "app-a/"
}

// OverlayPlugin combines config plugins with last-writer-wins semantics.
// Later plugins take precedence for overlapping paths.
func OverlayPlugin(plugins ...Plugin) Plugin {
    return &overlayPlugin{layers: plugins}
}
```

Usage:

```go
func main() {
    appA := launchConfigPlugin("zhi-config-app-a")
    appB := launchConfigPlugin("zhi-config-app-b")

    merged := config.MergedPlugin(
        config.MountedPlugin{Plugin: appA, Prefix: "app-a/"},
        config.MountedPlugin{Plugin: appB, Prefix: "app-b/"},
    )

    goplugin.Serve(&goplugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]goplugin.Plugin{
            "config": &config.GRPCPlugin{Impl: merged},
        },
        GRPCServer: goplugin.DefaultGRPCServer,
    })
}
```

### 3. Store Composition Helpers

```go
// pkg/zhiplugin/store/compose.go

// MirroredPlugin writes to all stores but reads from the primary (first).
func MirroredPlugin(primary Plugin, mirrors ...Plugin) Plugin {
    return &mirrorPlugin{primary: primary, mirrors: mirrors}
}

// FailoverPlugin tries each store in order, returning on first success.
func FailoverPlugin(stores ...Plugin) Plugin {
    return &failoverPlugin{stores: stores}
}

// InterceptPlugin wraps a store with before/after hooks for specific
// operations.
func InterceptPlugin(base Plugin, hooks ...Hook) Plugin {
    return &interceptPlugin{base: base, hooks: hooks}
}

// Hook defines a before/after interceptor for a store operation.
type Hook struct {
    Operation string                           // "PutValues", "GrantAccess", etc.
    Before    func(ctx context.Context) error  // runs before, can abort
    After     func(ctx context.Context) error  // runs after, receives result
}
```

### 4. Plugin Launcher Helper

```go
// pkg/zhiplugin/launch.go

// LaunchPlugin launches a child plugin binary and returns the plugin
// interface. This is a convenience wrapper around hashicorp/go-plugin
// for use by meta-plugins.
//
// The pluginType must be one of: "config", "transform", "store", "ui".
func LaunchPlugin[T any](binary string, pluginType string) (T, func(), error) {
    var pluginMap map[string]goplugin.Plugin
    switch pluginType {
    case "config":
        pluginMap = config.PluginMap
    case "transform":
        pluginMap = transform.PluginMap
    case "store":
        pluginMap = store.PluginMap
    case "ui":
        pluginMap = ui.PluginMap
    }

    client := goplugin.NewClient(&goplugin.ClientConfig{
        HandshakeConfig:  Handshake,
        Plugins:          pluginMap,
        Cmd:              exec.Command(binary),
        AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
    })

    rpcClient, err := client.Client()
    if err != nil {
        client.Kill()
        return *new(T), nil, err
    }

    raw, err := rpcClient.Dispense(pluginType)
    if err != nil {
        client.Kill()
        return *new(T), nil, err
    }

    return raw.(T), client.Kill, nil
}
```

Usage:

```go
vault, cleanup, err := zhiplugin.LaunchPlugin[store.Plugin](
    "/home/user/.zhi/plugins/zhi-store-vault", "store",
)
defer cleanup()
```

### 5. Multi-Language SDK Support

For other languages, the SDK helpers would be implemented natively:

**Python SDK** (hypothetical):

```python
from zhi_sdk.store import DelegatingPlugin, MirroredPlugin

class VaultManagedStore(DelegatingPlugin):
    """Store that extends Vault with AppRole management."""

    def put_values(self, ctx, tree_id, values, opts=None):
        # Delegate to base vault store
        self.base.put_values(ctx, tree_id, values, opts)
        # Create/update policies
        self._sync_policies(ctx, tree_id, values)

    def grant_access(self, ctx, tree_id, user, permissions):
        return self._create_approle(ctx, tree_id, user, permissions)
```

**Rust SDK** (hypothetical):

```rust
use zhi_sdk::store::{DelegatingPlugin, Plugin};

struct VaultManagedStore {
    base: Box<dyn Plugin>,
}

impl DelegatingPlugin for VaultManagedStore {
    fn base(&self) -> &dyn Plugin {
        &*self.base
    }
}

// Only override what's needed
impl VaultManagedStore {
    async fn put_values(&self, ctx: &Context, id: &str, values: HashMap<String, Value>, opts: Option<PutOptions>) -> Result<()> {
        self.base.put_values(ctx, id, &values, opts.as_ref()).await?;
        self.sync_policies(ctx, id, &values).await
    }
}
```

## Pros

- **Dramatically reduces boilerplate**: A 23-method delegation becomes
  a struct embed with 2 overrides.
- **Composable building blocks**: `MergedPlugin`, `MirroredPlugin`,
  `FailoverPlugin`, and `InterceptPlugin` cover the most common
  composition patterns without custom code.
- **Type-safe**: The compiler verifies all interface methods are
  satisfied. Go embedding ensures delegation is correct.
- **Works with Approach 3**: This is the natural companion to the
  meta-plugin approach. It makes meta-plugins practical.
- **Multi-language potential**: The patterns (delegation, interception)
  translate naturally to Python, Rust, Java, etc.
- **No engine changes**: Everything is in the SDK. The engine is
  unaware of composition.
- **Testable**: Each helper is independently testable with mock plugins.

## Cons

- **Still requires code**: Users must write and compile a meta-plugin
  binary, even if it is just a few lines.
- **Go-centric initially**: The Go SDK gets these helpers first. Other
  language SDKs would follow but require significant development.
- **Interface evolution risk**: If the plugin interface gains new
  methods, the `DelegatingPlugin` must be updated. This is a one-time
  cost per release but must not be forgotten.
- **Discovery of child binaries**: Meta-plugins must find their child
  plugin binaries. The SDK can help by providing a `DiscoverPlugin()`
  function that searches standard locations.
- **Testing across processes**: Unit tests can mock children, but
  integration tests require actual child binaries.

## Relationship to Other Approaches

This approach is a **building block**, not a complete solution. It pairs
with:

- **Approach 3 (Meta-Plugin)**: SDK helpers make meta-plugins practical.
- **Approach 1 (Engine Multi-Provider)**: The engine could use these
  same helpers internally to implement prefix/overlay/mirror strategies.
- **Approach 5 (gRPC Proxy)**: The proxy could use these helpers for
  its routing logic.

## When This Approach Works Well

- Plugin developers building custom composition plugins
- Any scenario where Approach 3 is the right choice (this reduces the
  cost of Approach 3)
- Building reusable marketplace plugins that compose other plugins

## When This Approach Falls Short

- Non-developer users who need composition via configuration only
- When there is no SDK for the user's preferred language
