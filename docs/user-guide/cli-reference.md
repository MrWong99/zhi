# CLI Reference

zhi provides a subcommand-based CLI for all operations. Pass `--help` to any command for usage details.

## Global Flags

| Flag | Description |
|------|-------------|
| `--workspace` | Path to workspace directory (default: current directory, walks up to find `zhi.yaml`) |
| `--verbose` | Enable debug-level logging to stderr |
| `--version` | Print version information |

## Commands

### `zhi init`

Scaffold a new workspace in the current (or specified) directory.

```sh
zhi init
zhi init --config-provider structuredfile --store-provider jsonfile
zhi init --force  # overwrite existing zhi.yaml
```

Creates `zhi.yaml`, starter configuration files, sample export templates, and the `.zhi/` directory.

| Flag | Default | Description |
|------|---------|-------------|
| `--config-provider` | `structuredfile` | Config provider to use |
| `--store-provider` | `jsonfile` | Store provider to use (built-in: `jsonfile`; external plugins such as `zhi-store-json` work once installed) |
| `--force` | `false` | Overwrite existing `zhi.yaml` |

### `zhi edit`

![tui-gif](../assets/tui.gif)

Launch the interactive UI to browse, edit, and manage configuration.

```sh
zhi edit
zhi edit production        # edit a specific stored tree (positional [tree-id])
zhi edit --ui tui
zhi edit --ui webui
zhi edit --ui mcp-stdio
```

Accepts an optional positional `[tree-id]` argument to select which stored tree
to edit; when omitted, the default tree is used.

| Flag | Default | Description |
|------|---------|-------------|
| `--ui` | `tui` | UI driver to use (`tui`, `webui`, `mcp-stdio`, or an external plugin) |

**TUI key bindings:**

| Key | Action |
|-----|--------|
| `j` / `k` / arrows | Navigate |
| `Enter` | Edit selected value |
| `s` | Save tree to store |
| `v` | Validate configuration |
| `a` | Run apply command |
| `e` | Open export view |
| `c` | Open component view |
| `r` | Reload tree |
| `/` | Filter paths |
| `q` / `Ctrl+C` | Quit |
| `Esc` | Return to tree view |

### `zhi get`

Retrieve a single configuration value.

```sh
zhi get database/host
zhi get database/host --raw
zhi get database/host --json
```

| Flag | Description |
|------|-------------|
| `--raw` | Print just the value, no metadata |
| `--json` | Output as JSON including metadata |

### `zhi set`

Set a single configuration value.

```sh
zhi set database/host mydb.example.com
zhi set database/port 5432 --validate
```

| Flag | Description |
|------|-------------|
| `--validate` | Validate after setting |

### `zhi validate`

Validate the configuration tree and print results grouped by severity.

```sh
zhi validate
zhi validate --path database/port
zhi validate --json
```

Exit code 1 if any Blocking results, 0 otherwise.

| Flag | Description |
|------|-------------|
| `--path` | Validate only a specific path |
| `--json` | Output as JSON |

### `zhi doctor`

Run a battery of health checks: workspace integrity, plugin installation and compatibility, store connectivity (with Vault-specific probes), config validation, and optionally marketplace updates. See [Doctor](doctor.md) for detail.

```sh
zhi doctor
zhi doctor --check plugins --check store
zhi doctor --check updates
zhi doctor --json | jq .summary
```

Exit code 1 if any check reports an error, 0 otherwise. Warnings do not affect exit code.

| Flag | Description |
|------|-------------|
| `--check` | Run only the given categories (`workspace`, `plugins`, `store`, `config`, `updates`, or `all`). Repeatable. |
| `--json` | Machine-readable output |
| `--quiet` | Suppress OK and skipped rows |
| `--deep` | Enable slow checks (e.g., plugin binary digest verification) |
| `--timeout` | Per-check timeout (default 10s) |
| `--no-color` | Disable ANSI output (also honours `NO_COLOR`) |
| `--fix` | Reserved for future releases — no fixers registered in v1 |

**Example output:**

```
BLOCKING  database/port    Port must be between 1024 and 65535
WARNING   app/log-level    Consider using "info" in production
INFO      app/name         Value is using default

Validation: 1 blocking, 1 warning, 1 info
```

### `zhi list`

List information about the workspace.

```sh
zhi list providers    # Show built-in and external providers
zhi list trees        # Show stored tree IDs
zhi list paths        # Show all config paths
zhi list components   # Show components with status
```

