# Plugin Development Overview

zhi uses [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) for its plugin system. Plugins run as separate processes and communicate with the host over gRPC via stdio. This provides process isolation, language independence (in theory), and clean crash boundaries.

## Plugin Types

zhi has four plugin types:

| Type | Purpose | Communication |
|------|---------|---------------|
| [Config](config-plugin.md) | Manage configuration values (List, Get, Set, Validate) | Host -> Plugin |
| [Transform](transform-plugin.md) | Mutate configuration before display or after save | Host -> Plugin |
| [Store](store-plugin.md) | Persist and retrieve configuration trees | Host -> Plugin |
| [UI](ui-plugin.md) | Provide interactive frontends | Bidirectional |

## Shared Handshake

All plugins share the same handshake defined in `pkg/zhiplugin/plugin.go`:

- Magic cookie key: `ZHI_PLUGIN`
- Magic cookie value: `zhiplugin-v1`
- Protocol version: `1`

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin"

// Use zhiplugin.Handshake in your plugin.Serve config
```

## Configuration Tree Model

All plugin types work with the shared configuration tree model (`pkg/zhiplugin/config/`):

- **`Tree`** -- a flat key-value store with slash-delimited paths
- **`TreeReader`** -- read-only interface (`Get`, `List`)
- **`Value`** -- holds `Val any`, optional `Metadata`, and local `Validators`
- **`ValidationResult`** -- carries `Severity` (Info, Warning, Blocking) and `Message`

### Path Format

Paths use `/` as the hierarchy separator. Each segment must match:

```
[a-z][a-z0-9._-]*[a-z0-9]
```

Examples: `database/host`, `app/tls/cert.pem`, `pokedex/trainer.name`

## gRPC Layer

Proto definitions live in `api/proto/zhiplugin/v1/`. Generated Go stubs are placed in `pkg/zhiplugin/{type}/proto/`.

Each plugin type has:

- `grpc_client.go` -- host-side gRPC client
- `grpc_server.go` -- plugin-side gRPC server

Configuration values are JSON-encoded for wire transfer. Validator closures (functions) never cross the wire -- they are local to the plugin process.

## Plugin Binary Structure

A minimal plugin binary needs a `main` function that calls `plugin.Serve`:

```go
package main

import (
    "github.com/hashicorp/go-plugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]plugin.Plugin{
            "config": &config.GRPCPlugin{Impl: &myPlugin{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

Replace `"config"` and `config.GRPCPlugin` with the appropriate type for your plugin.

## Naming Convention

Plugin binaries should follow the naming pattern `zhi-<type>-<name>`:

- `zhi-config-pokedex`
- `zhi-transform-evolve`
- `zhi-store-vault`
- `zhi-ui-httpapi`

This allows zhi to discover and categorize plugins automatically from `~/.zhi/plugins/`.

## Testing

Use `goplugin.TestPluginGRPCConn()` for in-process gRPC testing without starting a subprocess:

```go
func TestMyPlugin(t *testing.T) {
    client, server := goplugin.TestPluginGRPCConn(t, true, map[string]goplugin.Plugin{
        "config": &config.GRPCPlugin{Impl: &myPlugin{}},
    })
    defer client.Close()
    defer server.Stop()

    raw, err := client.Dispense("config")
    require.NoError(t, err)
    p := raw.(config.Plugin)

    // Test your plugin through the gRPC interface
    paths, err := p.List(context.Background())
    require.NoError(t, err)
    // ...
}
```

## Examples

The [`examples/`](../../examples/) directory contains working reference implementations:

| Example | Type | Description |
|---------|------|-------------|
| [zhi-config-pokedex](../../examples/zhi-config-pokedex/) | Config | Typed values, metadata, single and cross-value validation |
| [zhi-transform-pokedex](../../examples/zhi-transform-pokedex/) | Transform | Tree mutation, value mapping |
| [zhi-store-json](../../examples/zhi-store-json/) | Store | File-based persistence |
| [zhi-store-memory](../../examples/zhi-store-memory/) | Store | Minimal in-memory store |
| [zhi-ui-httpapi](../../examples/zhi-ui-httpapi/) | UI | HTTP/JSON API with SSE streaming |
| [zhi-config-javabean](../../examples/zhi-config-javabean/) | Config | Java bean with Bean Validation, GraalVM native-image |

## Non-Go Plugin Development

Plugins communicate over gRPC, so any language with gRPC support can implement a zhi plugin. See the language-specific guides:

- [Java Plugin Development](java-plugin.md) -- Gradle setup, Bean Validation, GraalVM native-image

## Built-in Provider Reference

- [Structured File Provider](structuredfile-provider.md) -- loads configuration from JSON and YAML files

## Publishing Plugins

Once your plugin is built and tested, you can share it via OCI registries.

1. Create a `zhi-plugin.yaml` manifest:

```sh
zhi plugin init --name my-config --type config --version 1.0.0
```

2. Build binaries for your target platforms (e.g. using GoReleaser or `GOOS`/`GOARCH` cross-compilation).

3. Publish to a registry:

```sh
zhi plugin publish --registry ghcr.io/myorg --sign
```

See the [Sharing and Registries guide](../user-guide/sharing-and-registries.md) for the full publishing workflow, including signing, marketplace registration, and version management.

## Further Reading

- [Config Plugin API](config-plugin.md)
- [Transform Plugin API](transform-plugin.md)
- [Store Plugin API](store-plugin.md)
- [UI Plugin API](ui-plugin.md)
- [Java Plugin Development](java-plugin.md)
- [Sharing and Registries](../user-guide/sharing-and-registries.md)
