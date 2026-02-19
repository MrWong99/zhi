# Plugin Scaffolding

The `zhi plugin new` command generates a complete, standalone plugin project from templates. The generated project includes implementation stubs, tests, build automation, CI/CD workflows, and a sample workspace -- everything needed to start developing and publishing a zhi plugin.

## Quick Start

```sh
zhi plugin new --name my-config --type config
cd zhi-config-my-config
# Update go.mod with the correct zhi version, then:
go mod tidy
make build
make test
```

The generated project is ready to be pushed as its own Git repository.

## Command Reference

```sh
zhi plugin new --name <name> --type <type> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Plugin short name, must match `[a-z][a-z0-9-]*` |
| `--type` | (required) | Plugin type: `config`, `transform`, `store`, `ui` |
| `--lang` | `go` | Project language (extensible; only Go is supported currently) |
| `--module` | auto-derived | Go module path (default: `github.com/<author>/zhi-<type>-<name>`) |
| `--registry` | | Target OCI registry for publishing (e.g. `ghcr.io/myorg`) |
| `--author` | | Author or organisation name |
| `--license` | | SPDX license identifier (e.g. `MIT`, `Apache-2.0`) |
| `--description` | | Short description of the plugin |
| `--output-dir`, `-o` | `./zhi-<type>-<name>` | Output directory |

### Examples

```sh
# Config plugin with full metadata
zhi plugin new --name my-config --type config --author myorg --license MIT

# Store plugin targeting a specific registry
zhi plugin new --name redis-store --type store \
  --registry ghcr.io/myorg --author myorg

# Custom module path and output directory
zhi plugin new --name my-transform --type transform \
  --module github.com/myorg/zhi-transform-my-transform \
  --output-dir ./plugins/my-transform

