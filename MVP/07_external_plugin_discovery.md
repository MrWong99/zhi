# Step 7: External Plugin Discovery

## Overview

Extend the provider registry to discover and launch external plugins from `~/.zhi/plugins/` (and user-configured directories). External plugins are separate binaries that communicate via hashicorp/go-plugin over gRPC, using the existing handshake and protocol definitions.

## Relevant Existing Files

- `pkg/zhiplugin/plugin.go` — shared handshake (`ZHI_PLUGIN=zhiplugin-v1`, protocol version 1)
- `pkg/zhiplugin/config/plugin.go` — `GRPCPlugin` struct with `GRPCClient()` and `GRPCServer()`, `PluginMap`
- `pkg/zhiplugin/config/grpc_client.go` — host-side gRPC client for config plugins
- `pkg/zhiplugin/config/grpc_server.go` — plugin-side gRPC server for config plugins
- `pkg/zhiplugin/transform/plugin.go` — same pattern for transform
- `pkg/zhiplugin/store/plugin.go` — same pattern for store
- `internal/core/registry.go` — provider registry from Step 1 (will be extended)
- `internal/core/workspace.go` — workspace config
- `examples/zhi-config-pokedex/main.go` — example external config plugin
- `examples/zhi-transform-pokedex/main.go` — example external transform plugin
- `examples/zhi-store-json/main.go` — example external store plugin
- `examples/zhi-store-memory/main.go` — example external store plugin

## Implementation Plan

### 7.1 Plugin Discovery (`internal/core/discovery.go`)

Scan directories for external plugin binaries and make them available to the registry.

**Components:**

- `DiscoveryConfig` struct:
  - `Directories []string` — list of directories to scan (default: `~/.zhi/plugins/`)
  - `NamingConvention string` — how plugin names map to binary names
- `PluginInfo` struct:
  - `Name string` — provider name (e.g., `zhi-config-pokedex`)
  - `Type string` — plugin type: `config`, `transform`, or `store`
  - `Path string` — absolute path to the binary
- `Discover(config DiscoveryConfig) ([]PluginInfo, error)` — scan directories and return found plugins

**Naming convention:**

External plugin binaries follow the pattern: `zhi-<type>-<name>` or `zhi-<name>`

Examples:
- `zhi-config-pokedex` → config provider named `pokedex`
- `zhi-transform-evolve` → transform provider named `evolve`
- `zhi-store-vault` → store provider named `vault`
- `zhi-store-json` → store provider named `json`

If the binary name doesn't include a type prefix, discovery attempts to determine the type by launching the plugin and checking which gRPC services it exposes (or by convention from subdirectories).

**Directory structure:**
```
~/.zhi/plugins/
  zhi-config-pokedex        # config plugin
  zhi-transform-evolve      # transform plugin
  zhi-store-vault           # store plugin
```

Alternatively, type-based subdirectories:
```
~/.zhi/plugins/
  config/
    pokedex                 # config plugin named "pokedex"
  transform/
    evolve                  # transform plugin named "evolve"
  store/
    vault                   # store plugin named "vault"
```

Both layouts are supported. Flat naming takes precedence.

### 7.2 Plugin Launcher (`internal/core/launcher.go`)

Launch an external plugin binary using hashicorp/go-plugin and return a typed provider interface.

**Components:**

- `LaunchConfig(path string) (config.Plugin, func(), error)` — launch a config plugin, return the plugin interface and a cleanup function (to call `client.Kill()`)
- `LaunchTransform(path string) (transform.Plugin, func(), error)` — same for transform
- `LaunchStore(path string) (store.Plugin, func(), error)` — same for store

**Implementation (for config, others follow the same pattern):**

```go
func LaunchConfig(path string) (config.Plugin, func(), error) {
    client := plugin.NewClient(&plugin.ClientConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: config.PluginMap,
        Cmd:             exec.Command(path),
        AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
    })

    rpcClient, err := client.Client()
    if err != nil {
        client.Kill()
        return nil, nil, fmt.Errorf("connecting to plugin %s: %w", path, err)
    }

    raw, err := rpcClient.Dispense("config")
    if err != nil {
        client.Kill()
        return nil, nil, fmt.Errorf("dispensing config plugin %s: %w", path, err)
    }

    return raw.(config.Plugin), client.Kill, nil
}
```

