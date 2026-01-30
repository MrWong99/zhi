# Step 6: Terminal User Interface (TUI)

## Overview

Build the Bubbletea-based TUI as the **first concrete implementation** of the `UIDriver` interface defined in Step 5. The TUI runs in-process and calls the core engine through the `UIController`. This step adds interactive tree browsing, value editing, component toggling, validation display, apply streaming, and export preview.

## Relevant Existing Files

- `internal/ui/driver.go` — UIDriver interface and UIController (from Step 5)
- `internal/ui/registry.go` — UI driver registry (from Step 5)
- `internal/core/engine.go` — core engine (accessed through UIController)
- `internal/core/component.go` — component manager (accessed through UIController)
- `internal/core/export.go` — export system (triggered by UIController export calls)
- `internal/core/apply.go` — apply runner (output consumed via UIController)
- `internal/cli/edit.go` — CLI `zhi edit` command (resolves and launches UIDriver)
- `pkg/zhiplugin/config/config.go` — `Tree`, `Value`, `ValidationResult`, `Severity` types
- `go.mod` — needs Bubbletea dependencies

## Implementation Plan

### 6.1 Dependencies

Add to `go.mod`:
```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles    # reusable components (textinput, viewport, list, table)
github.com/charmbracelet/lipgloss   # styling
```

### 6.2 Driver Registration

Register the TUI driver with the UI registry so that `zhi edit` (and `zhi edit --ui tui`) can resolve it.

```go
// internal/ui/tui/register.go
package tui

import "github.com/MrWong99/zhi/internal/ui"

func init() {
    ui.Register("tui", func() ui.UIDriver {
        return &TUIDriver{}
    })
}
```

The `internal/cli/edit.go` imports `_ "github.com/MrWong99/zhi/internal/ui/tui"` to trigger registration.

### 6.3 TUI Application Model (`internal/ui/tui/app.go`)

The root Bubbletea model that manages the overall TUI state and view routing. Implements `UIDriver`.

**Components:**

- `TUIDriver` struct (implements `ui.UIDriver`):
  - `Run(ctx, controller) error` — creates the `App` model, runs `bubbletea.NewProgram`

- `App` struct (implements `tea.Model`):
  - `controller *ui.UIController` — reference to the UI controller (NOT directly to the engine)
  - `tree *config.Tree` — the current configuration tree
  - `activeView` — enum: `viewTree`, `viewEditor`, `viewComponent`, `viewValidation`, `viewApply`, `viewExport`
  - `treeView TreeView` — tree browser sub-model
  - `editorView ValueEditor` — value editor sub-model
  - `componentView ComponentView` — component toggle sub-model
  - `validationView ValidationView` — validation results sub-model
  - `applyView ApplyView` — apply output sub-model
  - `exportView ExportView` — export sub-model
  - `statusMsg string` — bottom status bar message
  - `err error` — last error
- `NewApp(controller) (*App, error)` — load tree from controller, initialize sub-models
- `Init() tea.Cmd` — initial command (load tree)
- `Update(msg) (tea.Model, tea.Cmd)` — route messages to active view
- `View() string` — render active view with header and status bar

**Global key bindings (handled in `Update` regardless of active view):**

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help overlay |
| `Esc` | Return to tree view from any sub-view |

### 6.4 Tree View (`internal/ui/tui/tree_view.go`)

Browse configuration paths as a navigable list. Paths belonging to disabled components are visually dimmed.

**Components:**

- `TreeView` struct:
  - `paths []string` — sorted list of config paths
  - `cursor int` — currently highlighted path
  - `values map[string]*config.Value` — cached values for display
  - `filter string` — optional path filter
  - `list bubbles.List` — use bubbles list component for navigation
  - `componentStates map[string]bool` — cached component enabled/disabled state for display
- Display format: each row shows `path  →  value  [metadata]  [component-name]`
- Color coding:
  - Paths with blocking validation issues: red
  - Paths with warnings: yellow
  - Paths belonging to disabled components: dimmed/gray with strikethrough indicator
  - Component name badge shown next to paths that belong to a component

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
| `c` | Switch to component view |
| `r` | Reload tree from config provider |

### 6.5 Component View (`internal/ui/tui/component_view.go`)

Toggle components on and off. This is a new view specific to the component model.

**Components:**

- `ComponentView` struct:
  - `components []core.ComponentState` — list of all components
  - `cursor int` — currently highlighted component
  - `message string` — feedback message (e.g., "Enabled 'redis' (also enabled: database)")
  - `err string` — error message (e.g., "Cannot disable 'database': mandatory")
- Display format: each row shows:
  ```
  [✓] database       PostgreSQL database configuration     MANDATORY  paths: database/
  [✓] redis          Redis cache layer                     deps: database   paths: redis/
  [ ] monitoring     Prometheus and Grafana monitoring                paths: monitoring/
  ```
- Mandatory components show a lock icon or "MANDATORY" badge
- Dependencies are listed inline
- Enabled components: bright text with checkmark
- Disabled components: dimmed text with empty checkbox

