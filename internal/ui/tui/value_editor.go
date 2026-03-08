package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

// mapEditEntry is a key-value pair for the TUI map editor.
type mapEditEntry struct {
	Key   string
	Value string
}

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

	// Map/list editor state.
	isMap       bool
	isList      bool
	mapEntries  []mapEditEntry
	listItems   []string
	collCursor  int  // cursor position in map/list entries
	editingColl bool // true when editing an entry inline
	editField   int  // 0=editing key, 1=editing value (map only)
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

	ed := ValueEditor{
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

	// Map type: extract entries for interactive editing.
	if labels.IsMapType(meta) {
		ed.isMap = true
		if value != nil {
			ed.mapEntries = extractMapEditEntries(value.Val)
		}
	}

	// List type: extract items for interactive editing.
	if labels.IsListType(meta) {
		ed.isList = true
		if value != nil {
			ed.listItems = extractListEditItems(value.Val)
		}
	}

	return ed
}

// extractMapEditEntries converts a value to sorted mapEditEntry slice.
func extractMapEditEntries(v any) []mapEditEntry {
	var entries []mapEditEntry
	switch m := v.(type) {
	case map[string]any:
		for k, val := range m {
			entries = append(entries, mapEditEntry{Key: k, Value: fmt.Sprintf("%v", val)})
		}
	case map[string]string:
		for k, val := range m {
			entries = append(entries, mapEditEntry{Key: k, Value: val})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries
}

// extractListEditItems converts a value to a string slice.
func extractListEditItems(v any) []string {
	switch items := v.(type) {
	case []string:
		result := make([]string, len(items))
		copy(result, items)
		return result
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	}
	return nil
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

	// Map editor mode.
	if e.isMap {
		return e.updateMapEditor(msg)
	}

	// List editor mode.
	if e.isList {
		return e.updateListEditor(msg)
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

func (e ValueEditor) updateMapEditor(msg tea.Msg) (ValueEditor, tea.Cmd) {
	if e.readonly {
		return e, nil
	}

	// When editing a field inline, delegate to the text input.
	if e.editingColl {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				// Commit the inline edit.
				val := e.input.Value()
				if e.collCursor < len(e.mapEntries) {
					if e.editField == 0 {
						e.mapEntries[e.collCursor].Key = val
					} else {
						e.mapEntries[e.collCursor].Value = val
					}
				}
				e.editingColl = false
				e.input.Blur()
				e.dirty = true
				return e, nil
			case "esc":
				e.editingColl = false
				e.input.Blur()
				return e, nil
			}
		}
		var cmd tea.Cmd
		e.input, cmd = e.input.Update(msg)
		return e, cmd
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "j", "down":
			if e.collCursor < len(e.mapEntries)-1 {
				e.collCursor++
			}
			return e, nil
		case "k", "up":
			if e.collCursor > 0 {
				e.collCursor--
			}
			return e, nil
		case "enter":
			// Edit the selected entry's value.
			if len(e.mapEntries) > 0 {
				e.editingColl = true
				e.editField = 1
				e.input.SetValue(e.mapEntries[e.collCursor].Value)
				e.input.Focus()
			}
			return e, textinput.Blink
		case "e":
			// Edit the selected entry's key.
			if len(e.mapEntries) > 0 {
				e.editingColl = true
				e.editField = 0
				e.input.SetValue(e.mapEntries[e.collCursor].Key)
				e.input.Focus()
			}
			return e, textinput.Blink
		case "a":
			// Add a new entry.
			e.mapEntries = append(e.mapEntries, mapEditEntry{})
			e.collCursor = len(e.mapEntries) - 1
			e.editingColl = true
			e.editField = 0
			e.input.SetValue("")
			e.input.Focus()
			e.dirty = true
			return e, textinput.Blink
		case "d":
			// Delete the selected entry.
			if len(e.mapEntries) > 0 {
				e.mapEntries = append(e.mapEntries[:e.collCursor], e.mapEntries[e.collCursor+1:]...)
				if e.collCursor >= len(e.mapEntries) && e.collCursor > 0 {
					e.collCursor--
				}
				e.dirty = true
			}
			return e, nil
		}
	}
	return e, nil
}

func (e ValueEditor) updateListEditor(msg tea.Msg) (ValueEditor, tea.Cmd) {
	if e.readonly {
		return e, nil
	}

	// When editing an item inline, delegate to the text input.
	if e.editingColl {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				val := e.input.Value()
				if e.collCursor < len(e.listItems) {
					e.listItems[e.collCursor] = val
				}
				e.editingColl = false
				e.input.Blur()
				e.dirty = true
				return e, nil
			case "esc":
				e.editingColl = false
				e.input.Blur()
				return e, nil
			}
		}
		var cmd tea.Cmd
		e.input, cmd = e.input.Update(msg)
		return e, cmd
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "j", "down":
			if e.collCursor < len(e.listItems)-1 {
				e.collCursor++
			}
			return e, nil
		case "k", "up":
			if e.collCursor > 0 {
				e.collCursor--
			}
			return e, nil
		case "enter":
			// Edit the selected item.
			if len(e.listItems) > 0 {
				e.editingColl = true
				e.input.SetValue(e.listItems[e.collCursor])
				e.input.Focus()
			}
			return e, textinput.Blink
		case "a":
			// Add a new item.
			e.listItems = append(e.listItems, "")
			e.collCursor = len(e.listItems) - 1
			e.editingColl = true
			e.input.SetValue("")
			e.input.Focus()
			e.dirty = true
			return e, textinput.Blink
		case "d":
			// Delete the selected item.
			if len(e.listItems) > 0 {
				e.listItems = append(e.listItems[:e.collCursor], e.listItems[e.collCursor+1:]...)
				if e.collCursor >= len(e.listItems) && e.collCursor > 0 {
					e.collCursor--
				}
				e.dirty = true
			}
			return e, nil
		}
	}
	return e, nil
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

