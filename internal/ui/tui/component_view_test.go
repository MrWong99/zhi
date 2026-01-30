package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/ui/tui"
)

func TestComponentView_Display(t *testing.T) {
	ctrl := setupTestController(t)

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	view := cv.View()
	if view == "" {
		t.Error("expected non-empty component view")
	}

	// Should display component names.
	if !strings.Contains(view, "database") {
		t.Error("expected view to contain 'database'")
	}
	if !strings.Contains(view, "app") {
		t.Error("expected view to contain 'app'")
	}
	if !strings.Contains(view, "redis") {
		t.Error("expected view to contain 'redis'")
	}
}

func TestComponentView_MandatoryBadge(t *testing.T) {
	ctrl := setupTestController(t)

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	view := cv.View()
	if !strings.Contains(view, "MANDATORY") {
		t.Error("expected view to show MANDATORY badge for database component")
	}
}

func TestComponentView_Navigation(t *testing.T) {
	ctrl := setupTestController(t)

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	// Navigate down.
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Navigate up.
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	// Should not panic.
	view := cv.View()
	if view == "" {
		t.Error("expected non-empty view after navigation")
	}
}

func TestComponentView_ToggleEnable(t *testing.T) {
	ctrl := setupTestController(t)

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	// Navigate to a non-mandatory disabled component (app is second, index 1).
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Toggle enable with Enter.
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyEnter})

	// Check that the component was enabled.
	if !ctrl.IsComponentEnabled("app") {
		t.Error("expected app to be enabled after toggle")
	}
}

func TestComponentView_ToggleDisable(t *testing.T) {
	ctrl := setupTestController(t)

	// First enable app.
	_, err := ctrl.EnableComponent("app")
	if err != nil {
		t.Fatal(err)
	}

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	// Navigate to app (second component, index 1).
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Toggle disable with Space.
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	if ctrl.IsComponentEnabled("app") {
		t.Error("expected app to be disabled after toggle")
	}
}

func TestComponentView_MandatoryCannotDisable(t *testing.T) {
	ctrl := setupTestController(t)

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	// First component is 'database' (mandatory, enabled).
	// Try to toggle it off.
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyEnter})

	// Database should still be enabled.
	if !ctrl.IsComponentEnabled("database") {
		t.Error("expected database to remain enabled (mandatory)")
	}
}

func TestComponentView_EnableWithDeps(t *testing.T) {
	ctrl := setupTestController(t)

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	// Navigate to redis (third component, index 2).
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Enable redis - database is already enabled (mandatory), so only redis
	// should appear in the enabled list.
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyEnter})

	if !ctrl.IsComponentEnabled("redis") {
		t.Error("expected redis to be enabled")
	}
}

func TestComponentView_InfoPanel(t *testing.T) {
	ctrl := setupTestController(t)

	cv := tui.NewComponentView(ctrl)
	cv.SetSize(120, 30)

	// Press 'i' to show info.
	cv, _ = cv.UpdateComponent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	view := cv.View()
	if !strings.Contains(view, "Component:") {
		t.Error("expected info panel to show component details")
	}
}
