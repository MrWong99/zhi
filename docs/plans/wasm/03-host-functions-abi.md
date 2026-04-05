# 03 — Host Functions ABI

This document specifies the Application Binary Interface (ABI) between the
zhi host process and WASM plugin modules. The ABI defines:

1. How the host calls into the WASM module (exported functions)
2. How the WASM module calls back to the host (imported host functions)
3. How complex data (strings, JSON, errors) is passed across the boundary

## Design Principles

1. **JSON on the wire** — All structured data crosses the WASM boundary as
   JSON-encoded bytes, matching the existing gRPC layer's use of
   JSON-encoded values. This keeps the ABI language-agnostic.

2. **One export per method** — Each method in the Go plugin interface maps
   to one WASM export. This avoids multiplexing complexity and makes the
   ABI self-documenting.

3. **Error protocol** — Functions return a status code. On error, the error
   message is written to a shared buffer readable by the host.

4. **Host-managed memory** — The host allocates memory in the WASM module
   for passing data in. The WASM module exports `malloc` and `free`
   functions for this purpose (standard with `GOOS=wasip1` and TinyGo).

## Memory Protocol

WASM linear memory is a flat byte array. Passing complex data (strings,
JSON objects) requires a shared convention for allocation and deallocation.

### Required module exports

Every WASM plugin must export:

```
malloc(size: i32) -> ptr: i32
free(ptr: i32, size: i32)
```

These are automatically provided by Go's `wasip1` target and TinyGo. For
Rust plugins, `#[no_mangle]` wrappers around the allocator are needed.

### Data passing convention

**Host → Plugin (input):**
1. Host serializes input to JSON bytes.
2. Host calls `malloc(len)` in the WASM module to get a pointer.
3. Host writes JSON bytes into WASM memory at that pointer.
4. Host calls the exported function with `(ptr, len)` parameters.
5. Plugin reads from its own memory, deserializes JSON.

**Plugin → Host (output):**
1. Plugin serializes output to JSON bytes.
2. Plugin writes to a preallocated output buffer or calls a host-provided
   `host_set_output(ptr, len)` function.
3. Host reads the bytes from WASM memory after the function returns.

**Error reporting:**
1. Each exported function returns an `i32` status: `0` = success, `1` = error.
2. On error, the plugin calls `host_set_error(ptr, len)` with a UTF-8
   error message before returning.

### Shared buffer pattern

To avoid complex multi-return conventions, the ABI uses a "shared output"
pattern:

```
host module "zhi":
    host_set_output(ptr: i32, len: i32)     // plugin writes result here
    host_set_error(ptr: i32, len: i32)      // plugin writes error here
```

The host reads the last-set output/error after the exported function
returns. This is a common pattern in WASM plugin systems (used by Extism,
knqyf263/go-plugin, and others).

---

## Config Plugin ABI

The `config.Plugin` interface has 4 methods:

### Exports (plugin provides)

```
zhi_config_list()                           -> i32 (status)
zhi_config_get(path_ptr: i32, path_len: i32) -> i32 (status)
zhi_config_set(req_ptr: i32, req_len: i32)   -> i32 (status)
zhi_config_validate(req_ptr: i32, req_len: i32) -> i32 (status)
```

### Imports (host provides)

```
module "zhi":
    host_set_output(ptr: i32, len: i32)
    host_set_error(ptr: i32, len: i32)
    host_log(level: i32, ptr: i32, len: i32)  // optional: structured logging
```

### Data formats

**`zhi_config_list`:**
- Input: none
- Output (via `host_set_output`): `["path/one", "path/two", ...]`

**`zhi_config_get`:**
- Input: path as raw UTF-8 string (not JSON)
- Output: `{"found": true, "value": <json>, "metadata": <json>}` or
  `{"found": false}`

**`zhi_config_set`:**
- Input:
  ```json
  {
    "path": "database/host",
    "value": "localhost",
    "metadata": {"source": "user"}
  }
  ```
- Output: none (status 0 = success)

**`zhi_config_validate`:**
- Input:
  ```json
  {
    "path": "database/host",
    "tree": {
      "database/host": {"value": "localhost", "metadata": {}},
      "database/port": {"value": 5432, "metadata": {}}
    }
  }
  ```
  The `tree` field is a flat map providing the full `TreeReader` view.
