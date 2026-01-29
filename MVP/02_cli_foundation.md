# Step 2: CLI Foundation

## Overview

Build the CLI entry point and subcommands that expose core engine operations from the terminal. This step makes zhi usable as a command-line tool (without the TUI, which comes in Step 5).

## Relevant Existing Files

- `cmd/zhi/zhi.go` — empty `main()` function, build target for `make build`
- `internal/core/engine.go` — core engine from Step 1
- `internal/core/registry.go` — provider registry from Step 1
- `internal/core/workspace.go` — workspace config from Step 1
- `Makefile` — build commands (`make build` targets `cmd/zhi/`)
- `go.mod` — current dependencies (no CLI framework yet)

## Implementation Plan

### 2.1 CLI Framework Choice

Use `cobra` (github.com/spf13/cobra) for subcommand routing. It's the standard in the Go ecosystem (used by kubectl, Hugo, GitHub CLI) and provides help generation, flag parsing, and shell completion out of the box.

Add to `go.mod`:
```
github.com/spf13/cobra
```

### 2.2 Root Command (`internal/cli/root.go`)

The root command sets up shared state and subcommand registration.

**Components:**

- `rootCmd` — top-level cobra.Command with:
  - `Use: "zhi"`
  - `Short: "Security-first configuration management and provisioning"`
  - Persistent flags: `--workspace` (default: current directory), `--verbose`
- `Execute() error` — called from `main()`, runs the root command
- Pre-run hook: load workspace config, create default registry, build engine — store in command context

### 2.3 Entry Point Update (`cmd/zhi/zhi.go`)

Update the empty `main()` to call `cli.Execute()`.

```go
package main

import (
    "os"
    "github.com/MrWong99/zhi/internal/cli"
)

func main() {
    if err := cli.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### 2.4 `zhi init` Command (`internal/cli/init.go`)

Scaffold a new zhi workspace in the current (or specified) directory.

**Behavior:**

1. Create `zhi.yaml` with default config (structuredfile provider, json-store, empty transforms)
2. Create `./config/` directory with a starter configuration file (e.g., `app.yaml`)
3. Create `./templates/` directory with a sample export template
4. Create `./.zhi/store/` directory for local store data
5. Print a summary of created files

**Flags:**

- `--config-provider` (default: `structuredfile`) — which config provider to use
- `--store-provider` (default: `json-store`) — which store provider to use
- `--force` — overwrite existing `zhi.yaml` if present

### 2.5 `zhi list` Command (`internal/cli/list.go`)

List available information about the workspace.

**Subcommands:**

- `zhi list providers` — list all registered providers (built-in + external) grouped by type
- `zhi list trees` — list all stored tree IDs (calls `storePlugin.ListTrees()`)
- `zhi list paths` — list all config paths in the current tree (calls `configPlugin.List()`)

**Output format:** plain text table by default. Optional `--json` flag for machine-readable output.

### 2.6 `zhi validate` Command (`internal/cli/validate.go`)

Validate the current configuration tree and print results.

**Behavior:**

1. Load tree from config provider via engine
2. Run transforms (BeforeDisplay) if any
3. Call engine.Validate() for all paths
4. Print results grouped by severity (Blocking, Warning, Info)
5. Exit code 1 if any Blocking results, 0 otherwise

**Flags:**

- `--path` — validate only a specific path instead of all
- `--json` — output validation results as JSON

**Output example:**
```
BLOCKING  database/port    Port must be between 1024 and 65535
WARNING   app/log-level    Consider using "info" in production
INFO      app/name         Value is using default

Validation: 1 blocking, 1 warning, 1 info
```

### 2.7 `zhi get` Command (`internal/cli/get.go`)

Get a single value from the configuration tree.

**Behavior:**

1. Load tree from config provider
2. Run transforms (BeforeDisplay)
3. Print the value at the specified path

**Flags:**

- `--raw` — print just the value, no metadata
- `--json` — output as JSON including metadata

### 2.8 `zhi set` Command (`internal/cli/set.go`)

Set a single value in the configuration tree.

**Behavior:**

1. Call `engine.SetValue(ctx, path, value)`
2. Optionally validate after set (`--validate` flag)
3. Print confirmation or validation errors

### 2.9 Cleanup: Remove/Repurpose `internal/app/zhi/`

The existing `internal/app/zhi/zhi.go` is empty. Either:
- Remove it entirely if all logic lives in `internal/cli/` and `internal/core/`
- Or repurpose it as the application initialization logic called by the CLI

### 2.10 Tests

- `internal/cli/init_test.go` — test workspace scaffolding creates expected files
- `internal/cli/list_test.go` — test output format with mock providers
- `internal/cli/validate_test.go` — test output and exit codes for different validation scenarios
- `internal/cli/get_test.go` — test value retrieval
- `internal/cli/set_test.go` — test value setting
- Integration test: `zhi init` → `zhi list paths` → `zhi validate` in a temp directory

## Verification Criteria

1. `go build ./cmd/zhi/` produces a working binary
2. `zhi --help` shows all subcommands with descriptions
3. `zhi init` creates a valid workspace directory structure
4. `zhi list providers` shows `structuredfile` as a built-in config provider
5. `zhi validate` loads config from structuredfile and prints validation results
6. `zhi get <path>` retrieves and prints a config value
7. `zhi set <path> <value>` sets a value successfully
8. Exit codes: 0 for success, 1 for validation failure or errors
9. `make build` still works (binary output to `bin/`)
10. All tests pass with `go test -race ./internal/cli/...`
