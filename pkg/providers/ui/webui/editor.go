package webui

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// handleEditForm renders the value edit form for a given path (HTMX fragment).
// GET /tree/edit/{path...}
func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	tree, err := s.ctrl.LoadTree(ctx)
	if err != nil {
		http.Error(w, "failed to load tree", http.StatusInternalServerError)
		return
	}

	value, ok := tree.Get(path)
	if !ok {
		http.Error(w, "path not found", http.StatusNotFound)
		return
	}

	components, _ := s.ctrl.ListComponents(ctx)
	compMap := buildComponentMap(components)

	ed := newEditorData(path, value, compMap)
	if err := s.engine.renderFragment(w, "value_form", ed); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleSaveValue saves a value and returns the display or form fragment.
// POST /tree/values/{path...}
func (s *Server) handleSaveValue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	tree, err := s.ctrl.LoadTree(ctx)
	if err != nil {
		http.Error(w, "failed to load tree", http.StatusInternalServerError)
		return
	}

	// Preserve existing metadata.
	existing, _ := tree.Get(path)

	// Reject edits to readonly or immutable values.
	if labels.IsReadonly(existing.Metadata) || labels.IsImmutable(existing.Metadata) {
		http.Error(w, "this value is read-only", http.StatusForbidden)
		return
	}

	valType := r.FormValue("value_type")
	val := parseFormValueFromRequest(r, valType)

	newValue := config.Value{
		Val:      val,
		Metadata: existing.Metadata,
	}

	if err := s.ctrl.SetValue(ctx, path, newValue); err != nil {
		http.Error(w, "failed to set value: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Run validation and check for blocking errors on this path.
	results, _ := s.ctrl.Validate(ctx)
	pathResults := filterValidationForPath(results, path)
	blocking := hasBlockingResults(pathResults)

	components, _ := s.ctrl.ListComponents(ctx)
	compMap := buildComponentMap(components)

	if blocking {
		// Return the form with validation errors.
		ed := newEditorData(path, newValue, compMap)
		ed.ValidationResults = toValidationItems(pathResults)
		ed.Error = "Validation errors must be resolved"
		if err := s.engine.renderFragment(w, "value_form", ed); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Success: return display mode and mark unsaved.
	// Also trigger valueChanged so tree content can refresh (for showIf visibility).
	w.Header().Set("HX-Trigger", `{"markUnsaved": true, "valueChanged": true}`)
	ed := newEditorData(path, newValue, compMap)
	if err := s.engine.renderFragment(w, "value_display", ed); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleDisplayValue returns the read-only display fragment for a path.
// GET /tree/display/{path...}
func (s *Server) handleDisplayValue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	tree, err := s.ctrl.LoadTree(ctx)
	if err != nil {
		http.Error(w, "failed to load tree", http.StatusInternalServerError)
		return
	}

	value, ok := tree.Get(path)
	if !ok {
		http.Error(w, "path not found", http.StatusNotFound)
		return
	}

	components, _ := s.ctrl.ListComponents(ctx)
	compMap := buildComponentMap(components)

	ed := newEditorData(path, value, compMap)
	if err := s.engine.renderFragment(w, "value_display", ed); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleInlineValidation sets a value and returns validation results for that path.
// POST /validate/inline/{path...}
func (s *Server) handleInlineValidation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	valType := r.FormValue("value_type")
	val := parseFormValueFromRequest(r, valType)

	tree, err := s.ctrl.LoadTree(ctx)
	if err != nil {
		http.Error(w, "failed to load tree", http.StatusInternalServerError)
		return
	}

	existing, _ := tree.Get(path)
	newValue := config.Value{
		Val:      val,
		Metadata: existing.Metadata,
	}

	if err := s.ctrl.SetValue(ctx, path, newValue); err != nil {
		http.Error(w, "failed to set value: "+err.Error(), http.StatusInternalServerError)
		return
	}

	results, _ := s.ctrl.Validate(ctx)
	pathResults := filterValidationForPath(results, path)

	data := validationBadgeData{
		Results: toValidationItems(pathResults),
	}

	if err := s.engine.renderFragment(w, "validation_badge", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// editorData holds data for value_form and value_display fragments.
type editorData struct {
	Path              string
	PathID            string
	Name              string
	DisplayName       string // ui.displayName: human-readable name shown instead of raw path segment
	DisplayValue      string
	EditValue         string
	ValueType         string
	Component         string
	ComponentEnabled  bool
	Metadata          map[string]any
	IsMultiline       bool
	Options           []string // ui.enum: allowed values for dropdown
	Placeholder       string   // ui.placeholder
	IsReadonly        bool     // ui.readonly or config.immutable
	IsPassword        bool     // ui.password: mask input
	IsRequired        bool     // config.required
	IsDeprecated      bool     // core.deprecated is set
	DeprecatedMsg     string   // core.deprecated message text
	ShouldConfirm     bool     // ui.confirm: require confirmation before save
	Pattern           string   // ui.pattern: regex for input validation
	Description       string   // core.description
	DocURL            string   // core.doc: link to documentation
	SemanticType      string   // core.type: email, url, hostname, etc.
	Unit              string   // core.unit: seconds, MB, percent, etc.
	Example           string   // core.example
	Format            string   // ui.format: json, yaml, code, etc.
	Section           string   // ui.section
	Group             string   // ui.group
	Error             string
	ValidationResults []validationItem

	// Map editor fields (core.type: map)
	IsMap               bool
	MapEntries          []mapEntry
	MapKeyPlaceholder   string
	MapValuePlaceholder string

	// List editor fields (core.type: list)
	IsList              bool
	ListItems           []string
	ListItemPlaceholder string

	// YAML editor fields (core.type: yaml)
	IsYAML     bool
	YAMLSchema string
}

// mapEntry is a single key-value pair for the map editor.
type mapEntry struct {
	Key   string
	Value string
}

// validationBadgeData holds data for the validation_badge fragment.
type validationBadgeData struct {
	Results []validationItem
}

func newEditorData(path string, value config.Value, compMap map[string]ui.ComponentInfo) editorData {
	segments := strings.Split(path, "/")
	name := segments[len(segments)-1]

	meta := value.Metadata

	ed := editorData{
		Path:         path,
		PathID:       pathToID(path),
		Name:         name,
		DisplayValue: formatValue(value.Val),
		EditValue:    formatEditValue(value.Val),
		ValueType:    valueType(value.Val),
		Metadata:     meta,
	}

	// Component ownership.
	if comp, ok := findComponent(path, compMap); ok {
		ed.Component = comp.Name
		ed.ComponentEnabled = comp.Enabled
	}

	// Extract all metadata labels using the labels helpers.
	ed.DisplayName = labels.GetDisplayName(meta)
	ed.Placeholder = labels.GetPlaceholder(meta)
	ed.IsMultiline = labels.IsMultiline(meta)
	ed.IsReadonly = labels.IsReadonly(meta) || labels.IsImmutable(meta)
	ed.IsPassword = labels.IsPassword(meta)
	ed.IsRequired = labels.IsRequired(meta)
	ed.ShouldConfirm = labels.ShouldConfirm(meta)
	ed.Pattern = labels.GetPattern(meta)
	ed.Description = labels.GetDescription(meta)
	ed.DocURL = labels.GetDocURL(meta)
	ed.SemanticType = labels.GetSemanticType(meta)
	ed.Unit = labels.GetString(meta, labels.LabelCoreUnit, "")
	ed.Format = labels.GetString(meta, labels.LabelUIFormat, "")
	ed.Section = labels.GetSection(meta)
	ed.Group = labels.GetGroup(meta)

	// core.deprecated
	ed.DeprecatedMsg = labels.GetString(meta, labels.LabelCoreDeprecated, "")
	ed.IsDeprecated = ed.DeprecatedMsg != ""

	// core.example
	if ex, ok := labels.GetAny(meta, labels.LabelCoreExample); ok {
		ed.Example = fmt.Sprintf("%v", ex)
	}

	// ui.enum: dropdown options.
	if enumOpts := labels.GetEnum(meta); len(enumOpts) > 0 {
		ed.Options = enumOpts
	}

	// Password values: mask display.
	if ed.IsPassword {
		ed.DisplayValue = "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"
	}

	// Multiline detection: strings with newlines.
	if s, ok := value.Val.(string); ok && strings.Contains(s, "\n") {
		ed.IsMultiline = true
	}

	// Map type support.
	if labels.IsMapType(meta) {
		ed.IsMap = true
		ed.MapKeyPlaceholder = labels.GetMapKeyPlaceholder(meta)
		ed.MapValuePlaceholder = labels.GetMapValuePlaceholder(meta)
		ed.MapEntries = extractMapEntries(value.Val)
		ed.ValueType = "map"
	}

	// List type support.
	if labels.IsListType(meta) {
		ed.IsList = true
		ed.ListItemPlaceholder = labels.GetListItemPlaceholder(meta)
		ed.ListItems = extractListItems(value.Val)
		ed.ValueType = "list"
	}

	// YAML type support: implies multiline.
	if labels.IsYAMLType(meta) {
		ed.IsYAML = true
		ed.IsMultiline = true
		ed.YAMLSchema = labels.GetYAMLSchema(meta)
		ed.ValueType = "string"
	}

	return ed
}

// extractMapEntries converts a value to a sorted slice of mapEntry.
func extractMapEntries(v any) []mapEntry {
	var entries []mapEntry
	switch m := v.(type) {
	case map[string]any:
		for k, val := range m {
			entries = append(entries, mapEntry{Key: k, Value: fmt.Sprintf("%v", val)})
		}
	case map[string]string:
		for k, val := range m {
			entries = append(entries, mapEntry{Key: k, Value: val})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries
}

// extractListItems converts a value to a slice of strings.
func extractListItems(v any) []string {
	switch items := v.(type) {
	case []string:
		return items
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	}
	return nil
}

// formatEditValue returns the value formatted for form input fields.
func formatEditValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseFormValue converts a form string value to the appropriate Go type
// based on the declared value type.
func parseFormValue(raw string, valType string) any {
	switch valType {
	case "number":
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return f
		}
		return raw
	case "bool":
		return raw == "true" || raw == "on"
	default:
		return raw
	}
}

// parseFormValueFromRequest extracts the value from the request, handling
// map and list types which use multiple form fields.
func parseFormValueFromRequest(r *http.Request, valType string) any {
	switch valType {
	case "map":
		keys := r.Form["map_key"]
		values := r.Form["map_value"]
		result := make(map[string]string, len(keys))
		for i, k := range keys {
			if k == "" {
				continue
			}
			v := ""
			if i < len(values) {
				v = values[i]
			}
			result[k] = v
		}
		return result
	case "list":
		items := r.Form["list_item"]
		result := make([]string, 0, len(items))
		for _, item := range items {
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	default:
		return parseFormValue(r.FormValue("value"), valType)
	}
}

// pathToID converts a slash-delimited path to a valid HTML ID.
func pathToID(path string) string {
	return strings.ReplaceAll(path, "/", "--")
}