| Flag | Description |
|------|-------------|
| `--json` | Machine-readable JSON output |

**Component list example:**

```
NAME          STATUS     MANDATORY  DEPENDENCIES  PATHS
database      enabled    yes        -             database/
redis         enabled    no         database      redis/
monitoring    disabled   no         -             monitoring/
```

### `zhi component`

Manage component state.

```sh
zhi component list              # List all components
zhi component enable redis      # Enable a component (and dependencies)
zhi component disable monitoring # Disable a component
zhi component info database     # Show detailed component info
```

| Flag | Description |
|------|-------------|
| `--json` | JSON output |
| `--force` | Cascading disable (also disable dependent components) |

See [Components](components.md) for details on how components work.

### `zhi drift`

Detect configuration drift by comparing exported files on disk against what the workspace would currently generate.

```sh
zhi drift                                          # Check all exports for drift
zhi drift --json                                   # JSON output for scripting/CI
zhi drift --watch --interval 5m                    # Watch mode (foreground)
zhi drift --watch --interval 5m --on-drift "cmd"   # Watch with notification hook
```

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Output as JSON |
| `--watch` | `false` | Run continuously, checking at `--interval` |
| `--interval` | `1m` | Check interval for watch mode (e.g. `30s`, `5m`) |
| `--on-drift` | | Shell command to run when drift is detected |

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | No drift (all files in sync) |
| `1` | Drift detected |
| `2` | Error during drift check |

**Example output:**

```
Drift Detection Report

DRIFTED (1):
  ansible/inventory/hosts.yml:
    --- ansible/inventory/hosts.yml (current)
    +++ ansible/inventory/hosts.yml (expected)
    @@ -10,3 +10,3 @@
    -upstream: 8.8.8.8
    +upstream: 1.1.1.1

IN SYNC (3):
  ansible/group_vars/webservers.yml
  ansible/group_vars/databases.yml
  config/app.toml

Run `zhi export` to reconcile, or `zhi export --diff` for full details.
```

**Watch mode** only outputs on state transitions (clean→drifted or drifted→clean). The first check treats the initial state as unknown, so if drift exists at startup the hook fires immediately. On graceful shutdown (SIGINT/SIGTERM) the command exits 0.

### `zhi export`

Export configuration to files using templates or built-in formats.

```sh
zhi export                                      # Export all templates in zhi.yaml
zhi export --format json                        # Built-in JSON format
zhi export --format yaml --output config.yml    # YAML to file
zhi export --template ./templates/my.tmpl       # Custom template
zhi export --format dotenv --all-components     # Include disabled components
zhi export --dry-run                            # Preview without writing
```

| Flag | Description |
|------|-------------|
| `--template` | Path to a Go template file |
| `--format` | Built-in format: `json`, `yaml`, `toml`, `dotenv` |
| `--output` / `-o` | Output file path (default: stdout) |
| `--prefix` | Only export paths under this prefix |
| `--all-components` | Include all components regardless of state |
| `--dry-run` | Print output without writing files |

See [Export and Templates](export-and-templates.md) for template syntax.

### `zhi apply`

Run the configured provisioning command.

