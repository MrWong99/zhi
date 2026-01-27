# Examples for Plugins in zhi

## pokedex-config

A configuration plugin that manages a small Pokedex. Demonstrates:

- Implementing `config.Plugin` (List, Get, Set, Validate)
- Typed values with metadata
- Single-value validation (allowed starters, valid regions, goal bounds)
- Cross-value validation (starter must match region)
- Wiring a plugin binary with `plugin.Serve`

See [pokedex-config/main.go](pokedex-config/main.go) and the
[Configuration Plugin docs](../docs/config-plugin.md) for details.
