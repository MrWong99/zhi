# TODOs

Tracks remaining work and future ideas.

## Engine & UI Integration

- [ ] Expose Capabilities in the UI so users see versioning/encryption/auth status
- [ ] Wire authentication flow into the UI (prompt for auth method + credentials before accessing a tree)
- [ ] Add CAS conflict resolution UI (show diff, let user choose)
- [ ] Wire version browsing and rollback into the TUI
- [ ] Wire access control management into the TUI (grant/revoke for admin users)
- [ ] Consider Vault identity entities/groups for user representation

---

## Future Ideas (Sharing System)

Ideas worth considering as natural extensions of the sharing system. These are not committed to a timeline.

### Plugin Templates / Scaffolding

Generate complete plugin projects from templates, lowering the barrier to plugin development:

```bash
zhi plugin new --type config --name my-plugin --lang go
```

Generates a standalone Go project (`cmd/`, `pkg/`, `workspace/`) with:
- Plugin implementation with correct interface stubs and handshake
- Tests using in-process gRPC (`goplugin.TestPluginGRPCConn`)
- `go.mod` with zhi dependencies
- `Makefile` with build, test, lint, and cross-compilation targets
- `zhi-plugin.yaml` manifest pre-populated with metadata
- GitHub Actions workflow for automated build, sign, and publish
- Sample workspace (`zhi.yaml`) for local testing with the plugin
- README with usage instructions

Additional flags: `--module`, `--registry`, `--author`, `--license`, `--description`, `--output-dir`.

The scaffolding system (`internal/scaffold/`) is language-agnostic: implement
the `Scaffolder` interface and register it to add support for new languages.

### Workspace Templates

Pre-configured workspaces for common scenarios, installable as starting points:

```bash
zhi workspace new --from template/kubernetes
zhi workspace new --from template/ansible-infra
zhi workspace new --from template/developer-laptop
```

Templates are published as workspace artifacts tagged with a `template` category. `zhi workspace new` is sugar over `zhi workspace install` that also runs interactive prompts to customize the workspace (rename, set initial config values, etc.).

### Plugin Composition

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

### WebAssembly Plugin Support

For maximum portability and sandboxing, support WASM-based plugins via [Wazero](https://wazero.io/) (pure Go WebAssembly runtime):

**Benefits**:
- Plugins compiled to WASM run on any platform without cross-compilation
- Memory-safe sandbox with explicit capability grants (filesystem paths, network access)
- Smaller binary sizes (typically 1-5 MB)
- No gRPC transport needed -- function calls through WASM imports/exports

**Trade-offs**:
- No direct filesystem or network access without WASI
- Performance overhead compared to native binaries
- Ecosystem is less mature than native Go plugins
- Not all languages compile to WASM equally well

### Plugin Dependency Graph

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

### Marketplace Federation

Multiple marketplace instances that discover each other's catalogs:

```yaml
# ~/.zhi/config.yaml
sharing:
  marketplaces:
    - url: https://marketplace.zhi.dev      # official
    - url: https://zhi.company.internal     # corporate
    - url: https://zhi-community.org        # community
```

- `zhi plugin search` fans out to all configured marketplaces in parallel
- Results are merged and deduplicated
- Install resolves from the marketplace that has the artifact
- Ratings and verification are per-marketplace

### CI/CD Integration Plugin

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

### Plugin Analytics Dashboard

For plugin publishers -- a dashboard showing:
- Download trends over time
- Platform distribution (which OS/arch variants are most used)
- Version adoption curve (how quickly users upgrade)
- Geographic distribution (anonymized)
- Compatibility reports (which zhi versions are in use)

### Offline Plugin Development

Tools for developing and testing plugins without publishing to a registry:

```bash
# Build and install locally in one step
zhi plugin dev-install ./path/to/my-plugin

# Watch mode: rebuild and reinstall on file changes
zhi plugin dev-watch ./path/to/my-plugin

# Test against a workspace without installing
zhi run --override-plugin config=./path/to/my-plugin/bin/zhi-config-myplugin
```
