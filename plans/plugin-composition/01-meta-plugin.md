# Meta-Plugin (Composition Plugin)

**Status**: Active
**Date**: 2026-02-18

## Summary

A **meta-plugin** is a regular zhi plugin binary that internally launches
and delegates to one or more child plugins. It implements the standard
plugin interface (config, store, etc.) and the engine treats it like any
other plugin. The composition logic lives entirely in the meta-plugin's
code.

The key difference from all other approaches is that the meta-plugin is a
real, compiled binary written by the plugin developer. This gives maximum
flexibility at the cost of requiring code.

## Architecture

```
Engine  ──gRPC──>  Meta-Plugin (e.g., vault-managed-store)
                      │
                      ├──launches──>  Child Plugin A (zhi-store-vault)
                      │                    │
                      │                    └── gRPC (stdio)
                      │
                      └──launches──>  Child Plugin B (vault-policy-manager)
                                           │
                                           └── gRPC (stdio)
```

The meta-plugin:

1. Receives calls from the engine via the normal gRPC interface.
2. Launches child plugins as sub-processes using hashicorp/go-plugin.
3. Delegates calls to the appropriate child.
4. Adds its own logic before/after delegation.
5. Returns results to the engine.

## Example: Vault Store with AppRole Management

```go
package main

import (
    "context"
    goplugin "github.com/hashicorp/go-plugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin/config"
    "github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

type vaultManagedStore struct {
    vault     store.Plugin   // child: standard vault store
    cleanup   func()         // child process cleanup
}

func (s *vaultManagedStore) Capabilities(ctx context.Context) (*store.Capabilities, error) {
    // Delegate to vault, but advertise additional capabilities
    caps, err := s.vault.Capabilities(ctx)
    if err != nil {
        return nil, err
    }
    caps.AccessControl = true // we manage this ourselves
    return caps, nil
}

func (s *vaultManagedStore) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
    // First write to vault
    if err := s.vault.PutValues(ctx, id, values, opts); err != nil {
        return err
    }
    // Then create/update Vault policies for these paths
    return s.syncVaultPolicies(ctx, id, values)
}

func (s *vaultManagedStore) GrantAccess(ctx context.Context, id string, user string, perms []store.Permission) error {
    // Create an AppRole with a Vault policy scoped to these paths
    return s.createAppRole(ctx, id, user, perms)
}

// ... delegate all other methods to s.vault ...

func main() {
    // Launch the child vault plugin
    vaultPlugin, cleanup := launchChild("zhi-store-vault")
    defer cleanup()

    goplugin.Serve(&goplugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]goplugin.Plugin{
            "store": &store.GRPCPlugin{
                Impl: &vaultManagedStore{vault: vaultPlugin, cleanup: cleanup},
            },
        },
        GRPCServer: goplugin.DefaultGRPCServer,
    })
}
```

### Launching Child Plugins

The meta-plugin uses the same hashicorp/go-plugin mechanism that the
engine uses. It becomes a "host" for its child plugins:

```go
func launchChild(binaryPath string) (store.Plugin, func()) {
    client := goplugin.NewClient(&goplugin.ClientConfig{
        HandshakeConfig:  zhiplugin.Handshake,
        Plugins:          store.PluginMap,
        Cmd:              exec.Command(binaryPath),
        AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
    })
    rpcClient, _ := client.Client()
    raw, _ := rpcClient.Dispense("store")
    return raw.(store.Plugin), client.Kill
}
```

## Example: Merged Config Plugin

```go
type mergedConfig struct {
    children []prefixedChild
}

type prefixedChild struct {
    plugin config.Plugin
    prefix string
}

func (m *mergedConfig) List(ctx context.Context) ([]string, error) {
    var all []string
    for _, child := range m.children {
        paths, err := child.plugin.List(ctx)
        if err != nil {
            return nil, err
        }
        for _, p := range paths {
            all = append(all, child.prefix+p)
        }
    }
    return all, nil
}

func (m *mergedConfig) Get(ctx context.Context, path string) (config.Value, bool, error) {
    for _, child := range m.children {
        if strings.HasPrefix(path, child.prefix) {
            return child.plugin.Get(ctx, strings.TrimPrefix(path, child.prefix))
        }
    }
    return config.Value{}, false, nil
}

func (m *mergedConfig) Set(ctx context.Context, path string, v config.Value) error {
    for _, child := range m.children {
        if strings.HasPrefix(path, child.prefix) {
            return child.plugin.Set(ctx, strings.TrimPrefix(path, child.prefix))
        }
    }
    return fmt.Errorf("no provider manages path %q", path)
}

func (m *mergedConfig) Validate(ctx context.Context, path string, tree config.TreeReader) ([]config.ValidationResult, error) {
    for _, child := range m.children {
        if strings.HasPrefix(path, child.prefix) {
            // Build a sub-tree scoped to this child's prefix
            subTree := filterTreeByPrefix(tree, child.prefix)
            return child.plugin.Validate(ctx, strings.TrimPrefix(path, child.prefix), subTree)
        }
    }
    return nil, nil
}
```

