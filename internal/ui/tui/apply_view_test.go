package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui/tui"
)

func TestApplyView_Initial(t *testing.T) {
	av := tui.NewApplyView()
	av.SetSize(120, 30)

	view := av.View()
	if view == "" {
		t.Error("expected non-empty initial apply view")
	}
}

func TestApplyView_OutputMessages(t *testing.T) {
	av := tui.NewApplyView()
	av.SetSize(120, 30)

	// Simulate receiving output.
	lines := []core.ApplyOutput{
		{Line: "Starting deployment...", Stream: "stdout", Time: time.Now()},
		{Line: "Pulling images...", Stream: "stdout", Time: time.Now()},
		{Line: "Warning: old image", Stream: "stderr", Time: time.Now()},
	}

	av, _ = av.UpdateApply(tui.ApplyOutputMsg{Lines: lines})

	view := av.View()
	if !strings.Contains(view, "Starting deployment") {
		t.Error("expected output to contain 'Starting deployment'")
	}
	if !strings.Contains(view, "Pulling images") {
		t.Error("expected output to contain 'Pulling images'")
	}
}

func TestApplyView_Completion(t *testing.T) {
	av := tui.NewApplyView()
	av.SetSize(120, 30)

	// Simulate completion.
	av, _ = av.UpdateApply(tui.ApplyDoneMsg{
		Result: &core.ApplyResult{
			ExitCode: 0,
			Duration: 5 * time.Second,
		},
	})

	view := av.View()
	if !strings.Contains(view, "completed") {
		t.Error("expected completion message in view")
	}
}

func TestApplyView_FailedCompletion(t *testing.T) {
	av := tui.NewApplyView()
	av.SetSize(120, 30)

	av, _ = av.UpdateApply(tui.ApplyDoneMsg{
		Result: &core.ApplyResult{
			ExitCode: 1,
			Duration: 3 * time.Second,
		},
	})

	view := av.View()
	if !strings.Contains(view, "failed") {
		t.Error("expected failure message in view")
	}
}

func TestApplyView_Scrolling(t *testing.T) {
	av := tui.NewApplyView()
	av.SetSize(80, 10)

	// Add many lines.
	var lines []core.ApplyOutput
	for i := range 30 {
		lines = append(lines, core.ApplyOutput{
			Line:   strings.Repeat("x", i+1),
			Stream: "stdout",
			Time:   time.Now(),
		})
	}
	av, _ = av.UpdateApply(tui.ApplyOutputMsg{Lines: lines})

	// Jump to top.
	av, _ = av.UpdateApply(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	// Scroll down.
	for range 5 {
		av, _ = av.UpdateApply(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// Jump to bottom.
	av, _ = av.UpdateApply(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	view := av.View()
	if view == "" {
		t.Error("expected non-empty view after scrolling")
	}
}
