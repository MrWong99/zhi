# Approach 1: Engine-Level Multi-Provider Support

**Status**: Draft
**Date**: 2026-02-18

## Summary

Extend the engine to natively support multiple providers per slot (config,
store) with built-in merge/routing strategies. Today the engine already
supports a list of transform plugins applied in sequence. This approach
generalizes that pattern to config and store plugins.

## Current State

The engine today has:

```go
type Engine struct {
    configPlugin     config.Plugin          // exactly one
    transformPlugins []transform.Plugin     // already a list
    storePlugin      store.Plugin           // exactly one or nil
}
```

Transform plugins already demonstrate sequential composition: each plugin
receives the tree after the previous one modified it.

## Proposed Changes

### WorkspaceConfig

```yaml
version: "1"

config:
  # New: list of config providers with merge strategy
  providers:
    - provider: structuredfile
      options:
        directory: ./config/app-a
      prefix: "app-a/"            # mount under this prefix
    - provider: pokedex
      prefix: "pokedex/"
  merge: prefix                   # "prefix", "overlay", "union"

store:
  # New: list of store providers with strategy
  providers:
    - provider: vault
      role: primary
      options:
        addr: "http://vault:8200"
    - provider: json
      role: mirror
      options:
        directory: ./backup
  strategy: mirror                # "primary-failover", "mirror", "sharded"
```

Backward-compatible: the existing single-provider form continues to work.
The engine detects whether `config.provider` (singular) or
`config.providers` (list) is used.

### Config Merge Strategies

| Strategy | Behavior |
|----------|----------|
| `prefix` | Each provider's paths are mounted under a distinct prefix. No overlap allowed. `List()` returns the union. |
| `overlay` | Providers are ordered. A later provider's `Get()` wins for overlapping paths. `List()` is the union. |
| `union` | All providers must manage disjoint paths. Engine enforces no overlap at startup and returns the union. |

### Store Strategies

| Strategy | Behavior |
|----------|----------|
| `primary-failover` | Reads/writes go to the first provider. If it errors, fall back to the next. |
| `mirror` | Writes go to all providers. Reads go to the primary (first in list). |
| `sharded` | Trees are routed to specific providers based on tree ID patterns. |

### Engine Implementation Sketch

```go
type compositeConfig struct {
    providers []prefixedConfig
    strategy  string
}

type prefixedConfig struct {
    plugin config.Plugin
    prefix string
}

func (c *compositeConfig) List(ctx context.Context) ([]string, error) {
    var all []string
    for _, p := range c.providers {
        paths, err := p.plugin.List(ctx)
        if err != nil {
            return nil, err
        }
        for _, path := range paths {
            all = append(all, p.prefix+path)
        }
    }
    return all, nil
}

func (c *compositeConfig) Get(ctx context.Context, path string) (Value, bool, error) {
    for _, p := range c.providers {
        if strings.HasPrefix(path, p.prefix) {
            return p.plugin.Get(ctx, strings.TrimPrefix(path, p.prefix))
        }
    }
    return Value{}, false, nil
}
```

## Pros

- **Zero plugin development required**: Users compose plugins purely through
  configuration. No Go code, no new binaries.
- **Deeply integrated**: The engine manages lifecycle, error handling, and
  cleanup for all child plugins transparently.
- **Cross-language for free**: Since the engine launches each child plugin
  as a separate gRPC process, the implementation language of each child
  is irrelevant.
- **Familiar pattern**: Users already understand the transform list in
  `zhi.yaml`. Extending this pattern to config and store is intuitive.
- **Atomic operations**: The engine can coordinate cross-provider operations
  (e.g., transactional writes across mirrored stores).

## Cons

- **Limited to predefined strategies**: Users can only combine plugins in
  ways the engine supports. Adding a new strategy requires engine changes.
- **Complexity in the engine**: Merge logic, conflict resolution, and error
  handling for composite providers increase engine complexity.
- **No custom logic**: Cannot express "extend vault with AppRole creation"
  because the composition is purely data-routing, not behavioral.
- **Store composition is tricky**: CAS (check-and-set), versioning, and
  encryption semantics are hard to define across multiple stores.
- **Validation complexity**: When `Validate()` receives the full tree,
  but each child only knows its own subtree, cross-subtree validation
  requires special handling.
- **Performance**: Every operation must route through the composite layer,
  potentially making multiple gRPC calls for a single user request.

## When This Approach Works Well

- Merging multiple config sources into one tree (prefix-mounting)
- Simple store mirroring or failover
- Scenarios where no custom logic is needed beyond routing/merging

## When This Approach Falls Short

- Extending a plugin with new behavior (the Vault+AppRole example)
- Complex merge logic that depends on value content
- Scenarios requiring pre/post hooks around delegated operations
