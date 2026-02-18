# Approach Comparison and Recommendation

**Status**: Draft
**Date**: 2026-02-18

## Side-by-Side Comparison

| Dimension | 1. Engine Multi-Provider | 2. Declarative zhi.yaml | 3. Meta-Plugin | 4. SDK Helpers | 5. gRPC Proxy |
|-----------|--------------------------|-------------------------|----------------|----------------|---------------|
| **Code required** | None | None | Yes (per composition) | Yes (per composition) | None (YAML only) |
| **Engine changes** | Significant | Significant | None | None | None |
| **Custom logic** | No | Limited (hooks) | Unlimited | Unlimited | Limited (hooks) |
| **Cross-language** | Automatic | Automatic | Yes (children) | Go-first, others later | Automatic |
| **Process overhead** | None | +1 per hook provider | +1 per child | +1 per child | +1 proxy + children |
| **Boilerplate** | None | None | High (without SDK) | Low (with SDK) | None |
| **Debugging** | Good (engine control) | Medium | Hard (layered) | Hard (layered) | Hard (layered) |
| **Reusability** | Per-workspace | Per-workspace | Marketplace-ready | Marketplace-ready | Shareable specs |
| **Merge strategies** | Built-in set | Built-in + hooks | Any | Any | Built-in set |
| **Store composition** | Limited | Limited | Full control | Full control | Limited |
| **Interface evolution** | Handled by engine | Handled by engine | Manual delegation | SDK auto-updates | Proxy updates needed |
| **User skill needed** | YAML only | YAML only | Go/Rust/Python | Go/Rust/Python | YAML only |

## Evaluation Against Motivating Examples

### Example 1: Vault Store with AppRole Management

| Approach | Feasibility | Notes |
|----------|-------------|-------|
| 1. Engine Multi-Provider | **Poor** | Cannot add AppRole creation logic |
| 2. Declarative | **Fair** | Could use exec hooks, but complex |
| 3. Meta-Plugin | **Excellent** | Full control over Vault API calls |
| 4. SDK Helpers | **Excellent** | `DelegatingPlugin` + 2 overrides |
| 5. gRPC Proxy | **Fair** | Exec hooks can call Vault API, but brittle |

### Example 2: Merged Config Plugins

| Approach | Feasibility | Notes |
|----------|-------------|-------|
| 1. Engine Multi-Provider | **Excellent** | Prefix-mount is a perfect fit |
| 2. Declarative | **Excellent** | Supported natively |
| 3. Meta-Plugin | **Good** | Works but too much ceremony |
| 4. SDK Helpers | **Excellent** | `MergedPlugin()` one-liner |
| 5. gRPC Proxy | **Good** | Route-by-prefix strategy |

### Example 3: Store with Backup Mirror

| Approach | Feasibility | Notes |
|----------|-------------|-------|
| 1. Engine Multi-Provider | **Good** | Mirror strategy fits |
| 2. Declarative | **Good** | Mirror strategy in compose section |
| 3. Meta-Plugin | **Good** | Full control over failure handling |
| 4. SDK Helpers | **Excellent** | `MirroredPlugin()` one-liner |
| 5. gRPC Proxy | **Good** | Fanout strategy |

### Example 4: Config with Secret Injection

| Approach | Feasibility | Notes |
|----------|-------------|-------|
| 1. Engine Multi-Provider | **Good** | Overlay strategy fits |
| 2. Declarative | **Good** | Overlay merge in compose section |
| 3. Meta-Plugin | **Good** | Full control over merge logic |
| 4. SDK Helpers | **Good** | `OverlayPlugin()` helper |
| 5. gRPC Proxy | **Fair** | Would need overlay routing strategy |

## Key Trade-Offs

### Simplicity vs. Power

```
Simple (config-only)                     Powerful (code-based)
  ┌──────────┐    ┌───────────┐    ┌──────────┐    ┌──────────┐
  │ Approach  │    │ Approach  │    │ Approach  │    │ Approach  │
  │ 1: Engine │    │ 2: Decl.  │    │ 5: Proxy  │    │ 3+4:     │
  │           │    │           │    │           │    │ Meta+SDK  │
  └──────────┘    └───────────┘    └───────────┘    └──────────┘
```

- **Left side**: Zero code, limited to predefined strategies.
- **Right side**: Full programming language, unlimited possibilities.
- There is no single approach that is both zero-code AND unlimited.

### Engine Coupling vs. Independence

Approaches 1 and 2 require engine changes, which means:
- **Pros**: Tighter integration, better error handling, no extra processes.
- **Cons**: Every new composition pattern needs an engine release.

Approaches 3, 4, and 5 are engine-independent:
- **Pros**: Composition evolves independently of zhi core.
- **Cons**: Extra processes, less integrated error handling.

### Cross-Language Considerations

All approaches support cross-language child plugins because all plugin
communication is over gRPC. The differences are:

