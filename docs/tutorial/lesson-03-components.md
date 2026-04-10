# Lesson 3: Components

Components are named groups of configuration values that can be toggled on or off. They let users selectively activate feature sets without deleting configuration. This lesson covers defining, managing, and using components.

## Prerequisites

- Completed [Lesson 1](lesson-01-getting-started.md) and [Lesson 2](lesson-02-export-and-apply.md)
- A workspace created with `zhi init`

## Understanding Components

The scaffolded workspace already defines three components in `zhi.yaml`:

```yaml
components:
  - name: pokedex-web
    description: "Pokedex web UI - your digital gateway to the Pokemon world"
    paths:
      - "pokedex-web/"
    mandatory: true

  - name: pokemon-api
    description: "Pokemon data API - the source of all Pokemon knowledge"
    paths:
      - "pokemon-api/"
    mandatory: true

  - name: trainer-info
    description: "Trainer details for a personalized Pokedex"
    paths:
      - "trainer-info/"
    mandatory: false
```

Each component has:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique identifier matching `[a-z][a-z0-9-]*` |
| `description` | no | Human-readable description |
| `paths` | yes | Config path prefixes this component owns |
| `mandatory` | no | If `true`, the component cannot be disabled |
| `dependencies` | no | Other components that must be enabled when this one is |

## Listing Components

```sh
zhi component list
```

Output:

```
NAME          STATUS    MANDATORY  DEPENDENCIES  PATHS
pokedex-web   enabled   yes        -             pokedex-web/
pokemon-api   enabled   yes        -             pokemon-api/
trainer-info  disabled  no         -             trainer-info/
```

The scaffolded workspace starts with `trainer-info` disabled. Before demonstrating the disable command, enable it first:

```sh
zhi component enable trainer-info
```

## Disabling a Component

Now disable the `trainer-info` component:

```sh
zhi component disable trainer-info
```

Now export and observe the difference:

```sh
zhi export --format json
```

The `trainer-info/` paths are excluded from the output. The configuration values still exist in the tree -- they are just filtered out of exports when the component is disabled.

## Enabling a Component

Re-enable it:

```sh
zhi component enable trainer-info
```

## Mandatory Components

Try disabling a mandatory component:

```sh
zhi component disable pokedex-web
```

This fails with an error:

```
Error: error: cannot disable "pokedex-web": component is mandatory
```

Even `--force` cannot override mandatory components.

## Dependencies

Components can declare dependencies on other components. When component A depends on B:

- Enabling A automatically enables B (and B's transitive dependencies)
- Disabling B fails if A is still enabled (unless you use `--force` to cascade)

Example configuration with dependencies:

```yaml
components:
  - name: database
    paths: ["database/"]
    mandatory: true

  - name: redis
    paths: ["redis/"]
    dependencies:
      - database

  - name: monitoring
    paths: ["monitoring/"]
```

Enabling `redis` would automatically enable `database`. Disabling `database` while `redis` is enabled would fail.

## Components in Templates

Export templates can query component state to conditionally include sections:

```yaml
# templates/docker-compose.yml.tmpl
services:
  db:
    environment:
      POSTGRES_HOST: {{ .Get "database/host" }}
{{ if .ComponentEnabled "redis" }}
  redis:
    image: redis:7
{{ end }}
```

Available template functions:

| Function | Description |
|----------|-------------|
| `.ComponentEnabled "name"` | Returns `true` if the component is enabled |
| `.EnabledComponents` | List of enabled component names |
| `.DisabledComponents` | List of disabled component names |
| `.ComponentPaths "name"` | Path prefixes for a component |

## Components in the TUI

Press `c` in the TUI (`zhi edit`) to open the component management view. You can toggle components with `Enter` or `Space`.

## Summary

In this lesson you learned how to:

- Define components with path prefixes, descriptions, mandatory flags, and dependencies
- List, enable, and disable components via the CLI
- Understand how mandatory components and dependencies work
- Use component state in export templates

## Further Reading

- [Components](../user-guide/components.md) -- full component reference including path ownership rules and validation
- [Export and Templates](../user-guide/export-and-templates.md) -- template functions for component-aware exports
- [CLI Reference](../user-guide/cli-reference.md#zhi-component) -- complete `zhi component` command reference
