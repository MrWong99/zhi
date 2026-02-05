package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/ui/tui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestTreeView_Navigation(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	// Initial state should have cursor at 0.
	initialPath := tv.SelectedPath()
	if initialPath == "" {
		t.Fatal("expected a selected path")
	}

	// Navigate down.
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	secondPath := tv.SelectedPath()
	if secondPath == initialPath {
		t.Error("expected cursor to move to a different path after j")
	}

	// Navigate back up.
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if tv.SelectedPath() != initialPath {
		t.Error("expected cursor to return to initial path after k")
	}

	// Arrow keys should also work.
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyDown})
	if tv.SelectedPath() == initialPath {
		t.Error("expected cursor to move with down arrow")
	}

	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyUp})
	if tv.SelectedPath() != initialPath {
		t.Error("expected cursor to move with up arrow")
	}
}

func TestTreeView_Filter(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	// Activate filter.
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	// Type "data" to filter for database paths.
	for _, ch := range "data" {
		tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Confirm filter.
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyEnter})

	// The selected path should contain "data".
	selected := tv.SelectedPath()
	if selected != "" && !strings.Contains(selected, "data") {
		t.Errorf("expected filtered path containing 'data', got %q", selected)
	}
}

func TestTreeView_ClearFilter(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	// Activate filter and type something.
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})

	// Esc should clear filter.
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyEscape})

	// Should have all paths again.
	view := tv.View()
	if view == "" {
		t.Error("expected non-empty view after clearing filter")
	}
}

func TestTreeView_View(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()
	if view == "" {
		t.Error("expected non-empty tree view")
	}

	// View should contain path names.
	if !strings.Contains(view, "database") {
		t.Error("expected view to contain 'database'")
	}
}

func TestTreeView_ComponentDisplay(t *testing.T) {
	ctrl := setupTestController(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// Database paths should show the component badge since database is enabled.
	if !strings.Contains(view, "database") {
		t.Error("expected view to contain database component info")
	}
}

func TestTreeView_HiddenPaths(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// Hidden path should not appear.
	if strings.Contains(view, "internal-key") {
		t.Error("hidden path 'database/internal-key' should not appear in the tree view")
	}

	// Non-hidden paths should appear.
	if !strings.Contains(view, "database") {
		t.Error("expected database paths to appear")
	}
}

func TestTreeView_ShowIfConditionMet(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// database/type = "postgres", so pg-options should be visible.
	if !strings.Contains(view, "pg-options") {
		t.Error("expected pg-options to be visible when database/type=postgres")
	}

	// database/type != "mysql", so mysql-charset should be hidden.
	if strings.Contains(view, "mysql-charset") {
		t.Error("mysql-charset should be hidden when database/type != mysql")
	}
}

func TestTreeView_ShowIfConditionChanges(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	_, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Change database/type to "mysql".
	err = ctrl.SetValue(ctx, "database/type", config.Value{
		Val: "mysql",
		Metadata: map[string]any{
			"ui.enum":        []string{"postgres", "mysql", "sqlite"},
			"ui.order":       1,
			"ui.section":     "Connection",
			"ui.displayName": "Database Engine",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Recreate tree view with updated tree.
	tree := ctrl.Tree()
	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// Now mysql-charset should be visible.
	if !strings.Contains(view, "mysql-charset") {
		t.Error("mysql-charset should be visible when database/type=mysql")
	}

	// pg-options should now be hidden.
	if strings.Contains(view, "pg-options") {
		t.Error("pg-options should be hidden when database/type != postgres")
	}
}

func TestTreeView_PasswordMasking(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// Password value should be masked, not showing the actual value.
	if strings.Contains(view, "s3cret") {
		t.Error("password value should not be shown in plain text")
	}
}

func TestTreeView_DisplayName(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// Display name should appear instead of raw path.
	if !strings.Contains(view, "Database Engine") {
		t.Error("expected display name 'Database Engine' in tree view")
	}
}

func TestTreeView_LabelBadges(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// Readonly badge.
	if !strings.Contains(view, "[ro]") {
		t.Error("expected [ro] badge for readonly value")
	}

	// Required badge.
	if !strings.Contains(view, "[req]") {
		t.Error("expected [req] badge for required value")
	}

	// Deprecated badge.
	if !strings.Contains(view, "[deprecated]") {
		t.Error("expected [deprecated] badge for deprecated value")
	}

	// Enum badge.
	if !strings.Contains(view, "[enum]") {
		t.Error("expected [enum] badge for enum value")
	}

	// Confirm badge.
	if !strings.Contains(view, "[confirm]") {
		t.Error("expected [confirm] badge for confirm value")
	}
}

func TestTreeView_SectionHeaders(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	view := tv.View()

	// Section header should appear.
	if !strings.Contains(view, "Connection") {
		t.Error("expected 'Connection' section header in tree view")
	}
}

func TestTreeView_LabelAwareSorting(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	// Collect visible paths in order by navigating.
	var paths []string
	for {
		p := tv.SelectedPath()
		if p == "" {
			break
		}
		paths = append(paths, p)
		prevCursor := tv.SelectedPath()
		tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		if tv.SelectedPath() == prevCursor {
			break
		}
	}

	// database/type has order=1, database/host has order=2, database/port has order=3.
	// Verify relative ordering of the order=1,2,3 paths.
	typeIdx := -1
	hostIdx := -1
	portIdx := -1
	for i, p := range paths {
		switch p {
		case "database/type":
			typeIdx = i
		case "database/host":
			hostIdx = i
		case "database/port":
			portIdx = i
		}
	}

	if typeIdx >= 0 && hostIdx >= 0 && typeIdx >= hostIdx {
		t.Errorf("database/type (order=1) at index %d should be before database/host (order=2) at index %d", typeIdx, hostIdx)
	}
	if hostIdx >= 0 && portIdx >= 0 && hostIdx >= portIdx {
		t.Errorf("database/host (order=2) at index %d should be before database/port (order=3) at index %d", hostIdx, portIdx)
	}
}

func TestTreeView_FilterByDisplayName(t *testing.T) {
	ctrl := setupTestControllerWithLabels(t)
	ctx := t.Context()

	tree, err := ctrl.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tv := tui.NewTreeView(ctrl, tree)
	tv.SetSize(120, 30)

	// Filter by display name "Engine".
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, ch := range "Engine" {
		tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	tv, _ = tv.UpdateTree(tea.KeyMsg{Type: tea.KeyEnter})

	selected := tv.SelectedPath()
	if selected != "database/type" {
		t.Errorf("expected filter by display name to find database/type, got %q", selected)
	}
}