```sh
zhi apply              # Run the default apply target
zhi apply destroy      # Run a named target
zhi apply --dry-run    # Show command without executing
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Print command without executing |
| `--no-export` | Skip the pre-export step |
| `--timeout` | Override configured timeout (seconds) |
| `--env KEY=VALUE` | Add/override environment variables (repeatable) |

See [Apply](apply.md) for details.

### `zhi plugin`

Manage shared plugins from OCI registries.

#### `zhi plugin install`

Download and install a plugin from an OCI registry.

```sh
zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
zhi plugin install ghcr.io/org/zhi-config-custom:v1.0.0 --force
zhi plugin install ghcr.io/org/zhi-config-custom:v1.0.0 --skip-verify
```

| Flag | Description |
|------|-------------|
| `--platform` | Override platform detection (e.g. `linux/amd64`) |
| `--force` | Overwrite existing plugin even if same version |
| `--skip-verify` | Skip signature verification (not recommended) |

#### `zhi plugin uninstall`

Remove an installed shared plugin binary and its metadata.

```sh
zhi plugin uninstall ansible-config
```

#### `zhi plugin list`

List all installed shared plugins with version, signing status, and source.

```sh
zhi plugin list
zhi plugin list --json
```

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

#### `zhi plugin info`

Show detailed information about a plugin including signer identity, binary digest, trust level, ratings, and download statistics. If the plugin is not installed locally, the marketplace is queried.

```sh
zhi plugin info ansible-config
zhi plugin info ansible-config --json
```

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

#### `zhi plugin rate`

Rate a plugin on the marketplace (1-5 stars with optional review comment). Requires a marketplace API key in `~/.zhi/config.yaml`.

```sh
zhi plugin rate zhi-project/ansible-config 5
zhi plugin rate zhi-project/ansible-config 4 --comment "Works well but missing docs"
```

| Flag | Description |
|------|-------------|
| `--comment` | Optional review text (max 2000 characters) |

#### `zhi plugin verify`

Verify a plugin artifact's signature without installing. Useful for auditing and compliance.

```sh
zhi plugin verify oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
```

#### `zhi plugin publish`

Build an OCI artifact from a `zhi-plugin.yaml` manifest and push to a registry.

```sh
zhi plugin publish --registry ghcr.io/myorg
zhi plugin publish --registry ghcr.io/myorg --sign
zhi plugin publish --registry ghcr.io/myorg --sign --key cosign.key
```

| Flag | Description |
|------|-------------|
| `--registry` | Target OCI registry (required) |
| `--tag` | OCI tag (default: `v{version}` from manifest) |
| `--sign` | Sign the artifact with cosign after pushing |
| `--key` | Path to cosign private key (default: keyless via Fulcio/OIDC) |

#### `zhi plugin new`

Scaffold a complete plugin project from a template. Generates a standalone repository with implementation stubs, tests, build automation, CI/CD workflows, and a sample workspace. See the [Scaffolding Guide](../plugin-development/scaffolding.md) for the full reference.

```sh
zhi plugin new --name my-config --type config
zhi plugin new --name my-store --type store --author myorg --license MIT
zhi plugin new --name my-transform --type transform --registry ghcr.io/myorg
```

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Plugin short name, must match `[a-z][a-z0-9-]*` |
| `--type` | (required) | Plugin type: config, transform, store, ui |
| `--lang` | `go` | Project language |
| `--module` | auto-derived | Go module path |
| `--registry` | | Target OCI registry for publishing |
| `--author` | | Author or organisation name |
| `--license` | | SPDX license identifier |
| `--description` | | Short description of the plugin |
| `--output-dir`, `-o` | `./zhi-<type>-<name>` | Output directory |

#### `zhi plugin init`

Generate a `zhi-plugin.yaml` manifest for an existing plugin project. For new projects, prefer [`zhi plugin new`](#zhi-plugin-new) which generates the full project structure.

```sh
zhi plugin init --name my-config --type config --version 1.0.0
```

| Flag | Description |
|------|-------------|
| `--name` | Plugin name (required) |
| `--type` | Plugin type: config, transform, store, ui (required) |
| `--version` | Initial version (required) |
| `--description` | Plugin description |
| `--author` | Author name |
| `--license` | SPDX license identifier |

#### `zhi plugin update`

Check for and install plugin updates from the marketplace.

```sh
zhi plugin update --check              # Check for available updates
zhi plugin update --check --changelog  # Show changelogs for available updates
zhi plugin update ansible-config       # Update a specific plugin
zhi plugin update --all                # Update all plugins
```

| Flag | Description |
|------|-------------|
| `--check` | Only check for updates, don't install |
| `--all` | Update all plugins with available updates |
| `--changelog` | Show changelogs for available updates |
| `--json` | Output as JSON |

Pinned plugins are skipped during `--all` updates but can still be updated explicitly by name.

#### `zhi plugin pin`

Pin a plugin at its current version to prevent it from being updated during `zhi plugin update --all`.

```sh
zhi plugin pin ansible-config
```

#### `zhi plugin unpin`

Remove the version pin from a plugin, making it eligible for updates again.

```sh
zhi plugin unpin ansible-config
```

#### `zhi plugin rollback`

Restore a plugin to the version that was installed before the most recent update. The previous binary is kept as a backup during each update.

```sh
zhi plugin rollback ansible-config
```

If no backup exists, use `zhi plugin install <name>@<version> --force` to install a specific version.

### `zhi plugin search`

Search the marketplace for plugins.

```sh
zhi plugin search ansible
zhi plugin search "vault store" --type store
zhi plugin search --type ui --sort rating --verified
```

| Flag | Description |
|------|-------------|
| `--type` | Filter by plugin type (config, transform, store, ui) |
| `--sort` | Sort by: relevance, downloads, rating, updated (default: relevance) |
| `--verified` | Show only verified plugins |
| `--json` | Output as JSON |
| `--limit` | Maximum results (default: 20) |

#### `zhi plugin register`

Register a published plugin with the central marketplace.

```sh
zhi plugin register ghcr.io/myorg/zhi-config-custom:v1.0.0
```

### `zhi workspace`

Manage shareable workspaces.

#### `zhi workspace install`

Install a workspace and its plugin dependencies from an OCI registry or marketplace.

```sh
zhi workspace install k8s-cluster
zhi workspace install oci://ghcr.io/org/zhi-workspace-k8s:v1.0 ./my-cluster
```

| Flag | Description |
|------|-------------|
| `--skip-plugins` | Don't install plugin dependencies |
| `--skip-tools-check` | Don't check for required external tools |

#### `zhi workspace publish`

Publish the current workspace as a shareable OCI artifact.

```sh
zhi workspace publish --registry ghcr.io/myorg
zhi workspace publish --registry ghcr.io/myorg --sign
```

| Flag | Description |
|------|-------------|
| `--registry` | Target OCI registry (required) |
| `--sign` | Sign the artifact with cosign |
| `--tag` | OCI tag (default: from workspace version) |

#### `zhi workspace lock`

Generate or update `zhi-plugins.lock` by pinning the currently-installed plugins to their exact OCI digests. This snapshots the plugins already present in the local metadata store; it does not resolve or update plugin references.

```sh
zhi workspace lock              # Pin currently-installed plugin digests
```

This command takes no flags.

#### `zhi workspace setup`

Install all plugins at the exact versions pinned in `zhi-plugins.lock`. Intended for CI/CD where reproducibility matters.

```sh
zhi workspace setup --from-lock
```

| Flag | Description |
|------|-------------|
| `--from-lock` | Install from `zhi-plugins.lock` (required) |

### `zhi registry`

Manage OCI registry authentication.

#### `zhi registry login`

Authenticate with an OCI registry.

```sh
zhi registry login ghcr.io --username myuser
echo $GHCR_TOKEN | zhi registry login ghcr.io --username myuser --password-stdin
```

| Flag | Description |
|------|-------------|
| `--username` | Registry username |
| `--password` | Registry password |
| `--password-stdin` | Read password from stdin |

#### `zhi registry logout`

Remove stored credentials for a registry.

```sh
zhi registry logout ghcr.io
```

#### `zhi registry list`

List configured registries and their authentication status.

```sh
zhi registry list
```

### `zhi vault-credentials`

Manage Vault credentials for applications deployed with the `zhi-store-vault-manager` meta-plugin. See [Vault Credential Management](vault-credentials.md) for the full guide.

#### `zhi vault-credentials refresh`

Generate fresh credentials (AppRole wrapped secret_ids or tokens) without a full export/apply cycle.

```sh
zhi vault-credentials refresh
zhi vault-credentials refresh --app web-api --output env
zhi vault-credentials refresh --output quiet
zhi vault-credentials refresh --export-template docker-compose
```

| Flag | Default | Description |
|------|---------|-------------|
| `--app` | (all apps) | Refresh only a specific app's credentials |
| `--output` | `json` | Output format: `json`, `env`, `quiet` |
| `--export-template` | | Re-run a specific export template with fresh credentials |

**Example JSON output:**

```
{
  "vault/credentials/web-api/auth-method": "approle",
  "vault/credentials/web-api/role-id": "abc-123",
  "vault/credentials/web-api/wrapped-secret-id": "hvs.CAES..."
}
```

**Example env output:**

```
VAULT_CREDENTIALS_WEB_API_AUTH_METHOD=approle
VAULT_CREDENTIALS_WEB_API_ROLE_ID=abc-123
VAULT_CREDENTIALS_WEB_API_WRAPPED_SECRET_ID=hvs.CAES...
```

#### `zhi vault-credentials bootstrap`

Generate the Vault ACL policy required by the vault-manager's admin token. Run once during initial setup.

```sh
zhi vault-credentials bootstrap --dry-run
zhi vault-credentials bootstrap
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Print the policy HCL without instructions for applying it |

The output includes the HCL policy and a ready-to-run `vault policy write` command.

### `zhi version`

Print version, commit hash, and build date.

```sh
$ zhi version
zhi v0.1.0 (commit: abc1234, built: 2026-01-29)
```
