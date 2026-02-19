# 05 — Plugin Development

This document covers how plugin authors build WASM plugins for zhi:
supported languages, build toolchains, the Plugin Development Kit (PDK),
and the developer workflow.

## Supported Languages

Any language that compiles to WebAssembly with WASI support can produce
a zhi plugin. The ABI is language-agnostic (JSON over shared memory).
However, the quality of the developer experience varies by language:

### Tier 1: Go (primary target)

Go is the natural first choice since zhi itself is Go and most existing
plugins are Go.

**Toolchains:**

| Toolchain | Binary Size | Stdlib | Reflection | Build Command |
|-----------|-------------|--------|------------|---------------|
| Go gc (`GOOS=wasip1`) | ~2.5 MB | Full | Full | `GOOS=wasip1 GOARCH=wasm go build` |
| TinyGo | ~160 KB–500 KB | Partial | Partial | `tinygo build -target=wasip1` |

**Go gc (standard toolchain) — Recommended for most plugins:**
- Full standard library support including `encoding/json`, `reflect`, etc.
- `//go:wasmexport` directive (Go 1.24+) for exporting functions.
- Reactor mode via `-buildmode=c-shared` for long-lived modules.
- Larger binary but complete language compatibility.

**TinyGo — Recommended when binary size matters:**
- Dramatically smaller binaries (5–15× smaller).
- WASI P2 support (ahead of standard Go).
- Limited `reflect` support — `encoding/json` may panic at runtime.
- Not all stdlib packages compile.
- `recover()` does not work on WASM target.

**Recommendation:** Start with standard Go toolchain for maximum
compatibility. The PDK should work with both, and plugin authors can
choose based on their needs.

### Tier 2: Rust

Rust has excellent WASM support and produces small, fast binaries.

- Compile with `--target wasm32-wasip1`.
- Full control over memory layout.
- No garbage collector overhead.
- Requires implementing the ABI manually (JSON serialization, export
  functions, memory management).
- Binary sizes typically 100 KB–2 MB.

A Rust PDK crate (`zhi-plugin-sdk`) could be provided in the future
but is out of scope for the initial implementation.

### Tier 3: C/C++, Zig, AssemblyScript

These languages compile to WASM and could produce zhi plugins, but
no first-party PDK would be provided initially. The ABI documentation
and examples should be sufficient for authors using these languages.

## Go Plugin Development Kit (PDK)

The PDK is a Go library that plugin authors import. It handles:

1. ABI boilerplate (memory protocol, JSON serialization, error reporting)
2. Type-safe interfaces matching zhi's plugin contracts
3. Build helpers and scaffolding

### Package structure

```
pkg/zhiplugin/wasmpdk/
├── pdk.go              # Core ABI helpers (set_output, set_error, memory)
├── config.go           # Config plugin registration
├── transform.go        # Transform plugin registration
├── store.go            # Store plugin registration
├── ui.go               # UI plugin registration
├── hostcall.go         # Wrappers for calling host functions (fs, http)
└── types.go            # Shared types (Value, Tree, ValidationResult, etc.)
```

### Config plugin example

A complete WASM config plugin in Go using the PDK:

```go
package main

import (
    "github.com/MrWong99/zhi/pkg/zhiplugin/wasmpdk"
)

// MyConfig implements the config plugin interface.
type MyConfig struct {
    data map[string]wasmpdk.Value
}

func (c *MyConfig) List() ([]string, error) {
    paths := make([]string, 0, len(c.data))
    for k := range c.data {
        paths = append(paths, k)
    }
    return paths, nil
}

func (c *MyConfig) Get(path string) (wasmpdk.Value, bool, error) {
    v, ok := c.data[path]
    return v, ok, nil
}

func (c *MyConfig) Set(path string, v wasmpdk.Value) error {
    c.data[path] = v
    return nil
}

func (c *MyConfig) Validate(path string, tree wasmpdk.TreeReader) ([]wasmpdk.ValidationResult, error) {
    // Custom validation logic
    return nil, nil
}

func main() {
    wasmpdk.RegisterConfig(&MyConfig{
        data: map[string]wasmpdk.Value{
            "app/name":    {Val: "my-app"},
            "app/version": {Val: "1.0.0"},
        },
    })
    // wasmpdk.Run() blocks and handles ABI dispatch.
    wasmpdk.Run()
}
```

