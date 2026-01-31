# Workspace Configuration

A zhi workspace is defined by a `zhi.yaml` file in your project root. This file declares which providers to use, how components are organized, and what exports and apply commands are available.

## File Format

```yaml
version: "1"

config:
  provider: structuredfile
  options:
    directory: ./config

transform: []
  # - provider: my-transform
  #   options: {}

store:
  provider: zhi-store-json
  options:
    directory: ./.zhi/store

components:
  - name: database
    description: "PostgreSQL database configuration"
    paths:
      - database/
    mandatory: true

  - name: redis
    description: "Redis cache layer"
    paths:
      - redis/
    dependencies:
      - database

  - name: monitoring
    description: "Prometheus and Grafana monitoring stack"
    paths:
      - monitoring/

export:
  templates:
    - name: docker-compose
      template: ./templates/docker-compose.yml.tmpl
      output: ./docker-compose.override.yml
    - name: env-file
      format: dotenv
      output: ./.env
      prefix: app/env

apply:
  command: "docker compose up -d"
  workdir: "."
  pre-export: true
  env:
    COMPOSE_PROJECT_NAME: "myapp"
  timeout: 300

plugins:
  directories:
    - ~/.zhi/plugins
    - ./plugins
```

## Sections

### `config`

Specifies the configuration provider that loads and manages config values.

| Field | Description |
|-------|-------------|
| `provider` | Provider name (e.g., `structuredfile`). Must match a built-in or external plugin. |
| `options` | Provider-specific options passed to the factory function. |

The built-in `structuredfile` provider loads JSON and YAML files from a directory. See [Structured File Provider](../plugin-development/structuredfile-provider.md) for details.

### `transform`

A list of transform providers applied in order. Each entry has the same `provider` and `options` fields as `config`. Transform plugins mutate the configuration tree before display or after save.

### `store`

Specifies the storage backend for persisting configuration trees, component state, and optionally versioning and encryption.

| Field | Description |
|-------|-------------|
| `provider` | Store provider name (e.g., `zhi-store-json`). |
| `options` | Provider-specific options. |

### `components`

Defines named groups of configuration paths. See [Components](components.md) for full details.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique identifier matching `[a-z][a-z0-9-]*` |
| `description` | no | Human-readable description |
| `paths` | yes | List of config path prefixes this component owns |
| `mandatory` | no | If `true`, the component cannot be disabled |
| `dependencies` | no | Names of components this one depends on |

### `export`

Configures export templates. See [Export and Templates](export-and-templates.md).

### `apply`

Configures the provisioning command. See [Apply](apply.md).

The `apply` section can also define multiple named targets:

```yaml
apply:
  default:
    command: "docker compose up -d"
    pre-export: true
  destroy:
    command: "docker compose down -v"
  restart:
    command: "docker compose restart"
```

### `plugins`

Optional section to configure where zhi looks for external plugin binaries. Default: `~/.zhi/plugins/`.

```yaml
plugins:
  directories:
    - ~/.zhi/plugins
    - /usr/local/lib/zhi/plugins
    - ./plugins
```

## Workspace Discovery

When you run any zhi command, it looks for `zhi.yaml` in the current directory, then walks up to parent directories until it finds one. You can override this with the `--workspace` flag:

```sh
zhi --workspace /path/to/project validate
```

## Verbose Logging

Pass `--verbose` to any command to enable debug-level logging to stderr:

```sh
zhi validate --verbose
```

Logs go to stderr so that commands like `zhi export --format json` can still pipe cleanly to other tools.