# UI plugin
zhi plugin new --name dashboard --type ui --description "Web dashboard UI"
```

## Generated Project Structure

For a config plugin named `my-config`:

```
zhi-config-my-config/
├── cmd/zhi-config-my-config/
│   └── main.go                    plugin entry point (handshake + serve)
├── pkg/plugin/
│   ├── plugin.go                  interface implementation
│   └── plugin_test.go             tests via in-process gRPC
├── workspace/
│   ├── zhi.yaml                   sample workspace using the plugin
│   └── config/
│       └── defaults.yaml          default configuration values
├── .github/workflows/
│   └── release.yml                build, sign, and publish on tag push
├── go.mod                         module definition with zhi dependency
├── Makefile                       build, test, lint, cross-compile
├── zhi-plugin.yaml                OCI distribution manifest
├── .gitignore
└── README.md
```

### Key files

**`cmd/zhi-<type>-<name>/main.go`** -- the plugin entry point. Sets up logging, registers the plugin implementation with the correct type key, and calls `goplugin.Serve` with the shared `zhiplugin.Handshake`.

**`pkg/plugin/plugin.go`** -- the plugin implementation. Contains a struct that implements the appropriate interface (`config.Plugin`, `store.Plugin`, `transform.Plugin`, or `ui.Plugin`) with working stubs that you extend with your logic.

**`pkg/plugin/plugin_test.go`** -- tests that use `goplugin.TestPluginGRPCConn` to test the plugin through the full gRPC stack without spawning a subprocess. This catches serialisation bugs early.

**`workspace/zhi.yaml`** -- a sample workspace configuration that references your plugin binary from `../bin/`. Use this for local development and testing:

```sh
make build
cd workspace
zhi list
zhi validate
```

**`Makefile`** -- provides the standard targets:

| Target | Description |
|--------|-------------|
| `make build` | Build for the current platform |
| `make cross-compile` | Build for linux/darwin x amd64/arm64 |
| `make test` | Run all tests with race detection |
| `make lint` | Run golangci-lint |
| `make check` | Format + vet + lint + test |
| `make clean` | Remove build artifacts |

**`.github/workflows/release.yml`** -- a GitHub Actions workflow that triggers on version tags (`v*`). It runs tests, cross-compiles, logs into the OCI registry, and publishes with `zhi plugin publish --sign` using keyless Sigstore signing.

## What Each Plugin Type Generates

### Config

A config plugin that manages two paths (`<name>/host` and `<name>/port`) with typed values, metadata, and validation. The `Validate` method demonstrates type checking and range validation.

### Store

A store plugin with an in-memory map backend implementing the full `store.Plugin` interface. All unsupported capabilities (versioning, encryption, authentication, access control) return descriptive errors. Replace the map with your storage backend.

### Transform

A transform plugin with no-op `BeforeDisplay` and `AfterSave` methods and commented-out examples showing how to add computed display values and revert them before saving. `ValidatePolicy` returns `ValidateBeforeTransform`.

### UI

A UI plugin that starts a basic HTTP server with `/health` and `/tree` endpoints. The `Run` method receives a `ui.Controller` for accessing the full zhi engine. `RequiresTTY` is `false` since the plugin communicates over gRPC.

## Development Workflow

1. **Generate** the project:

   ```sh
   zhi plugin new --name my-config --type config --author myorg
   ```

2. **Configure dependencies** -- update `go.mod` with the correct zhi version:

   ```
   require github.com/MrWong99/zhi v1.2.3
   ```

   Or for local development against a zhi checkout:

   ```
   replace github.com/MrWong99/zhi => /path/to/zhi
   ```

   Then run `go mod tidy`.

3. **Implement** your plugin logic in `pkg/plugin/plugin.go`.

4. **Test** through the gRPC layer:

   ```sh
   make test
   ```

5. **Try it locally** with the sample workspace:

   ```sh
   make build
   cd workspace
   zhi list
   zhi validate
   zhi export
   ```

6. **Publish** when ready:

   ```sh
   make cross-compile
   zhi plugin publish --registry ghcr.io/myorg --sign
   ```

   Or push a version tag to trigger the GitHub Actions release workflow.

## Extending the Scaffolding System

The scaffolding system is designed to support multiple languages. It uses a registry of language-specific `Scaffolder` implementations.

### Architecture

```
internal/scaffold/
├── scaffold.go                  Scaffolder interface, Data struct, registry
└── golang/
    ├── scaffolder.go            Go scaffolder (renders embedded templates)
    └── templates/               embedded template files
        ├── common/              shared across all plugin types
        │   ├── go.mod.tmpl
        │   ├── Makefile.tmpl
        │   ├── gitignore.tmpl
        │   ├── README.md.tmpl
        │   ├── zhi-plugin.yaml.tmpl
        │   └── release.yml.tmpl
        ├── config/              config-specific templates
        ├── store/               store-specific templates
        ├── transform/           transform-specific templates
        └── ui/                  ui-specific templates
```

### Adding a new language

1. Create a new package under `internal/scaffold/` (e.g. `internal/scaffold/python/`).
2. Implement the `scaffold.Scaffolder` interface:

   ```go
   type Scaffolder interface {
       Scaffold(data Data, outputDir string) error
       Language() string
   }
   ```

3. Register it in an `init()` function:

   ```go
   func init() {
       scaffold.Register(&pythonScaffolder{})
   }
   ```

4. Import the package in `internal/cli/plugin_new.go` with a blank import:

   ```go
   _ "github.com/MrWong99/zhi/internal/scaffold/python"
   ```

The `Data` struct provides all template variables (plugin name, type, module path, author, etc.). Templates use `[[ ]]` delimiters to avoid conflicts with Go source `{}` and GitHub Actions `${{ }}` syntax.

## Further Reading

- [Plugin Development Overview](overview.md) -- all plugin types and shared concepts
- [Config Plugin API](config-plugin.md)
- [Transform Plugin API](transform-plugin.md)
- [Store Plugin API](store-plugin.md)
- [UI Plugin API](ui-plugin.md)
- [Sharing and Registries](../user-guide/sharing-and-registries.md) -- publishing workflow
