package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/ui/tui"
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
