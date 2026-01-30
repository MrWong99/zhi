package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/ui/tui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestValidationView_Empty(t *testing.T) {
	vv := tui.NewValidationViewWith(nil)
	vv.SetSize(120, 30)

	view := vv.View()
	if !strings.Contains(view, "passed") {
		t.Error("expected 'passed' message for empty results")
	}
}

func TestValidationView_BlockingResults(t *testing.T) {
	results := []config.ValidationResult{
		{Severity: config.Blocking, Message: "database/host is required"},
		{Severity: config.Warning, Message: "using localhost is not recommended"},
		{Severity: config.Info, Message: "default port 5432 is being used"},
	}

	vv := tui.NewValidationViewWith(results)
	vv.SetSize(120, 30)

	view := vv.View()
	if !strings.Contains(view, "BLOCKING") {
		t.Error("expected BLOCKING label in view")
	}
	if !strings.Contains(view, "WARNING") {
		t.Error("expected WARNING label in view")
	}
	if !strings.Contains(view, "INFO") {
		t.Error("expected INFO label in view")
	}
}

func TestValidationView_ComponentErrors(t *testing.T) {
	results := []config.ValidationResult{
		{Severity: config.Blocking, Message: "component 'redis' is enabled but its dependency 'database' is disabled"},
	}

	vv := tui.NewValidationViewWith(results)
	vv.SetSize(120, 30)

	view := vv.View()
	if !strings.Contains(view, "Component") {
		t.Error("expected component error section in view")
	}
}

func TestValidationView_Scrolling(t *testing.T) {
	// Create many results to test scrolling.
	var results []config.ValidationResult
	for i := range 20 {
		results = append(results, config.ValidationResult{
			Severity: config.Warning,
			Message:  strings.Repeat("w", i+1),
		})
	}

	vv := tui.NewValidationViewWith(results)
	vv.SetSize(80, 10)

	// Scroll down.
	for range 5 {
		vv, _ = vv.UpdateValidation(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	view := vv.View()
	if view == "" {
		t.Error("expected non-empty view after scrolling")
	}

	// Scroll up.
	for range 3 {
		vv, _ = vv.UpdateValidation(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	}

	view = vv.View()
	if view == "" {
		t.Error("expected non-empty view after scrolling up")
	}
}

func TestValidationView_Summary(t *testing.T) {
	results := []config.ValidationResult{
		{Severity: config.Blocking, Message: "error1"},
		{Severity: config.Blocking, Message: "error2"},
		{Severity: config.Warning, Message: "warn1"},
		{Severity: config.Info, Message: "info1"},
	}

	vv := tui.NewValidationViewWith(results)
	vv.SetSize(120, 30)

	view := vv.View()
	if !strings.Contains(view, "Blocking: 2") {
		t.Error("expected summary showing 2 blocking issues")
	}
	if !strings.Contains(view, "Warning: 1") {
		t.Error("expected summary showing 1 warning")
	}
}