- **Writing the compositor**: Approaches 1 and 2 are engine-native (Go
  only, but users don't write code). Approach 5 is a fixed binary.
  Approaches 3 and 4 require the compositor to be written in a
  supported language.
- **SDK availability**: Approach 4 requires SDK support in each language.
  Go SDK is immediate; others require community investment.

## Recommendation: Layered Strategy (Approaches 1 + 3 + 4)

No single approach covers all use cases well. The recommendation is to
implement a **layered strategy** that combines the strengths of multiple
approaches:

### Layer 1: Engine Multi-Provider (Approach 1)

**Scope**: Simple data-routing compositions.

Add support for multiple config providers with merge strategies directly
in the engine and `zhi.yaml`. This covers the common case of merging
config trees from multiple sources, which requires no custom logic.

```yaml
config:
  providers:
    - provider: structuredfile
      options: { directory: ./config/app-a }
      mount: "app-a/"
    - provider: pokedex
      mount: "pokedex/"
  merge: prefix
```

This is the lowest-effort, highest-value change. It enables Example 2
(merged configs) and Example 4 (secret injection via overlay) with zero
code. Similarly, basic store mirroring and failover can be built into the
engine.

**Implementation priority**: High. This is the 80% solution.

### Layer 2: SDK Composition Helpers (Approach 4)

**Scope**: Developer tooling for building meta-plugins.

Add `DelegatingPlugin` base types, `MergedPlugin()`, `MirroredPlugin()`,
and `LaunchPlugin()` helpers to the Go SDK. This makes Approach 3
(meta-plugin) practical by eliminating boilerplate.

```go
type vaultManagedStore struct {
    store.DelegatingPlugin
}

func (s *vaultManagedStore) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
    if err := s.Base.PutValues(ctx, id, values, opts); err != nil {
        return err
    }
    return s.syncPolicies(ctx, id, values)
}
```

This enables Example 1 (Vault + AppRole) with minimal code.

**Implementation priority**: High. This is the escape hatch for cases
that engine-level composition cannot handle.

### Layer 3: Meta-Plugin Pattern (Approach 3)

**Scope**: Complex behavioral composition via real plugins.

This is not a feature to build. It is a **pattern** that emerges
naturally from Layer 2 (SDK helpers). Document the pattern, provide
examples, and ensure the plugin launcher helper (`LaunchPlugin()`) makes
it easy to write meta-plugins.

**Implementation priority**: Medium. Primarily documentation + examples.

### What to Skip (For Now)

- **Approach 2 (Declarative composition in zhi.yaml)**: Overlaps heavily
  with Approach 1 but adds a hook DSL that risks scope creep. If hooks
  are needed, users should write a meta-plugin (Layer 2/3).

- **Approach 5 (gRPC Proxy)**: Adds a new binary, a new configuration
  language, and process overhead. The value over Approach 1 (engine
  multi-provider) is marginal for data-routing, and the value over
  Approach 3+4 (meta-plugin + SDK) is negative for behavioral
  composition. Can be revisited if users demand a "no-code" behavioral
  composition mechanism.

## Implementation Roadmap

### Phase 1: Engine Multi-Provider Config

1. Extend `WorkspaceConfig` to accept `config.providers` (list) alongside
   the existing `config.provider` (singular).
2. Implement `compositeConfigPlugin` with prefix-mount and overlay
   strategies.
3. Update engine to use composite config when multiple providers are
   configured.
4. Update workspace validation.
5. Add tests and documentation.

### Phase 2: SDK Composition Helpers

1. Add `DelegatingPlugin` base types for all four plugin types.
2. Add `MergedPlugin()` and `OverlayPlugin()` config helpers.
3. Add `MirroredPlugin()` and `FailoverPlugin()` store helpers.
4. Add `LaunchPlugin()` helper for meta-plugins to launch children.
5. Write example meta-plugin (Vault + AppRole) using the SDK.

### Phase 3: Engine Multi-Provider Store

1. Extend `WorkspaceConfig` to accept `store.providers` (list).
2. Implement composite store with mirror and failover strategies.
3. Handle CAS, versioning, and capability negotiation across stores.
4. Add tests.

### Phase 4: Documentation and Examples

1. Document composition patterns in `docs/plugin-development/`.
2. Add example composed plugins to `examples/`.
3. Update workspace configuration reference.
4. Write a tutorial: "Composing Plugins".

## Open Questions

1. **Should engine multi-provider support transform composition?**
   Transforms are already composed as a list. The question is whether
   finer-grained composition (e.g., "apply transform A only to paths
   under prefix X") is needed.

2. **How should component definitions interact with composed configs?**
   If config plugin A manages `app-a/*` and plugin B manages `app-b/*`,
   should components be defined per-child or globally?

3. **What happens when a child plugin crashes mid-composition?**
   For mirror writes, should the engine/proxy retry? Roll back the
   primary write? Log and continue?

4. **Should composed plugins appear in the marketplace?**
   Meta-plugins are already marketplace-ready. But should
   `compose.yaml` specs be publishable too?

5. **Should the SDK helpers be generated from the plugin interfaces?**
   This would ensure `DelegatingPlugin` stays in sync automatically,
   but adds a code generation step.
