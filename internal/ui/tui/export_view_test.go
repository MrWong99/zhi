package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/ui/tui"
)

func TestExportView_Display(t *testing.T) {
	ctrl := setupTestController(t)

	ev := tui.NewExportView(ctrl)
	ev.SetSize(120, 30)

	view := ev.View()
	if view == "" {
		t.Error("expected non-empty export view")
	}

	// Should show built-in formats.
	if !strings.Contains(view, "json") {
		t.Error("expected view to contain 'json' format")
	}
	if !strings.Contains(view, "yaml") {
		t.Error("expected view to contain 'yaml' format")
	}
}

func TestExportView_Navigation(t *testing.T) {
	ctrl := setupTestController(t)

	ev := tui.NewExportView(ctrl)
	ev.SetSize(120, 30)

	// Navigate down.
	ev, _ = ev.UpdateExport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, t.Context())
	ev, _ = ev.UpdateExport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, t.Context())

	// Navigate up.
	ev, _ = ev.UpdateExport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, t.Context())

	view := ev.View()
	if view == "" {
		t.Error("expected non-empty view after navigation")
	}
}

func TestExportView_ComponentContext(t *testing.T) {
	ctrl := setupTestController(t)

	// Enable some components.
	_, _ = ctrl.EnableComponent("app")

	ev := tui.NewExportView(ctrl)
	ev.SetSize(120, 30)

	view := ev.View()
	// Should show enabled components.
	if !strings.Contains(view, "Enabled components") {
		t.Error("expected enabled components header")
	}
	if !strings.Contains(view, "database") {
		t.Error("expected database in enabled components")
	}
}
