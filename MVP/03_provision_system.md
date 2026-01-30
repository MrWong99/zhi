# Step 3: Export/Provision System

## Overview

Build the template-based export system that renders configuration trees into standard file formats. This is zhi's provisioning mechanism: rather than embedding Docker/K8s SDKs, zhi exports configuration to files that external tools consume. The export system is **component-aware**: by default, only paths belonging to enabled components (and unmanaged paths) are included in exports. Templates can also query component state directly for conditional rendering.

## Relevant Existing Files

- `internal/core/engine.go` — core engine from Step 1 (will be extended)
- `internal/core/workspace.go` — workspace config, already defines `export.templates` section
- `internal/cli/root.go` — CLI root from Step 2
- `pkg/zhiplugin/config/config.go` — `Tree`, `TreeReader` interfaces (export reads from these)
- `go.mod` — dependencies (may need `github.com/BurntSushi/toml` for TOML output, `github.com/pelletier/go-toml/v2`, or similar)

## Implementation Plan

### 3.1 Export Engine (`internal/core/export.go`)

The export engine renders a Go template file with the configuration tree as data. It supports multiple output formats through template helper functions.

**Components:**

- `ExportConfig` struct — holds template path, output path, and format options (from workspace config)
- `ExportResult` struct — rendered content, output path, any warnings
- `Export(ctx, tree TreeReader, components *ComponentManager, config ExportConfig) (*ExportResult, error)` — main export function. The tree is filtered through the component manager before rendering: only paths belonging to enabled components (and unmanaged paths) are available to the template.
- `ExportAll(ctx, tree TreeReader, components *ComponentManager, configs []ExportConfig) ([]*ExportResult, error)` — export all templates defined in workspace

**Template execution:**

1. Read the template file from disk
2. Filter the tree through `ComponentManager.FilterTree()` to include only enabled components' paths
3. Create a `text/template.Template` with custom function map (including component functions)
4. Execute the template with a `TreeData` wrapper around `TreeReader` and `ComponentManager`
5. Write output to the configured path (or stdout if `-` is specified)

### 3.2 Template Data (`internal/core/export_data.go`)

A wrapper around `TreeReader` and `ComponentManager` that provides template-friendly access methods.

**Tree access functions available in `.tmpl` files:**

- `.Get "path"` — get a value as string (returns empty string if missing or if path belongs to a disabled component)
- `.GetOr "path" "default"` — get a value with a default fallback
- `.Has "path"` — returns bool indicating whether the path exists and belongs to an enabled component
- `.All` — returns all key-value pairs as a `map[string]any` (filtered to enabled components)
- `.Prefix "prefix"` — returns all key-value pairs under a path prefix as a flat map (filtered)
- `.Nested "prefix"` — returns all key-value pairs under a prefix as a nested map (splitting on `/`) (filtered)
- `.Meta "path" "key"` — get a metadata value for a path

**Component access functions:**

- `.ComponentEnabled "name"` — returns `bool` indicating whether the named component is enabled
- `.EnabledComponents` — returns `[]string` of all enabled component names
- `.DisabledComponents` — returns `[]string` of all disabled component names
- `.ComponentPaths "name"` — returns `[]string` of path prefixes belonging to the named component

