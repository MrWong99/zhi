# 04 — Security Model

WASM plugins are designed to be a **more secure alternative** to native
gRPC plugins. This document defines the capability-based sandboxing model
that enforces the principle of least privilege.

## Threat Model

### Native plugins (current)

A native gRPC plugin is a subprocess with the same OS-level privileges as
the user running zhi. It can:

- Read/write any file the user can access
- Open network connections to any host
- Read environment variables (including secrets)
- Execute other programs
- Access hardware devices

The only isolation is process separation (no shared memory with the host).
Binary auditing (`launch/audit.go`) provides integrity checks but does not
restrict runtime behavior.

### WASM plugins (proposed)

A WASM plugin runs **in-process** inside the wazero sandbox. By default,
it can do **nothing** beyond pure computation:

- No filesystem access
- No network access
- No environment variables
- No process spawning
- No system clock (unless explicitly granted)
- No access to host memory outside its own linear memory

Every capability must be explicitly granted by the host.

## Capability Model

Capabilities are declared in two places:

1. **`zhi-plugin.yaml`** — The plugin manifest declares what capabilities
   the plugin *requires*. This is informational and used for display/audit.
2. **`zhi.yaml`** — The workspace configuration *grants* capabilities to
   specific plugins. Only granted capabilities are enforced.

### Capability types

```yaml
wasm:
  capabilities:
    # Filesystem access
    fs:
      - path: ./data
        mode: read           # read | readwrite
      - path: /etc/ssl/certs
        mode: read

    # Network access
    net:
      - host: vault.example.com
        port: 8200
        scheme: https
      - host: "*.internal.company.com"
        port: 443
        scheme: https

    # Environment variables
    env:
      - VAULT_ADDR
      - VAULT_TOKEN

    # System clock
    clock: true

    # Random number generation
    random: true

    # Stdin/stdout (for debugging)
    stdio: false

    # Execution limits
    limits:
      memory_mb: 64          # max WASM linear memory (default: 32 MB)
      fuel: 1000000000       # max instructions (0 = unlimited)
      timeout_seconds: 30    # max wall-clock time per call (0 = unlimited)
```

### Default capabilities

Plugins receive these capabilities by default (no explicit grant needed):

| Capability | Default | Rationale |
|------------|---------|-----------|
| `clock` | `true` | Many operations need timestamps |
| `random` | `true` | Needed for UUIDs, nonces, etc. |
| `stdio` | `false` | Disabled by default for security |
| `fs` | `[]` (none) | Must be explicitly granted |
| `net` | `[]` (none) | Must be explicitly granted |
| `env` | `[]` (none) | Must be explicitly granted |
| `limits.memory_mb` | `32` | Prevents runaway memory use |
| `limits.fuel` | `0` (unlimited) | Can be set for untrusted plugins |
| `limits.timeout_seconds` | `30` | Per-method-call timeout |

### Capability enforcement

Each capability maps to a concrete wazero configuration:

**Filesystem:**
```go
// If fs capabilities granted, configure WASI with scoped directories.
fsConfig := wazero.NewFSConfig()
for _, grant := range caps.FS {
    absPath := filepath.Join(workspaceDir, grant.Path)
    switch grant.Mode {
    case "read":
        fsConfig = fsConfig.WithReadOnlyDirMount(absPath, grant.Path)
    case "readwrite":
        fsConfig = fsConfig.WithDirMount(absPath, grant.Path)
    }
}
moduleConfig = moduleConfig.WithFSConfig(fsConfig)
```

Additionally, custom `host_fs_*` functions enforce path validation:

```go
func hostFSRead(ctx context.Context, mod api.Module, pathPtr, pathLen uint32) uint32 {
    path := readString(mod, pathPtr, pathLen)

    // Resolve to absolute, check against allowed directories.
    absPath, err := securepath.Resolve(path, allowedDirs)
    if err != nil {
        setError(mod, fmt.Sprintf("fs access denied: %s", path))
        return 1
    }

    data, err := os.ReadFile(absPath)
    // ... write to output buffer ...
}
```

**Network:**
```go
func hostHTTPRequest(ctx context.Context, mod api.Module, reqPtr, reqLen uint32) uint32 {
    var req httpRequest
    json.Unmarshal(readBytes(mod, reqPtr, reqLen), &req)

    // Validate URL against allowed hosts.
    if !isAllowedURL(req.URL, allowedHosts) {
        setError(mod, fmt.Sprintf("network access denied: %s", req.URL))
        return 1
    }

    // Execute request with timeout from context.
    resp, err := httpClient.Do(req.toHTTP().WithContext(ctx))
    // ... write response to output buffer ...
}
```

