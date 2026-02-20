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

#### Store authentication from workspace config

You can configure store authentication credentials directly in the workspace options as a fallback mechanism. This avoids having to use the interactive `login` tool or set environment variables for secrets.

```yaml
store:
  provider: vault
  options:
    addr: "http://127.0.0.1:8200"
    mount: "kv"
    auth:
      method: token
      credentials:
        token: "hvs.xxxxx"
```

The `auth` section is consumed by the engine during initialization. If present, the engine attempts to auto-login after connecting to the store. The `method` must match one of the store's supported auth methods, and `credentials` is a map of key-value pairs specific to that method.

Supported auth method examples for the Vault store:

| Method | Credentials |
|--------|-------------|
| `token` | `token` |
| `userpass` | `username`, `password` |
| `approle` | `role_id`, `secret_id` |
| `ldap` | `username`, `password` |
| `kubernetes` | `role`, `jwt` |

If auto-login fails, a warning is logged and the session remains unauthenticated. Users can still log in interactively via the UI or MCP tools.

For workspaces that need automatic credential management for deployed applications, use the `vault-manager` store provider instead of `vault`. It wraps the standard vault store and adds AppRole/token generation driven by metadata labels. See [Vault Credential Management](vault-credentials.md) for configuration details.

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

### `ui`

Configures which UI provider `zhi edit` uses by default and passes provider-specific options.

| Field | Description |
|-------|-------------|
| `provider` | UI provider name (e.g., `tui`, `webui`, `mcp-stdio`). Overridden by `zhi edit --ui`. |
| `options` | Provider-specific options passed to the factory function. |

**Available built-in UI providers:**

| Provider | Description |
|----------|-------------|
| `tui` | Terminal UI using Bubbletea (default, requires TTY) |
| `webui` | Browser-based UI served on localhost. See [Web UI](web-ui.md). |
| `mcp-stdio` | MCP server over stdin/stdout for LLM clients (Claude Desktop, Claude Code, Cursor) |

Example for the MCP stdio provider:

```yaml
ui:
  provider: mcp-stdio
  options:
    read_only: false
```

The `mcp-stdio` provider redirects `os.Stdout` to stderr internally so that stray log output does not corrupt the MCP JSON-RPC stream. LLM clients launch `zhi edit --ui mcp-stdio` as a child process and communicate over stdin/stdout.

An external MCP SSE plugin (`zhi-ui-mcp-sse`) is also available for network-based access:

```yaml
ui:
  provider: mcp-sse
  options:
    addr: "0.0.0.0:9090"
    token: "my-secret-token"
```

| Option | Env Fallback | Default | Description |
|--------|-------------|---------|-------------|
| `addr` | `ZHI_MCP_ADDR` | `127.0.0.1:8091` | HTTP listen address |
| `token` | `ZHI_MCP_TOKEN` | (empty = no auth) | Bearer token for authentication |

Options from `zhi.yaml` take precedence over environment variables. See the [examples](../../examples/zhi-ui-mcp-sse/) directory.

**Plugin options for external plugins:** All provider options from `zhi.yaml` are propagated to external plugin processes via the `ZHI_PLUGIN_OPTIONS` environment variable (JSON-encoded). External plugins can read these using the `pluginopts` helper package from `pkg/zhiplugin/pluginopts/`.

### `plugins`

Optional section to configure where zhi looks for external plugin binaries. Default: `~/.zhi/plugins/`.

```yaml
plugins:
  directories:
    - ~/.zhi/plugins
    - /usr/local/lib/zhi/plugins
    - ./plugins
```

### `sharing` (global)

Sharing configuration is set in `~/.zhi/config.yaml` (not in the workspace `zhi.yaml`). It controls registry defaults, marketplace settings, verification policy, and update behavior:

```yaml
# ~/.zhi/config.yaml
sharing:
  defaultRegistry: ghcr.io
  marketplace:
    url: https://marketplace.zhi.dev
    apiKey: zhk_abc123...
  verification:
    requireSignatures: false
    trustedPublishers:
      - zhi-project
  updates:
    checkInterval: 24h
    autoCheck: true
    autoInstall: false
```

See [Sharing and Registries](sharing-and-registries.md) for details.

### Lock Files

Workspaces can include a `zhi-plugins.lock` file that pins plugin dependencies to exact OCI digests for reproducible installations. Generate or update it with:

```sh
zhi workspace lock
```

See [Sharing and Registries](sharing-and-registries.md#lock-files) for details.

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
