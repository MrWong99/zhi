# Implementation Roadmap

Phased delivery plan where each phase delivers standalone value. Later phases build on earlier ones but are not required for the system to be useful.

## Phase 1: Foundation — Plugin Pull & Install

**Goal**: Users can install plugins from OCI registries via CLI.

### Deliverables

1. **Plugin manifest format** (`zhi-plugin.yaml`)
   - Define the schema for plugin metadata
   - Implement parsing and validation in `pkg/sharing/manifest/`

2. **OCI client library** (`pkg/sharing/client/`)
   - Integrate oras-go v2 for push/pull operations
   - Multi-platform manifest resolution (select correct OS/arch)
   - Authentication support (Basic, Bearer, credential helpers)
   - Local blob cache in `~/.zhi/cache/oci/`

3. **CLI commands** (`internal/cli/`)
   - `zhi plugin install <oci-ref>`
   - `zhi plugin uninstall <name>`
   - `zhi plugin list` (extend existing to show installed version and source)
   - `zhi registry login/logout`

4. **Local metadata store** (`~/.zhi/metadata/`)
   - Track installed plugins with version, digest, and install date
   - Used by update checks in later phases

5. **Integration with existing discovery**
   - Installed plugins land in `~/.zhi/plugins/` with standard naming
   - `RefreshExternal()` in `internal/core/registry.go` picks them up automatically

### Key Files to Modify

| File | Change |
|---|---|
| `internal/core/discovery.go` | No changes needed (installed plugins use existing discovery) |
| `internal/core/registry.go` | Optional: add metadata awareness for `ListConfigProviders()` etc. |
| `internal/cli/root.go` | Register new `plugin` and `registry` command groups |
| `internal/cli/list.go` | Extend provider listing with version/update columns |

### New Packages

```
pkg/sharing/
├── client/        # OCI artifact operations (oras-go wrapper)
├── manifest/      # Plugin/workspace manifest types and parsing
├── registry/      # Registry configuration and authentication
└── metadata/      # Local installed-plugin metadata store
```

### Dependencies

```
oras.land/oras-go/v2    # OCI artifact push/pull
```

### Exit Criteria

- A user can run `zhi plugin install oci://ghcr.io/example/plugin:v1.0` and use the plugin in their workspace
- Multi-platform support works (correct binary selected for OS/arch)
- Authentication works with GHCR, Docker Hub, and Harbor

---

## Phase 2: Publishing — Plugin Push & Workspace Sharing

**Goal**: Plugin authors can publish to OCI registries. Workspaces can be packaged and shared.

### Deliverables

1. **`zhi plugin init`** — Generate `zhi-plugin.yaml` interactively
2. **`zhi plugin publish`** — Build OCI artifact from manifest + binaries, push to registry
3. **Multi-platform image index** — Create OCI Image Index from per-platform builds
4. **Workspace artifacts**
   - `zhi workspace publish` — Package `zhi.yaml` + templates as OCI artifact
   - `zhi workspace install` — Pull workspace, resolve plugin dependencies
5. **Lock file**
   - `zhi workspace lock` — Generate `zhi-plugins.lock` with pinned digests
   - `zhi workspace setup --from-lock` — Reproducible install from lock file

### Exit Criteria

- A developer can publish a plugin with `zhi plugin publish --registry ghcr.io/myorg`
- A workspace with plugin dependencies can be installed in one command
- Lock files enable reproducible CI/CD installations

---

## Phase 3: Security — Signing & Verification

**Goal**: All artifacts can be signed and verified. Binary integrity is enforced.

### Deliverables

1. **Sigstore/cosign integration**
   - Sign artifacts during `zhi plugin publish --sign`
   - Verify signatures during `zhi plugin install`
   - Support both keyless (OIDC) and key-based signing
2. **Binary integrity checks**
   - Compare installed binary hash against OCI digest on every launch
   - Extend existing `auditPluginBinary()` with digest verification
3. **Trust store** (`~/.zhi/keys/`)
   - Trusted publisher keys and identities
   - Policy file for strict mode (`requireSignatures: true`)
4. **Verification display**
   - Show signer identity during install
   - Show verification status in `zhi plugin list` and `zhi plugin info`

### New Dependencies

```
github.com/sigstore/sigstore-go   # Signature verification
```

### Exit Criteria

- `zhi plugin publish --sign` creates a cosign signature
- `zhi plugin install` verifies signatures and shows signer identity
- Strict mode rejects unsigned plugins
- Binary tampering post-install is detected at launch time

---

## Phase 4: Discovery — Marketplace API

**Goal**: Users can search for plugins without knowing OCI references.

### Deliverables

1. **Marketplace server** (`cmd/zhi-marketplace/`)
   - REST API for search, detail, version listing
   - PostgreSQL storage for metadata, SQLite for single-node deployments
   - Publisher registration via GitHub OAuth
   - Plugin registration via API (`zhi plugin register`)
2. **CLI integration**
   - `zhi plugin search <query>`
   - `zhi plugin info <name>` (pulls from marketplace)
   - Short-name resolution: `ansible-config` → OCI reference lookup
3. **Marketplace website** (optional)
   - Static site generated from marketplace API data
   - Plugin detail pages with README rendering
   - Category browsing

### Exit Criteria

