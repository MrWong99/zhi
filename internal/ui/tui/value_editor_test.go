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

func TestValueEditor_Readonly(t *testing.T) {
	val := &config.Value{
		Val:      "locked-value",
		Metadata: map[string]any{"ui.readonly": true},
	}

	editor := tui.NewValueEditorFor("test/readonly", val, "", false)

	// Try to type something - should be blocked.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	committed := editor.CommitValue()
	// The value should not have changed.
	if committed.Val != "locked-value" {
		t.Errorf("readonly editor should not change value, got %v", committed.Val)
	}

	view := editor.View()
	if !strings.Contains(view, "read-only") {
		t.Error("expected read-only notice in view")
	}
	if !strings.Contains(view, "[readonly]") {
		t.Error("expected [readonly] badge")
	}
}

func TestValueEditor_Immutable(t *testing.T) {
	val := &config.Value{
		Val:      "immutable-value",
		Metadata: map[string]any{"config.immutable": true},
	}

	editor := tui.NewValueEditorFor("test/immutable", val, "", false)

	// Try to type something - should be blocked.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	committed := editor.CommitValue()
	if committed.Val != "immutable-value" {
		t.Errorf("immutable editor should not change value, got %v", committed.Val)
	}
}

func TestValueEditor_Description(t *testing.T) {
	val := &config.Value{
		Val:      "value",
		Metadata: map[string]any{"core.description": "This is a helpful description"},
	}

	editor := tui.NewValueEditorFor("test/desc", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "This is a helpful description") {
		t.Error("expected description to be shown")
	}
}

func TestValueEditor_Deprecated(t *testing.T) {
	val := &config.Value{
		Val:      "old-value",
		Metadata: map[string]any{"core.deprecated": "Use test/new-field instead"},
	}

	editor := tui.NewValueEditorFor("test/old", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "Deprecated") {
		t.Error("expected deprecation warning")
	}
	if !strings.Contains(view, "Use test/new-field instead") {
		t.Error("expected migration guidance in deprecation warning")
	}
}

func TestValueEditor_PatternValidation(t *testing.T) {
	val := &config.Value{
		Val:      "",
		Metadata: map[string]any{"ui.pattern": `^\d+$`},
	}

	editor := tui.NewValueEditorFor("test/port", val, "", false)

	// Type invalid input.
	for _, ch := range "abc" {
		editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	view := editor.View()
	if !strings.Contains(view, "Pattern error") {
		t.Error("expected pattern error for non-numeric input with numeric pattern")
	}
	if !editor.HasPatternError() {
		t.Error("HasPatternError() should return true for invalid input")
	}
}

func TestValueEditor_PatternValidation_Valid(t *testing.T) {
	val := &config.Value{
		Val:      "",
		Metadata: map[string]any{"ui.pattern": `^\d+$`},
	}

	editor := tui.NewValueEditorFor("test/port", val, "", false)

	// Type valid input.
	for _, ch := range "1234" {
		editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	if editor.HasPatternError() {
		t.Error("HasPatternError() should return false for valid numeric input")
	}
}

func TestValueEditor_Confirmation(t *testing.T) {
	val := &config.Value{
		Val:      "danger",
		Metadata: map[string]any{"ui.confirm": true},
	}

	editor := tui.NewValueEditorFor("test/dangerous", val, "", false)

	// Type new value.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	// Should need confirmation.
	if !editor.NeedsConfirmation() {
		t.Error("NeedsConfirmation() should be true when dirty and ui.confirm=true")
	}

	// Start confirmation.
	editor.StartConfirmation()
	if !editor.IsConfirming() {
		t.Error("IsConfirming() should be true after StartConfirmation()")
	}

	view := editor.View()
	if !strings.Contains(view, "Are you sure") {
		t.Error("expected confirmation prompt in view")
	}

	// Decline with 'n'.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if editor.IsConfirming() {
		t.Error("should no longer be confirming after pressing 'n'")
	}
}

func TestValueEditor_Confirmation_Accept(t *testing.T) {
	val := &config.Value{
		Val:      "danger",
		Metadata: map[string]any{"ui.confirm": true},
	}

	editor := tui.NewValueEditorFor("test/dangerous", val, "", false)

	// Type new value.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	editor.StartConfirmation()

	// Accept with 'y'.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if editor.IsConfirming() {
		t.Error("should no longer be confirming after pressing 'y'")
	}
}

