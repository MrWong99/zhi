# Future Ideas (Post-Roadmap)

Ideas worth considering after the core 8 phases are delivered. These are not committed to a timeline but represent natural extensions of the sharing system.

## Plugin Templates / Scaffolding

Generate complete plugin projects from templates, lowering the barrier to plugin development:

```bash
zhi plugin new --type config --name my-plugin --lang go
```

Generates:
- `main.go` with plugin interface stubs (correct handshake, interface implementation)
- `Makefile` with cross-compilation targets for all supported platforms
- `zhi-plugin.yaml` manifest pre-populated with metadata
- GitHub Actions workflow for automated building and publishing (uses [Phase 2](phase-2-publishing.md) `zhi plugin publish`)
- README template with usage instructions
- `.goreleaser.yaml` for GoReleaser-based builds

**Design references**: Plugin interface contracts in the [Architecture Overview](../architecture.md), publishing flow in [Phase 2](phase-2-publishing.md).

## Workspace Templates

Pre-configured workspaces for common scenarios, installable as starting points:

```bash
zhi workspace new --from template/kubernetes
zhi workspace new --from template/ansible-infra
zhi workspace new --from template/developer-laptop
```

Templates are published as workspace artifacts ([Phase 2](phase-2-publishing.md)) tagged with a `template` category. `zhi workspace new` is sugar over `zhi workspace install` that also runs interactive prompts to customize the workspace (rename, set initial config values, etc.).

**Design references**: Workspace artifacts in the [Distribution Format](../distribution-format.md), categories in the [Marketplace Server](../marketplace-server.md).

## Plugin Composition

Plugins that wrap or extend other plugins, enabling layered functionality:

```yaml
# zhi-plugin.yaml
name: cached-vault
type: store
extends: vault    # wraps the vault store plugin
description: Vault store with local caching layer
```

The `extends` field tells zhi to load the base plugin and inject it into the extending plugin via a new Controller method. The extending plugin intercepts calls and delegates to the base.

Use cases:
- Caching layer over any store plugin
- Logging/auditing wrapper for any plugin type
- Fallback chains (try plugin A, fall back to plugin B)

This requires extending the plugin interface model and is a significant architectural addition.

## WebAssembly Plugin Support

For maximum portability and sandboxing, support WASM-based plugins via [Wazero](https://wazero.io/) (pure Go WebAssembly runtime):

**Benefits**:
- Plugins compiled to WASM run on any platform without cross-compilation — no need for multi-platform OCI Image Index
- Memory-safe sandbox with explicit capability grants (filesystem paths, network access)
- Smaller binary sizes (typically 1-5 MB)
- No gRPC transport needed — function calls through WASM imports/exports

**Trade-offs**:
- No direct filesystem or network access without WASI
- Performance overhead compared to native binaries
- Ecosystem is less mature than native Go plugins
- Not all languages compile to WASM equally well

**Approach**: Define a WASM plugin interface that mirrors the gRPC interface semantics. Support both native (gRPC) and WASM plugins simultaneously. The registry resolves either type based on the artifact media type.

**Design references**: Sandboxing considerations in [Security & Trust](../security.md), WASM OCI artifact precedent in [Distribution Format](../distribution-format.md).

## Plugin Dependency Graph

Plugins that depend on other plugins (not just workspaces depending on plugins):

```yaml
# zhi-plugin.yaml
name: advanced-config
type: config
dependencies:
  - name: base-config
    type: config
    version: ">=1.0.0"
```

Use cases:
- A config plugin that reads from a base source and adds validation
- A transform plugin that chains with another transform
- A UI plugin that requires a specific store for its settings

This requires dependency resolution during install (topological sort, conflict detection) and extends the plugin lifecycle in the engine.

## Marketplace Federation

Multiple marketplace instances that discover each other's catalogs:

```yaml
# ~/.zhi/config.yaml
sharing:
  marketplaces:
    - url: https://marketplace.zhi.dev      # official
    - url: https://zhi.company.internal     # corporate
    - url: https://zhi-community.org        # community
```

**Behavior**:
- `zhi plugin search` fans out to all configured marketplaces in parallel
- Results are merged and deduplicated (same plugin from multiple marketplaces shows the source)
- Install resolves from the marketplace that has the artifact
- Ratings and verification are per-marketplace (no cross-marketplace trust)

**Design references**: Marketplace API in [Marketplace Server](../marketplace-server.md), client configuration in [CLI Integration](../cli-integration.md).

## CI/CD Integration Plugin

A dedicated plugin or CLI extension for CI/CD pipelines:

```yaml
# .github/workflows/deploy.yml
- name: Install zhi plugins
  run: zhi workspace setup --from-lock --ci
```

The `--ci` flag:
- Non-interactive (no prompts)
- Strict verification (fail on unsigned plugins)
- Digest-pinned from lock file only
- Reports installed plugin versions as a build artifact
- Integrates with GitHub Actions caching for `~/.zhi/cache/`

## Plugin Analytics Dashboard

For plugin publishers — a dashboard showing:
- Download trends over time
- Platform distribution (which OS/arch variants are most used)
- Version adoption curve (how quickly users upgrade)
- Geographic distribution (anonymized, from marketplace CDN logs)
- Compatibility reports (which zhi versions are in use)

Could be a feature of the marketplace website or a separate publisher portal.

## Offline Plugin Development

Tools for developing and testing plugins without publishing to a registry:

```bash
# Build and install locally in one step
zhi plugin dev-install ./path/to/my-plugin

# Watch mode: rebuild and reinstall on file changes
zhi plugin dev-watch ./path/to/my-plugin

# Test against a workspace without installing
zhi run --override-plugin config=./path/to/my-plugin/bin/zhi-config-myplugin
```

This makes the develop-test cycle faster than the publish-pull-install loop.
