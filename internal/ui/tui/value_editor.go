package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
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
	readonly      bool
	confirming    bool   // true when awaiting y/n confirmation
	patternErr    string // current pattern validation error
	enumOptions   []string
	enumCursor    int
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

	var meta map[string]any
	if value != nil {
		meta = value.Metadata
	}

	isReadonly := labels.IsReadonly(meta) || labels.IsImmutable(meta)

	if value != nil {
		ti.SetValue(fmt.Sprintf("%v", value.Val))
	}

	// Apply label-driven input configuration.
	if labels.IsPassword(meta) {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
	}

	if placeholder := labels.GetPlaceholder(meta); placeholder != "" {
		ti.Placeholder = placeholder
	}

	enumOpts := labels.GetEnum(meta)
	enumCursor := 0
	if len(enumOpts) > 0 && value != nil {
		// Find the current value in enum options.
		current := fmt.Sprintf("%v", value.Val)
		for i, opt := range enumOpts {
			if opt == current {
				enumCursor = i
				break
			}
		}
	}

	return ValueEditor{
		path:          path,
		value:         value,
		input:         ti,
		metadata:      meta,
		componentName: componentName,
		disabled:      disabled,
		readonly:      isReadonly,
		enumOptions:   enumOpts,
		enumCursor:    enumCursor,
	}
}

// Init returns the initial command for the editor (focus the text input).
func (e ValueEditor) Init() tea.Cmd {
	return textinput.Blink
}

// UpdateEditor handles messages for the value editor.
func (e ValueEditor) UpdateEditor(msg tea.Msg) (ValueEditor, tea.Cmd) {
	if e.confirming {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				e.confirming = false
				// Return with dirty=true so the caller commits.
				return e, nil
			case "n", "N", "esc":
				e.confirming = false
				e.dirty = false
				return e, nil
			}
		}
		return e, nil
	}

	// For enum values, handle j/k selection instead of free text.
	if len(e.enumOptions) > 0 {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "j", "down":
				if e.enumCursor < len(e.enumOptions)-1 {
					e.enumCursor++
					e.input.SetValue(e.enumOptions[e.enumCursor])
				}
				e.updateDirty()
				return e, nil
			case "k", "up":
				if e.enumCursor > 0 {
					e.enumCursor--
					e.input.SetValue(e.enumOptions[e.enumCursor])
				}
				e.updateDirty()
				return e, nil
			}
		}
	}

	if e.readonly {
		// Don't process input changes for readonly values.
		return e, nil
	}

	var cmd tea.Cmd
	e.input, cmd = e.input.Update(msg)

	e.updateDirty()
	e.validatePattern()

	return e, cmd
}

func (e *ValueEditor) updateDirty() {
	if e.value != nil {
		currentVal := fmt.Sprintf("%v", e.value.Val)
		e.dirty = e.input.Value() != currentVal
	} else {
		e.dirty = e.input.Value() != ""
	}
}

func (e *ValueEditor) validatePattern() {
	e.patternErr = ""
	pattern := labels.GetPattern(e.metadata)
	if pattern == "" {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.patternErr = fmt.Sprintf("invalid pattern: %s", err)
		return
	}
	val := e.input.Value()
	if val != "" && !re.MatchString(val) {
		e.patternErr = fmt.Sprintf("does not match pattern: %s", pattern)
	}
}

// NeedsConfirmation returns true if this value requires confirmation and has
// been modified.
func (e *ValueEditor) NeedsConfirmation() bool {
	return labels.ShouldConfirm(e.metadata) && e.dirty && !e.confirming
}

// StartConfirmation enters the confirmation state.
func (e *ValueEditor) StartConfirmation() {
	e.confirming = true
}

// IsConfirming returns whether the editor is waiting for confirmation.
func (e *ValueEditor) IsConfirming() bool {
	return e.confirming
}