- Output:
  ```json
  [
    {"path": "database/host", "severity": "blocking", "message": "host required"}
  ]
  ```

### Host-side adapter

The host creates a Go struct that implements `config.Plugin` by calling
the WASM exports:

```go
type wasmConfigAdapter struct {
    module wazero.Module
    // ... wazero function references
}

func (a *wasmConfigAdapter) List(ctx context.Context) ([]string, error) {
    status := a.callExport(ctx, "zhi_config_list", nil)
    if status != 0 {
        return nil, a.lastError()
    }
    var paths []string
    json.Unmarshal(a.lastOutput(), &paths)
    return paths, nil
}

func (a *wasmConfigAdapter) Get(ctx context.Context, path string) (Value, bool, error) {
    status := a.callExport(ctx, "zhi_config_get", []byte(path))
    // ... deserialize output
}
```

---

## Transform Plugin ABI

The `transform.Plugin` interface has 3 methods:

### Exports (plugin provides)

```
zhi_transform_before_display(tree_ptr: i32, tree_len: i32) -> i32
zhi_transform_after_save(tree_ptr: i32, tree_len: i32)     -> i32
zhi_transform_validate_policy()                              -> i32
```

### Data formats

**`zhi_transform_before_display` / `zhi_transform_after_save`:**
- Input: The full tree as a JSON object:
  ```json
  {
    "database/host": {"value": "localhost", "metadata": {}},
    "database/port": {"value": 5432, "metadata": {}}
  }
  ```
- Output: The mutated tree in the same format. The host replaces the
  tree contents with the returned value.

Transform plugins receive and return the full tree because they need
mutable access. This is consistent with the gRPC layer, which also
serializes the entire tree.

**`zhi_transform_validate_policy`:**
- Input: none
- Output: `{"policy": "before"}` | `{"policy": "after"}` | `{"policy": "both"}`

---

## Store Plugin ABI

The `store.Plugin` interface has 23 methods. Each maps to a WASM export:

### Exports (plugin provides)

```
// Capabilities
zhi_store_capabilities()                                    -> i32

// Authentication
zhi_store_auth_methods()                                    -> i32
zhi_store_login(req_ptr: i32, req_len: i32)                -> i32
zhi_store_login_interactive(req_ptr: i32, req_len: i32)    -> i32
zhi_store_login_interactive_cb(req_ptr: i32, req_len: i32) -> i32

// Tree management
zhi_store_list_trees()                                      -> i32
zhi_store_delete_tree(req_ptr: i32, req_len: i32)          -> i32

// Value operations
zhi_store_get_values(req_ptr: i32, req_len: i32)           -> i32
zhi_store_put_values(req_ptr: i32, req_len: i32)           -> i32
zhi_store_delete_values(req_ptr: i32, req_len: i32)        -> i32

// Tree-level versioning
zhi_store_list_tree_versions(req_ptr: i32, req_len: i32)   -> i32
zhi_store_get_tree_version(req_ptr: i32, req_len: i32)     -> i32
zhi_store_rollback_tree(req_ptr: i32, req_len: i32)        -> i32
zhi_store_delete_tree_version(req_ptr: i32, req_len: i32)  -> i32

// Value-level versioning
zhi_store_list_value_versions(req_ptr: i32, req_len: i32)  -> i32
zhi_store_get_value_version(req_ptr: i32, req_len: i32)    -> i32
zhi_store_rollback_value(req_ptr: i32, req_len: i32)       -> i32
zhi_store_delete_value_version(req_ptr: i32, req_len: i32) -> i32

// Encryption
zhi_store_init_encryption(req_ptr: i32, req_len: i32)      -> i32
zhi_store_rotate_encryption(req_ptr: i32, req_len: i32)    -> i32

// Access control
zhi_store_grant_access(req_ptr: i32, req_len: i32)         -> i32
zhi_store_revoke_access(req_ptr: i32, req_len: i32)        -> i32
zhi_store_list_access(req_ptr: i32, req_len: i32)          -> i32
```

### Host functions for store plugins

Store plugins need to interact with external systems (filesystems,
network services like Vault). Since WASM has no direct access, the host
provides capability-gated functions:

