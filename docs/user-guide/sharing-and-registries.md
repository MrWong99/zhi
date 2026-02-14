# Sharing and Registries

zhi plugins and workspaces can be shared via OCI-compatible container registries (GHCR, Docker Hub, Harbor, ECR, ACR, GAR, or any self-hosted OCI registry). This guide covers installing, publishing, searching, updating, and securing shared artifacts.

## Official Plugin Registry

The zhi project publishes example plugins to the GitHub Container Registry. These are built, signed, and released automatically on every tagged release:

| Plugin | OCI Reference |
|--------|---------------|
| zhi-config-pokedex | `oci://ghcr.io/mrwong99/zhi/zhi-config-pokedex` |
| zhi-transform-pokedex | `oci://ghcr.io/mrwong99/zhi/zhi-transform-pokedex` |
| zhi-store-json | `oci://ghcr.io/mrwong99/zhi/zhi-store-json` |
| zhi-store-memory | `oci://ghcr.io/mrwong99/zhi/zhi-store-memory` |
| zhi-store-vault | `oci://ghcr.io/mrwong99/zhi/zhi-store-vault` |
| zhi-ui-httpapi | `oci://ghcr.io/mrwong99/zhi/zhi-ui-httpapi` |
| zhi-ui-webui | `oci://ghcr.io/mrwong99/zhi/zhi-ui-webui` |

Install any of them with:

```sh
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-memory:v0.0.9
```

All official plugins are signed with keyless cosign via Sigstore. No registry authentication is required to pull public plugins from GHCR.

## Installing Plugins

Install a plugin from an OCI registry reference or a marketplace short name:

```sh
# From an OCI reference
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-config-pokedex:v0.0.9

# From a marketplace short name (resolves via marketplace API)
zhi plugin install ansible-config

# Specific version
zhi plugin install ansible-config@1.2.0
```

Installed plugins are placed in `~/.zhi/plugins/` and are immediately available for use in `zhi.yaml`.

| Flag | Description |
|------|-------------|
| `--platform` | Override platform detection (e.g. `linux/amd64`) |
| `--force` | Overwrite existing plugin even if same version |
| `--skip-verify` | Skip signature verification (not recommended) |

## Publishing Plugins

To publish a plugin, create a `zhi-plugin.yaml` manifest in your project root:

```yaml
schemaVersion: "1"
name: pokedex
type: config
version: 0.0.1
zhiProtocolVersion: "1"
description: The zhi example pokedex plugin
author: MrWong99
license: MIT
homepage: https://mrwong99.github.io/zhi/docs/plugin-development/overview.html
keywords:
  - example
  - config
  - pokedex

# Build artifacts (paths relative to project root)
binaries:
  linux/amd64: dist/zhi-config-pokedex
```

Generate a manifest interactively:

```sh
zhi plugin init --name my-config --type config --version 1.0.0
```

Build your binaries for each platform, then publish:

```sh
zhi plugin publish --registry ghcr.io/myorg
zhi plugin publish --registry ghcr.io/myorg --sign          # sign with cosign
zhi plugin publish --registry ghcr.io/myorg --sign --key cosign.key  # sign with key
```

| Flag | Description |
|------|-------------|
| `--registry` | Target OCI registry (required) |
| `--tag` | OCI tag (default: `v{version}` from manifest) |
| `--sign` | Sign the artifact with cosign after pushing |
| `--key` | Path to cosign private key (default: keyless via Fulcio/OIDC) |

## Searching the Marketplace

Search the marketplace for plugins by name, type, or keywords:

```sh
zhi plugin search ansible
zhi plugin search "vault store" --type store
zhi plugin search --type ui --sort rating --verified
```

| Flag | Description |
|------|-------------|
| `--type` | Filter by plugin type (config, transform, store, ui) |
| `--sort` | Sort by: relevance, downloads, rating, updated |
| `--verified` | Show only verified plugins |
| `--json` | Output as JSON |
| `--limit` | Maximum results (default: 20) |

Get detailed information about a plugin:

```sh
zhi plugin info ansible-config
```

This shows the publisher, version, rating, download count, signing status, and whether an update is available.

## Ratings

Rate plugins on the marketplace (requires an API key in `~/.zhi/config.yaml`):

```sh
zhi plugin rate zhi-project/ansible-config 5
zhi plugin rate zhi-project/ansible-config 4 --comment "Works well but missing docs"
```

## Updates

Check for and install plugin updates:

