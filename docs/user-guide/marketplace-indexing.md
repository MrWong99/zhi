# Adding Plugins to the Marketplace Search Index

This guide explains how to add plugins to a zhi-marketplace instance so they appear in search results. There are two sides to this: **running a marketplace server** that accepts registrations, and **registering plugins** as a publisher.

## Running a Marketplace Server

Start the marketplace server with at least one API key mapped to a publisher ID:

```sh
zhi-marketplace \
  --db marketplace.db \
  --listen :8080 \
  --oci-registries ghcr.io,docker.io \
  --api-keys "zhk_abc123=myorg,zhk_def456=partner-team"
```

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | `marketplace.db` | Path to the database file |
| `--listen` | `:8080` | HTTP listen address |
| `--oci-registries` | `ghcr.io,docker.io` | Comma-separated list of known OCI registries (advertised in service discovery) |
| `--api-keys` | (none) | Comma-separated `key=publisherID` pairs for authenticating publishers |
| `--api-keys-file` | (none) | Path to a file containing `key=publisherID` pairs (one per line); **file is deleted after reading** |

The server automatically creates publisher records for any publisher ID referenced in `--api-keys` or `--api-keys-file` on first startup. Publishers can then use their API key to register plugins and versions.

### API Keys from a File

For production deployments, passing API keys on the command line exposes them in process listings. Use `--api-keys-file` instead to read keys from a file that is automatically deleted after reading:

```sh
# Create a keys file (e.g. from a secret mount or init container)
cat > /run/secrets/api-keys <<EOF
# Publisher API keys
zhk_abc123=myorg
zhk_def456=partner-team
EOF

zhi-marketplace \
  --db marketplace.db \
  --api-keys-file /run/secrets/api-keys
```

The file format is one `key=publisherID` pair per line. Blank lines and lines starting with `#` are ignored. Both `--api-keys` and `--api-keys-file` can be used together; keys from both sources are merged.

### Service Discovery

The server exposes `/.well-known/zhi-marketplace.json` for automatic client configuration:

```json
{
  "api": "http://localhost:8080/api/v1",
  "version": "1",
  "registries": ["ghcr.io", "docker.io"]
}
```

## Registering a Plugin

Once a plugin has been published to an OCI registry (see [Sharing and Registries](sharing-and-registries.md)), register it with the marketplace so it appears in search results.

### Prerequisites

1. A `zhi-plugin.yaml` manifest in your project directory.
2. The plugin binary already pushed to an OCI registry via `zhi plugin publish`.
3. An API key from the marketplace operator, configured in `~/.zhi/config.yaml`:

```yaml
marketplace:
  url: https://marketplace.example.com    # or http://localhost:8080
  apiKey: zhk_abc123
```

### Register the Plugin

From the directory containing your `zhi-plugin.yaml`:

```sh
zhi plugin register
```

This reads the manifest and sends the plugin metadata (name, type, OCI reference, description, keywords) to the marketplace. The plugin immediately becomes searchable.

### Register a New Version

When you publish a new version, notify the marketplace:

```sh
zhi plugin register --version-only --digest sha256:abc123def...
```

| Flag | Description |
|------|-------------|
| `--version-only` | Skip full registration; only notify of a new version |
| `--digest` | OCI manifest digest of the new version |
| `--changelog` | Changelog text for this version |

The version, platforms, and protocol version are read from `zhi-plugin.yaml`.

## How Search Indexing Works

The marketplace maintains an in-memory index of all registered plugins. When a user runs `zhi plugin search`, the server performs a full-text match across three fields:

- **Plugin name** (case-insensitive substring match)
- **Description** (case-insensitive substring match)
- **Keywords** (case-insensitive substring match on each keyword)

To make your plugin easier to find, use a descriptive name, write a clear description in your manifest, and add relevant keywords:

```yaml
name: ansible
type: config
description: Import and manage Ansible inventory and variable files
keywords:
  - ansible
  - inventory
  - automation
  - devops
```

Search results include aggregated metadata: latest version, total downloads, average rating (Bayesian-smoothed), and supported platforms.

## Complete Workflow Example

This walks through the full process of getting a plugin into the marketplace search index.

### 1. Create the Plugin Manifest

```yaml
# zhi-plugin.yaml
schemaVersion: "1"
name: redis
type: store
version: 1.0.0
zhiProtocolVersion: "1"
description: Redis-backed store for zhi configuration trees
author: myorg
license: Apache-2.0
homepage: https://github.com/myorg/zhi-store-redis
keywords:
  - redis
  - store
  - cache

binaries:
  linux/amd64: dist/zhi-store-redis-linux-amd64
  linux/arm64: dist/zhi-store-redis-linux-arm64
  darwin/amd64: dist/zhi-store-redis-darwin-amd64
  darwin/arm64: dist/zhi-store-redis-darwin-arm64
```

### 2. Publish to an OCI Registry

```sh
zhi plugin publish --registry ghcr.io/myorg --sign
```

### 3. Register with the Marketplace

```sh
zhi plugin register
```

### 4. Verify It Appears in Search

```sh
zhi plugin search redis --type store
```

Output:

```
NAME     TYPE   AUTHOR  VERSION  DOWNLOADS  RATING  DESCRIPTION
redis    store  myorg   1.0.0    0          -       Redis-backed store for zhi configuration trees
```

## API Reference

These are the marketplace API endpoints involved in plugin indexing. All write endpoints require a `Authorization: Bearer <api-key>` header.

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/plugins` | Required | Register a new plugin |
| POST | `/api/v1/plugins/{type}/{publisher}/{name}/versions` | Required | Register a new version |
| GET | `/api/v1/search?q=&type=&sort=&verified=&page=&per_page=` | Public | Search the index |
| GET | `/api/v1/plugins/{type}/{publisher}/{name}` | Public | Get plugin details |
| GET | `/api/v1/plugins/{type}/{publisher}/{name}/versions` | Public | List all versions |

## See Also

- [Sharing and Registries](sharing-and-registries.md) -- publishing plugins to OCI registries
- [Enterprise Mirror](enterprise-mirror.md) -- running a local mirror for air-gapped environments
- [Plugin Discovery](plugin-discovery.md) -- filesystem-based plugin discovery