**Environment:**
```go
// Only pass allowed env vars to the WASM module.
for _, name := range caps.Env {
    if val, ok := os.LookupEnv(name); ok {
        moduleConfig = moduleConfig.WithEnv(name, val)
    }
}
```

**Memory limits:**
```go
// wazero enforces memory limits at the runtime level.
moduleConfig = moduleConfig.WithMemoryLimitPages(caps.Limits.MemoryMB * 16)
// 1 WASM page = 64 KiB, so 32 MB = 512 pages
```

**Fuel (instruction counting):**
```go
// Fuel limits the number of instructions executed.
if caps.Limits.Fuel > 0 {
    // wazero doesn't have built-in fuel metering.
    // Use context deadline + periodic yield points instead.
}
```

Note: wazero does not natively support Wasmtime-style fuel metering.
Execution limits are enforced primarily via Go context deadlines
(wall-clock timeouts). If fine-grained instruction counting is needed in
the future, a metering transform pass on the WASM module can be applied
at load time using a tool like `wasm-metering`.

**Timeouts:**
```go
// Each method call gets a context with deadline.
ctx, cancel := context.WithTimeout(ctx, time.Duration(caps.Limits.TimeoutSeconds)*time.Second)
defer cancel()
```

## Comparison: Native vs WASM Security

| Aspect | Native Plugin | WASM Plugin |
|--------|---------------|-------------|
| Execution isolation | Separate process | In-process sandbox |
| Memory isolation | OS process boundary | WASM linear memory boundary |
| Filesystem access | Full (user-level) | None by default; scoped grants |
| Network access | Full | None by default; host-allowlisted |
| Environment variables | Inherited (or isolated) | None by default; explicit list |
| Process spawning | Possible | Impossible |
| System calls | All | Only via host functions |
| Crash behavior | Subprocess dies; host unaffected | wazero catches trap; host unaffected |
| Binary integrity | Audit at launch time | Inherent (WASM is validated at load) |
| Code review | Requires decompiling native code | WASM text format (`.wat`) is readable |

## Plugin Manifest Capabilities

The `zhi-plugin.yaml` manifest declares required capabilities:

```yaml
# zhi-plugin.yaml
name: vault-store
type: store
format: wasm
requires:
  capabilities:
    net:
      - description: "Vault API access"
        host: "*.vault.example.com"
        port: 8200
    env:
      - VAULT_ADDR
      - VAULT_TOKEN
    clock: true
```

### Install-time verification

When `zhi plugin install` installs a WASM plugin, it displays the
required capabilities and asks for confirmation:

```
Installing vault-store (WASM plugin)...

This plugin requires the following capabilities:
  Network: *.vault.example.com:8200
  Environment: VAULT_ADDR, VAULT_TOKEN
  Clock: yes

These capabilities must be granted in your workspace zhi.yaml.
Continue? [y/N]
```

### Runtime capability mismatch

If a WASM plugin calls a host function for a capability that wasn't
granted, the host function returns an error immediately. The plugin
receives an error string; it does not crash or panic. This allows plugins
to degrade gracefully.

```go
// Example: plugin calls host_fs_read but no fs capability was granted.
// Result: error "filesystem access not granted for this plugin"
```

## Audit Trail

WASM plugin capability usage is logged:

```
INFO  wasm plugin "vault-store" loaded
INFO  capabilities granted: net=[vault.example.com:8200], env=[VAULT_ADDR, VAULT_TOKEN]
DEBUG wasm plugin "vault-store" called host_http_request -> vault.example.com:8200/v1/sys/health
DEBUG wasm plugin "vault-store" called host_http_request -> vault.example.com:8200/v1/secret/data/myapp
```

For sensitive operations, the log can record:
- Which host functions were called and how often
- Which URLs were accessed
- Which filesystem paths were read/written
- Execution time per call

This gives administrators visibility into what WASM plugins actually do
at runtime, which is superior to native plugins where syscall tracing
requires external tools like `strace`.

## Future: Code Signing and Verification

WASM modules are deterministic bytecode. This enables:

1. **Content-addressable integrity** — The SHA-256 hash of a `.wasm` file
   uniquely identifies its behavior. No need for binary auditing heuristics.

2. **Reproducible builds** — Given the same source and toolchain, the
   `.wasm` output is bitwise identical. This allows third-party verification.

3. **Cosign integration** — WASM artifacts published to OCI registries can
   be signed with Sigstore/cosign, using the same keyless signing already
   in the plugin release pipeline.

4. **Allowlisted hashes** — A workspace could pin to specific `.wasm`
   hashes in `zhi.lock`:
   ```yaml
   plugins:
     - name: vault-store
       type: store
       format: wasm
       digest: sha256:abc123...
   ```