func TestValueEditor_EnumSelection(t *testing.T) {
	val := &config.Value{
		Val:      "postgres",
		Metadata: map[string]any{"ui.enum": []string{"postgres", "mysql", "sqlite"}},
	}

	editor := tui.NewValueEditorFor("database/type", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "postgres") {
		t.Error("expected postgres in enum options")
	}
	if !strings.Contains(view, "mysql") {
		t.Error("expected mysql in enum options")
	}
	if !strings.Contains(view, "sqlite") {
		t.Error("expected sqlite in enum options")
	}
	if !strings.Contains(view, "select with j/k") {
		t.Error("expected enum selection hint")
	}

	// Navigate to mysql.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	committed := editor.CommitValue()
	if committed.Val != "mysql" {
		t.Errorf("expected committed value to be 'mysql' after j, got %v", committed.Val)
	}

	// Navigate to sqlite.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	committed = editor.CommitValue()
	if committed.Val != "sqlite" {
		t.Errorf("expected committed value to be 'sqlite' after j, got %v", committed.Val)
	}

	// Navigate back up.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	committed = editor.CommitValue()
	if committed.Val != "mysql" {
		t.Errorf("expected committed value to be 'mysql' after k, got %v", committed.Val)
	}
}

func TestValueEditor_RequiredBadge(t *testing.T) {
	val := &config.Value{
		Val:      "",
		Metadata: map[string]any{"config.required": true},
	}

	editor := tui.NewValueEditorFor("test/required", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "[required]") {
		t.Error("expected [required] badge in editor view")
	}
}

func TestValueEditor_SemanticType(t *testing.T) {
	val := &config.Value{
		Val:      "user@example.com",
		Metadata: map[string]any{"core.type": "email"},
	}

	editor := tui.NewValueEditorFor("test/email", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "type: email") {
		t.Error("expected semantic type badge in editor view")
	}
}

func TestValueEditor_DocURL(t *testing.T) {
	val := &config.Value{
		Val:      "value",
		Metadata: map[string]any{"core.doc": "https://example.com/docs"},
	}

	editor := tui.NewValueEditorFor("test/documented", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "https://example.com/docs") {
		t.Error("expected doc URL in editor view")
	}
}

func TestValueEditor_Example(t *testing.T) {
	val := &config.Value{
		Val:      "",
		Metadata: map[string]any{"core.example": "user@example.com"},
	}

	editor := tui.NewValueEditorFor("test/email", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "Example: user@example.com") {
		t.Error("expected example value in editor view")
	}
}

func TestValueEditor_MapDisplay(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      map[string]string{"app": "web", "env": "prod"},
		Metadata: map[string]any{"core.type": "map"},
	}

	editor := tui.NewValueEditorFor("test/labels", val, "", false)

	if !editor.IsCollectionEditor() {
		t.Error("expected IsCollectionEditor() to be true for map type")
	}

	view := editor.View()
	if !strings.Contains(view, "app = web") {
		t.Error("expected 'app = web' in map editor view")
	}
	if !strings.Contains(view, "env = prod") {
		t.Error("expected 'env = prod' in map editor view")
	}
	if !strings.Contains(view, "a: add") {
		t.Error("expected add hint in map editor view")
	}
	if !strings.Contains(view, "d: delete") {
		t.Error("expected delete hint in map editor view")
	}
}

func TestValueEditor_MapCommitValue(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      map[string]string{"key1": "val1"},
		Metadata: map[string]any{"core.type": "map"},
	}

	editor := tui.NewValueEditorFor("test/map", val, "", false)

	committed := editor.CommitValue()
	m, ok := committed.Val.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", committed.Val)
	}
	if m["key1"] != "val1" {
		t.Errorf("expected key1=val1, got key1=%s", m["key1"])
	}
}

func TestValueEditor_MapAddEntry(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      map[string]string{},
		Metadata: map[string]any{"core.type": "map"},
	}

	editor := tui.NewValueEditorFor("test/map", val, "", false)

	// Press 'a' to add a new entry.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if !editor.IsEditingInline() {
		t.Error("expected to be editing inline after pressing 'a'")
	}

	// Type the key.
	for _, ch := range "newkey" {
		editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	// Confirm with enter.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyEnter})

	if editor.IsEditingInline() {
		t.Error("should not be editing inline after enter")
	}

	committed := editor.CommitValue()
	m, ok := committed.Val.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", committed.Val)
	}
	if _, exists := m["newkey"]; !exists {
		t.Error("expected 'newkey' to exist in committed map")
	}
}

