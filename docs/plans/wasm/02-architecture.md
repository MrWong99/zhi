# 02 — Architecture

This document describes how WASM plugins integrate into zhi's existing
plugin architecture. The design goal is minimal disruption: the engine,
registry, workspace configuration, and CLI should treat WASM plugins as
just another plugin format, not a separate subsystem.

## Current Architecture (Recap)

```
┌─────────────────────────────────────────────┐
│                   Engine                     │
│  ┌─────────┐ ┌───────────┐ ┌─────────┐     │
│  │ Config  │ │ Transform │ │  Store  │     │
│  │ Plugin  │ │  Plugin   │ │ Plugin  │     │
│  └────┬────┘ └─────┬─────┘ └────┬────┘     │
│       │             │            │           │
│  ┌────┴─────────────┴────────────┴────┐     │
│  │            Registry                │     │
│  │  built-in factories + externals    │     │
│  └────┬───────────────────────────────┘     │
│       │                                      │
│  ┌────┴────────────────────┐                │
│  │    Launch (go-plugin)   │                │
│  │  binary → subprocess    │                │
│  │  → gRPC over stdio     │                │
│  └─────────────────────────┘                │
└─────────────────────────────────────────────┘
```

External plugins are native binaries launched as subprocesses via
hashicorp/go-plugin. Communication is gRPC over stdio.

## Proposed Architecture (with WASM)

```
┌───────────────────────────────────────────────────┐
│                      Engine                        │
│  ┌─────────┐ ┌───────────┐ ┌─────────┐ ┌────┐   │
│  │ Config  │ │ Transform │ │  Store  │ │ UI │   │
│  │ Plugin  │ │  Plugin   │ │ Plugin  │ │Plug│   │
│  └────┬────┘ └─────┬─────┘ └────┬────┘ └──┬─┘   │
│       │             │            │          │      │
│  ┌────┴─────────────┴────────────┴──────────┴──┐  │
│  │                 Registry                     │  │
│  │  built-in factories + externals + wasm       │  │
│  └──┬──────────────────────────────────┬───────┘  │
│     │                                  │          │
│  ┌──┴────────────────┐  ┌──────────────┴───────┐  │
│  │ Launch (go-plugin)│  │  WASMLoader (wazero) │  │
│  │ binary→subprocess │  │  .wasm → in-process  │  │
│  │ → gRPC over stdio │  │  → host fn calls     │  │
│  └───────────────────┘  └──────────────────────┘  │
└───────────────────────────────────────────────────┘
```

The key addition is a **WASMLoader** that sits alongside the existing
Launch mechanism. WASM plugins are loaded **in-process** (no subprocess,
no gRPC) via wazero, with host functions providing the bridge between the
WASM module and zhi's Go interfaces.

## Discovery

### File naming

WASM plugins follow the same naming conventions as native plugins, with
a `.wasm` extension:

**Flat naming:**
```
~/.zhi/plugins/zhi-config-pokedex.wasm    → config plugin "pokedex"
~/.zhi/plugins/zhi-transform-encrypt.wasm → transform plugin "encrypt"
~/.zhi/plugins/zhi-store-memory.wasm      → store plugin "memory"
~/.zhi/plugins/zhi-ui-http.wasm           → UI plugin "http"
```

**Subdirectory layout:**
```
~/.zhi/plugins/config/pokedex.wasm
~/.zhi/plugins/transform/encrypt.wasm
~/.zhi/plugins/store/memory.wasm
~/.zhi/plugins/ui/http.wasm
```

### Changes to `discovery.go`

The existing `Discover()` function checks `isExecutable()` for native
binaries. For WASM, we add a parallel check for `.wasm` files:

```go
// PluginInfo gains a Format field.
type PluginInfo struct {
    Name   string
    Type   PluginType
    Path   string
    Format PluginFormat  // new
}

type PluginFormat int

const (
    PluginFormatNative PluginFormat = iota
    PluginFormatWASM
)
```

