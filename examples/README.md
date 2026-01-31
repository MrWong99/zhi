# Examples for Plugins in zhi

## zhi-config-pokedex

A configuration plugin that manages a small Pokedex. Demonstrates:

- Implementing `config.Plugin` (List, Get, Set, Validate)
- Typed values with metadata
- Single-value validation (allowed starters, valid regions, goal bounds)
- Cross-value validation (starter must match region)
- Wiring a plugin binary with `plugin.Serve`

See [zhi-config-pokedex/main.go](zhi-config-pokedex/main.go) and the
[Configuration Plugin docs](../docs/config-plugin.md) for details.

## zhi-transform-pokedex

A transform plugin that evolves starter Pokemon before display. Demonstrates:

- Implementing `transform.Plugin` (BeforeDisplay, AfterSave, ValidatePolicy)
- Reading and mutating the shared configuration tree
- Mapping transformed values back to their original form on save

See [zhi-transform-pokedex/main.go](zhi-transform-pokedex/main.go) and the
[Transform Plugin docs](../docs/transform-plugin.md) for details.

## zhi-store-json

A store plugin that persists configuration trees as JSON files on disk. Demonstrates:

- Implementing `store.Plugin` (Save, Load, Delete, ListTrees)
- File-based persistence with one JSON file per tree ID
- Configurable storage directory via `ZHI_JSON_STORE_DIR` environment variable

This example does not support versioning or encryption.

See [zhi-store-json/main.go](zhi-store-json/main.go) and the
[Store Plugin docs](../docs/store-plugin.md) for details.

## zhi-store-memory

A store plugin that persists configuration trees in memory. Demonstrates:

- Implementing `store.Plugin` with the simplest possible backend (a Go map)
- Minimal store plugin structure for quick prototyping

This example does not support versioning or encryption.

See [zhi-store-memory/main.go](zhi-store-memory/main.go) and the
[Store Plugin docs](../docs/store-plugin.md) for details.

## zhi-ui-httpapi

A UI plugin that exposes the zhi configuration engine as an HTTP/JSON API. Demonstrates:

- Implementing `ui.Plugin` (Run, Capabilities) with `RequiresTTY: false`
- Using the `Controller` interface for all core operations (tree, export, apply, components)
- SSE (Server-Sent Events) for streaming `Apply` output to HTTP clients
- Graceful HTTP server lifecycle tied to the plugin context
- Configurable listen address via `ZHI_HTTP_ADDR` environment variable

See [zhi-ui-httpapi/main.go](zhi-ui-httpapi/main.go) and the
[UI Plugin docs](../docs/ui-plugin.md) for details.