// HasPatternError returns whether the current value has a pattern validation error.
func (e *ValueEditor) HasPatternError() bool {
	return e.patternErr != ""
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

	// Show description if available.
	if desc := labels.GetDescription(e.metadata); desc != "" {
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  %s", desc)))
		sb.WriteString("\n\n")
	}

	// Show deprecation warning.
	if msg := labels.GetString(e.metadata, labels.LabelCoreDeprecated, ""); msg != "" {
		sb.WriteString(WarningStyle.Render(fmt.Sprintf("  ⚠ Deprecated: %s", msg)))
		sb.WriteString("\n\n")
	}

	if e.componentName != "" {
		badge := ComponentBadgeStyle.Render("[" + e.componentName + "]")
		sb.WriteString(fmt.Sprintf("  Component: %s\n", badge))
		if e.disabled {
			sb.WriteString(WarningStyle.Render("  Warning: This path belongs to disabled component '" + e.componentName + "'. It will not be included in exports."))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Label badges line.
	var inlineBadges []string
	if e.readonly {
		inlineBadges = append(inlineBadges, ReadonlyBadgeStyle.Render("[readonly]"))
	}
	if labels.IsRequired(e.metadata) {
		inlineBadges = append(inlineBadges, RequiredBadgeStyle.Render("[required]"))
	}
	if labels.ShouldConfirm(e.metadata) {
		inlineBadges = append(inlineBadges, ConfirmStyle.Render("[requires confirmation]"))
	}
	if st := labels.GetSemanticType(e.metadata); st != "" {
		inlineBadges = append(inlineBadges, DimStyle.Render("[type: "+st+"]"))
	}
	if unit := labels.GetString(e.metadata, labels.LabelCoreUnit, ""); unit != "" {
		inlineBadges = append(inlineBadges, DimStyle.Render("[unit: "+unit+"]"))
	}
	if len(inlineBadges) > 0 {
		sb.WriteString("  " + strings.Join(inlineBadges, " "))
		sb.WriteString("\n\n")
	}

	// Show doc URL if available.
	if doc := labels.GetDocURL(e.metadata); doc != "" {
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  Docs: %s", doc)))
		sb.WriteString("\n\n")
	}

	// Enum selection mode.
	if len(e.enumOptions) > 0 {
		sb.WriteString("  Value (select with j/k):\n")
		for i, opt := range e.enumOptions {
			marker := "  "
			if i == e.enumCursor {
				marker = "> "
			}
			style := ValueStyle
			if i == e.enumCursor {
				style = ActiveStyle
			}
			sb.WriteString(fmt.Sprintf("  %s%s\n", marker, style.Render(opt)))
		}
	} else {
		sb.WriteString("  Value:\n")
		sb.WriteString(fmt.Sprintf("  %s\n", e.input.View()))
	}
	sb.WriteString("\n")

	// Pattern error.
	if e.patternErr != "" {
		sb.WriteString(ErrorStyle.Render(fmt.Sprintf("  Pattern error: %s", e.patternErr)))
		sb.WriteString("\n")
	}

	// Example hint.
	if example, ok := labels.GetAny(e.metadata, labels.LabelCoreExample); ok {
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  Example: %v", example)))
		sb.WriteString("\n")
	}

	if e.dirty {
		sb.WriteString(InfoStyle.Render("  (modified)"))
		sb.WriteString("\n")
	}

	if e.confirming {
		sb.WriteString("\n")
		sb.WriteString(WarningStyle.Render("  Are you sure you want to change this value? (y/n)"))
		sb.WriteString("\n")
	}

	// Show remaining metadata that isn't already rendered as a badge/field.
	displayedLabels := map[string]bool{
		labels.LabelCoreDescription: true,
		labels.LabelCoreDeprecated:  true,
		labels.LabelCoreDoc:         true,
		labels.LabelCoreExample:     true,
		labels.LabelCoreType:        true,
		labels.LabelCoreUnit:        true,
		labels.LabelUIReadonly:      true,
		labels.LabelUIPassword:      true,
		labels.LabelUIHidden:        true,
		labels.LabelUIPattern:       true,
		labels.LabelUIPlaceholder:   true,
		labels.LabelUIConfirm:       true,
		labels.LabelUIDisplayName:   true,
		labels.LabelUIEnum:          true,
		labels.LabelUISection:       true,
		labels.LabelUIShowIf:        true,
		labels.LabelUIMultiline:     true,
		labels.LabelUIOrder:         true,
		labels.LabelUIGroup:         true,
		labels.LabelUIFormat:        true,
		labels.LabelConfigRequired:  true,
		labels.LabelConfigImmutable: true,
	}

	var extraMeta []string
	for k, v := range e.metadata {
		if displayedLabels[k] {
			continue
		}
		extraMeta = append(extraMeta, fmt.Sprintf("    %s: %v", k, v))
	}
	if len(extraMeta) > 0 {
		sb.WriteString("\n")
		sb.WriteString(DimStyle.Render("  Metadata:"))
		sb.WriteString("\n")
		for _, m := range extraMeta {
			sb.WriteString(DimStyle.Render(m))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	if e.readonly {
		sb.WriteString(DimStyle.Render("  This value is read-only. Press Esc to go back."))
	} else if len(e.enumOptions) > 0 {
		sb.WriteString(DimStyle.Render("  j/k: select  Enter: save  Esc: cancel"))
	} else {
		sb.WriteString(DimStyle.Render("  Press Enter to save, Esc to cancel"))
	}
	sb.WriteString("\n")

	return sb.String()
}
