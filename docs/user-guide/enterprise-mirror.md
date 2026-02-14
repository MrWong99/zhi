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

## TLS Configuration

Enable TLS to encrypt traffic between clients and the mirror. The mirror supports server-side TLS and mutual TLS (mTLS) for client certificate authentication.

### Server TLS

```sh
zhi-mirror serve --listen :5050 \
  --tls-cert /etc/zhi-mirror/server.crt \
  --tls-key /etc/zhi-mirror/server.key
```

### Mutual TLS (mTLS)

Require clients to present a certificate signed by a trusted CA:

```sh
zhi-mirror serve --listen :5050 \
  --tls-cert /etc/zhi-mirror/server.crt \
  --tls-key /etc/zhi-mirror/server.key \
  --tls-client-ca /etc/zhi-mirror/client-ca.crt
```

### TLS Version and Cipher Suites

Control the minimum TLS version and allowed cipher suites:

```sh
zhi-mirror serve --listen :5050 \
  --tls-cert server.crt --tls-key server.key \
  --tls-min-version 1.3 \
  --tls-cipher-suites TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384
```

### TLS Flags Reference

| Flag | Description | Default |
|------|-------------|---------|
| `--tls-cert` | Path to TLS certificate PEM file | _(none)_ |
| `--tls-key` | Path to TLS private key PEM file | _(none)_ |
| `--tls-client-ca` | Path to CA PEM for client cert verification (mTLS) | _(none)_ |
| `--tls-min-version` | Minimum TLS version (`1.2` or `1.3`) | `1.2` |
| `--tls-cipher-suites` | Comma-separated list of allowed cipher suites | Go defaults |

TLS is enabled when both `--tls-cert` and `--tls-key` are provided. Cipher suite names match the Go standard library names (e.g. `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`).

## See Also

- [Sharing and Registries](sharing-and-registries.md) -- plugin sharing overview
- [Plugin Discovery](plugin-discovery.md) -- filesystem-based plugin discovery
