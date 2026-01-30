package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// ValueEditor allows editing a single configuration value.
type ValueEditor struct {
	path          string
	value         *config.Value
	input         textinput.Model
	metadata      map[string]any
	componentName string
	disabled      bool
	dirty         bool
}

// NewValueEditor creates an empty value editor.
func NewValueEditor() ValueEditor {
	ti := textinput.New()
	return ValueEditor{input: ti}
}

// NewValueEditorFor creates a value editor for a specific path.
func NewValueEditorFor(path string, value *config.Value, componentName string, disabled bool) ValueEditor {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = 60

	if value != nil {
		ti.SetValue(fmt.Sprintf("%v", value.Val))
	}

	var meta map[string]any
	if value != nil {
		meta = value.Metadata
	}

	return ValueEditor{
		path:          path,
		value:         value,
		input:         ti,
		metadata:      meta,
		componentName: componentName,
		disabled:      disabled,
	}
}

// Init returns the initial command for the editor (focus the text input).
func (e ValueEditor) Init() tea.Cmd {
	return textinput.Blink
}

// UpdateEditor handles messages for the value editor.
func (e ValueEditor) UpdateEditor(msg tea.Msg) (ValueEditor, tea.Cmd) {
	var cmd tea.Cmd
	e.input, cmd = e.input.Update(msg)

	// Track dirty state.
	if e.value != nil {
		currentVal := fmt.Sprintf("%v", e.value.Val)
		e.dirty = e.input.Value() != currentVal
	} else {
		e.dirty = e.input.Value() != ""
	}

	return e, cmd
}

// CommitValue returns the edited value.
func (e *ValueEditor) CommitValue() config.Value {
	val := config.Value{
		Val:      e.input.Value(),
		Metadata: e.metadata,
	}
	return val
}

// View renders the value editor.
func (e ValueEditor) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(HeaderStyle.Render(fmt.Sprintf(" Editing: %s ", e.path)))
	sb.WriteString("\n\n")

	if e.componentName != "" {
		badge := ComponentBadgeStyle.Render("[" + e.componentName + "]")
		sb.WriteString(fmt.Sprintf("  Component: %s\n", badge))
		if e.disabled {
			sb.WriteString(WarningStyle.Render("  Warning: This path belongs to disabled component '" + e.componentName + "'. It will not be included in exports."))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  Value:\n")
	sb.WriteString(fmt.Sprintf("  %s\n", e.input.View()))
	sb.WriteString("\n")

	if e.dirty {
		sb.WriteString(InfoStyle.Render("  (modified)"))
		sb.WriteString("\n")
	}

	if len(e.metadata) > 0 {
		sb.WriteString("\n")
		sb.WriteString(DimStyle.Render("  Metadata:"))
		sb.WriteString("\n")
		for k, v := range e.metadata {
			sb.WriteString(DimStyle.Render(fmt.Sprintf("    %s: %v", k, v)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(DimStyle.Render("  Press Enter to save, Esc to cancel"))
	sb.WriteString("\n")

	return sb.String()
}
