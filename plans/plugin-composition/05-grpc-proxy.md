# Approach 5: Generic gRPC Composition Proxy

**Status**: Draft
**Date**: 2026-02-18

## Summary

Build a single, reusable binary (`zhi-compose`) that acts as a gRPC proxy
implementing any plugin interface. It reads a composition spec from a
configuration file and routes/transforms gRPC calls to child plugins
based on declarative rules.

Unlike Approach 3 (meta-plugin), the proxy is a **single generic binary**
rather than a per-composition binary. Unlike Approach 2 (declarative in
zhi.yaml), the composition spec lives outside the workspace config as a
standalone file, making it reusable and shareable.

## Architecture

```
Engine ──gRPC──> zhi-compose (generic proxy)
                    │
                    │  reads: compose.yaml (routing rules)
                    │
                    ├──gRPC──> Child Plugin A
                    └──gRPC──> Child Plugin B
```

The proxy:

1. Is registered like any plugin (e.g., `zhi-compose --config compose.yaml`)
2. Reads a `compose.yaml` that describes the composition
3. Launches child plugins as sub-processes
4. Intercepts every gRPC call and routes it per the rules
5. Returns responses to the engine

## Composition Spec (`compose.yaml`)

```yaml
type: config
name: merged-apps

children:
  app-a:
    binary: zhi-config-app-a
    # or: binary: /home/user/.zhi/plugins/zhi-config-app-a
  app-b:
    binary: zhi-config-pokedex

routing:
  list:
    strategy: merge-all
    # List() returns the union of all children's paths,
    # each prefixed with the child's mount point.
    prefix_with_child_name: true

  get:
    strategy: route-by-prefix
    # Get("app-a/foo") routes to child "app-a" as Get("foo")

  set:
    strategy: route-by-prefix

  validate:
    strategy: route-by-prefix
    # The TreeReader passed to Validate is filtered to the
    # child's prefix scope.
```

### Store Composition Spec

```yaml
type: store
name: vault-with-backup

children:
  primary:
    binary: zhi-store-vault
    env:
      VAULT_ADDR: "http://vault:8200"
  backup:
    binary: zhi-store-json
    env:
      ZHI_JSON_STORE_DIR: /var/backup/zhi

routing:
  capabilities:
    strategy: primary
    # Return primary's capabilities

  get_values:
    strategy: primary
    fallback: backup
    # Try primary, fall back to backup on error

  put_values:
    strategy: fanout
    # Write to all children. Fail if primary fails.
    # Log warnings if secondary fails.
    required: [primary]
    best_effort: [backup]

  list_trees:
    strategy: primary

  # Methods not listed delegate to primary by default
  default:
    strategy: primary
```

### Hooks in Composition Spec

```yaml
type: store
name: vault-managed

children:
  vault:
    binary: zhi-store-vault

hooks:
  after:
    put_values:
      exec: /usr/local/bin/sync-vault-policies
      args: ["--tree-id", "{{.TreeID}}"]
      # The hook binary receives the operation context as JSON on stdin
    grant_access:
      exec: /usr/local/bin/create-approle
      args: ["--tree-id", "{{.TreeID}}", "--user", "{{.User}}"]
```

Hooks can be:

- **exec**: Run an external binary with template arguments
- **grpc**: Call another zhi plugin's custom RPC
- **http**: POST to a webhook URL

## Integration with zhi.yaml

```yaml
config:
  provider: zhi-compose
  options:
    config: ./compose-config.yaml

store:
  provider: zhi-compose
  options:
    config: ./compose-store.yaml
```

Or, the proxy can be installed as a named plugin:

```bash
# Install the generic proxy
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-compose:v1.0.0

# Install its dependencies
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-config-pokedex:v0.0.9
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-vault:v0.0.9
```

## Pros

- **No custom code**: Users compose plugins by writing a YAML file and
  running the generic proxy binary. No Go/Python/Rust code needed.
- **One binary for all compositions**: `zhi-compose` is a single binary
  that handles config, store, and transform compositions. No need to
  compile a new binary per composition.
- **Cross-language by design**: The proxy communicates with all children
  via gRPC. Implementation language of children is irrelevant.
- **Shareable composition specs**: `compose.yaml` files can be shared
  in repositories, published as OCI artifacts, or checked into Git
  alongside `zhi.yaml`.
- **Extensible routing**: New strategies can be added to the proxy over
  time without changing the engine or child plugins.
- **Hook support**: External hooks (exec, gRPC, HTTP) enable behavioral
  extension without modifying the proxy.
- **No engine changes**: The engine sees a standard plugin.

## Cons

- **New binary to maintain**: `zhi-compose` is a non-trivial binary that
  must understand all four plugin interfaces and their gRPC protocols.
  As interfaces evolve, the proxy must be updated.
- **Routing is limited to call-level granularity**: The proxy can route
  entire calls but cannot easily intercept and modify individual
  arguments or return values (e.g., "if the value at path X has
  metadata Y, add field Z").
- **Performance overhead**: Every gRPC call passes through the proxy,
  adding latency. For fanout strategies, the proxy makes multiple
  downstream calls sequentially or in parallel.
- **Debugging is hard**: A user sees an error from the proxy, which
  came from a child, which may have come from a downstream service.
  The proxy must provide good error context.
- **Process cost**: The proxy is yet another process. With 2 children,
  that's 3 processes (proxy + 2 children) for one logical provider.
- **Hook expressiveness**: Exec hooks are limited to what a shell
  command can do. They receive context as stdin JSON and cannot modify
  the response. Complex side-effects require a real plugin, negating
  the "no code" benefit.
- **Configuration complexity**: The `compose.yaml` routing DSL is
  another configuration language users must learn. For complex
  compositions, it approaches the complexity of actual code.
- **Incomplete strategy coverage**: Not all combinations of strategies
  make sense for all methods. The proxy must handle edge cases (what
  does "fanout" mean for `Capabilities()`? Merge results? Pick first?).

## Comparison with Approach 2

Both approaches are declarative, but:

| Aspect | Approach 2 (in zhi.yaml) | Approach 5 (gRPC proxy) |
|--------|--------------------------|-------------------------|
| Where config lives | zhi.yaml | Separate compose.yaml |
| Who interprets it | Engine | Proxy binary |
| Engine changes | Yes | No |
| Reusability | Per-workspace | Shareable across workspaces |
| Process cost | None (engine handles it) | +1 process (the proxy) |
| Plugin interface awareness | Engine already knows interfaces | Proxy must implement all interfaces |

## When This Approach Works Well

- Users who want composition without writing code
- Compositions that are primarily routing-based (merge, fanout, failover)
- Reusable composition patterns shared across teams

## When This Approach Falls Short

- Complex behavioral composition (AppRole creation needs real logic)
- When the routing DSL becomes the bottleneck
- Performance-sensitive scenarios where the proxy overhead matters
