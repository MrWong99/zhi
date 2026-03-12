---
title: "feat: Config Drift Detection"
type: feat
status: completed
date: 2026-03-12
issue: "#114"
brainstorm: docs/brainstorms/2026-03-12-drift-detection-brainstorm.md
---

# Config Drift Detection

## Overview

Add a `zhi drift` command that detects when exported config files on disk have diverged from what the workspace would currently generate. Re-renders all export templates (dry-run) and diffs them against files on disk using the existing `DiffExport` engine. Includes a `--watch` mode for continuous monitoring with an optional `--on-drift` shell hook.

## Problem Statement

After `zhi apply`, config files can drift silently — someone edits a file directly, a process overwrites env vars, a template output gets manually tweaked. Without drift detection, the workspace config becomes a lie. This command closes the gap between "what zhi thinks is deployed" and "what's actually on disk."

## Proposed Solution

Thin wrapper over the existing export + diff pipeline. No snapshots, no new state files — always re-renders templates and compares against disk. The core logic is:

```
PrepareTreeData → ExpandTemplates (dry-run) → for each: Export → DiffExport → aggregate
```

### CLI Interface

```bash
zhi drift                                          # check all exports
zhi drift --json                                   # JSON output
zhi drift --watch --interval 5m                    # watch mode (foreground)
zhi drift --watch --interval 5m --on-drift "cmd"   # watch with notification hook
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No drift (clean) |
| 1 | Drift detected |
| 2 | Error during drift check |

### Human-Readable Output

```
Drift Detection Report

DRIFTED (1):
  ansible/inventory/hosts.yml:
    --- ansible/inventory/hosts.yml (current)
    +++ ansible/inventory/hosts.yml (expected)
    @@ -10,3 +10,3 @@
    -upstream: 8.8.8.8
    +upstream: 1.1.1.1

IN SYNC (3):
  ansible/group_vars/webservers.yml
  ansible/group_vars/databases.yml
  config/app.toml

Run `zhi export` to reconcile, or `zhi export --diff` for full details.
```

### JSON Output Schema

```json
{
  "drifted": [
    {
      "name": "inventory-yaml",
      "output_path": "ansible/inventory/hosts.yml",
      "is_new": false,
      "diff": "@@ -10,3 +10,3 @@\n-upstream: 8.8.8.8\n+upstream: 1.1.1.1\n"
    }
  ],
  "in_sync": [
    {
      "name": "group-vars/webservers",
      "output_path": "ansible/group_vars/webservers.yml"
    }
  ]
}
```

In `--watch --json` mode: one NDJSON line per state-change event.

### Watch Mode Behavior

- Edge-triggered: logs only on state transitions (clean→drifted or drifted→clean)
- First check: state starts as "unknown". Transition to drifted fires the hook.
- `--on-drift` hook failure: logged as warning to stderr, watch loop continues
- `--interval` defaults to `1m` when `--watch` is set without `--interval`
- Workspace loaded once at startup (restart watch to pick up `zhi.yaml` changes)
- Graceful shutdown (SIGINT/SIGTERM): exits 0 regardless of last drift state

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| No export templates configured | Exit 0, print "no export templates configured; nothing to check" |
| Stdout-only template (no output path) | Skip with warning: "template 'foo' has no output path; skipping" |
| File doesn't exist on disk yet | Reported as drifted with `is_new: true` |
| Template render error | Per-template error in output, does NOT trigger `--on-drift` hook, exit 2 |
| File permission error | Per-file error, continue checking remaining files, exit 2 |
| Orphaned iterate files (tree children removed) | Out of scope for initial implementation |

## Technical Approach

### Files to Create

#### `internal/core/drift.go` — Core drift logic

```go
// DriftCheckResult holds the outcome of a full drift check.
type DriftCheckResult struct {
    Drifted []DriftEntry
    InSync  []DriftEntry
    Errors  []DriftError
}

type DriftEntry struct {
    Name       string // template name
    OutputPath string
    IsNew      bool
    Diff       string // unified diff (empty for in-sync)
}

type DriftError struct {
    Name       string
    OutputPath string
    Err        error
}

// CheckDrift runs the full drift detection pipeline.
func CheckDrift(ctx context.Context, eng *Engine) (*DriftCheckResult, error)
```

Implementation:
1. Call `PrepareTreeData(ctx, eng, false, "")` to get the current tree
2. Call `ExpandTemplates(ws.Export.Templates, ws.Dir, true)` with dry-run
3. For each config:
   - Skip if `OutputPath` is empty or `"-"` (stdout-only)
   - Call `Export(ctx, td, cfg)` or `ExportIterate(ctx, td, cfg)` to render
   - Call `DiffExport(result.OutputPath, result.Content)` to compare
   - Categorize into Drifted/InSync/Errors
4. Return aggregated result

#### `internal/core/drift_test.go` — Core tests

Test cases:
- No drift (rendered matches disk)
- Drift detected (rendered differs from disk)
- New file (file doesn't exist on disk)
- No export templates configured
- Stdout-only template skipped
- Iterate template with multiple children
- Template render error (malformed template)
- File permission error

Uses same test helpers as `export_test.go`: `newTestTree()`, `NewTreeData()`, `t.TempDir()`.

#### `internal/cli/drift.go` — CLI command

Pattern follows `export.go` and `apply.go`:

```go
var driftCmd = &cobra.Command{
    Use:               "drift",
    Short:             "Detect configuration drift in exported files",
    Long:              "Compare exported config files on disk against what the workspace would currently generate.",
    PersistentPreRunE: withEngine,
    RunE:              runDrift,
}

