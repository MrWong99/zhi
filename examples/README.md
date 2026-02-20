# 🧪 Example Plugins for zhi

Welcome to the examples! Each plugin below is a fully working, buildable project you can use as a starting point for your own plugins. Build them all with `make build-examples` or go explore one at a time. 🚀

---

## 🔴 zhi-config-pokedex

A configuration plugin that manages a small Pokedex -- because every good config system needs Pokemon data. Demonstrates:

- Implementing `config.Plugin` (List, Get, Set, Validate)
- Typed values with metadata
- Single-value validation (allowed starters, valid regions, goal bounds)
- Cross-value validation (starter must match region)
- Wiring a plugin binary with `plugin.Serve`

See [zhi-config-pokedex/main.go](zhi-config-pokedex/main.go) and the
[Configuration Plugin docs](../docs/plugin-development/config-plugin.md) for details.

---

## ⚡ zhi-transform-pokedex

A transform plugin that evolves starter Pokemon before display. It's super effective! Demonstrates:

- Implementing `transform.Plugin` (BeforeDisplay, AfterSave, ValidatePolicy)
- Reading and mutating the shared configuration tree
- Mapping transformed values back to their original form on save

See [zhi-transform-pokedex/main.go](zhi-transform-pokedex/main.go) and the
[Transform Plugin docs](../docs/plugin-development/transform-plugin.md) for details.

---

## 📁 zhi-store-json

A store plugin that persists configuration trees as JSON files on disk. Simple, readable, debuggable. Demonstrates:

- Implementing `store.Plugin` (Save, Load, Delete, ListTrees)
- File-based persistence with one JSON file per tree ID
- Configurable storage directory via `ZHI_JSON_STORE_DIR` environment variable

This example does not support versioning or encryption.

See [zhi-store-json/main.go](zhi-store-json/main.go) and the
[Store Plugin docs](../docs/plugin-development/store-plugin.md) for details.

---

## 🧠 zhi-store-memory

A store plugin that persists configuration trees in memory. Poof -- gone when the process exits, perfect for testing. Demonstrates:

- Implementing `store.Plugin` with the simplest possible backend (a Go map)
- Minimal store plugin structure for quick prototyping

This example does not support versioning or encryption.

See [zhi-store-memory/main.go](zhi-store-memory/main.go) and the
[Store Plugin docs](../docs/plugin-development/store-plugin.md) for details.

---

## 🔑 zhi-store-vault

A store plugin backed by HashiCorp Vault's KV v2 secrets engine. For when your configs need a bodyguard. Demonstrates:

- Using the Vault store provider as an external plugin binary
- Configuration via environment variables (`VAULT_ADDR`, `VAULT_TOKEN`, `ZHI_VAULT_MOUNT`, `ZHI_VAULT_PREFIX`)
- Value-level versioning, authentication, encryption, and access control via Vault

See [zhi-store-vault/main.go](zhi-store-vault/main.go) and the
[Store Plugin docs](../docs/plugin-development/store-plugin.md) for details.

---

## 🪞 zhi-store-mirror

A meta-plugin that mirrors writes to multiple stores -- write once, persist everywhere. This is the prime example of the meta-plugin SDK in action. Demonstrates:

- Using `pkg/zhiplugin/launch/` to start child plugin processes
- Using `store.DelegatingPlugin` to forward calls to a base store
- Using `store.MirroredPlugin` to replicate writes across a primary and backup store
- Binary validation and integrity auditing for child plugins

See [zhi-store-mirror/main.go](zhi-store-mirror/main.go) and the
[Meta-Plugin SDK docs](../docs/plugin-development/meta-plugin.md) for details.

---

## ☕ zhi-config-javabean

A config plugin written in Java using Bean Validation and GraalVM native-image. Proof that zhi plugins aren't Go-only! Demonstrates:

- Implementing the gRPC ConfigService in Java
- Mapping Java beans to zhi configuration paths with annotations (`@ConfigPrefix`, `@ConfigProperty`)
- Using Jakarta Bean Validation for single-value and cross-value validation
- Building a native binary with GraalVM `native-image`

See [zhi-config-javabean/](zhi-config-javabean/) and the
[Java Plugin Development docs](../docs/plugin-development/java-plugin.md) for details.

---

## 🌐 zhi-ui-httpapi

A UI plugin that exposes the zhi configuration engine as an HTTP/JSON API. REST lovers, this one's for you. Demonstrates:

- Implementing `ui.Plugin` (Run, Capabilities) with `RequiresTTY: false`
- Using the `Controller` interface for all core operations (tree, export, apply, components)
- SSE (Server-Sent Events) for streaming `Apply` output to HTTP clients
- Graceful HTTP server lifecycle tied to the plugin context
- Configurable listen address via `ZHI_HTTP_ADDR` environment variable

See [zhi-ui-httpapi/main.go](zhi-ui-httpapi/main.go) and the
[UI Plugin docs](../docs/plugin-development/ui-plugin.md) for details.

---

## 🤖 zhi-ui-mcp-sse

A UI plugin that exposes the zhi configuration engine as an MCP (Model Context Protocol) server over HTTP. Let your LLM do the configuring! Demonstrates:

- Implementing `ui.Plugin` (Run, Capabilities) with `RequiresTTY: false`
- Using the shared `pkg/mcpbridge/` library to register all MCP tools, resources, and prompts
- Streamable HTTP transport via `mcp.NewStreamableHTTPHandler`
- Bearer token authentication with `crypto/subtle.ConstantTimeCompare`
- Configurable listen address via `ZHI_MCP_ADDR` and auth token via `ZHI_MCP_TOKEN`

A builtin MCP stdio plugin (`mcp-stdio`) is also available for direct use with LLM clients that support stdio transport (e.g. Claude Desktop, Claude Code). Launch it with `zhi edit --ui mcp-stdio`.

See [zhi-ui-mcp-sse/main.go](zhi-ui-mcp-sse/main.go) and the
[UI Plugin docs](../docs/plugin-development/ui-plugin.md) for details.

---

## 🖥️ zhi-ui-webui

A browser-based Web UI plugin that serves a full configuration editor on localhost. For when the terminal isn't fancy enough. Demonstrates:

- Implementing `ui.Plugin` (Run, Capabilities) with `RequiresTTY: false`
- Serving a web frontend with an embedded HTTP server
- Integration with the `Controller` interface for all core operations

See [zhi-ui-webui/](zhi-ui-webui/) and the
[UI Plugin docs](../docs/plugin-development/ui-plugin.md) for details.