```
module "zhi_store":
    // Filesystem (if fs capability granted)
    host_fs_read(path_ptr: i32, path_len: i32)  -> i32
    host_fs_write(req_ptr: i32, req_len: i32)   -> i32
    host_fs_list(path_ptr: i32, path_len: i32)  -> i32
    host_fs_delete(path_ptr: i32, path_len: i32) -> i32
    host_fs_mkdir(path_ptr: i32, path_len: i32)  -> i32
    host_fs_stat(path_ptr: i32, path_len: i32)   -> i32

    // HTTP (if network capability granted)
    host_http_request(req_ptr: i32, req_len: i32) -> i32
```

**Filesystem host functions** are scoped to allowed directories (see
[04-security-model.md](04-security-model.md)). Paths outside granted
directories return an error.

**HTTP host function** input:
```json
{
  "method": "GET",
  "url": "https://vault.example.com/v1/secret/data/myapp",
  "headers": {"X-Vault-Token": "s.xxxxx"},
  "body": null
}
```
Output:
```json
{
  "status": 200,
  "headers": {"Content-Type": "application/json"},
  "body": "{...}"
}
```

HTTP requests are subject to URL allowlisting from the capability config.

### Example: in-memory store (no capabilities needed)

A simple in-memory store plugin needs no filesystem or network access.
It stores all data in WASM linear memory. This is the simplest store
plugin case and requires zero host function calls beyond the standard ABI.

### Example: file-based store (filesystem capability)

A JSON-file store plugin needs `host_fs_read` and `host_fs_write` to
persist data. The workspace grants:

```yaml
store:
  provider: json-file
  wasm:
    capabilities:
      fs:
        - path: ./data
          mode: readwrite
```

### Example: Vault store (network capability)

A Vault-backed store needs `host_http_request` to talk to the Vault API.
The workspace grants:

```yaml
store:
  provider: vault
  wasm:
    capabilities:
      net:
        - host: vault.example.com
          port: 8200
          scheme: https
```

---

## UI Plugin ABI

UI plugins are the most complex because they use bidirectional
communication: the host calls `Run()` on the plugin, and the plugin calls
back into the host via the `Controller` interface.

### Exports (plugin provides)

```
zhi_ui_run()             -> i32
zhi_ui_capabilities()    -> i32
```

### Imports (host provides — Controller)

The `Controller` interface is exposed as host functions that the plugin
can call during `zhi_ui_run()`:

```
module "zhi_ui":
    // Core
    host_ctrl_load_tree()                                    -> i32
    host_ctrl_set_value(req_ptr: i32, req_len: i32)         -> i32
    host_ctrl_validate()                                     -> i32
    host_ctrl_save_tree()                                    -> i32

    // Export/Apply
    host_ctrl_export_templates()                              -> i32
    host_ctrl_export(req_ptr: i32, req_len: i32)             -> i32
    host_ctrl_apply(req_ptr: i32, req_len: i32)              -> i32

    // Components
    host_ctrl_list_components()                               -> i32
    host_ctrl_enable_component(name_ptr: i32, name_len: i32) -> i32
    host_ctrl_disable_component(name_ptr: i32, name_len: i32)-> i32
    host_ctrl_workspace_name()                                -> i32

    // Marketplace
    host_ctrl_search_marketplace(req_ptr: i32, req_len: i32) -> i32
    host_ctrl_marketplace_detail(req_ptr: i32, req_len: i32) -> i32
    host_ctrl_install_plugin(ref_ptr: i32, ref_len: i32)     -> i32
    host_ctrl_uninstall_plugin(req_ptr: i32, req_len: i32)   -> i32
    host_ctrl_list_installed()                                 -> i32
    host_ctrl_check_updates()                                  -> i32
    host_ctrl_update_plugin(req_ptr: i32, req_len: i32)       -> i32
    host_ctrl_rate_plugin(req_ptr: i32, req_len: i32)         -> i32

    // Store auth
    host_ctrl_store_auth_methods()                             -> i32
    host_ctrl_store_login(req_ptr: i32, req_len: i32)         -> i32
    host_ctrl_store_login_interactive(req_ptr: i32, req_len: i32)    -> i32
    host_ctrl_store_login_interactive_cb(req_ptr: i32, req_len: i32) -> i32
    host_ctrl_store_auth_status()                              -> i32
    host_ctrl_store_logout()                                   -> i32
```

All host functions use the same output/error convention: the result is
read via `host_set_output` after the function returns status `0`.

### Apply streaming

The `Apply` method in the Go Controller interface takes a callback for
streaming events. In the WASM ABI, streaming is modeled differently:

