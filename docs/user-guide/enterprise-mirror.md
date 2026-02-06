# Enterprise Mirror (`zhi-mirror`)

`zhi-mirror` is a local registry mirror for enterprise and air-gapped environments. It acts as a pull-through cache between zhi clients and upstream OCI registries and the marketplace API.

## Features

- **OCI pull-through cache** -- transparently caches artifacts from upstream registries
- **Marketplace API proxy** -- caches and filters marketplace search results
- **Policy engine** -- control which publishers, artifact types, and plugins are allowed
- **Audit logging** -- structured JSON logs of all pull operations for SIEM integration
- **Pre-population sync** -- scheduled syncing of approved artifacts
- **Air-gapped export/import** -- bundle artifacts for transfer to disconnected networks

## Starting the Mirror

```sh
zhi-mirror serve --listen :5050 --upstream-registry ghcr.io
```

Clients point to the mirror by setting `sharing.defaultRegistry` in `~/.zhi/config.yaml` or via the `ZHI_REGISTRY` environment variable:

```yaml
# ~/.zhi/config.yaml
sharing:
  defaultRegistry: mirror.company.internal:5050
```

```sh
export ZHI_REGISTRY=mirror.company.internal:5050
```

## Policy Engine

The mirror includes a policy engine that controls which artifacts are available to clients:

```yaml
# mirror-policy.yaml
publishers:
  allowed:
    - zhi-project
    - company-internal
  blocked:
    - untrusted-publisher
artifacts:
  blocked:
    - "*/zhi-*-deprecated:*"
plugins:
  requireVerified: true
```

Policies can restrict by publisher identity, artifact name patterns, and verification status.

## Audit Logging

All pull operations are logged as structured JSON for SIEM integration:

```json
{
  "event": "artifact_pull",
  "timestamp": "2026-01-30T10:15:30Z",
  "artifact": "ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
  "client_ip": "10.0.1.42",
  "user": "deploy-pipeline",
  "digest": "sha256:abc123...",
  "cached": true
}
```

## Pre-Population Sync

Schedule background syncing to keep approved artifacts warm in the cache:

```sh
zhi-mirror sync --config sync.yaml
```

```yaml
# sync.yaml
schedule: "0 2 * * *"    # daily at 2 AM
artifacts:
  - ghcr.io/zhi-project/zhi-config-ansible:v1.*
  - ghcr.io/zhi-project/zhi-store-vault:v2.*
```

## Air-Gapped Export/Import

Bundle artifacts for physical transfer to disconnected networks:

```sh
# Export artifacts to a portable bundle
zhi-mirror export \
  --artifacts ghcr.io/zhi-project/zhi-config-ansible:v1.2.0 \
  --output bundle.tar

# Import into an air-gapped mirror
zhi-mirror import --input bundle.tar
```

## See Also

- [Sharing and Registries](sharing-and-registries.md) -- plugin sharing overview
- [Plugin Discovery](plugin-discovery.md) -- filesystem-based plugin discovery
