# Phase 7: Enterprise — Local Proxy & Air-Gapped

**Goal**: Organizations can run their own registry mirror with policy controls.

**Prerequisites**: [Phase 1 (Foundation)](phase-1-foundation.md) — OCI pull must work. [Phase 4 (Discovery)](phase-4-discovery.md) — marketplace API provides the upstream to proxy.

## Overview

This phase addresses enterprise and air-gapped deployment scenarios with `zhi-mirror`, a local server that acts as a caching proxy for OCI registries and the marketplace API. Organizations use it to reduce bandwidth, enforce policies (allowlists, signature requirements), audit all plugin activity, and operate without internet access.

The full architecture, policy engine, and air-gapped workflows are described in the [Local Proxy](../local-proxy.md) plan.

## Deliverables

### 1. `zhi-mirror` Server (`cmd/zhi-mirror/`)

A single Go binary implementing:

**OCI Pull-Through Cache**:
- Implements the OCI Distribution API (`/v2/` endpoints)
- Acts as a transparent pull-through cache between zhi clients and upstream OCI registries
- Cache flow: client requests artifact → mirror checks local storage → if miss, fetches from upstream, caches, returns → if hit, returns from cache
- TTL-based revalidation for tags (digests are immutable and cached permanently)
- Storage in OCI Layout format on local filesystem (see [Local Proxy](../local-proxy.md) for directory structure)

**Marketplace API Proxy**:
- Proxies and caches marketplace API responses (search, detail, versions)
- Filters results based on policy engine (removes blocked plugins from search results)
- Caches metadata in SQLite for fast local queries

**Audit Logging**:
- All pull operations logged with timestamp, client IP, artifact reference, digest, platform, and user
- Structured JSON format for SIEM integration
- Configurable output: file, stdout, or syslog

### 2. Policy Engine

YAML-based policy that controls what can be pulled through the mirror (full specification in [Local Proxy](../local-proxy.md)):

```yaml
# /etc/zhi-mirror/policy.yaml
allowedPublishers:
  - zhi-project
  - myorg
blockedArtifacts:
  - untrusted/bad-plugin
allowedTypes:
  - config
  - transform
  - store
  - ui
requireSignatures: true
```

Policy enforcement points:
- OCI pull requests: reject artifacts from blocked publishers or types
- Marketplace search proxy: filter results to only show allowed artifacts
- Signature requirement: reject unsigned artifacts even if the publisher is allowed

### 3. Pre-Population Sync

Automated sync of approved artifacts into the mirror cache:

```yaml
# In policy.yaml
sync:
  - ref: oci://ghcr.io/zhi-project/zhi-config-ansible:v1.*
    schedule: "0 2 * * *"   # daily at 2 AM
  - ref: oci://ghcr.io/zhi-project/zhi-store-vault:v2.*
    schedule: "0 2 * * *"
```

The mirror pulls matching artifacts on schedule, ensuring they are always available even if the upstream is temporarily unreachable.

### 4. Air-Gapped Export/Import

For networks with no internet access:

**`zhi-mirror export`**:
- Download specified artifacts (all platform variants) to an OCI Layout directory
- Include marketplace metadata for exported artifacts
- Include signature data for offline verification
- Package as a tarball for physical transfer

```bash
# On internet-connected machine
zhi-mirror export \
  --artifacts zhi-project/zhi-config-ansible:v1.2.0 \
              zhi-project/zhi-store-vault:v2.0.0 \
  --all-platforms \
  --output /media/usb/zhi-bundle.tar
```

**`zhi-mirror export --workspace`**:
- Export all plugins required by a workspace's `zhi.yaml` `sharing.plugins` section
- Simpler than listing individual artifacts

**`zhi-mirror import`**:
- Seed a mirror's local storage from an exported bundle
- Import marketplace metadata into the local cache

```bash
# On air-gapped mirror
zhi-mirror import --input /media/usb/zhi-bundle.tar
```

### 5. Client Configuration

Clients point to the local mirror via config or environment variables:

```yaml
# ~/.zhi/config.yaml
sharing:
  defaultRegistry: mirror.internal:5050
  marketplace:
    url: http://mirror.internal:5050
  registries:
    mirror.internal:5050:
      insecure: true   # allow HTTP for internal networks
```

Or:
```bash
export ZHI_REGISTRY=mirror.internal:5050
export ZHI_MARKETPLACE=http://mirror.internal:5050
```

The mirror is transparent — no changes to `zhi plugin install` or any other command. They work the same way, just against a different registry.

### 6. Retention Policy

Automatic cleanup of unused cached artifacts:

```yaml
retention:
  unusedDays: 90    # remove artifacts not pulled in 90 days
  maxStorage: 50GB  # hard cap on storage (LRU eviction)
```

## Key Files

```
cmd/zhi-mirror/
  main.go
  server/
    oci.go            # OCI Distribution API (pull-through cache)
    marketplace.go    # Marketplace API proxy
    policy.go         # Policy engine
    sync.go           # Pre-population sync scheduler
    audit.go          # Audit logging
  storage/
    oci_layout.go     # OCI Layout local storage
    metadata.go       # SQLite metadata cache
  airgap/
    export.go         # Bundle export
    import.go         # Bundle import
```

## Exit Criteria

- `zhi-mirror serve` runs an OCI pull-through cache that zhi clients can pull from
- Policy engine enforces publisher allowlists and blocks unauthorized artifacts
- Sync schedule pre-populates the cache with approved artifacts
- `zhi-mirror export` creates a bundle; `zhi-mirror import` seeds a mirror from it
- Air-gapped install works: export on connected machine → transfer → import → install from mirror
- Audit logs record all pull operations in structured JSON

## Design References

- [Local Proxy](../local-proxy.md) — Full architecture, policy engine specification, air-gapped workflows, storage layout, high availability
- [Security & Trust](../security.md) — Policy files, allowed registries, organization security policies
- [Architecture Overview](../architecture.md) — System diagram showing local proxy in the data flow
- [CLI Integration](../cli-integration.md) — Registry configuration in `~/.zhi/config.yaml`
