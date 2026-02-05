# Phase 2: Publishing — Plugin Push & Workspace Sharing

**Goal**: Plugin authors can publish to OCI registries. Workspaces can be packaged and shared.

**Prerequisites**: [Phase 1 (Foundation)](phase-1-foundation.md) — OCI client library and manifest format must exist.

## Overview

This phase completes the two-way story: Phase 1 enabled consuming plugins, and Phase 2 enables producing them. It also introduces workspace artifacts — bundles of `zhi.yaml`, templates, and plugin dependency declarations — as a second artifact type alongside plugins. Lock files are added for reproducible installations in CI/CD.

The OCI artifact structures for both plugins and workspaces are defined in the [Distribution Format](../distribution-format.md) plan. The CLI commands are fully specified in the [CLI Integration](../cli-integration.md) plan.

## Deliverables

### 1. `zhi plugin init`

Interactive scaffolding for new plugin projects:

- Prompts for name, type, description, author, license
- Generates `zhi-plugin.yaml` manifest file
- Optionally generates a `binaries:` section based on detected build artifacts in `dist/`
- Validates the name against the `[a-z][a-z0-9-]*` pattern

See [CLI Integration](../cli-integration.md) for the `zhi-plugin.yaml` format and examples.

### 2. `zhi plugin publish`

Build an OCI artifact from a manifest and pre-built binaries, then push to a registry:

- Read `zhi-plugin.yaml` for metadata and binary paths
- Construct OCI manifest with zhi-specific media types (see [Distribution Format](../distribution-format.md))
- Pack binary layers for each platform listed in the manifest
- For multi-platform plugins: construct an OCI Image Index (fat manifest) referencing per-platform manifests
- Push to the specified OCI registry using the oras-go client from Phase 1
- Support `--tag` override (default: `v{version}` from manifest)
- Support `--sign` flag (deferred to [Phase 3](phase-3-security.md), but the flag is accepted and warns if signing is not yet configured)

### 3. Multi-Platform Image Index

Create OCI Image Index manifests that list platform-specific sub-manifests:

```
OCI Image Index
├── linux/amd64 → Manifest → binary layer
├── linux/arm64 → Manifest → binary layer
├── darwin/amd64 → Manifest → binary layer
├── darwin/arm64 → Manifest → binary layer
└── windows/amd64 → Manifest → binary layer
```

The client from Phase 1 already selects the correct platform at pull time. This deliverable ensures the publishing side creates the correct multi-platform structure.

### 4. Workspace Artifacts

Two new commands for workspace sharing:

**`zhi workspace publish`**:
- Package `zhi.yaml`, template files, and any other workspace files into an OCI artifact
- Create a `WorkspaceManifest` config blob with dependency declarations (see [Architecture Overview](../architecture.md))
- Dependencies reference plugins by OCI reference, enabling `zhi workspace install` to resolve them
- Push the workspace artifact to an OCI registry

**`zhi workspace install <ref> [target-dir]`**:
- Pull the workspace artifact
- Extract workspace files to the target directory
- Read dependency declarations from the config blob
- For each plugin dependency: check if already installed at a compatible version, install if not (using Phase 1's `PullPlugin`)
- Check external tool requirements (kubectl, helm, etc.) and warn about missing tools
- Full flow described in the [Architecture Overview](../architecture.md) under "Data Flow > Installing a Workspace"

### 5. Lock File

Reproducible plugin installations via `zhi-plugins.lock`:

**`zhi workspace lock`**:
- Read plugin declarations from `zhi.yaml`'s `sharing.plugins` section
- Resolve each reference to an exact OCI digest for the current platform
- Write `zhi-plugins.lock` with pinned digests, platform-specific binary hashes, and signing status

**`zhi workspace setup --from-lock`**:
- Read `zhi-plugins.lock`
- Install each plugin at the exact pinned digest
- Fail if any digest doesn't match (tamper detection)
- Designed for CI/CD where reproducibility matters

Lock file format specified in the [Update Mechanism](../update-mechanism.md) plan and the [Distribution Format](../distribution-format.md) plan.

## Key Files to Modify

| File | Change |
|---|---|
| `internal/cli/root.go` | Register `workspace` command group |
| `pkg/sharing/client/` | Add `PushPlugin()` and `PushWorkspace()` methods |
| `pkg/sharing/manifest/` | Add `WorkspaceManifest` type |

## New Files

```
internal/cli/plugin_init.go       # zhi plugin init command
internal/cli/plugin_publish.go    # zhi plugin publish command
internal/cli/workspace_install.go # zhi workspace install command
internal/cli/workspace_publish.go # zhi workspace publish command
internal/cli/workspace_lock.go    # zhi workspace lock command
pkg/sharing/lockfile/             # Lock file parsing and generation
```

## Exit Criteria

- A developer can publish a plugin with `zhi plugin publish --registry ghcr.io/myorg`
- Multi-platform Image Index is created correctly for plugins with multiple platform binaries
- A workspace with plugin dependencies can be installed in one command with `zhi workspace install`
- `zhi workspace lock` generates a lock file; `zhi workspace setup --from-lock` installs from it
- Lock file detects digest mismatches and fails on tampered artifacts

## Design References

- [Distribution Format](../distribution-format.md) — OCI artifact structure for plugins and workspaces, lock file format
- [Architecture Overview](../architecture.md) — Publishing and workspace install data flows
- [CLI Integration](../cli-integration.md) — `zhi plugin publish`, `zhi plugin init`, `zhi workspace install/publish/lock` commands
- [Update Mechanism](../update-mechanism.md) — Lock file workflow and format
