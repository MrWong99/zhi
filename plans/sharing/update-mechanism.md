# Update Mechanism

How installed plugins and workspaces are kept up to date, from passive notifications to automatic updates.

## Update Detection

### Version Metadata Storage

When a plugin is installed, its metadata is stored locally:

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
  "publisher": "zhi-project",
  "verified": true,
  "autoUpdate": false,
  "pinned": false
}
```

### Update Check Flow

```
1. Gather installed plugins from ~/.zhi/metadata/*.json
2. For each plugin (in parallel):
   a. Query marketplace API: GET /api/v1/plugins/{publisher}/{name}/versions
   b. Compare installed version against latest available version
   c. Filter by platform availability
   d. Check for security advisories
3. Return list of available updates, sorted by priority:
   - Security fixes first
   - Major version updates (may have breaking changes)
   - Minor/patch updates
```

### When Updates Are Checked

| Trigger | Behavior | Configurable |
|---|---|---|
| `zhi plugin update --check` | Explicit check, always runs | N/A |
| `zhi run` startup | Check if `autoCheck: true` in config | `sharing.updates.autoCheck` |
| TUI/Web UI startup | Background check, notification shown | `sharing.updates.autoCheck` |
| Scheduled (cron-like) | Periodic background check | `sharing.updates.checkInterval` |

### Update Check Caching

To avoid hitting the marketplace API on every `zhi run`, update check results are cached:

```json
// ~/.zhi/cache/update-check.json
{
  "checkedAt": "2026-02-05T08:00:00Z",
  "updates": [
    {
      "name": "ansible-config",
      "installed": "1.1.0",
      "available": "1.2.0",
      "severity": "minor",
      "advisory": null
    }
  ]
}
```

The cache expires based on `sharing.updates.checkInterval` (default: 24h).

## Update Strategies

### Manual Update (Default)

The user explicitly decides when to update:

```bash
# Check what is available
zhi plugin update --check

# Update a specific plugin
zhi plugin update ansible-config

# Update all plugins
zhi plugin update --all
```

This is the safest approach — no surprises, full user control.

### Prompted Update

When `autoCheck` is enabled, zhi shows a non-blocking notification:

```
2 plugin updates available. Run 'zhi plugin update --check' for details.
```

In the TUI, this appears as a notification badge on the Installed tab.

### Automatic Update (Opt-In)

For environments that want hands-off updates:

```yaml
# ~/.zhi/config.yaml
sharing:
  updates:
    autoCheck: true
    autoInstall: true          # install updates automatically
    autoInstallScope: patch    # only auto-install patch updates
    # Options: patch, minor, all
    # "patch": 1.2.0 → 1.2.1 (auto), 1.2.0 → 1.3.0 (prompt)
    # "minor": 1.2.0 → 1.3.0 (auto), 1.2.0 → 2.0.0 (prompt)
    # "all": always auto-install (not recommended)
```

Automatic updates happen at the start of `zhi run`, before the engine initializes plugins.

### Pinned Versions

Plugins can be pinned to prevent any updates:

```bash
zhi plugin pin ansible-config        # Pin at current version
zhi plugin pin ansible-config@1.2.0  # Pin at specific version
zhi plugin unpin ansible-config      # Remove pin
```

Pinned plugins are skipped during `zhi plugin update --all`.

## Lock File Updates

The `zhi-plugins.lock` file ensures reproducible installations across machines and CI environments.

### Lock File Workflow

```bash
# Generate lock file from zhi.yaml plugin declarations
zhi workspace lock

# Update lock file (resolve latest versions, refresh digests)
zhi workspace lock --update

# Update a single plugin in the lock file
zhi workspace lock --update-plugin ansible-config

# Install exact versions from lock file (CI/CD use case)
zhi workspace setup --from-lock
```

### Lock File Format

```yaml
# zhi-plugins.lock
version: 1
generatedAt: "2026-02-05T10:00:00Z"
zhiVersion: "0.8.0"
plugins:
  - name: ansible-config
    type: config
    ref: oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
    digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    platforms:
      linux/amd64: sha256:abc123...   # per-platform binary digest
      darwin/arm64: sha256:def456...
    signed: true
    signingIdentity: release@zhi.dev
  - name: vault
    type: store
    ref: oci://ghcr.io/zhi-project/zhi-store-vault:v2.0.0
    digest: sha256:789xyz...
    platforms:
      linux/amd64: sha256:aaa111...
      darwin/arm64: sha256:bbb222...
    signed: true
    signingIdentity: release@zhi.dev
```

### Lock File in CI/CD

```yaml
# Example GitHub Actions workflow
- name: Setup zhi plugins
  run: |
    zhi workspace setup --from-lock
    # Installs exact plugin versions from zhi-plugins.lock
    # Fails if any digest doesn't match (tamper detection)
```

## Rollback

If an update causes problems, the user can roll back to the previous version:

```bash
# Roll back to previous version
zhi plugin rollback ansible-config

# Roll back to a specific version
zhi plugin install ansible-config@1.1.0 --force
```

### Rollback Storage

When a plugin is updated, the previous binary is moved to a backup directory:

```
~/.zhi/plugins/
  zhi-config-ansible           # current version (1.2.0)
~/.zhi/plugins/.backup/
  zhi-config-ansible.1.1.0     # previous version
```

Only the most recent previous version is kept. Older versions can be reinstalled from the registry.

## Version Constraints

Plugin dependencies (in workspace manifests) and update policies support semver constraints:

| Constraint | Meaning | Example Match |
|---|---|---|
| `1.2.0` | Exact version | 1.2.0 |
| `>=1.2.0` | Minimum version | 1.2.0, 1.3.0, 2.0.0 |
| `~1.2.0` | Patch updates only | 1.2.0, 1.2.1, 1.2.99 |
| `^1.2.0` | Minor updates | 1.2.0, 1.3.0, 1.99.0 |
| `>=1.0,<2.0` | Range | 1.0.0 through 1.99.99 |

Used in workspace declarations:

```yaml
sharing:
  plugins:
    - name: ansible-config
      type: config
      ref: oci://ghcr.io/zhi-project/zhi-config-ansible
      version: "^1.2.0"  # accept minor updates, lock in zhi-plugins.lock
```

## Changelog Display

When an update is available, the user can see what changed:

```bash
zhi plugin update --check --changelog

Updates available:
  ansible-config 1.1.0 → 1.2.0
    Changelog:
      - Added support for YAML inventory format
      - Fixed group variable inheritance (#42)
      - Improved error messages for missing inventory files
```

Changelogs are stored in the marketplace database, submitted by publishers during version registration.

## Deprecation and End-of-Life

Publishers can deprecate plugins:

```
⚠ Plugin "old-config" has been deprecated.
  Recommended replacement: "new-config" by zhi-project
  Migration guide: https://github.com/zhi-project/old-config/blob/main/MIGRATION.md
```

Deprecated plugins:
- Show a warning on install and during update checks
- Are not removed automatically
- Can specify a replacement plugin
- Are demoted in marketplace search results