**Key bindings:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` / `Space` | Toggle the selected component (enable if disabled, disable if enabled) |
| `i` | Show detailed info about the selected component (paths, dependents, dependency chain) |
| `Esc` | Return to tree view |

**Toggle behavior:**

- Toggling a disabled component ON: calls `controller.EnableComponent()`, which auto-enables dependencies. Display feedback message listing all components that were enabled.
- Toggling an enabled component OFF: calls `controller.DisableComponent()`. If rejected (mandatory or dependents), display error message. If successful, display confirmation.
- After any toggle, the tree view is refreshed to reflect the new component state (paths dimmed/undimmed).

### 6.6 Value Editor (`internal/ui/tui/value_editor.go`)

Edit a single configuration value.

**Components:**

- `ValueEditor` struct:
  - `path string` — the path being edited
  - `value *config.Value` — current value
  - `input bubbles.TextInput` — text input component
  - `metadata map[string]any` — displayed metadata (read-only)
  - `componentName string` — which component this path belongs to (if any)
  - `dirty bool` — whether the value has been modified
- Display: path at top, component badge (if applicable), current value in editable text input, metadata below, help at bottom
- If the path belongs to a disabled component, show a warning: "This path belongs to disabled component 'redis'. It will not be included in exports."
- Type-aware display: show type hint from metadata if available

**Key bindings:**

| Key | Action |
|-----|--------|
| `Enter` | Commit the edited value (calls `controller.SetValue()`) |
| `Esc` | Discard changes, return to tree view |
| `Tab` | Cycle through available values if metadata suggests options |

**Optimistic UI:** The text input updates locally on every keystroke. Only `Enter` triggers a controller call. This keeps typing responsive.

### 6.7 Validation View (`internal/ui/tui/validation_view.go`)

Display validation results with severity highlighting, including component dependency violations.

**Components:**

- `ValidationView` struct:
  - `results []config.ValidationResult` — validation results
  - `componentErrors []error` — component dependency validation errors
  - `viewport bubbles.Viewport` — scrollable view
  - `summary string` — count by severity
- Display: results grouped by severity (Blocking first, then Warning, then Info)
- Component dependency errors shown in a separate section at the top
- Color coding: red for Blocking, yellow for Warning, blue for Info, magenta for component dependency errors

**Key bindings:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `Esc` | Return to tree view |
| `Enter` | Jump to tree view with the selected path highlighted |

### 6.8 Apply View (`internal/ui/tui/apply_view.go`)

Stream apply command output in a scrollable viewport.

**Components:**

- `ApplyView` struct:
  - `viewport bubbles.Viewport` — scrollable output area
  - `output []core.ApplyOutput` — buffered output lines
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

### 6.9 Export View (`internal/ui/tui/export_view.go`)

Select and execute export operations. Shows component state as context.

**Components:**

- `ExportView` struct:
  - `templates []core.ExportConfig` — available export templates from workspace
  - `formats []string` — built-in format options
  - `cursor int` — selected item
  - `preview string` — preview of rendered output
  - `exported bool` — whether export has been executed
  - `enabledComponents []string` — currently enabled components (shown as context)
- Two-panel layout: left panel lists templates/formats, right panel shows preview
- Header shows which components are enabled (affecting what paths appear in output)

**Key bindings:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Execute selected export |
| `p` | Toggle preview for selected template |
| `Esc` | Return to tree view |

### 6.10 Styling (`internal/ui/tui/styles.go`)

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
- `DisabledComponentStyle` — dimmed/gray for paths belonging to disabled components
- `ComponentBadgeStyle` — colored badge for component name tags
- `MandatoryBadgeStyle` — distinctive style for mandatory component indicators
- `CheckboxEnabledStyle` — green checkmark for enabled components
- `CheckboxDisabledStyle` — gray empty box for disabled components

### 6.11 Tests

- `internal/ui/tui/app_test.go` — test app initialization, view switching
- `internal/ui/tui/tree_view_test.go` — test navigation, filtering, component-dimmed display
- `internal/ui/tui/component_view_test.go` — test component toggle, mandatory rejection, dependency auto-enable, display format
- `internal/ui/tui/value_editor_test.go` — test value editing, commit, cancel, disabled component warning
- `internal/ui/tui/validation_view_test.go` — test result display, grouping, component error section
- `internal/ui/tui/apply_view_test.go` — test output streaming and buffering
- `internal/ui/tui/export_view_test.go` — test template selection, preview toggle, component context display

Testing approach: use Bubbletea's `tea.TestModel` or manually call `Update()` with synthetic messages and assert on the resulting model state and `View()` output. The `UIController` can be constructed with a mock engine for isolated TUI testing.

## Verification Criteria

1. TUI registers itself as `"tui"` in the UI driver registry
2. `zhi edit` launches a full-screen TUI showing the configuration tree
3. Arrow keys / j/k navigate between configuration paths
4. Paths belonging to disabled components are visually dimmed
5. `Enter` opens a value editor with the current value pre-filled
6. Editing a value and pressing `Enter` calls `controller.SetValue()` and returns to tree view
7. `s` saves the tree and component state to the store plugin
8. `v` runs validation (including component dependencies) and displays results with color-coded severity
9. `a` runs the apply command and streams output in a scrollable viewport
10. `e` opens the export view with component context displayed
11. `c` opens the component view with toggle capability
12. Toggling a component in the component view updates the tree view (dimmed/undimmed paths)
13. Mandatory components cannot be toggled off (clear error message shown)
14. Enabling a component auto-enables its dependencies (with feedback)
15. `Esc` returns to tree view from any sub-view
16. `q` / `Ctrl+C` exits the TUI cleanly
17. The TUI renders correctly in standard 80x24 and larger terminal sizes
18. `--ui tui` flag works (and is the default)
19. All engine interaction goes through `UIController` — the TUI never imports `internal/core` directly
20. All tests pass with `go test -race ./internal/ui/tui/...`
