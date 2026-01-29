# Step 7: Security and Polish

## Overview

Harden the MVP with input validation, secure defaults, error handling improvements, and user-facing documentation. This step ensures zhi is production-quality before release.

## Relevant Existing Files

- `pkg/zhiplugin/config/config.go` — `ValidatePath()` already validates path segments
- `pkg/providers/config/structuredfile/validate.go` — validation using Yaegi interpreter
- `internal/core/apply.go` — apply runner (security-sensitive: runs external commands)
- `internal/core/export.go` — export system (writes files to disk)
- `internal/core/discovery.go` — plugin discovery (executes external binaries)
- `internal/core/workspace.go` — parses user-provided YAML config
- `SECURITY.md` — existing security policy
- `CONTRIBUTING.md` — existing contribution guide
- `README.md` — existing README

## Implementation Plan

### 7.1 Input Validation and Sanitization

**Path validation** is already handled by `config.ValidatePath()` in `pkg/zhiplugin/config/config.go`. Verify that all code paths use it:

- `engine.SetValue()` must validate the path before calling the config plugin
- CLI `zhi set` must validate the path before passing to the engine
- TUI value editor must validate the path before committing

**Value sanitization:**

- Values are `any` type — no HTML/SQL escaping needed since zhi doesn't serve web content or use SQL
- Template output: `text/template` (not `html/template`) is correct since we generate config files, not HTML
- Ensure exported templates use `{{ .Get "path" }}` which returns strings, not raw template directives

**Workspace config validation:**

- Validate `zhi.yaml` strictly: reject unknown keys, validate types, check referenced files exist
- Command strings in `apply.command`: document that these are passed to `sh -c` and users are responsible for their content — zhi does not sanitize shell commands
- Plugin directory paths: resolve to absolute paths, reject paths containing `..` traversal

### 7.2 Plugin Execution Security

**External plugin binaries:**

- Verify file permissions before launching: warn if the plugin binary is world-writable
- Log the full path and SHA-256 hash of each launched plugin (for audit)
- Do not pass sensitive environment variables to plugins by default — whitelist approach
- The hashicorp/go-plugin handshake provides basic identity verification (magic cookie)

**Process isolation:**

- External plugins already run as separate processes (hashicorp/go-plugin)
- Set timeouts for plugin operations: if a plugin call doesn't return within a configurable timeout (default 30s), kill the process
- Handle plugin panics gracefully: the gRPC connection will drop, detect this and report a clear error

### 7.3 File System Security

**Export file writing:**

- Validate output paths: reject absolute paths that write outside the workspace (unless explicitly configured)
- Create parent directories with `0755` permissions
- Write output files with `0644` permissions
- Use atomic writes (write to temp file, then rename) to avoid partial writes
- Don't follow symlinks when writing output files — resolve the real path first

**Workspace directory:**

- `.zhi/` directory should be created with `0700` permissions (user-only access)
- Store data files with `0600` permissions
- Add `.zhi/` to `.gitignore` recommendations in `zhi init`

### 7.4 Error Handling Audit

Review all error paths and ensure:

- Errors include context about what was being done (e.g., "loading config from structuredfile: open config/app.yaml: no such file")
- No stack traces in user-facing output — use `fmt.Errorf` with `%w` wrapping
- Plugin errors include the plugin name and type
- CLI errors print to stderr, not stdout
- TUI errors display in a status bar, not crash the application
- Apply command failures show the command that was run and its exit code

### 7.5 Graceful Shutdown

- CLI: handle `SIGINT` and `SIGTERM` — cancel context, clean up plugin processes, exit
- TUI: `Ctrl+C` and `q` trigger clean shutdown via Bubbletea
- Apply: running commands are terminated with SIGTERM → SIGKILL on shutdown
- Engine: `Close()` is always called, even on panic (use `defer`)

### 7.6 Logging

Add structured logging for debugging (not shown to users by default):

- Use `log/slog` (standard library, Go 1.21+)
- Default level: `WARN` (silent for normal operation)
- `--verbose` flag sets level to `DEBUG`
- Log key events: workspace loaded, provider resolved, plugin launched, export written, apply started/finished
- Logs go to stderr, not stdout (so `zhi export --format json` can pipe cleanly)

### 7.7 User-Facing Documentation Updates

**README.md updates:**

- Update "Getting Started" section with actual usage instructions
- Add quick start example: `zhi init` → `zhi edit` → `zhi export`
- Document workspace config format
- Document plugin discovery and naming conventions

**New documentation (only if explicitly needed, prefer README sections):**

- In-binary help text: ensure all `cobra.Command` instances have useful `Long` descriptions and `Example` fields
- `zhi init` should print a "what's next" message after scaffolding

### 7.8 Configuration Defaults and Conventions

Establish sensible defaults:

- Default config provider: `structuredfile`
- Default store: none (user must configure)
- Default plugin directory: `~/.zhi/plugins/`
- Default export output: stdout
- Default apply timeout: 300 seconds
- Default workspace file: `zhi.yaml` (also accept `zhi.json`)

### 7.9 Edge Case Handling

- **Empty tree**: `zhi edit` shows an empty state message, not a crash
- **No workspace**: `zhi edit` without `zhi.yaml` suggests running `zhi init`
- **No store configured**: `s` (save) in TUI shows a message that no store is configured
- **No apply configured**: `a` (apply) in TUI shows a message that no apply command is configured
- **Plugin not found**: clear error message listing available providers
- **Permission denied**: suggest checking file permissions
- **Workspace in read-only directory**: export and apply may fail — detect and report early

### 7.10 Tests

- `internal/core/security_test.go`:
  - Test path traversal rejection in export output paths
  - Test workspace config validation rejects unknown keys
  - Test file permissions on created directories and files
- `internal/core/engine_test.go` (extend):
  - Test error wrapping provides context
  - Test graceful handling of plugin crashes
  - Test timeout enforcement on plugin calls
- Integration tests in `test/`:
  - Full workflow: `init` → `set` → `validate` → `export` → `apply`
  - Test with example plugins as external plugins

## Verification Criteria

1. All file writes use correct permissions (`0644` for files, `0755` for directories, `0700` for `.zhi/`)
2. Path traversal in export output paths is rejected
3. Plugin binaries are logged with their SHA-256 hash at launch time
4. World-writable plugin binaries produce a warning
5. All errors include operation context and are printed to stderr
6. `--verbose` enables debug logging to stderr
7. `SIGINT` during any operation triggers clean shutdown
8. Empty/missing workspace produces a helpful error message
9. README.md contains working "Getting Started" instructions
10. All cobra commands have `Long` descriptions and `Example` fields
11. `make check` passes with no lint warnings
12. All tests pass with `go test -race ./...`
