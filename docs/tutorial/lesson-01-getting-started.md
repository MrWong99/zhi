# Lesson 1: Getting Started with zhi

This lesson introduces the zhi CLI and walks through creating your first workspace, inspecting configuration, making changes, and validating the result.

## Prerequisites

- zhi installed (see [Getting Started](../user-guide/getting-started.md) for installation options)
- A terminal / command line

## Step 1: Initialize a Workspace

A zhi workspace is a directory containing a `zhi.yaml` file that declares providers, components, exports, and apply commands. Create one with `zhi init`:

```sh
mkdir my-first-workspace && cd my-first-workspace
zhi init
```

You should see output similar to:

```
Initialized zhi workspace:
  .zhi/store/
  app-data/pokedex-api/pokedex.json
  app-data/pokedex-web/html/index.html
  app-data/pokedex-web/nginx.conf
  .zhi/components.json
  config/app.yaml
  templates/docker-compose.yml.tmpl
  zhi.yaml

What's next?
  1. Generate the Docker Compose file:  zhi export
  2. Start the Pokedex stack:           docker-compose up
  3. View the Pokedex at:               http://localhost:8080
```

The scaffolded workspace uses a built-in Pokedex example with a handful of configuration values to explore.

## Step 2: List Configuration Paths

See every configuration path that the workspace manages:

```sh
zhi list paths
```

Output:

```
pokedex-web/external_port
pokedex-web/network_name
pokemon-api/image
trainer-info/hometown
trainer-info/name
```

Each path follows the `segment/segment` convention. Paths are flat keys; the `/` is purely a hierarchy hint.

## Step 3: Read a Value

Retrieve a single value by path:

```sh
zhi get trainer-info/name
```

To get just the raw value (no formatting), add `--raw`:

```sh
zhi get trainer-info/name --raw
```

For machine-readable output including metadata:

```sh
zhi get trainer-info/name --json
```

## Step 4: Set a Value

Change the trainer name:

```sh
zhi set trainer-info/name "Ash Ketchum"
```

Verify:

```sh
zhi get trainer-info/name --raw
```

You should see `Ash Ketchum`.

## Step 5: Validate the Configuration

Run validators against all configuration values:

```sh
zhi validate
```

If everything is valid, you see:

```
Validation: 0 blocking, 0 warning, 0 info
```

zhi supports three severity levels:

| Severity | Meaning |
|----------|---------|
| **Blocking** | Must be fixed before the configuration can be used |
| **Warning** | Potential issue, but does not prevent usage |
| **Info** | Purely informational |

## Step 6: Export Configuration

Render the configuration to a file using the workspace's configured template:

```sh
zhi export
```

This writes files according to the `export.templates` section in `zhi.yaml`. For the scaffolded workspace, it produces a `docker-compose.yml`.

You can also export to a built-in format on stdout:

```sh
zhi export --format json
zhi export --format yaml
```

Use `--dry-run` to preview without writing any files:

```sh
zhi export --dry-run
```

## Step 7: List Providers

See which providers are available (both built-in and external plugins):

```sh
zhi list providers
```

## Summary

In this lesson you learned how to:

- Initialize a workspace with `zhi init`
- List all configuration paths with `zhi list paths`
- Read values with `zhi get`
- Change values with `zhi set`
- Validate with `zhi validate`
- Export configuration with `zhi export`

## Further Reading

- [Getting Started](../user-guide/getting-started.md) -- installation and initial setup
- [Workspace Configuration](../user-guide/workspace-configuration.md) -- the `zhi.yaml` file format
- [CLI Reference](../user-guide/cli-reference.md) -- all available commands and flags
