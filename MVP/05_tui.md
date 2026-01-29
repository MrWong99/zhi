# Step 5: Terminal User Interface (TUI)

## Overview

Build the Bubbletea-based TUI that provides an interactive terminal interface for browsing, editing, validating, and applying configuration. The TUI runs in-process (same binary, no gRPC) and calls the core engine directly through Go interfaces.

## Relevant Existing Files

- `internal/core/engine.go` — core engine (TUI calls this directly)
- `internal/core/export.go` — export system (triggered by TUI export action)
- `internal/core/apply.go` — apply runner (TUI consumes its output channel)
- `internal/cli/root.go` — CLI root (the `zhi edit` command launches the TUI)
- `pkg/zhiplugin/config/config.go` — `Tree`, `Value`, `ValidationResult`, `Severity` types
- `pkg/ui/.keep` — placeholder directory (may use `internal/ui/` instead)
- `go.mod` — needs Bubbletea dependencies

## Implementation Plan

### 5.1 Dependencies

Add to `go.mod`:
```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles    # reusable components (textinput, viewport, list, table)
github.com/charmbracelet/lipgloss   # styling
```

### 5.2 Application Model (`internal/ui/app.go`)

The root Bubbletea model that manages the overall TUI state and view routing.

**Components:**

- `App` struct (implements `tea.Model`):
  - `engine *core.Engine` — reference to the core engine
  - `tree *config.Tree` — the current configuration tree
  - `activeView` — enum: `viewTree`, `viewEditor`, `viewValidation`, `viewApply`, `viewExport`
  - `treeView TreeView` — tree browser sub-model
  - `editorView ValueEditor` — value editor sub-model
  - `validationView ValidationView` — validation results sub-model
  - `applyView ApplyView` — apply output sub-model
  - `exportView ExportView` — export sub-model
  - `statusMsg string` — bottom status bar message
  - `err error` — last error
- `NewApp(engine) (*App, error)` — load tree from engine, initialize sub-models
- `Init() tea.Cmd` — initial command (load tree)
- `Update(msg) (tea.Model, tea.Cmd)` — route messages to active view
- `View() string` — render active view with header and status bar

**Global key bindings (handled in `Update` regardless of active view):**

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help overlay |
| `Esc` | Return to tree view from any sub-view |

### 5.3 Tree View (`internal/ui/tree_view.go`)

Browse configuration paths as a navigable list.

**Components:**

- `TreeView` struct:
  - `paths []string` — sorted list of config paths
  - `cursor int` — currently highlighted path
  - `values map[string]*config.Value` — cached values for display
  - `filter string` — optional path filter
  - `list bubbles.List` — use bubbles list component for navigation
- Display format: each row shows `path  →  value  [metadata]`
- Color coding: paths with blocking validation issues in red, warnings in yellow

**Key bindings:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Open value editor for selected path |
| `/` | Start filtering paths |
| `s` | Save tree to store |
| `v` | Run validation, switch to validation view |
| `a` | Run apply, switch to apply view |
| `e` | Switch to export view |
| `r` | Reload tree from config provider |

### 5.4 Value Editor (`internal/ui/value_editor.go`)

Edit a single configuration value.

**Components:**

- `ValueEditor` struct:
  - `path string` — the path being edited
  - `value *config.Value` — current value
  - `input bubbles.TextInput` — text input component
  - `metadata map[string]any` — displayed metadata (read-only)
  - `dirty bool` — whether the value has been modified
- Display: path at top, current value in editable text input, metadata below, help at bottom
- Type-aware display: show type hint from metadata if available

**Key bindings:**

| Key | Action |
|-----|--------|
| `Enter` | Commit the edited value (calls `engine.SetValue()`) |
| `Esc` | Discard changes, return to tree view |
| `Tab` | Cycle through available values if metadata suggests options |

**Optimistic UI:** The text input updates locally on every keystroke. Only `Enter` triggers a core engine call. This keeps typing responsive.

### 5.5 Validation View (`internal/ui/validation_view.go`)

Display validation results with severity highlighting.

**Components:**

