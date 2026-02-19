# Examples for Plugins in zhi

## zhi-config-pokedex

A configuration plugin that manages a small Pokedex. Demonstrates:

- Implementing `config.Plugin` (List, Get, Set, Validate)
- Typed values with metadata
- Single-value validation (allowed starters, valid regions, goal bounds)
- Cross-value validation (starter must match region)
- Wiring a plugin binary with `plugin.Serve`

See [zhi-config-pokedex/main.go](zhi-config-pokedex/main.go) and the
[Configuration Plugin docs](../docs/plugin-development/config-plugin.md) for details.

## zhi-transform-pokedex

A transform plugin that evolves starter Pokemon before display. Demonstrates:

- Implementing `transform.Plugin` (BeforeDisplay, AfterSave, ValidatePolicy)
- Reading and mutating the shared configuration tree
- Mapping transformed values back to their original form on save

See [zhi-transform-pokedex/main.go](zhi-transform-pokedex/main.go) and the
[Transform Plugin docs](../docs/plugin-development/transform-plugin.md) for details.

## zhi-store-json

A store plugin that persists configuration trees as JSON files on disk. Demonstrates:

- Implementing `store.Plugin` (Save, Load, Delete, ListTrees)
- File-based persistence with one JSON file per tree ID
- Configurable storage directory via `ZHI_JSON_STORE_DIR` environment variable

This example does not support versioning or encryption.

See [zhi-store-json/main.go](zhi-store-json/main.go) and the
[Store Plugin docs](../docs/plugin-development/store-plugin.md) for details.

## zhi-store-memory

A store plugin that persists configuration trees in memory. Demonstrates:

- Implementing `store.Plugin` with the simplest possible backend (a Go map)
- Minimal store plugin structure for quick prototyping

This example does not support versioning or encryption.

See [zhi-store-memory/main.go](zhi-store-memory/main.go) and the
[Store Plugin docs](../docs/plugin-development/store-plugin.md) for details.

## zhi-store-vault

A store plugin backed by HashiCorp Vault's KV v2 secrets engine. Demonstrates:

- Using the Vault store provider as an external plugin binary
- Configuration via environment variables (`VAULT_ADDR`, `VAULT_TOKEN`, `ZHI_VAULT_MOUNT`, `ZHI_VAULT_PREFIX`)
- Value-level versioning, authentication, encryption, and access control via Vault

See [zhi-store-vault/main.go](zhi-store-vault/main.go) and the
[Store Plugin docs](../docs/plugin-development/store-plugin.md) for details.

## zhi-config-javabean

A config plugin written in Java using Bean Validation and GraalVM native-image. Demonstrates:

- Implementing the gRPC ConfigService in Java
- Mapping Java beans to zhi configuration paths with annotations (`@ConfigPrefix`, `@ConfigProperty`)
- Using Jakarta Bean Validation for single-value and cross-value validation
- Building a native binary with GraalVM `native-image`

See [zhi-config-javabean/](zhi-config-javabean/) and the
[Java Plugin Development docs](../docs/plugin-development/java-plugin.md) for details.

## zhi-ui-httpapi

A UI plugin that exposes the zhi configuration engine as an HTTP/JSON API. Demonstrates:

- Implementing `ui.Plugin` (Run, Capabilities) with `RequiresTTY: false`
- Using the `Controller` interface for all core operations (tree, export, apply, components)
- SSE (Server-Sent Events) for streaming `Apply` output to HTTP clients
- Graceful HTTP server lifecycle tied to the plugin context
- Configurable listen address via `ZHI_HTTP_ADDR` environment variable

See [zhi-ui-httpapi/main.go](zhi-ui-httpapi/main.go) and the
[UI Plugin docs](../docs/plugin-development/ui-plugin.md) for details.

## zhi-ui-mcp-sse

A UI plugin that exposes the zhi configuration engine as an MCP (Model Context Protocol) server over HTTP. Demonstrates:

- Implementing `ui.Plugin` (Run, Capabilities) with `RequiresTTY: false`
- Using the shared `pkg/mcpbridge/` library to register all MCP tools, resources, and prompts
- Streamable HTTP transport via `mcp.NewStreamableHTTPHandler`
- Bearer token authentication with `crypto/subtle.ConstantTimeCompare`
- Configurable listen address via `ZHI_MCP_ADDR` and auth token via `ZHI_MCP_TOKEN`

A builtin MCP stdio plugin (`mcp-stdio`) is also available for direct use with LLM clients that support stdio transport (e.g. Claude Desktop, Claude Code). Launch it with `zhi edit --ui mcp-stdio`.

See [zhi-ui-mcp-sse/main.go](zhi-ui-mcp-sse/main.go) and the
[UI Plugin docs](../docs/plugin-development/ui-plugin.md) for details.
