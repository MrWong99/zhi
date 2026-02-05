# Phase 1: Foundation — Plugin Pull & Install

**Goal**: Users can install plugins from OCI registries via CLI.

**Prerequisites**: None — this is the starting phase.

## Overview

This phase establishes the core distribution infrastructure: an OCI client library, CLI commands for installing and managing plugins, and a local metadata store. It builds directly on the existing plugin discovery mechanism in `internal/core/discovery.go` — installed plugins land in `~/.zhi/plugins/` with the standard `zhi-<type>-<name>` naming convention and are picked up automatically by `RefreshExternal()`.

The OCI artifact format and media types defined in the [Distribution Format](../distribution-format.md) plan are implemented here. The local directory layout described in the [Architecture Overview](../architecture.md) is established.

## Deliverables

### 1. Plugin Manifest Format (`zhi-plugin.yaml`)

Define the schema for plugin metadata as specified in the [Architecture Overview](../architecture.md) under "Core Components > `pkg/sharing/manifest`":

- `PluginManifest` struct: name, type, version, description, author, license, runtime requirements, keywords
- YAML parsing and validation
- Schema versioning for forward compatibility
- Validation rules: name matches `[a-z][a-z0-9-]*`, type is one of the four plugin types, version is valid semver

### 2. OCI Client Library (`pkg/sharing/client/`)

Integrate oras-go v2 for push/pull operations as recommended in the [Distribution Format](../distribution-format.md) plan:

- **Pull**: Fetch OCI Image Index, select platform-specific manifest, download config blob and binary layer
- **Multi-platform resolution**: Automatic selection of correct OS/arch from OCI Image Index (fat manifest)
- **Authentication**: Basic, Bearer, and credential helper support (Docker-compatible)
- **Local blob cache**: Content-addressed cache in `~/.zhi/cache/oci/` to avoid re-downloading unchanged layers
- **Media type handling**: Parse `application/vnd.zhi.plugin.config.v1+json` config blobs and `application/vnd.zhi.plugin.binary.v1` layers
- **Runtime layer extraction**: If a plugin includes a `application/vnd.zhi.plugin.runtime.v1+tar.gz` layer, extract it to `~/.zhi/runtimes/`

### 3. CLI Commands (`internal/cli/`)

As specified in the [CLI Integration](../cli-integration.md) plan:

- **`zhi plugin install <oci-ref>`** — Download and install a plugin from an OCI reference
  - Support full OCI references (`oci://ghcr.io/org/plugin:v1.0`)
  - Display progress, verification status, and usage instructions
- **`zhi plugin uninstall <name>`** — Remove an installed plugin binary and metadata
- **`zhi plugin list`** — Extend the existing `list providers` command to show installed version and source for shared plugins
- **`zhi registry login <host>`** — Authenticate with an OCI registry (credentials stored in `~/.zhi/config.yaml`)
- **`zhi registry logout <host>`** — Remove stored credentials

### 4. Local Metadata Store (`~/.zhi/metadata/`)

Track installed plugins for version comparison and update checks in later phases:

```json
// ~/.zhi/metadata/ansible-config.json
{
  "name": "ansible-config",
  "type": "config",
  "version": "1.2.0",
  "ref": "oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
  "digest": "sha256:abc123...",
  "platform": "linux/amd64",
  "installedAt": "2026-01-15T10:30:00Z",
  "publisher": "zhi-project"
}
```

### 5. Integration with Existing Discovery

No changes to `internal/core/discovery.go` are needed — installed plugins use the existing discovery mechanism:

- Binaries installed to `~/.zhi/plugins/zhi-<type>-<name>` (flat naming convention)
- Executable permissions set to `0755`
- `RefreshExternal()` in `internal/core/registry.go` picks them up at startup

Optional enhancement: extend `ProviderInfo` to include version and OCI reference from the metadata store.

## Key Files to Modify

| File | Change |
|---|---|
| `internal/core/discovery.go` | No changes needed (installed plugins use existing discovery) |
| `internal/core/registry.go` | Optional: add metadata awareness for `ListConfigProviders()` etc. |
| `internal/cli/root.go` | Register new `plugin` and `registry` command groups |
| `internal/cli/list.go` | Extend provider listing with version/update columns |

## New Packages

```
pkg/sharing/
├── client/        # OCI artifact operations (oras-go wrapper)
├── manifest/      # Plugin/workspace manifest types and parsing
├── registry/      # Registry configuration and authentication
└── metadata/      # Local installed-plugin metadata store
```

## New Dependencies

```
oras.land/oras-go/v2    # OCI artifact push/pull (CNCF project)
```

## Exit Criteria

- A user can run `zhi plugin install oci://ghcr.io/example/plugin:v1.0` and use the plugin in their workspace
- Multi-platform support works (correct binary selected for OS/arch)
- Authentication works with GHCR, Docker Hub, and Harbor
- `zhi plugin list` shows installed shared plugins with version information
- Local blob cache avoids redundant downloads on reinstall

## Design References

- [Distribution Format](../distribution-format.md) — OCI artifact structure, media types, addressing format
- [Architecture Overview](../architecture.md) — System components, Go package design, directory layout
- [CLI Integration](../cli-integration.md) — Full command specifications with flags and example output
