# Phase 8: Updates — Automatic Update System

**Goal**: Plugins stay current with minimal user effort.

**Prerequisites**: [Phase 1 (Foundation)](phase-1-foundation.md) — plugin install/metadata must exist. [Phase 4 (Discovery)](phase-4-discovery.md) — marketplace API needed for version listing and advisory data. [Phase 5 (Community)](phase-5-community.md) recommended for advisory integration.

## Overview

This phase closes the plugin lifecycle loop: after install and use, plugins need to be kept up to date. The system supports manual, prompted, and automatic update strategies, with rollback for when updates cause problems. Version constraints and lock file integration ensure teams can control exactly which versions are in use.

The full update mechanism design is in the [Update Mechanism](../update-mechanism.md) plan, including version detection, lock file workflows, update strategies, rollback, and deprecation handling.

## Deliverables

### 1. Update Detection

**Version check logic**:
1. Gather installed plugins from `~/.zhi/metadata/*.json` (established in [Phase 1](phase-1-foundation.md))
2. For each plugin (in parallel): query marketplace API for available versions
3. Compare installed version against latest available using semver
4. Filter by platform availability
5. Cross-reference with security advisories (from [Phase 5](phase-5-community.md))

**Check triggers** (configurable in `~/.zhi/config.yaml`):
| Trigger | Config Key |
|---|---|
| Explicit: `zhi plugin update --check` | Always runs |
| On startup: `zhi run` | `sharing.updates.autoCheck` |
| UI startup | `sharing.updates.autoCheck` |
| Scheduled | `sharing.updates.checkInterval` (default: 24h) |

**Result caching**:
- Update check results cached in `~/.zhi/cache/update-check.json`
- Cache expires based on `checkInterval`
- Avoids hitting the marketplace API on every `zhi run`

### 2. Update CLI Commands

**`zhi plugin update --check`**: Check for available updates without installing:
```
PLUGIN              INSTALLED   AVAILABLE   SEVERITY
ansible-config      1.1.0       1.2.0       minor
old-store           1.4.0       1.5.0       security ⚠
vault-store         2.0.0       (up to date)
```

**`zhi plugin update <name>`**: Update a specific plugin to latest (or constrained) version.

**`zhi plugin update --all`**: Update all plugins with available updates.

**`zhi plugin update --check --changelog`**: Show what changed between versions (changelog from marketplace metadata).

**`zhi plugin pin <name>`**: Pin plugin at current version, skip during `--all` updates.

**`zhi plugin unpin <name>`**: Remove version pin.

**`zhi plugin rollback <name>`**: Restore the previous version from backup.

Full command specifications with flags and output examples in [CLI Integration](../cli-integration.md).

### 3. Auto-Update (Opt-In)

Configurable automatic updates for hands-off environments:

```yaml
# ~/.zhi/config.yaml
sharing:
  updates:
    autoCheck: true
    autoInstall: true
    autoInstallScope: patch   # patch | minor | all
```

Scope controls:
| Scope | Auto-updates | Prompts |
|---|---|---|
| `patch` | `1.2.0 → 1.2.1` | `1.2.0 → 1.3.0`, `1.x → 2.x` |
| `minor` | `1.2.0 → 1.3.0` | `1.x → 2.0` |
| `all` | Everything (not recommended) | Nothing |

Auto-updates run at the start of `zhi run`, before the engine initializes plugins.

### 4. Rollback

When an update causes problems, rollback restores the previous version:

**Backup mechanism**: When a plugin is updated, the previous binary is moved to `~/.zhi/plugins/.backup/`:
```
~/.zhi/plugins/
  zhi-config-ansible               # current (1.2.0)
~/.zhi/plugins/.backup/
  zhi-config-ansible.1.1.0         # previous version
```

Only the most recent previous version is kept. Older versions can be reinstalled from the registry.

**`zhi plugin rollback <name>`**:
1. Move current binary to a temp location
2. Restore backup to the plugin path
3. Update metadata to reflect the rolled-back version
4. If no backup exists: suggest `zhi plugin install <name>@<version> --force`

### 5. Version Constraints

Support semver constraints in workspace plugin declarations and update policies (see [Update Mechanism](../update-mechanism.md)):

| Constraint | Meaning |
|---|---|
| `1.2.0` | Exact version |
| `>=1.2.0` | Minimum version |
| `~1.2.0` | Patch updates only (1.2.x) |
| `^1.2.0` | Minor updates (1.x.y) |
| `>=1.0,<2.0` | Range |

Used in `zhi.yaml`:
```yaml
sharing:
  plugins:
    - name: ansible-config
      ref: oci://ghcr.io/zhi-project/zhi-config-ansible
      version: "^1.2.0"
```

`zhi workspace lock --update` resolves the latest version matching the constraint.

### 6. Lock File Update Commands

Extending the lock file from [Phase 2](phase-2-publishing.md):

- `zhi workspace lock --update` — Update all plugins to latest matching their constraints, refresh digests
- `zhi workspace lock --update-plugin <name>` — Update a single plugin in the lock file
- Integration with `zhi plugin update`: after updating a plugin, offer to update the lock file if one exists

### 7. Deprecation Handling

Display deprecation warnings for end-of-life plugins:

```
⚠ Plugin "old-config" has been deprecated.
  Recommended replacement: "new-config" by zhi-project
  Migration guide: https://github.com/zhi-project/old-config/MIGRATION.md
```

Deprecation data comes from the marketplace API (publishers mark plugins as deprecated with a replacement reference).

## Key Files to Modify

| File | Change |
|---|---|
| `pkg/sharing/client/` | Add `CheckUpdates()` method |
| `pkg/sharing/metadata/` | Add backup/rollback logic, version pinning |
| `internal/cli/` | Add update, pin, unpin, rollback commands |
| `internal/core/engine.go` | Optional: auto-update check at startup |

## New Files

```
internal/cli/plugin_update.go    # zhi plugin update command
internal/cli/plugin_pin.go       # zhi plugin pin/unpin commands
internal/cli/plugin_rollback.go  # zhi plugin rollback command
pkg/sharing/semver/              # Semver constraint parsing and matching
pkg/sharing/update/              # Update detection, caching, auto-update logic
```

## Exit Criteria

- `zhi plugin update --check` shows available updates with severity indicators
- `zhi plugin update <name>` and `zhi plugin update --all` install updates
- `zhi plugin rollback <name>` restores the previous version
- `zhi plugin pin/unpin` controls which plugins are eligible for updates
- Auto-update works for patch versions when opted in (`autoInstall: true`, `autoInstallScope: patch`)
- Version constraints in `zhi.yaml` are respected by `zhi workspace lock --update`
- Changelog display shows what changed between versions
- Deprecated plugins show warnings with replacement suggestions

## Design References

- [Update Mechanism](../update-mechanism.md) — Full update detection flow, strategies, lock file format, rollback, version constraints, deprecation
- [CLI Integration](../cli-integration.md) — Update, pin, rollback command specifications
- [Ratings & Verification](../ratings-and-verification.md) — Vulnerability advisories that feed into update severity
- [Security & Trust](../security.md) — Advisory system integration
