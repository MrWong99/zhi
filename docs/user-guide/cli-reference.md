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
zhi init --config-provider structuredfile --store-provider zhi-store-json
zhi init --force  # overwrite existing zhi.yaml
```

Creates `zhi.yaml`, starter configuration files, sample export templates, and the `.zhi/` directory.

| Flag | Default | Description |
|------|---------|-------------|
| `--config-provider` | `structuredfile` | Config provider to use |
| `--store-provider` | `zhi-store-json` | Store provider to use |
| `--force` | `false` | Overwrite existing `zhi.yaml` |

### `zhi edit`

Launch the interactive UI to browse, edit, and manage configuration.

```sh
zhi edit
zhi edit --tree production
zhi edit --ui tui
```

| Flag | Default | Description |
|------|---------|-------------|
| `--tree` | (default tree) | Tree ID to load from store |
| `--ui` | `tui` | UI driver to use |

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

### `zhi version`

Print version, commit hash, and build date.

```sh
$ zhi version
zhi v0.1.0 (commit: abc1234, built: 2026-01-29)
```
