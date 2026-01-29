# Step 4: Apply System

## Overview

Build the apply system that runs a user-configured external command after exporting configuration. This is how zhi triggers provisioning tools (Docker Compose, kubectl, Ansible, etc.) without embedding their SDKs.

## Relevant Existing Files

- `internal/core/engine.go` — core engine (will be extended with apply method)
- `internal/core/export.go` — export system from Step 3 (apply may trigger export first)
- `internal/core/workspace.go` — workspace config, defines the `apply` section
- `internal/cli/root.go` — CLI root from Step 2

## Implementation Plan

### 4.1 Apply Runner (`internal/core/apply.go`)

The apply runner executes an external command as a subprocess and streams its output.

**Components:**

- `ApplyConfig` struct — command string, working directory, environment overrides, pre-export flag
- `ApplyResult` struct — exit code, duration, error (if any)
- `ApplyOutput` — channel-based output stream:
  ```go
  type ApplyOutput struct {
      Line   string
      Stream string // "stdout" or "stderr"
      Time   time.Time
  }
  ```
- `Apply(ctx, config ApplyConfig, output chan<- ApplyOutput) (*ApplyResult, error)` — main function

**Execution flow:**

1. If `config.PreExport` is true, run `ExportAll()` first to generate config files
2. Parse the command string using `shlex` splitting (or `sh -c` for shell features)
3. Create `exec.CommandContext` with the parsed command
4. Set working directory from config (default: workspace root)
5. Set environment: inherit current env + workspace-defined overrides + `ZHI_WORKSPACE` pointing to workspace root
6. Pipe stdout and stderr through separate scanners
7. Send each line as an `ApplyOutput` to the output channel
8. Wait for command completion
9. Return `ApplyResult` with exit code

**Context cancellation:** The command respects `ctx` cancellation. If the user cancels (e.g., Ctrl+C in TUI), the subprocess receives SIGTERM, then SIGKILL after a grace period.

### 4.2 Workspace Apply Configuration

Expand the `apply` section in `zhi.yaml`:

```yaml
apply:
  command: "docker compose up -d"
  workdir: "."                    # relative to workspace root
  pre-export: true                # run export before apply
  env:
    COMPOSE_PROJECT_NAME: "myapp"
  timeout: 300                    # seconds, 0 = no timeout
```

**Optional: named apply targets** for workspaces that need multiple commands:

```yaml
apply:
  default:
    command: "docker compose up -d"
    pre-export: true
  destroy:
    command: "docker compose down -v"
  restart:
    command: "docker compose restart"
```

### 4.3 CLI: `zhi apply` Command (`internal/cli/apply.go`)

Expose the apply system via CLI.

**Usage:**

```
zhi apply [target]
```

**Behavior:**

1. Load workspace config
2. If `target` is specified, use the named apply config; otherwise use `default`
3. If `pre-export` is true, run export first and print exported files
4. Run the command
5. Stream stdout/stderr to the terminal in real-time
6. Print exit code and duration on completion
7. Exit with the command's exit code

**Flags:**

- `--dry-run` — print the command that would be executed without running it
- `--no-export` — skip the pre-export step even if configured
- `--timeout` — override the configured timeout
- `--env KEY=VALUE` — add/override environment variables (repeatable)

**Output example:**
```
$ zhi apply
Exporting: docker-compose.override.yml ... done
Exporting: .env ... done
Running: docker compose up -d
[+] Running 3/3
 ✔ Network myapp_default  Created
 ✔ Container myapp-db-1   Started
 ✔ Container myapp-app-1  Started

Apply completed (exit code 0)
```

### 4.4 Apply with Streaming Interface

The `Apply` function uses a channel-based output interface so that both the CLI and TUI (Step 5) can consume output differently:

- **CLI consumer** (`internal/cli/apply.go`): reads from channel, writes each line to `os.Stdout`/`os.Stderr` directly
- **TUI consumer** (`internal/ui/apply_view.go`, Step 5): reads from channel, appends to a viewport buffer for scrollable display

This separation means the apply runner doesn't need to know about the presentation layer.

### 4.5 Error Handling

- **Command not found**: return a clear error with the resolved command path
- **Permission denied**: check executable bit before running, suggest `chmod +x`
- **Timeout**: send SIGTERM, wait 5 seconds, send SIGKILL if still running
- **Non-zero exit**: not treated as a Go error — return `ApplyResult` with the exit code, let the caller decide how to handle it
- **No apply config**: return an error suggesting the user add an `apply` section to `zhi.yaml`

### 4.6 Tests

- `internal/core/apply_test.go`:
  - Test successful command execution (e.g., `echo hello`)
  - Test non-zero exit code propagation
  - Test context cancellation terminates the subprocess
  - Test stdout and stderr are separated in output
  - Test environment variable passing
  - Test working directory setting
  - Test timeout enforcement
- `internal/cli/apply_test.go`:
  - Test `--dry-run` prints command without executing
  - Test `--no-export` flag
  - Test exit code propagation to CLI exit code

## Verification Criteria

1. `Apply()` runs an external command and streams stdout/stderr to a channel
2. Each output line includes the stream source (`stdout` or `stderr`) and timestamp
3. `ApplyResult` contains the correct exit code
4. Context cancellation sends SIGTERM to the subprocess
5. Timeout triggers SIGTERM → SIGKILL sequence
6. `zhi apply` streams command output to the terminal in real-time
7. `zhi apply --dry-run` prints the command without executing
8. Pre-export runs before the command when configured
9. Environment variables from workspace config and `--env` flags are passed to the subprocess
10. All tests pass with `go test -race ./internal/core/... ./internal/cli/...`