// IsCollectionEditor returns true if this editor is in map or list mode.
func (e *ValueEditor) IsCollectionEditor() bool {
	return e.isMap || e.isList
}

// IsEditingInline returns true if the user is editing an entry inline.
func (e *ValueEditor) IsEditingInline() bool {
	return e.editingColl
}

// CommitValue returns the edited value.
func (e *ValueEditor) CommitValue() config.Value {
	var v any
	switch {
	case e.isMap:
		m := make(map[string]string, len(e.mapEntries))
		for _, entry := range e.mapEntries {
			if entry.Key != "" {
				m[entry.Key] = entry.Value
			}
		}
		v = m
	case e.isList:
		items := make([]string, 0, len(e.listItems))
		for _, item := range e.listItems {
			if item != "" {
				items = append(items, item)
			}
		}
		v = items
	default:
		v = e.input.Value()
	}
	return config.Value{
		Val:      v,
		Metadata: e.metadata,
	}
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
		fmt.Fprintf(&sb, "  Component: %s\n", badge)
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

	// Map editor mode.
	if e.isMap {
		sb.WriteString("  Entries (j/k: navigate, a: add, d: delete, enter: edit value, e: edit key):\n")
		if len(e.mapEntries) == 0 {
			sb.WriteString(DimStyle.Render("    (empty map)"))
			sb.WriteString("\n")
		}
		for i, entry := range e.mapEntries {
			marker := "  "
			if i == e.collCursor {
				marker = "> "
			}
			style := ValueStyle
			if i == e.collCursor {
				style = ActiveStyle
			}
			if e.editingColl && i == e.collCursor {
				if e.editField == 0 {
					fmt.Fprintf(&sb, "  %s%s = %s\n", marker, e.input.View(), DimStyle.Render(entry.Value))
				} else {
					fmt.Fprintf(&sb, "  %s%s = %s\n", marker, DimStyle.Render(entry.Key), e.input.View())
				}
			} else {
				fmt.Fprintf(&sb, "  %s%s\n", marker, style.Render(fmt.Sprintf("%s = %s", entry.Key, entry.Value)))
			}
		}
		sb.WriteString("\n")
	} else if e.isList {
		// List editor mode.
		sb.WriteString("  Items (j/k: navigate, a: add, d: delete, enter: edit):\n")
		if len(e.listItems) == 0 {
			sb.WriteString(DimStyle.Render("    (empty list)"))
			sb.WriteString("\n")
		}
		for i, item := range e.listItems {
			marker := "  "
			if i == e.collCursor {
				marker = "> "
			}
			style := ValueStyle
			if i == e.collCursor {
				style = ActiveStyle
			}
			if e.editingColl && i == e.collCursor {
				fmt.Fprintf(&sb, "  %s%s\n", marker, e.input.View())
			} else {
				fmt.Fprintf(&sb, "  %s%s\n", marker, style.Render(item))
			}
		}
		sb.WriteString("\n")
	} else if len(e.enumOptions) > 0 {
		// Enum selection mode.
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
			fmt.Fprintf(&sb, "  %s%s\n", marker, style.Render(opt))
		}
	} else {
		sb.WriteString("  Value:\n")
		fmt.Fprintf(&sb, "  %s\n", e.input.View())
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
		labels.LabelCoreDescription:       true,
		labels.LabelCoreDeprecated:        true,
		labels.LabelCoreDoc:               true,
		labels.LabelCoreExample:           true,
		labels.LabelCoreType:              true,
		labels.LabelCoreUnit:              true,
		labels.LabelUIReadonly:            true,
		labels.LabelUIPassword:            true,
		labels.LabelUIHidden:              true,
		labels.LabelUIPattern:             true,
		labels.LabelUIPlaceholder:         true,
		labels.LabelUIConfirm:             true,
		labels.LabelUIDisplayName:         true,
		labels.LabelUIEnum:                true,
		labels.LabelUISection:             true,
		labels.LabelUIShowIf:              true,
		labels.LabelUIMultiline:           true,
		labels.LabelUIOrder:               true,
		labels.LabelUIGroup:               true,
		labels.LabelUIFormat:              true,
		labels.LabelUIMapKeyPlaceholder:   true,
		labels.LabelUIMapValuePlaceholder: true,
		labels.LabelUIListItemPlaceholder: true,
		labels.LabelUIYAMLSchema:          true,
		labels.LabelConfigRequired:        true,
		labels.LabelConfigImmutable:       true,
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
	} else if e.isMap || e.isList {
		if e.editingColl {
			sb.WriteString(DimStyle.Render("  Enter: confirm  Esc: cancel edit"))
		} else {
			sb.WriteString(DimStyle.Render("  j/k: navigate  a: add  d: delete  Enter: save  Esc: cancel"))
		}
	} else if len(e.enumOptions) > 0 {
		sb.WriteString(DimStyle.Render("  j/k: select  Enter: save  Esc: cancel"))
	} else {
		sb.WriteString(DimStyle.Render("  Press Enter to save, Esc to cancel"))
	}
	sb.WriteString("\n")

	return sb.String()
}
