# Components

Components are named groups of related configuration values that users can enable or disable. They let you selectively activate feature sets without deleting the underlying configuration.

## Defining Components

Components are defined in `zhi.yaml`:

```yaml
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
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique identifier matching `[a-z][a-z0-9-]*` |
| `description` | no | Human-readable description |
| `paths` | yes | Config path prefixes this component owns (e.g., `database/`) |
| `mandatory` | no | If `true`, the component cannot be disabled |
| `dependencies` | no | Other components that must be enabled when this one is |

## How Components Work

### Path Ownership

A configuration path belongs to a component if it starts with any of the component's path prefixes. For example, with `paths: [database/]`, all paths like `database/host`, `database/port`, and `database/credentials/user` belong to the `database` component.

Paths not claimed by any component are treated as "unmanaged" and are always included in exports.

Path prefixes cannot overlap across components -- this is validated when the workspace loads.

### Enabled/Disabled State

Each component can be toggled on or off. Disabled components' paths are:

- **Dimmed in the TUI** (still visible, but clearly inactive)
- **Excluded from exports** (unless `--all-components` is used)
- **Excluded from apply** pre-exports
- **Still present in the tree** for later re-enabling

Component state is persisted alongside the tree in the store plugin, or in `.zhi/components.json`.

### Dependencies

Components can declare dependencies. Rules:

- Enabling component A that depends on B **automatically enables B** (and B's transitive dependencies)
- Disabling component B when component A depends on it **fails** with a clear error listing the dependents
- Use `--force` to cascade-disable dependent components

### Mandatory Components

Components marked `mandatory: true` can never be disabled. Attempting to disable them results in an error, even with `--force`.

## Managing Components

### CLI

```sh
# List all components with status
zhi component list

# Enable a component (auto-enables dependencies)
zhi component enable redis
# Output: Enabled 'redis' (also enabled dependency: 'database')

# Disable a component
zhi component disable monitoring
# Output: Disabled 'monitoring'

# Detailed info about a component
zhi component info redis

# JSON output
zhi component list --json
```

**Error examples:**

```sh
$ zhi component disable database
Error: cannot disable 'database': component is mandatory

$ zhi component disable database --force
Error: cannot disable 'database': component is mandatory
```

### TUI

Press `c` in the TUI to open the component view:

```
[x] database       PostgreSQL database configuration     MANDATORY  paths: database/
[x] redis          Redis cache layer                     deps: database   paths: redis/
[ ] monitoring     Prometheus and Grafana monitoring                paths: monitoring/
```

Use `Enter` or `Space` to toggle a component. The tree view updates immediately to reflect which paths are active.

## Components and Exports

The export system is component-aware. By default, only paths belonging to enabled components (and unmanaged paths) are included in rendered output.

Templates can also query component state directly:

```yaml
# docker-compose.override.yml.tmpl
services:
  db:
    environment:
      POSTGRES_HOST: {{ .Get "database/host" }}
{{ if .ComponentEnabled "redis" }}
  redis:
    image: redis:7
    environment:
      REDIS_URL: {{ .Get "redis/url" }}
{{ end }}
```

See [Export and Templates](export-and-templates.md) for the full template function reference.

## Components and Apply

When `pre-export: true` is configured, the apply system generates files filtered to enabled components before running the provisioning command. The subprocess environment also includes:

- `ZHI_ENABLED_COMPONENTS` -- comma-separated list of enabled component names
- `ZHI_DISABLED_COMPONENTS` -- comma-separated list of disabled component names

## Validation

Component dependencies are checked during `zhi validate`. If enabled component A depends on disabled component B, a validation error is reported.

Additionally, circular dependencies are detected at workspace load time and rejected immediately.

## No Components Defined

If no components are defined in `zhi.yaml`, all paths are treated as unmanaged and are always included in exports. The component view in the TUI shows a message that no components are configured.