// Flags
var (
    driftJSON     bool
    driftWatch    bool
    driftInterval string // duration string, e.g. "5m"
    driftOnDrift  string // shell command
)
```

`runDrift` implementation:
1. Get engine via `engineFromCmd(cmd)`
2. If `!driftWatch`: call `core.CheckDrift`, format output, exit with appropriate code
3. If `driftWatch`: enter watch loop

Watch loop (modeled on `SyncScheduler` from `cmd/zhi-mirror/server/sync.go`):
- Parse `driftInterval` with `time.ParseDuration`, default `1m`
- Run first check immediately
- `time.NewTicker(interval)` + `select { case <-ctx.Done(): return; case <-ticker.C: check() }`
- Track previous state (`drifted bool`), only print/hook on transitions
- On transition to drifted: if `driftOnDrift != ""`, exec via `sh -c`
- On SIGINT/SIGTERM: context cancels, loop exits, return nil (exit 0)

Output formatting:
- Human mode: group into DRIFTED/IN SYNC sections, print unified diffs inline for drifted files, print remediation hint
- JSON mode: `json.MarshalIndent` with 2-space indent to `cmd.OutOrStdout()`
- Watch+JSON: one NDJSON line per state-change event

#### `internal/cli/drift_test.go` — CLI tests

Uses `setupTestEngine`, `setEngineContext`, `executeCommand` from `testhelper_test.go`.

Test cases:
- `zhi drift` with no drift → exit 0, output says "IN SYNC"
- `zhi drift` with drift → exit 1, output includes diff
- `zhi drift --json` → valid JSON, correct schema
- `zhi drift` with no templates → exit 0, informational message
- `zhi drift --watch` → starts and responds to context cancellation
- Flag validation: `--interval` without `--watch` is an error
- `--on-drift` without `--watch` is an error

### Files to Modify

None. The command is self-registering via `init()` in `drift.go`. All dependencies (`PrepareTreeData`, `ExpandTemplates`, `Export`, `ExportIterate`, `DiffExport`) are already public functions in `internal/core/`.

## Acceptance Criteria

### Functional Requirements

- [x] `zhi drift` re-renders all workspace export templates and compares against files on disk
- [x] Drifted files show unified diff output in human-readable mode
- [x] In-sync files listed by name
- [x] `--json` outputs valid JSON matching the defined schema
- [x] Exit code 0 when clean, 1 when drifted, 2 on error
- [x] `--watch` runs continuously, checking at `--interval` (default 1m)
- [x] Watch mode only outputs on state transitions (clean↔drifted)
- [x] `--on-drift` executes shell command on transition to drifted state
- [x] First watch cycle treats initial state as "unknown" — drift at t=0 fires the hook
- [x] Stdout-only templates skipped with warning
- [x] No-templates workspace exits 0 with informational message
- [x] Component filtering applied (disabled component paths excluded)

### Non-Functional Requirements

- [x] No new dependencies — uses stdlib + existing core functions only
- [x] All output written via `cmd.OutOrStdout()` (not `os.Stdout`)
- [x] Tests pass with `-race -count=1`
- [x] Follows existing CLI patterns (flag registration, engine context, test helpers)

## Implementation Phases

### Phase 1: Core Logic (`internal/core/drift.go` + tests)

1. Define `DriftCheckResult`, `DriftEntry`, `DriftError` types
2. Implement `CheckDrift(ctx, eng)` wrapping the export+diff pipeline
3. Handle edge cases: no templates, stdout-only, render errors
4. Write comprehensive unit tests

### Phase 2: CLI Command (`internal/cli/drift.go` + tests)

1. Define cobra command with flags
2. Implement `runDrift` for one-shot mode
3. Implement human-readable output formatting
4. Implement `--json` output
5. Write CLI tests for one-shot mode

### Phase 3: Watch Mode

1. Implement watch loop with `time.NewTicker` + context cancellation
2. Add state tracking (edge-triggered transitions)
3. Implement `--on-drift` hook execution via `sh -c`
4. Handle `--watch --json` as NDJSON
5. Write watch mode tests (use short intervals + context cancellation)
6. Flag validation (`--interval`/`--on-drift` require `--watch`)

## References

### Internal References

- CLI command pattern: `internal/cli/root.go:115` (`withEngine`), `internal/cli/export.go` (closest analog)
- Export pipeline: `internal/core/export.go` (`PrepareTreeData`, `ExpandTemplates`, `Export`, `ExportIterate`)
- Diff engine: `internal/core/diff.go` (`DiffExport`, `DiffResult`)
- Ticker pattern: `cmd/zhi-mirror/server/sync.go` (`SyncScheduler`)
- JSON output pattern: `internal/cli/validate.go:31-38`, `internal/cli/list.go:31-34`
- Test helpers: `internal/cli/testhelper_test.go` (`setupTestEngine`, `executeCommand`, `setEngineContext`)
- Signal handling: `internal/cli/root.go:61` (`signal.NotifyContext`)
- Rollback (similar file-state pattern): `internal/core/rollback.go`

### Related Issues

- #101 — Diff engine (provides `DiffExport`)
- #103 — File store (provides checksums)
- #113 — Doctor command (future integration point)