Build:
```bash
GOOS=wasip1 GOARCH=wasm go build -o zhi-config-myconfig.wasm .
```

### Transform plugin example

```go
package main

import (
    "strings"

    "github.com/MrWong99/zhi/pkg/zhiplugin/wasmpdk"
)

type UpperCaseTransform struct{}

func (t *UpperCaseTransform) BeforeDisplay(tree *wasmpdk.Tree) error {
    for _, path := range tree.List() {
        v, ok := tree.Get(path)
        if !ok {
            continue
        }
        if s, ok := v.Val.(string); ok {
            v.Val = strings.ToUpper(s)
            tree.Set(path, v)
        }
    }
    return nil
}

func (t *UpperCaseTransform) AfterSave(tree *wasmpdk.Tree) error {
    return nil // no-op
}

func (t *UpperCaseTransform) ValidatePolicy() (wasmpdk.ValidatePolicy, error) {
    return wasmpdk.ValidateBeforeTransform, nil
}

func main() {
    wasmpdk.RegisterTransform(&UpperCaseTransform{})
    wasmpdk.Run()
}
```

### Store plugin example (with host functions)

```go
package main

import (
    "encoding/json"
    "fmt"
    "path"

    "github.com/MrWong99/zhi/pkg/zhiplugin/wasmpdk"
)

type FileStore struct{}

func (s *FileStore) Capabilities() (*wasmpdk.Capabilities, error) {
    return &wasmpdk.Capabilities{
        Versioning: wasmpdk.VersioningNone,
        Encryption: wasmpdk.EncryptionNone,
    }, nil
}

func (s *FileStore) GetValues(id string, paths []string) (map[string]wasmpdk.Value, error) {
    // Read from host filesystem via host function.
    filePath := path.Join("data", id+".json")
    data, err := wasmpdk.HostFS.Read(filePath)
    if err != nil {
        return nil, fmt.Errorf("reading tree %s: %w", id, err)
    }

    var all map[string]wasmpdk.Value
    if err := json.Unmarshal(data, &all); err != nil {
        return nil, err
    }

    result := make(map[string]wasmpdk.Value)
    for _, p := range paths {
        if v, ok := all[p]; ok {
            result[p] = v
        }
    }
    return result, nil
}

// ... other store methods ...

func main() {
    wasmpdk.RegisterStore(&FileStore{})
    wasmpdk.Run()
}
```

### PDK internals

The PDK generates the WASM exports via `//go:wasmexport`:

```go
// In wasmpdk/config.go

var registeredConfig ConfigPlugin

func RegisterConfig(p ConfigPlugin) {
    registeredConfig = p
}

//go:wasmexport zhi_config_list
func zhiConfigList() int32 {
    paths, err := registeredConfig.List()
    if err != nil {
        setError(err.Error())
        return 1
    }
    data, _ := json.Marshal(paths)
    setOutput(data)
    return 0
}

//go:wasmexport zhi_config_get
func zhiConfigGet(pathPtr, pathLen int32) int32 {
    path := readStringFromMemory(pathPtr, pathLen)
    val, found, err := registeredConfig.Get(path)
    if err != nil {
        setError(err.Error())
        return 1
    }
    resp := getResponse{Found: found, Value: val.Val, Metadata: val.Metadata}
    data, _ := json.Marshal(resp)
    setOutput(data)
    return 0
}

//go:wasmexport zhi_abi_version
func zhiABIVersion() int32 {
    return 1
}
```

Host function calls are wrapped with `//go:wasmimport`:

