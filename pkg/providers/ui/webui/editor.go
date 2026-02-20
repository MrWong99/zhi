package webui

import (
	"fmt"
	"net/http"
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

	rawValue := r.FormValue("value")
	valType := r.FormValue("value_type")
	val := parseFormValue(rawValue, valType)

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

	rawValue := r.FormValue("value")
	valType := r.FormValue("value_type")
	val := parseFormValue(rawValue, valType)

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

	return ed
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

// pathToID converts a slash-delimited path to a valid HTML ID.
func pathToID(path string) string {
	return strings.ReplaceAll(path, "/", "--")
}
