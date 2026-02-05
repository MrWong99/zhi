# Phase 4: Discovery — Marketplace API

**Goal**: Users can search for plugins without knowing OCI references.

**Prerequisites**: [Phase 1 (Foundation)](phase-1-foundation.md) — plugin install by OCI reference must work. [Phase 2 (Publishing)](phase-2-publishing.md) — plugin publish must work for registration flow.

## Overview

This phase introduces the central marketplace server — a lightweight metadata and discovery service that indexes plugins published to OCI registries. The marketplace does **not** store artifacts; it stores metadata, maps short names to OCI references, and provides search. This is the same pattern used by Artifact Hub for Helm charts.

The full API design, database schema, and server architecture are described in the [Marketplace Server](../marketplace-server.md) plan.

## Deliverables

### 1. Marketplace Server (`cmd/zhi-marketplace/`)

A Go HTTP server implementing the REST API from the [Marketplace Server](../marketplace-server.md) plan:

**Core endpoints**:
- `GET /.well-known/zhi-marketplace.json` — Service discovery
- `GET /api/v1/search` — Full-text search with filters (type, sort, verified)
- `GET /api/v1/plugins/{publisher}/{name}` — Plugin detail with all versions
- `GET /api/v1/plugins/{publisher}/{name}/versions` — Version listing
- `GET /api/v1/plugins/{publisher}/{name}/{version}/resolve` — Download resolution (returns OCI reference + digest for a specific OS/arch)

**Publisher management**:
- `POST /api/v1/publishers` — Register via GitHub OAuth
- `POST /api/v1/plugins` — Register a new plugin (authenticated)
- `POST /api/v1/plugins/{publisher}/{name}/versions` — Notify of a new version after OCI push

**Storage**:
- PostgreSQL for the central hosted instance (schema in [Marketplace Server](../marketplace-server.md))
- SQLite option for self-hosted / single-node deployments
- Full-text search via PostgreSQL `tsvector` or SQLite FTS5

**Authentication**:
- GitHub OAuth for publisher accounts (web flow)
- API keys for CLI operations (`zhi plugin register`, version notifications)

**Metadata sync**:
- Periodic validation that OCI references still exist
- Pull README content from OCI artifact layers
- Update platform lists and signing status from OCI manifests

### 2. CLI Integration

**`zhi plugin search <query>`**:
- Query the marketplace API
- Display results in a table with name, type, version, rating, downloads, and verified badge
- Support `--type`, `--sort`, `--verified`, `--json` flags
- Full specification in [CLI Integration](../cli-integration.md)

**`zhi plugin info <name>`**:
- Fetch from marketplace API when the argument is a short name (not an OCI reference)
- Display detailed plugin information: description, author, versions, rating, platforms, signing status
- Show install status if the plugin is already installed locally

**Short-name resolution**:
- When `zhi plugin install ansible-config` is called without an OCI prefix:
  1. Query marketplace API: `GET /api/v1/plugins/*/ansible-config` (or search by exact name)
  2. Resolve to the full OCI reference
  3. Proceed with standard OCI pull from Phase 1
- The default marketplace URL is configured in `~/.zhi/config.yaml` under `sharing.marketplace.url`

**`zhi plugin register`** (called after publishing):
- Notify the marketplace of a new plugin or version
- Sends OCI reference, description, and metadata
- Requires API key authentication

### 3. Marketplace Website (Optional)

A static site generated from marketplace API data for browsing in a web browser:

- Plugin detail pages with rendered README (Markdown to HTML)
- Category browsing (as defined in [Marketplace Server](../marketplace-server.md) under "Categories and Curation")
- Search with filters
- Can be hosted on GitHub Pages or any static hosting

This is a nice-to-have for this phase — the CLI search is the primary interface, and [Phase 6](phase-6-ui-integration.md) adds in-app browsing.

## Key Files to Modify

| File | Change |
|---|---|
| `internal/cli/plugin_install.go` | Add short-name resolution fallback to marketplace |
| `pkg/sharing/client/` | Add `Search()` and `ListVersions()` methods |
| `pkg/sharing/registry/` | Add marketplace configuration (`MarketplaceConfig`) |

## New Files

```
cmd/zhi-marketplace/          # Marketplace server binary
  main.go
  server/
    routes.go                 # HTTP route definitions
    handlers.go               # Request handlers
    search.go                 # Full-text search logic
    sync.go                   # OCI metadata sync
  storage/
    postgres.go               # PostgreSQL storage backend
    sqlite.go                 # SQLite storage backend
    models.go                 # Database models
  auth/
    github.go                 # GitHub OAuth flow
    apikey.go                 # API key validation
internal/cli/plugin_search.go # zhi plugin search command
internal/cli/plugin_info.go   # zhi plugin info command
internal/cli/plugin_register.go # zhi plugin register command
pkg/sharing/marketplace/      # Client library for marketplace API
```

## Configuration

```yaml
# ~/.zhi/config.yaml
sharing:
  marketplace:
    url: https://marketplace.zhi.dev   # default central marketplace
    apiKey: zhk_abc123...              # for publishing and registration
```

## Exit Criteria

- `zhi plugin search ansible` returns results from the central marketplace
- `zhi plugin info ansible-config` shows detailed information from the marketplace
- `zhi plugin install ansible-config` resolves the short name via marketplace and installs
- Publishers can register plugins via `zhi plugin register`
- The marketplace server runs as a single binary with either PostgreSQL or SQLite
- Self-hosted marketplace works for organizations with internal registries

## Design References

- [Marketplace Server](../marketplace-server.md) — Full REST API design, database schema, server architecture, categories, self-hosted option
- [CLI Integration](../cli-integration.md) — `zhi plugin search`, `zhi plugin info`, `zhi plugin register` commands
- [Architecture Overview](../architecture.md) — System component diagram showing marketplace as a metadata layer over OCI registries
