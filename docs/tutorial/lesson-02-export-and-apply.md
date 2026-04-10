# Lesson 2: Export and Apply

This lesson covers the export system in more depth -- built-in formats, custom templates, workspace-level export configuration -- and then introduces the `zhi apply` command for running provisioning commands against your exported configuration.

## Prerequisites

- Completed [Lesson 1](lesson-01-getting-started.md)
- A workspace created with `zhi init`

## Exporting Configuration

### Built-in Formats

zhi can export the entire configuration tree (filtered to enabled components) in four built-in formats:

```sh
zhi export --format json
zhi export --format yaml
zhi export --format toml
zhi export --format dotenv
```

Write the output to a file instead of stdout:

```sh
zhi export --format json --output config.json
```

### Custom Templates

Templates use Go's `text/template` syntax with [Sprig](https://masterminds.github.io/sprig/) helper functions. The scaffolded workspace includes a template at `templates/docker-compose.yml.tmpl`. Here is a simplified example:

```yaml
# templates/docker-compose.yml.tmpl
services:
  web:
    image: {{ .Get "pokemon-api/image" }}
    ports:
      - "{{ .Get "pokedex-web/external_port" }}:80"
    networks:
      - {{ .Get "pokedex-web/network_name" }}
```

Export with a specific template:

```sh
zhi export --template ./templates/docker-compose.yml.tmpl
```

### Workspace Export Configuration

The `export` section in `zhi.yaml` defines one or more templates that `zhi export` (without arguments) renders all at once:

```yaml
export:
  templates:
    - name: docker-compose
      template: ./templates/docker-compose.yml.tmpl
      output: ./docker-compose.yml
    - name: env-file
      format: dotenv
      output: ./.env
      prefix: app/env
```

Running `zhi export` with no flags writes both files. Use `--dry-run` to preview without touching disk.

### Prefix Filtering

Export only values under a specific path prefix:

```sh
zhi export --format json --prefix pokedex-web
```

### Dry Run and Diff

Preview what would be written:

```sh
zhi export --dry-run
```

Compare what zhi would export against the files currently on disk:

```sh
zhi export --diff
```

## The Apply Command

After exporting configuration files, you typically want to execute a provisioning step -- starting Docker containers, running Ansible, deploying to Kubernetes, and so on. The `zhi apply` command handles this.

### How It Works

`zhi apply` runs the shell command defined in the `apply` section of `zhi.yaml`. Here is what the scaffolded workspace uses:

```yaml
apply:
  command: "docker compose up -d"
  workdir: "."
  pre-export: true
  env:
    COMPOSE_PROJECT_NAME: "pokedex_api"
  timeout: 300
```

When you run `zhi apply`:

1. If `pre-export: true` is set, zhi first runs all configured exports (same as `zhi export`)
2. The configured shell command is executed with the specified environment variables
3. stdout and stderr are streamed in real time

### Running Apply

```sh
# Run the default apply target
zhi apply

# Preview the command without executing
zhi apply --dry-run
```

The `--dry-run` flag shows what would happen:

```
Would run: docker compose up -d
Workdir: /path/to/workspace
Pre-export: enabled
Environment:
  COMPOSE_PROJECT_NAME=pokedex_api
```

### Skipping the Export Step

If you already exported and just want to re-run the command:

```sh
zhi apply --no-export
```

### Named Targets

Workspaces can define multiple apply targets for different operations:

```yaml
apply:
  targets:
    default:
      command: "docker compose up -d"
      pre-export: true
    destroy:
      command: "docker compose down -v"
    restart:
      command: "docker compose restart"
```

Run a specific target by name:

```sh
zhi apply           # runs "default"
zhi apply destroy   # runs "destroy"
zhi apply restart   # runs "restart"
```

### Additional Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Print the command without executing |
| `--no-export` | Skip the pre-export step |
| `--timeout N` | Override the configured timeout in seconds |
| `--env KEY=VALUE` | Add or override environment variables (repeatable) |

### Component Awareness

When `pre-export: true` is configured, the exported files are filtered to enabled components only. The subprocess environment also receives:

- `ZHI_WORKSPACE` -- absolute path to the workspace root
- `ZHI_ENABLED_COMPONENTS` -- comma-separated list of enabled component names
- `ZHI_DISABLED_COMPONENTS` -- comma-separated list of disabled component names

This lets your provisioning scripts react to which features are active.

## Drift Detection

After exporting and applying, you can monitor for configuration drift -- when someone or something modifies the exported files outside of zhi:

```sh
# One-shot check
zhi drift

# Continuous monitoring
zhi drift --watch --interval 5m

# Machine-readable output
zhi drift --json
```

The `zhi drift` command re-renders all workspace exports and compares them against files on disk. Exit code 0 means everything is in sync, 1 means drift was detected.

## Summary

In this lesson you learned how to:

- Export configuration in built-in formats (`json`, `yaml`, `toml`, `dotenv`)
- Use custom Go templates for rendering
- Configure workspace-level exports in `zhi.yaml`
- Run provisioning commands with `zhi apply`
- Use `--dry-run` to preview and `--no-export` to skip re-exporting
- Define multiple apply targets for different operations
- Detect drift with `zhi drift`

## Further Reading

- [Export and Templates](../user-guide/export-and-templates.md) -- full template function reference and file output controls
- [Apply](../user-guide/apply.md) -- apply system details, streaming output, cancellation
- [CLI Reference](../user-guide/cli-reference.md#zhi-export) -- complete flag reference for `zhi export`
- [CLI Reference](../user-guide/cli-reference.md#zhi-apply) -- complete flag reference for `zhi apply`
- [CLI Reference](../user-guide/cli-reference.md#zhi-drift) -- drift detection reference
