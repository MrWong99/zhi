# Approach 2: Declarative Composition in zhi.yaml

**Status**: Draft
**Date**: 2026-02-18

## Summary

Introduce a new `compose` section in `zhi.yaml` that lets users define
virtual providers by declaring how existing providers are combined. The
engine interprets these declarations and creates wrapper plugins at
runtime. Unlike Approach 1 (engine multi-provider), this approach uses a
richer declaration language that supports hooks, filters, and routing
rules.

## Proposed Configuration

```yaml
version: "1"

compose:
  # Define a virtual config provider from two real ones
  config:
    merged-apps:
      type: config
      sources:
        - provider: structuredfile
          options: { directory: ./config/app-a }
          mount: "app-a/"
        - provider: pokedex
          mount: "pokedex/"
      merge: prefix

  # Define a virtual store provider that extends vault
  store:
    vault-managed:
      type: store
      base: vault
      base_options:
        addr: "http://vault:8200"
      hooks:
        after_put_values:
          - provider: vault-policy-manager
            action: sync-policies
        on_create_tree:
          - provider: vault-approle-creator
            action: create-approle

# Reference the composed providers like any other provider
config:
  provider: merged-apps

store:
  provider: vault-managed
```

### Hook System

The `hooks` section allows attaching behavior before/after operations on
the base plugin. Hook providers are separate plugins that implement a
new **hook interface** (a simple RPC that receives the operation context
and can perform side-effects).

```protobuf
service HookService {
  rpc OnEvent(HookEvent) returns (HookResponse);
}

message HookEvent {
  string event_type = 1;        // "after_put_values", "on_create_tree", etc.
  string tree_id = 2;
  repeated TreeEntry values = 3;
  map<string, string> params = 4;
}

message HookResponse {
  bool ok = 1;
  string error = 2;
}
```

### Filter and Transform Declarations

```yaml
compose:
  config:
    filtered-config:
      type: config
      base: structuredfile
      base_options: { directory: ./config }
      filters:
        include: ["database/*", "app/*"]   # only expose these paths
        exclude: ["app/internal/*"]        # hide internal paths
      transforms:
        - rename: { from: "database/", to: "db/" }
```

## Pros

- **Expressive without code**: Covers more use cases than simple
  multi-provider (hooks, filters, renames).
- **Cross-language**: Hook plugins are separate gRPC processes in any
  language.
- **Composition as configuration**: Version-controlled, auditable, no
  compilation step.
- **Incremental adoption**: Users can start with simple prefix merging
  and add hooks/filters as needs grow.
- **Reusable declarations**: The `compose` section can be extracted into
  a shared file and imported across workspaces.

## Cons

- **New DSL to learn**: Users must learn the composition declaration
  language on top of existing `zhi.yaml` syntax.
- **Debugging difficulty**: When something fails inside a composed
  provider, tracing back through hooks and filters is non-obvious.
- **Hook interface is another plugin type**: Adding `HookService` means
  another proto definition, another gRPC translation layer, and another
  plugin type to maintain.
- **Limited logic**: Declarative rules can express routing and simple
  hooks, but cannot express complex conditional logic (e.g., "if the
  path matches X and the value is Y, delegate to provider Z").
- **Validation is hard**: How does `Validate()` work for a composed
  config with filters and renames? The composed layer must translate
  paths back and forth.
- **Lifecycle management**: The engine must manage multiple plugin
  processes for a single logical provider, complicating startup/shutdown.
- **Scope creep risk**: The declaration language tends to grow over time
  as users request more features (conditionals, loops, variables),
  eventually approaching a full programming language.

## Comparison with Approach 1

Approach 2 is a superset of Approach 1. It supports everything Approach 1
does (merge strategies) plus hooks and filters. The trade-off is
additional complexity in both the configuration language and the engine
implementation.

## When This Approach Works Well

- Simple merge + hook combinations (the Vault + AppRole example)
- Path filtering and renaming
- Scenarios where composition needs are predictable and declarative

## When This Approach Falls Short

- Complex behavioral composition (custom logic in hooks)
- Dynamic composition that depends on runtime state
- When the declaration language becomes the bottleneck
