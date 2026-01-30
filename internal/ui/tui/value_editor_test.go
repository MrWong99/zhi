package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/ui/tui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestValueEditor_Display(t *testing.T) {
	val := &config.Value{
		Val:      "localhost",
		Metadata: map[string]any{"description": "Database host"},
	}

	editor := tui.NewValueEditorFor("database/host", val, "database", false)

	view := editor.View()
	if view == "" {
		t.Error("expected non-empty editor view")
	}
	if !strings.Contains(view, "database/host") {
		t.Error("expected view to contain the path")
	}
	if !strings.Contains(view, "database") {
		t.Error("expected view to contain component name")
	}
	if !strings.Contains(view, "Metadata") {
		t.Error("expected view to show metadata")
	}
}

func TestValueEditor_DisabledComponentWarning(t *testing.T) {
	val := &config.Value{Val: "redis://localhost:6379"}

	editor := tui.NewValueEditorFor("redis/url", val, "redis", true)

	view := editor.View()
	if !strings.Contains(view, "disabled component") {
		t.Error("expected warning about disabled component")
	}
}

func TestValueEditor_NoComponentWarningWhenEnabled(t *testing.T) {
	val := &config.Value{Val: "localhost"}

	editor := tui.NewValueEditorFor("database/host", val, "database", false)

	view := editor.View()
	if strings.Contains(view, "disabled component") {
		t.Error("should not show disabled component warning for enabled component")
	}
}

func TestValueEditor_CommitValue(t *testing.T) {
	val := &config.Value{
		Val:      "localhost",
		Metadata: map[string]any{"type": "string"},
	}

	editor := tui.NewValueEditorFor("database/host", val, "", false)

	// Type new value.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	committed := editor.CommitValue()
	if committed.Val == nil {
		t.Error("expected non-nil committed value")
	}
}

func TestValueEditor_DirtyState(t *testing.T) {
	val := &config.Value{Val: "original"}

	editor := tui.NewValueEditorFor("test/path", val, "", false)

	// Initially not dirty.
	view := editor.View()
	if strings.Contains(view, "(modified)") {
		t.Error("should not show modified initially")
	}
}

func TestValueEditor_NoComponent(t *testing.T) {
	val := &config.Value{Val: "somevalue"}

	editor := tui.NewValueEditorFor("standalone/path", val, "", false)

	view := editor.View()
	if strings.Contains(view, "Component:") {
		t.Error("should not show component section when no component")
	}
}