**Option A: Polled iteration**
```
host_ctrl_apply_start(req_ptr: i32, req_len: i32) -> i32  // start apply
host_ctrl_apply_next()                              -> i32  // get next event
host_ctrl_apply_result()                            -> i32  // get final result
```

The plugin calls `apply_start`, then loops calling `apply_next` until it
returns a status indicating completion, then calls `apply_result`.

**Option B: Callback host function**

The plugin exports `zhi_ui_apply_event(event_ptr: i32, event_len: i32)`
which the host calls for each event during apply execution.

Option A is simpler and avoids re-entrant calls into the WASM module.
**Recommended: Option A.**

### TTY limitation

WASM plugins have no terminal access. `Capabilities()` for WASM UI
plugins always reports `RequiresTTY: false`. This means WASM UI plugins
are limited to:

- HTTP-based UIs (like `zhi-ui-http`)
- Headless/API UIs
- Any UI that doesn't need direct terminal I/O

If a WASM UI plugin needs to listen on a network port (for HTTP), the
host provides:

```
module "zhi_ui":
    host_http_listen(req_ptr: i32, req_len: i32) -> i32
    host_http_accept() -> i32
    host_http_respond(req_ptr: i32, req_len: i32) -> i32
```

This is a stretch goal and not required for the initial implementation.

---

## ABI Versioning

The host module name includes a version suffix:

```
module "zhi_v1":
    host_set_output(...)
    host_set_error(...)
    host_log(...)
```

WASM plugins import from `zhi_v1`. Future incompatible ABI changes
increment to `zhi_v2`, etc. The host can serve multiple ABI versions
simultaneously to maintain backward compatibility.

The plugin module must also export a version function:

```
zhi_abi_version() -> i32
```

Returns `1` for the initial ABI. The host checks this before calling any
other export and rejects modules with unsupported ABI versions.

---

## Summary: Export/Import Table

### Config Plugin

| Direction | Function | Purpose |
|-----------|----------|---------|
| Export | `zhi_abi_version` | ABI version check |
| Export | `malloc` | Memory allocation |
| Export | `free` | Memory deallocation |
| Export | `zhi_config_list` | List paths |
| Export | `zhi_config_get` | Get value |
| Export | `zhi_config_set` | Set value |
| Export | `zhi_config_validate` | Validate value |
| Import | `zhi_v1.host_set_output` | Return results |
| Import | `zhi_v1.host_set_error` | Return errors |
| Import | `zhi_v1.host_log` | Logging |

### Transform Plugin

| Direction | Function | Purpose |
|-----------|----------|---------|
| Export | `zhi_abi_version` | ABI version check |
| Export | `malloc` | Memory allocation |
| Export | `free` | Memory deallocation |
| Export | `zhi_transform_before_display` | Transform tree for display |
| Export | `zhi_transform_after_save` | Transform tree after save |
| Export | `zhi_transform_validate_policy` | Report validation policy |
| Import | `zhi_v1.host_set_output` | Return results |
| Import | `zhi_v1.host_set_error` | Return errors |
| Import | `zhi_v1.host_log` | Logging |

### Store Plugin

| Direction | Function | Purpose |
|-----------|----------|---------|
| Export | `zhi_abi_version` | ABI version check |
| Export | `malloc` / `free` | Memory management |
| Export | `zhi_store_*` (23 functions) | Store operations |
| Import | `zhi_v1.host_set_output` | Return results |
| Import | `zhi_v1.host_set_error` | Return errors |
| Import | `zhi_v1.host_log` | Logging |
| Import | `zhi_v1.host_fs_*` (6 functions) | Filesystem (gated) |
| Import | `zhi_v1.host_http_request` | HTTP client (gated) |

### UI Plugin

| Direction | Function | Purpose |
|-----------|----------|---------|
| Export | `zhi_abi_version` | ABI version check |
| Export | `malloc` / `free` | Memory management |
| Export | `zhi_ui_run` | Start UI |
| Export | `zhi_ui_capabilities` | Report capabilities |
| Import | `zhi_v1.host_set_output` | Return results |
| Import | `zhi_v1.host_set_error` | Return errors |
| Import | `zhi_v1.host_log` | Logging |
| Import | `zhi_v1.host_ctrl_*` (25 functions) | Controller callbacks |
