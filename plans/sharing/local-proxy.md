# Local Proxy / Registry Mirror

A local registry that acts as a caching proxy for the central marketplace and OCI registries. Designed for enterprise environments, air-gapped networks, and teams that want to curate which plugins are available internally.

## Use Cases

1. **Caching proxy**: Reduce bandwidth and latency by caching pulled artifacts locally. Repeated installs across a team hit the local mirror instead of the internet.

2. **Air-gapped environments**: Networks with no internet access can pre-populate the local registry with approved plugins, then point all zhi clients to the local mirror.

3. **Curated catalog**: Organizations restrict which plugins are available by only mirroring approved artifacts. The local proxy acts as a whitelist.

4. **Compliance**: All plugin downloads are logged centrally. Security teams can audit which plugins are in use and when they were updated.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                     Corporate Network                             │
│                                                                   │
│  ┌─────────┐     ┌──────────────────────────┐                    │
│  │ Dev A    │────▶│                          │                    │
│  └─────────┘     │   zhi-mirror             │                    │
│  ┌─────────┐     │                          │     ┌────────┐    │
│  │ Dev B    │────▶│   ┌─────────────────┐   │────▶│Internet│    │
│  └─────────┘     │   │ OCI Registry    │   │     │        │    │
│  ┌─────────┐     │   │ (distribution)  │   │     │ GHCR   │    │
│  │ CI/CD   │────▶│   └─────────────────┘   │     │ Docker │    │
│  └─────────┘     │   ┌─────────────────┐   │     │ Hub    │    │
│                  │   │ Marketplace API │   │     └────────┘    │
│                  │   │ (metadata cache)│   │                    │
│                  │   └─────────────────┘   │                    │
│                  │   ┌─────────────────┐   │                    │
│                  │   │ Policy Engine   │   │                    │
│                  │   │ (allow/block)   │   │                    │
│                  │   └─────────────────┘   │                    │
│                  └──────────────────────────┘                    │
└──────────────────────────────────────────────────────────────────┘
```

## `zhi-mirror` Server

A single Go binary that runs as a local service:

```bash
zhi-mirror serve \
  --listen :5050 \
  --storage /var/lib/zhi-mirror \
  --upstream-registry ghcr.io \
  --upstream-marketplace https://marketplace.zhi.dev \
  --policy /etc/zhi-mirror/policy.yaml
```

### Components

#### OCI Pull-Through Cache

The mirror implements the OCI Distribution API and acts as a pull-through cache:

1. Client requests `GET /v2/zhi-project/zhi-config-ansible/manifests/v1.2.0`
2. Mirror checks local storage
3. If not cached: fetches from upstream, stores locally, returns to client
4. If cached: returns from local storage (with optional TTL-based revalidation)

This is the same mechanism used by Docker Registry mirrors and Harbor proxy caches.

#### Marketplace API Proxy

The mirror also proxies the marketplace metadata API:

1. Client requests `GET /api/v1/search?q=ansible`
2. Mirror fetches from upstream marketplace (or returns cached results)
3. Filters results based on policy (removes blocked plugins)
4. Returns filtered results to client

#### Policy Engine

A YAML-based policy controls what can be pulled through the mirror:

```yaml
# /etc/zhi-mirror/policy.yaml

# Only mirror artifacts from these publishers
allowedPublishers:
  - zhi-project
  - myorg
  - approved-vendor

# Block specific artifacts
blockedArtifacts:
  - untrusted/bad-plugin

# Only mirror these plugin types
allowedTypes:
  - config
  - transform
  - store
  - ui

# Require signatures on all mirrored artifacts
requireSignatures: true

# Auto-sync these artifacts (pre-populate cache)
sync:
  - ref: oci://ghcr.io/zhi-project/zhi-config-ansible:v1.*
    schedule: "0 2 * * *"  # daily at 2 AM
  - ref: oci://ghcr.io/zhi-project/zhi-store-vault:v2.*
    schedule: "0 2 * * *"

# Retention: remove cached artifacts not pulled in 90 days
retention:
  unusedDays: 90
```

### Storage Backend

The mirror stores OCI blobs on the local filesystem in OCI Layout format:

```
/var/lib/zhi-mirror/
├── oci/
│   ├── index.json
│   └── blobs/
│       └── sha256/
│           ├── abc123...  (manifests)
│           ├── def456...  (config blobs)
│           └── 789xyz...  (binary layers)
├── metadata/
│   └── marketplace-cache.db  (SQLite cache of marketplace API responses)
└── logs/
    └── access.log  (who pulled what and when)
```

## Client Configuration

Users point their zhi client to the local mirror:

```yaml
# ~/.zhi/config.yaml
sharing:
  defaultRegistry: mirror.internal:5050
  marketplace:
    url: http://mirror.internal:5050
  registries:
    mirror.internal:5050:
      insecure: true  # if using HTTP internally
```

Or via environment variable:

```bash
export ZHI_REGISTRY=mirror.internal:5050
export ZHI_MARKETPLACE=http://mirror.internal:5050
```

## Air-Gapped Operation

### Seeding the Mirror

For networks with no internet access, the mirror is seeded from an export bundle:

```bash
# On internet-connected machine:
zhi-mirror export \
  --artifacts zhi-project/zhi-config-ansible:v1.2.0 \
              zhi-project/zhi-store-vault:v2.0.0 \
              zhi-project/zhi-ui-web:v1.0.0 \
  --all-platforms \
  --output /media/usb/zhi-bundle.tar

# Transfer USB to air-gapped network

# On air-gapped mirror:
zhi-mirror import --input /media/usb/zhi-bundle.tar
```

The export includes:
- OCI Layout directory with all artifacts and platform variants
- Marketplace metadata for exported artifacts
- Signature data for offline verification

### Workspace-Based Export

For simpler cases, export all plugins required by a workspace:

```bash
# Export everything a workspace needs
zhi-mirror export --workspace /path/to/zhi.yaml --output bundle.tar

# Includes all plugins referenced in zhi.yaml sharing.plugins section
# plus their signatures and metadata
```

## Audit Logging

The mirror logs all pull operations for compliance:

```json
{
  "timestamp": "2026-02-05T10:30:00Z",
  "action": "pull",
  "client": "10.0.1.42",
  "artifact": "zhi-project/zhi-config-ansible",
  "version": "v1.2.0",
  "platform": "linux/amd64",
  "digest": "sha256:abc123...",
  "cached": true,
  "user": "alice@company.com"
}
```

Logs can be forwarded to SIEM systems via syslog or structured JSON output.

## High Availability

For teams that need high availability:

1. **Multiple mirrors**: Run multiple `zhi-mirror` instances behind a load balancer. They share a common storage backend (NFS, S3, or shared filesystem).

2. **Mirror-to-mirror sync**: A secondary mirror can be configured to sync from a primary mirror instead of the upstream internet registries. This creates a tiered caching hierarchy.

3. **Warm cache**: Use the `sync` policy to pre-populate the cache with commonly used plugins on a schedule, so they are always available even if the upstream is temporarily unreachable.