func TestValueEditor_MapDeleteEntry(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      map[string]string{"remove": "me", "keep": "this"},
		Metadata: map[string]any{"core.type": "map"},
	}

	editor := tui.NewValueEditorFor("test/map", val, "", false)

	// Entries are sorted, so cursor 0 = "keep", cursor 1 = "remove".
	// Navigate to "remove".
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	// Delete it.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	committed := editor.CommitValue()
	m, ok := committed.Val.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", committed.Val)
	}
	if _, exists := m["remove"]; exists {
		t.Error("expected 'remove' key to be deleted")
	}
	if m["keep"] != "this" {
		t.Error("expected 'keep' key to remain")
	}
}

func TestValueEditor_MapReadonly(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      map[string]string{"k": "v"},
		Metadata: map[string]any{"core.type": "map", "ui.readonly": true},
	}

	editor := tui.NewValueEditorFor("test/map-ro", val, "", false)

	// Try to add - should be blocked.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	committed := editor.CommitValue()
	m, ok := committed.Val.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", committed.Val)
	}
	if len(m) != 1 {
		t.Errorf("readonly map should not be modified, got %d entries", len(m))
	}
}

func TestValueEditor_ListDisplay(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      []string{"alpha", "beta"},
		Metadata: map[string]any{"core.type": "list"},
	}

	editor := tui.NewValueEditorFor("test/list", val, "", false)

	if !editor.IsCollectionEditor() {
		t.Error("expected IsCollectionEditor() to be true for list type")
	}

	view := editor.View()
	if !strings.Contains(view, "alpha") {
		t.Error("expected 'alpha' in list editor view")
	}
	if !strings.Contains(view, "beta") {
		t.Error("expected 'beta' in list editor view")
	}
}

func TestValueEditor_ListCommitValue(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      []string{"item1", "item2"},
		Metadata: map[string]any{"core.type": "list"},
	}

	editor := tui.NewValueEditorFor("test/list", val, "", false)

	committed := editor.CommitValue()
	items, ok := committed.Val.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", committed.Val)
	}
	if len(items) != 2 || items[0] != "item1" || items[1] != "item2" {
		t.Errorf("expected [item1, item2], got %v", items)
	}
}

func TestValueEditor_ListAddItem(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      []string{},
		Metadata: map[string]any{"core.type": "list"},
	}

	editor := tui.NewValueEditorFor("test/list", val, "", false)

	// Press 'a' to add.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if !editor.IsEditingInline() {
		t.Error("expected inline editing after 'a'")
	}

	// Type item value.
	for _, ch := range "newitem" {
		editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyEnter})

	committed := editor.CommitValue()
	items, ok := committed.Val.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", committed.Val)
	}
	if len(items) != 1 || items[0] != "newitem" {
		t.Errorf("expected [newitem], got %v", items)
	}
}

func TestValueEditor_ListDeleteItem(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      []string{"keep", "remove"},
		Metadata: map[string]any{"core.type": "list"},
	}

	editor := tui.NewValueEditorFor("test/list", val, "", false)

	// Navigate to "remove" (index 1).
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	// Delete.
	editor, _ = editor.UpdateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	committed := editor.CommitValue()
	items, ok := committed.Val.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", committed.Val)
	}
	if len(items) != 1 || items[0] != "keep" {
		t.Errorf("expected [keep], got %v", items)
	}
}

func TestValueEditor_MapEmptyDisplay(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      map[string]string{},
		Metadata: map[string]any{"core.type": "map"},
	}

	editor := tui.NewValueEditorFor("test/map-empty", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "(empty map)") {
		t.Error("expected empty map message")
	}
}

func TestValueEditor_ListEmptyDisplay(t *testing.T) {
	t.Parallel()
	val := &config.Value{
		Val:      []string{},
		Metadata: map[string]any{"core.type": "list"},
	}

	editor := tui.NewValueEditorFor("test/list-empty", val, "", false)

	view := editor.View()
	if !strings.Contains(view, "(empty list)") {
		t.Error("expected empty list message")
	}
}