```go
// In wasmpdk/hostcall.go

//go:wasmimport zhi_v1 host_set_output
func hostSetOutput(ptr, len int32)

//go:wasmimport zhi_v1 host_set_error
func hostSetError(ptr, len int32)

//go:wasmimport zhi_v1 host_fs_read
func hostFSRead(pathPtr, pathLen int32) int32

//go:wasmimport zhi_v1 host_http_request
func hostHTTPRequest(reqPtr, reqLen int32) int32
```

## Build Process

### Standard Go build

```bash
# Config plugin
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared \
    -o bin/zhi-config-myplugin.wasm ./cmd/zhi-config-myplugin/

# Transform plugin
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared \
    -o bin/zhi-transform-mytransform.wasm ./cmd/zhi-transform-mytransform/
```

The `-buildmode=c-shared` flag produces a WASI reactor (long-lived module
with `_initialize` instead of `_start`), which is appropriate for plugins
that handle multiple calls.

### TinyGo build

```bash
tinygo build -target=wasip1 -o bin/zhi-config-myplugin.wasm \
    ./cmd/zhi-config-myplugin/
```

### Makefile integration

The project Makefile gains WASM build targets:

```makefile
build-wasm-examples:
    GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared \
        -o bin/examples/zhi-config-pokedex.wasm ./examples/zhi-config-pokedex-wasm/
    # ... more examples ...

build-all: build build-examples build-wasm-examples
```

### Cross-platform note

Unlike native binaries, WASM plugins do not need cross-compilation.
A single `go build` invocation produces a `.wasm` file that runs on
all platforms via wazero. This is a significant improvement for the
plugin distribution story.

## Testing WASM Plugins

### Unit testing

WASM plugins are regular Go packages. Unit tests run with the standard
Go test runner (not compiled to WASM):

```bash
go test ./cmd/zhi-config-myplugin/...
```

The PDK interfaces (`ConfigPlugin`, `TransformPlugin`, etc.) are pure Go
interfaces testable without WASM.

### Integration testing

To test the full WASM lifecycle (compile → load → call), an integration
test helper loads the `.wasm` file via the wasmloader:

```go
func TestMyConfigWASM(t *testing.T) {
    p, cleanup, err := wasmloader.LoadConfig("bin/zhi-config-myplugin.wasm")
    require.NoError(t, err)
    defer cleanup()

    paths, err := p.List(context.Background())
    require.NoError(t, err)
    assert.Contains(t, paths, "app/name")
}
```

### CLI testing

```bash
# Install locally and test with a workspace
cp bin/zhi-config-myplugin.wasm ~/.zhi/plugins/
zhi validate  # uses the WASM plugin through normal workspace config
```

## Plugin Scaffolding

The `zhi plugin new` scaffolding command (from TODOS.md) would support
a `--format wasm` flag:

```bash
zhi plugin new --type config --name my-plugin --format wasm
```

Generates:
- `main.go` with PDK imports and interface stubs
- `go.mod` with the wasmpdk dependency
- `Makefile` with `GOOS=wasip1 GOARCH=wasm` build target
- `zhi-plugin.yaml` with `format: wasm` and capability stubs
- `.github/workflows/build.yml` for WASM builds + OCI publish

## Size Optimization

WASM binary sizes for Go plugins:

| Plugin | Go gc | TinyGo | Native (linux/amd64) |
|--------|-------|--------|---------------------|
| Minimal config | ~2.5 MB | ~200 KB | ~8 MB |
| Config + JSON | ~3 MB | ~400 KB | ~9 MB |
| Store + HTTP | ~4 MB | N/A* | ~12 MB |

*TinyGo may not support all packages needed for complex store plugins.

Optimization techniques:
- `go build -ldflags="-s -w"` strips debug info (~20% reduction)
- TinyGo with `-no-debug` for minimal builds
- wazero compiles and caches modules, so load time is amortized