Discovery logic for flat naming becomes:

```go
func parseFlatName(name string) (PluginType, string, PluginFormat, bool) {
    // Strip .wasm suffix if present.
    format := PluginFormatNative
    base := name
    if strings.HasSuffix(name, ".wasm") {
        format = PluginFormatWASM
        base = strings.TrimSuffix(name, ".wasm")
    }
    // ... existing prefix parsing on `base` ...
    return pluginType, pluginName, format, true
}
```

Native and WASM plugins with the same type+name are a conflict. If both
`zhi-config-pokedex` and `zhi-config-pokedex.wasm` exist, the native
binary takes precedence (matching the existing "flat before subdirectory"
precedence rule). A warning is logged.

### Changes to `registry.go`

The `launchExternal*` methods dispatch based on `PluginInfo.Format`:

```go
func (r *Registry) launchExternalConfig(name string) (config.Plugin, error) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if p, ok := r.cachedConfig[name]; ok {
        return p, nil
    }

    info, ok := r.findExternal(name, PluginTypeConfig)
    if !ok {
        return nil, fmt.Errorf("unknown config provider: %q", name)
    }

    var p config.Plugin
    var cleanup func()
    var err error

    switch info.Format {
    case PluginFormatNative:
        p, cleanup, err = LaunchConfig(info.Path)
    case PluginFormatWASM:
        p, cleanup, err = LoadWASMConfig(info.Path)
    }
    // ... cache and return ...
}
```

## WASMLoader

The new `pkg/zhiplugin/wasmloader/` package provides the WASM equivalent
of `pkg/zhiplugin/launch/`:

```go
package wasmloader

// LoadConfig loads a .wasm file and returns a config.Plugin backed by WASM.
func LoadConfig(wasmPath string, opts ...Option) (config.Plugin, func(), error)

// LoadTransform loads a .wasm file and returns a transform.Plugin backed by WASM.
func LoadTransform(wasmPath string, opts ...Option) (transform.Plugin, func(), error)

// LoadStore loads a .wasm file and returns a store.Plugin backed by WASM.
func LoadStore(wasmPath string, opts ...Option) (store.Plugin, func(), error)

// LoadUI loads a .wasm file and returns a ui.Plugin backed by WASM.
func LoadUI(wasmPath string, opts ...Option) (ui.Plugin, func(), error)
```

Each function:

1. Reads and compiles the `.wasm` file via `wazero.NewRuntime()` +
   `runtime.CompileModule()`.
2. Instantiates the host module with the host functions for that plugin
   type (see [03-host-functions-abi.md](03-host-functions-abi.md)).
3. Optionally instantiates `wasi_snapshot_preview1` if the WASM module
   imports WASI functions (for plugins built with `GOOS=wasip1`).
4. Instantiates the WASM module.
5. Returns an adapter that implements the Go plugin interface by calling
   the module's exported functions.

The `cleanup` function closes the wazero runtime and releases resources.

### Module lifecycle

```
 Load .wasm file
       │
       ▼
 wazero.CompileModule()      ← validates + AOT compiles
       │
       ▼
 Instantiate host module     ← register host functions
       │
       ▼
 Instantiate wasi (optional) ← if module imports wasi_snapshot_preview1
       │
       ▼
 Instantiate plugin module   ← links imports, runs _start or _initialize
       │
       ▼
 Return adapter + cleanup
```

### Compiled module caching

wazero's `CompiledModule` is reusable and thread-safe. If the same `.wasm`
file is loaded multiple times (e.g., multiple workspaces using the same
plugin), the compiled module can be cached and only instantiation (which is
cheap) is repeated.

```go
type moduleCache struct {
    mu       sync.Mutex
    compiled map[string]wazero.CompiledModule // keyed by file hash
}
```

## Workspace Configuration

WASM plugins are referenced in `zhi.yaml` identically to native plugins.
The format is transparent to workspace authors:

```yaml
config:
  provider: pokedex
  # The registry resolves "pokedex" to either a native binary
  # or a .wasm file — the user doesn't need to know which.

transforms:
  - provider: encrypt
```

### Explicit WASM capabilities

When a WASM plugin needs filesystem or network access beyond the default
sandbox, the workspace configuration grants capabilities:

```yaml
config:
  provider: my-file-reader
  options:
    directory: ./configs
  wasm:
    capabilities:
      fs:
        - path: ./configs
          mode: read
```

See [04-security-model.md](04-security-model.md) for the full capability
model.

## Package Structure

New packages:

```
pkg/zhiplugin/wasmloader/
├── loader.go          # LoadConfig, LoadTransform, LoadStore, LoadUI
├── options.go         # WithLogger, WithCapabilities, etc.
├── runtime.go         # wazero runtime management, module cache
├── host_config.go     # Host functions for config plugins
├── host_transform.go  # Host functions for transform plugins
├── host_store.go      # Host functions for store plugins
├── host_ui.go         # Host functions for UI plugins
├── adapter_config.go  # config.Plugin adapter (calls WASM exports)
├── adapter_transform.go
├── adapter_store.go
├── adapter_ui.go
├── memory.go          # Shared memory helpers (alloc/free, JSON serde)
└── loader_test.go
```

## OCI Distribution

WASM `.wasm` files are single-platform artifacts — no cross-compilation
needed. The existing OCI distribution pipeline
(`zhi plugin publish` / `zhi plugin install`) gains WASM support:

```yaml
# zhi-plugin.yaml for a WASM plugin
name: pokedex
type: config
format: wasm
binaries:
  - os: any
    arch: any
    path: bin/zhi-config-pokedex.wasm
```

The `os: any, arch: any` platform signals that this is a universal
artifact. `zhi plugin install` detects the `.wasm` extension and places it
in the appropriate plugin directory with the correct naming.

See also the existing `pkg/sharing/` code for manifest and OCI client
details.

## Interaction with Meta-Plugin SDK

Meta-plugins (plugins that compose child plugins via
`pkg/zhiplugin/launch/`) currently only launch native binaries. The
meta-plugin SDK would gain WASM-aware variants:

```go
// In pkg/zhiplugin/launch/
func LaunchOrLoadConfig(path string, opts ...Option) (config.Plugin, func(), error) {
    if strings.HasSuffix(path, ".wasm") {
        return wasmloader.LoadConfig(path, toWASMOpts(opts)...)
    }
    return LaunchConfig(path, opts...)
}
```

This allows meta-plugins to compose both native and WASM child plugins
transparently.

## Error Handling

WASM module panics are caught by the wazero runtime and surfaced as Go
errors — they never crash the host process. This is a significant safety
improvement over native plugins, where a segfault in the subprocess
terminates the gRPC connection but doesn't affect the host.

Error categories:

| Error | Source | Handling |
|-------|--------|----------|
| Module compilation failure | Invalid `.wasm` file | Return error from Load* |
| Missing required export | Plugin doesn't export expected function | Return error from Load* |
| WASM trap (panic, OOB, etc.) | Plugin runtime error | Caught by wazero, returned as Go error |
| Host function error | Error in host-provided function | Returned to WASM via error protocol |
| Timeout | Plugin exceeds execution limit | wazero fuel/deadline cancellation |
| OOM | Plugin exceeds memory limit | wazero memory limit enforcement |

## Backward Compatibility

This design is **fully backward-compatible**:

- Existing native gRPC plugins work unchanged.
- No changes to the handshake protocol or gRPC transport.
- No changes to the `zhi-plugin.yaml` manifest format (only new optional
  fields).
- Workspace `zhi.yaml` files that reference native plugins work unchanged.
- The `PluginInfo.Format` field defaults to `PluginFormatNative` if
  unset, preserving existing behavior.