- `zhi plugin search ansible` returns results from the central marketplace
- `zhi plugin install ansible-config` resolves the short name and installs
- Publishers can register plugins via `zhi plugin register`

---

## Phase 5: Community — Ratings & Verification

**Goal**: Trust signals help users choose reliable plugins.

### Deliverables

1. **Rating system**
   - Submit ratings via CLI (`zhi plugin rate <name>`) or marketplace API
   - Bayesian weighted averages
   - Helpfulness voting
2. **Verified publisher program**
   - Application and review process
   - Automated checks (license, security scan, signing)
   - ✓ badge in search results and plugin info
3. **Download statistics**
   - Track download counts (anonymized)
   - Display trends in marketplace
4. **Vulnerability advisories**
   - Advisory database in marketplace
   - `zhi plugin update --check` warns about affected plugins

### Exit Criteria

- Users can rate plugins (1-5 stars + review)
- Verified publishers show ✓ in all UIs
- `zhi plugin update --check` warns about security advisories

---

## Phase 6: UI Integration — Marketplace in TUI/Web

**Goal**: Marketplace is browsable from any UI plugin.

### Deliverables

1. **Controller interface extension**
   - Add marketplace methods to `Controller` in `pkg/zhiplugin/ui/plugin.go`
   - Implement in `UIController` (`internal/ui/controller.go`)
   - gRPC protocol extension in `api/proto/zhiplugin/v1/ui.proto`
2. **TUI marketplace view**
   - Search, browse, and install from the TUI
   - Plugin detail view with ratings
   - Installed plugins tab with update indicators
3. **HTTP API endpoints**
   - Marketplace proxy endpoints for web UI plugins
   - The `zhi-ui-httpapi` example can serve as reference implementation

### Exit Criteria

- The TUI has a working Marketplace tab with search and install
- The HTTP API example exposes marketplace endpoints
- Update notifications appear in UIs on startup

---

## Phase 7: Enterprise — Local Proxy & Air-Gapped

**Goal**: Organizations can run their own registry mirror with policy controls.

### Deliverables

1. **`zhi-mirror` server** (`cmd/zhi-mirror/`)
   - OCI pull-through cache
   - Marketplace metadata proxy
   - Policy engine (allow/block publishers, require signatures)
   - Audit logging
2. **Export/import for air-gapped networks**
   - `zhi-mirror export` — bundle artifacts for offline transfer
   - `zhi-mirror import` — seed a mirror from a bundle
   - `zhi-mirror export --workspace` — export all plugins for a workspace
3. **Client configuration**
   - Point zhi to local mirror via config or environment variable
   - Mirror acts as transparent proxy (no client changes needed)

### Exit Criteria

- `zhi-mirror serve` runs an OCI pull-through cache
- Policy engine enforces publisher allowlists
- Air-gapped install works via export/import cycle

---

## Phase 8: Updates — Automatic Update System

**Goal**: Plugins stay current with minimal user effort.

### Deliverables

1. **Update detection**
   - Background version check with configurable interval
   - Cache results to avoid repeated API calls
   - Security advisory integration
2. **Update CLI**
   - `zhi plugin update --check`
   - `zhi plugin update [--all]`
   - `zhi plugin pin/unpin`
   - `zhi plugin rollback`
3. **Auto-update (opt-in)**
   - Configurable scope (patch only, minor, all)
   - Runs at `zhi run` startup
   - Backup of previous version for rollback
4. **Changelog display**
   - Show what changed between installed and available version
   - Sourced from marketplace or OCI config metadata

### Exit Criteria

- `zhi plugin update --check` shows available updates
- `zhi plugin update --all` updates everything
- Rollback restores the previous version
- Auto-update works for patch versions when opted in

---

## Additional Ideas (Post-Roadmap)

These are worth considering but not planned for the initial roadmap:

### Plugin Templates / Scaffolding

```bash
zhi plugin new --type config --name my-plugin --lang go
# Generates a complete plugin project with:
# - main.go with plugin interface stubs
# - Makefile with cross-compilation targets
# - zhi-plugin.yaml manifest
# - GitHub Actions workflow for publishing
# - README template
```

### Workspace Templates

Pre-configured workspaces for common scenarios, installable as starting points:

```bash
zhi workspace new --from template/kubernetes
zhi workspace new --from template/ansible-infra
zhi workspace new --from template/developer-laptop
```

### Plugin Composition

Plugins that wrap or extend other plugins:

```yaml
# zhi-plugin.yaml
name: cached-vault
type: store
extends: vault    # wraps the vault store plugin
description: Vault store with local caching layer
```

### WebAssembly Plugin Support

For maximum portability and sandboxing, support WASM plugins via Wazero:
- Plugins compiled to WASM run on any platform without cross-compilation
- Memory-safe sandbox with explicit capability grants
- Smaller binary sizes
- Trade-off: no direct filesystem/network access without WASI

### Plugin Dependency Graph

Plugins that depend on other plugins (not just workspaces depending on plugins):

```yaml
# zhi-plugin.yaml
dependencies:
  - name: base-config
    type: config
    version: ">=1.0.0"
```

### Marketplace Federation

Multiple marketplace instances that can discover each other's catalogs:

```yaml
sharing:
  marketplaces:
    - url: https://marketplace.zhi.dev      # official
    - url: https://zhi.company.internal     # corporate
```

Search queries fan out to all configured marketplaces, with results merged and deduplicated.