- `ValidationView` struct:
  - `results []config.ValidationResult` — validation results
  - `viewport bubbles.Viewport` — scrollable view
  - `summary string` — count by severity
- Display: results grouped by severity (Blocking first, then Warning, then Info)
- Color coding: red for Blocking, yellow for Warning, blue for Info

**Key bindings:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `Esc` | Return to tree view |
| `Enter` | Jump to tree view with the selected path highlighted |

### 5.6 Apply View (`internal/ui/apply_view.go`)

Stream apply command output in a scrollable viewport.

**Components:**

- `ApplyView` struct:
  - `viewport bubbles.Viewport` — scrollable output area
  - `output []ApplyOutput` — buffered output lines
  - `running bool` — whether the command is still running
  - `result *core.ApplyResult` — final result after completion
  - `spinner bubbles.Spinner` — activity indicator while running
- Uses a `tea.Cmd` that reads from the apply output channel and sends `applyOutputMsg` messages
- Batches incoming lines: accumulate lines for ~50ms before re-rendering to avoid flicker

**Key bindings:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `G` | Jump to bottom (follow mode) |
| `g` | Jump to top |
| `Ctrl+C` | Cancel the running command |
| `Esc` / `q` | Return to tree view (only after command completes) |

### 5.7 Export View (`internal/ui/export_view.go`)

Select and execute export operations.

**Components:**

- `ExportView` struct:
  - `templates []core.ExportConfig` — available export templates from workspace
  - `formats []string` — built-in format options
  - `cursor int` — selected item
  - `preview string` — preview of rendered output
  - `exported bool` — whether export has been executed
- Two-panel layout: left panel lists templates/formats, right panel shows preview

**Key bindings:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Execute selected export |
| `p` | Toggle preview for selected template |
| `Esc` | Return to tree view |

### 5.8 CLI: `zhi edit` Command (`internal/cli/edit.go`)

Launch the TUI.

**Usage:**

```
zhi edit [tree-id]
```

**Behavior:**

1. Load workspace config and build engine
2. Create `App` with the engine
3. Run `bubbletea.NewProgram(app, tea.WithAltScreen())` — full-screen mode
4. On exit, print any final status message

**Flags:**

- `--tree` — specify which tree ID to load (if store has multiple)

### 5.9 Styling (`internal/ui/styles.go`)

Define a consistent visual theme using lipgloss.

**Style definitions:**

- `HeaderStyle` — bold, colored header bar
- `PathStyle` — dim for path segments, bright for leaf name
- `ValueStyle` — normal text for values
- `BlockingStyle` — red background/text for blocking issues
- `WarningStyle` — yellow for warnings
- `InfoStyle` — blue for informational messages
- `StatusBarStyle` — bottom bar with key hints
- `ActiveStyle` — highlighted/selected item

### 5.10 Tests

- `internal/ui/app_test.go` — test app initialization, view switching
- `internal/ui/tree_view_test.go` — test navigation, filtering, key handling
- `internal/ui/value_editor_test.go` — test value editing, commit, cancel
- `internal/ui/validation_view_test.go` — test result display and grouping
- `internal/ui/apply_view_test.go` — test output streaming and buffering

Testing approach: use Bubbletea's `tea.TestModel` or manually call `Update()` with synthetic messages and assert on the resulting model state and `View()` output.

## Verification Criteria

1. `zhi edit` launches a full-screen TUI showing the configuration tree
2. Arrow keys / j/k navigate between configuration paths
3. `Enter` opens a value editor with the current value pre-filled
4. Editing a value and pressing `Enter` calls `engine.SetValue()` and returns to tree view
5. `s` saves the tree to the store plugin
6. `v` runs validation and displays results with color-coded severity
7. `a` runs the apply command and streams output in a scrollable viewport
8. `e` opens the export view with available templates
9. `Esc` returns to tree view from any sub-view
10. `q` / `Ctrl+C` exits the TUI cleanly
11. The TUI renders correctly in standard 80x24 and larger terminal sizes
12. No gRPC calls — all engine interaction is through direct Go interfaces
13. All tests pass with `go test -race ./internal/ui/...`