This matches the pattern used in the example plugin tests (e.g., `examples/zhi-config-pokedex/main_test.go`).

### 7.3 Registry Extension (`internal/core/registry.go` update)

Extend the registry to support external plugin resolution as a fallback.

**Changes to `Registry`:**

- Add `discovery *DiscoveryConfig` field
- Add `externalPlugins []PluginInfo` — cached discovery results
- Add `cleanups []func()` — cleanup functions for launched plugins
- Modify `ConfigProvider(name)`: if built-in lookup fails, search `externalPlugins` for a matching config plugin, launch it, cache the result
- Add `Close()` method — call all cleanup functions to kill external plugin processes
- Add `RefreshExternal() error` — re-scan plugin directories

**Lazy loading:** External plugins are only launched when first requested, not at startup. This avoids unnecessary process spawning for unused plugins.

**Caching:** Once launched, the plugin process stays alive and the interface is cached. Subsequent calls to `ConfigProvider("pokedex")` return the cached instance.

### 7.4 Workspace Plugin Directories

Add optional `plugins` section to `zhi.yaml`:

```yaml
plugins:
  directories:
    - ~/.zhi/plugins
    - /usr/local/lib/zhi/plugins
    - ./plugins
```

Default if not specified: `~/.zhi/plugins/` only.

### 7.5 CLI: `zhi list plugins` Enhancement

Update `zhi list providers` to show both built-in and discovered external plugins:

```
$ zhi list providers
CONFIG PROVIDERS:
  structuredfile    (built-in)
  pokedex           (external: ~/.zhi/plugins/zhi-config-pokedex)

TRANSFORM PROVIDERS:
  evolve            (external: ~/.zhi/plugins/zhi-transform-evolve)

STORE PROVIDERS:
  zhi-store-json        (external: ~/.zhi/plugins/zhi-store-json)
```

### 7.6 Plugin Health and Error Handling

- **Binary not executable**: skip with a warning logged, don't fail discovery
- **Plugin crash**: detect via `client.Exited()`, return error to caller, remove from cache so next call re-launches
- **Handshake failure**: log error with binary path and expected handshake values
- **Protocol mismatch**: log clear error about version incompatibility
- **Timeout on connect**: configurable timeout (default 10s), return error if exceeded

### 7.7 Engine Shutdown

The engine's cleanup must kill all external plugin processes:

```go
func (e *Engine) Close() {
    e.registry.Close()
}
```

The CLI root command should defer `engine.Close()` to ensure cleanup on exit.

### 7.8 Tests

- `internal/core/discovery_test.go`:
  - Test flat directory layout discovery
  - Test subdirectory layout discovery
  - Test non-executable files are skipped
  - Test empty directory returns empty list
  - Test naming convention parsing
- `internal/core/launcher_test.go`:
  - Build example plugins (`make build-examples`), then test launching them
  - Test config plugin launch and `List()` call
  - Test transform plugin launch and `BeforeDisplay()` call
  - Test store plugin launch and `Save()`/`Load()` calls
  - Test cleanup function kills the process
- `internal/core/registry_test.go` (extend from Step 1):
  - Test external plugin fallback
  - Test caching of launched plugins
  - Test `Close()` kills all plugins
  - Test `RefreshExternal()` picks up new plugins
- Integration test: `zhi list providers` shows built-in + external plugins

## Verification Criteria

1. Plugins in `~/.zhi/plugins/` are discovered and listed by `zhi list providers`
2. Both naming conventions work: `zhi-config-pokedex` (flat) and `config/pokedex` (subdirectory)
3. External plugins are launched lazily (only when used)
4. Launched plugin processes are cached — second request reuses the same process
5. `engine.Close()` kills all external plugin processes
6. Plugin crashes are detected and reported with clear error messages
7. The existing example plugins (`zhi-config-pokedex`, `zhi-transform-pokedex`, `zhi-store-json`, `zhi-store-memory`) work as external plugins
8. The workspace `zhi.yaml` can reference external plugins by name (e.g., `provider: pokedex`)
9. `zhi validate` works end-to-end with an external config plugin
10. No changes to any files in `pkg/zhiplugin/` — the existing plugin API is sufficient
11. All tests pass with `go test -race ./internal/core/...`