## Configuration

The meta-plugin reads its own configuration to know which children to
launch. This can come from environment variables, a config file, or
plugin options passed via `zhi.yaml`:

```yaml
config:
  provider: merged-apps
  options:
    children:
      - binary: "zhi-config-pokedex"
        prefix: "pokedex/"
      - binary: "zhi-config-structuredfile"
        prefix: "infra/"
        options:
          directory: ./config/infra
```

The `options` map is passed to the meta-plugin, which parses the
`children` list and launches each binary.

## Cross-Language Support

This approach works across languages in two ways:

1. **Meta-plugin in Go, children in any language**: The meta-plugin uses
   hashicorp/go-plugin to launch child processes. The children only need
   to implement the gRPC protocol. A child written in Python or Rust
   works identically.

2. **Meta-plugin in any language**: Any language with gRPC support can
   implement a meta-plugin. The meta-plugin would:
   - Implement the zhi plugin gRPC server (for the engine)
   - Launch child plugins as sub-processes
   - Act as a gRPC client to each child

   However, non-Go meta-plugins would need to reimplement the
   hashicorp/go-plugin handshake protocol (magic cookie, protocol
   negotiation). This is not trivial but is well-documented.

## Manifest

The meta-plugin's `zhi-plugin.yaml` declares its dependencies:

```yaml
schemaVersion: "1"
name: vault-managed
type: store
version: 0.1.0
description: Vault store with automatic AppRole and policy management

dependencies:
  - name: vault
    type: store
    version: ">=0.0.5"

binaries:
  linux/amd64: dist/zhi-store-vault-managed_linux_amd64
  # ...
```

The marketplace/installer ensures dependencies are present before the
meta-plugin runs.

## Pros

- **Maximum flexibility**: Full programming language for composition
  logic. Can implement any behavior, not just routing.
- **Standard plugin interface**: The engine sees a normal plugin. No
  engine changes required.
- **Cross-language**: Children can be in any language that speaks gRPC.
- **Testable**: The meta-plugin is a regular Go package with regular
  unit tests.
- **Distributable**: Ships as a single binary via the existing OCI
  marketplace. Dependencies declared in the manifest.
- **Self-contained**: All composition logic is in the meta-plugin.
  No changes to the engine, no new config syntax.
- **Behavioral composition**: Can add real logic (AppRole creation,
  audit logging, caching) around delegated operations.

## Cons

- **Requires code**: Users must write (or find) a meta-plugin binary.
  Not accessible to non-developers.
- **Boilerplate**: Delegating all interface methods to a child requires
  writing a lot of pass-through code (the store interface has 20+
  methods). This can be mitigated with SDK helpers (see Approach 4).
- **Process overhead**: Each meta-plugin runs as a separate process,
  and each child is another process. A meta-plugin with 2 children
  means 3 OS processes for one logical provider.
- **Error propagation**: Errors from children must be surfaced
  meaningfully. The meta-plugin adds a layer of indirection that can
  obscure root causes.
- **Configuration passing**: Getting plugin options from `zhi.yaml`
  through the engine to the meta-plugin and then to children requires
  careful option forwarding.
- **Lifecycle complexity**: The meta-plugin must manage child process
  lifecycles. If a child crashes, the meta-plugin must handle it
  gracefully (or crash itself and let the engine restart).
- **Binary discovery**: The meta-plugin needs to find child binaries
  on disk. The new `pkg/zhiplugin/discovery` package (see implementation
  plan) solves this by exporting the discovery mechanism.

## When This Approach Works Well

- Extending a plugin with custom behavior (Vault + AppRole)
- Complex composition logic that cannot be expressed declaratively
- Plugin developers building reusable composition plugins for the
  marketplace

## When This Approach Falls Short

- Non-developer users who just want to combine plugins via config
- When process overhead matters (many children = many processes)

## Relationship to SDK Composition Helpers

The SDK helpers ([02-sdk-composition-helpers.md](02-sdk-composition-helpers.md))
are the natural companion to this pattern. They make meta-plugins practical
by eliminating boilerplate. See the full implementation plan at
[`docs/plans/2026-02-18-feat-plugin-composition-meta-plugin-sdk-plan.md`](../../docs/plans/2026-02-18-feat-plugin-composition-meta-plugin-sdk-plan.md).