These functions allow templates to conditionally render sections:

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
{{ if .ComponentEnabled "monitoring" }}
  prometheus:
    image: prom/prometheus
    environment:
      SCRAPE_INTERVAL: {{ .Get "monitoring/scrape-interval" }}
{{ end }}
```

**Built-in format helper functions (registered in the template FuncMap):**

- `toJSON` — marshal a value to JSON (indented)
- `toJSONCompact` — marshal to compact JSON
- `toYAML` — marshal to YAML
- `toTOML` — marshal to TOML
- `toDotenv` — render a flat map as `KEY=value` lines (keys uppercased, `/` replaced with `_`)
- `quote` — shell-safe quoting
- `indent` — indent a multi-line string by N spaces
- `upper`, `lower`, `replace` — string manipulation helpers

### 3.3 Built-in Format Shortcuts (`internal/core/export_formats.go`)

For common cases where users just want "export this tree as JSON", provide built-in templates that don't require a `.tmpl` file:

- `ExportAsJSON(ctx, tree, components, output) error` — render enabled components' config as JSON
- `ExportAsYAML(ctx, tree, components, output) error` — render enabled components' config as YAML
- `ExportAsTOML(ctx, tree, components, output) error` — render enabled components' config as TOML
- `ExportAsDotenv(ctx, tree, components, output) error` — render enabled components' config as dotenv

These are implemented as hardcoded templates that use the `.All` or `.Nested` data functions. Since `.All` already filters to enabled components, these shortcuts automatically respect component state.

### 3.4 CLI: `zhi export` Command (`internal/cli/export.go`)

Expose the export system via CLI.

**Usage:**

```
zhi export [--template <path>] [--format <json|yaml|toml|dotenv>] [--output <path>]
```

**Behavior:**

1. If `--template` is specified: use that template file
2. If `--format` is specified (without template): use the built-in format shortcut
3. If neither: export all templates defined in `zhi.yaml`
4. If `--output` is `-` or not specified with a single template: write to stdout
5. Print summary of exported files

**Flags:**

- `--template` — path to a Go template file
- `--format` — built-in format (`json`, `yaml`, `toml`, `dotenv`)
- `--output` / `-o` — output file path (default: stdout for single, workspace-defined for batch)
- `--prefix` — only export paths under this prefix
- `--all-components` — include all components regardless of enabled/disabled state (bypass component filtering)
- `--dry-run` — print rendered output without writing to disk

### 3.5 Workspace Export Configuration

The workspace `zhi.yaml` already defines an export section. Expand the schema:

```yaml
export:
  templates:
    - name: docker-compose
      template: ./templates/docker-compose.yml.tmpl
      output: ./docker-compose.override.yml
    - name: env-file
      format: dotenv          # use built-in format instead of template
      output: ./.env
      prefix: app/env         # only export paths under this prefix
```

### 3.6 Sample Templates

Create sample templates in the init scaffolding (Step 2's `zhi init` should create these):

**`templates/config.json.tmpl`:**
```
{{ .All | toJSON }}
```

**`templates/dotenv.tmpl`:**
```
# Generated by zhi - do not edit manually
{{ .All | toDotenv }}
```

### 3.7 Tests

- `internal/core/export_test.go` — test template rendering with mock trees:
  - `.Get` returns correct values
  - `.GetOr` falls back to default
  - `.Has` returns correct booleans
  - `.Prefix` filters correctly
  - `.Nested` builds nested maps
  - `toJSON`, `toYAML`, `toTOML`, `toDotenv` format correctly
  - `.ComponentEnabled` returns correct state
  - `.EnabledComponents` and `.DisabledComponents` return correct lists
  - `.Get` returns empty string for paths belonging to disabled components
  - `.All` excludes disabled components' paths
  - `.Has` returns false for disabled components' paths
- `internal/core/export_formats_test.go` — test built-in format shortcuts produce valid output, test that disabled components' paths are excluded
- `internal/cli/export_test.go` — test CLI flag combinations and output, test `--all-components` flag
- Use fixture data from `pkg/providers/config/structuredfile/testdata/` as input

## Verification Criteria

1. A `.tmpl` file can reference `.Get "database/host"` and render the correct value
2. `toJSON` produces valid, indented JSON from a tree
3. `toYAML` produces valid YAML from a tree
4. `toDotenv` produces `KEY=value` lines with uppercased, underscore-delimited keys
5. `zhi export --format json` outputs the tree as JSON to stdout (only enabled components)
6. `zhi export --template ./templates/my.tmpl --output ./out.txt` writes rendered output to file
7. `zhi export` (no flags) exports all templates defined in `zhi.yaml`
8. `--dry-run` prints output without writing files
9. Missing paths in templates produce empty strings (not errors) by default
10. Paths belonging to disabled components are excluded from `.All`, `.Prefix`, `.Nested`, and `.Has`
11. `.ComponentEnabled "redis"` returns correct boolean based on component state
12. `{{ if .ComponentEnabled "redis" }}` conditional blocks render correctly
13. `--all-components` bypasses component filtering
14. All tests pass with `go test -race ./internal/core/... ./internal/cli/...`