```sh
zhi plugin update --check              # Check for available updates
zhi plugin update --check --changelog  # Show changelogs for available updates
zhi plugin update ansible-config       # Update a specific plugin
zhi plugin update --all                # Update all plugins
```

### Version Pinning

Pin a plugin to prevent automatic updates:

```sh
zhi plugin pin ansible-config      # Pin at current version
zhi plugin unpin ansible-config    # Allow updates again
```

Pinned plugins are skipped during `zhi plugin update --all` but can still be updated explicitly by name.

### Rollback

Restore a plugin to its previous version (the pre-update binary is kept as a backup):

```sh
zhi plugin rollback ansible-config
```

If no backup exists, install a specific version with `zhi plugin install ansible-config@1.1.0 --force`.

## Signature Verification

Plugins are signed using [Sigstore](https://sigstore.dev/) (cosign) for artifact integrity and publisher identity:

```sh
# Verify a plugin's signature without installing
zhi plugin verify oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
```

### Verification Levels

| Level | Description |
|-------|-------------|
| None (`--skip-verify`) | No signature check. Binary digest still verified against OCI manifest. |
| Signed (default) | Artifact has a valid cosign signature from any identity. |
| Verified publisher | Signer identity matches the registered marketplace publisher. |
| Strict (`requireSignatures: true`) | Only signed artifacts from trusted publishers can be installed. |

### Binary Integrity

At launch time, zhi compares the plugin binary's SHA-256 digest against the digest recorded during installation. A mismatch indicates post-install tampering and is logged as an error.

### Security Policy

Configure `~/.zhi/policy.yaml` to enforce security requirements:

```yaml
requireSignatures: true
allowedRegistries:
  - ghcr.io
  - harbor.company.internal
trustedPublishers:
  - zhi-project
  - myorg
```

Trusted signing keys are managed in `~/.zhi/keys/`.

## Workspaces

Workspaces can be shared as OCI artifacts that bundle `zhi.yaml`, templates, apply scripts, and plugin dependency declarations.

### Installing a Workspace

```sh
zhi workspace install k8s-cluster
zhi workspace install oci://ghcr.io/org/zhi-workspace-k8s:v1.0 ./my-cluster
```

This pulls the workspace files, installs any declared plugin dependencies, and checks for required external tools.

| Flag | Description |
|------|-------------|
| `--skip-plugins` | Don't install plugin dependencies |
| `--skip-tools-check` | Don't check for required external tools |
| `--dry-run` | Show what would be installed |

### Publishing a Workspace

```sh
zhi workspace publish --registry ghcr.io/myorg
zhi workspace publish --registry ghcr.io/myorg --sign
```

### Lock Files

Generate a `zhi-plugins.lock` file that pins all plugin dependencies to exact OCI digests for reproducible builds:

```sh
zhi workspace lock              # Lock current versions
zhi workspace lock --update     # Update all plugins and re-lock
```

Lock files ensure CI/CD pipelines and team members install identical plugin versions.

## Registry Authentication

Authenticate with OCI registries:

```sh
zhi registry login ghcr.io --username myuser
echo $GHCR_TOKEN | zhi registry login ghcr.io --username myuser --password-stdin
zhi registry logout ghcr.io
zhi registry list               # List configured registries
```

Credentials are stored in `~/.zhi/config.yaml` or delegated to Docker credential helpers.

## Marketplace Server TLS

The `zhi-marketplace` server supports TLS and mutual TLS (mTLS) using the same flags as `zhi-mirror`:

```sh
zhi-marketplace --listen :8443 \
  --tls-cert /etc/zhi-marketplace/server.crt \
  --tls-key /etc/zhi-marketplace/server.key \
  --tls-min-version 1.3
```

For mTLS, add `--tls-client-ca /path/to/client-ca.crt`. See [Enterprise Mirror](enterprise-mirror.md#tls-configuration) for the full flag reference.

## Global Configuration

Configure sharing defaults in `~/.zhi/config.yaml`:

```yaml
default: ghcr.io
marketplace:
  url: https://marketplace.zhi.dev
  apiKey: zhk_abc123...

# use the zhi registry login command to create this section
registries:
  ghcr.io:
    username: myuser
    password: ghp_aRS...
```

## See Also

- [Plugin Discovery](plugin-discovery.md) -- filesystem-based plugin discovery
- [Enterprise Mirror](enterprise-mirror.md) -- local OCI mirror for air-gapped environments
- [CLI Reference](cli-reference.md) -- full command reference
- [Plugin Development Overview](../plugin-development/overview.md) -- building plugins
