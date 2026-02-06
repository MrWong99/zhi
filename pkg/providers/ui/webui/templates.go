package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

type contextKey int

const (
	nonceCtxKey contextKey = iota
	csrfCtxKey
)

// templateEngine parses and renders HTML templates from the embedded FS.
type templateEngine struct {
	layout    *template.Template
	pages     map[string]*template.Template
	fragments map[string]*template.Template
}

// newTemplateEngine parses all templates from the embedded filesystem.
func newTemplateEngine() (*templateEngine, error) {
	funcMap := template.FuncMap{
		"csrfToken":    csrfTokenFunc,
		"csrfField":    csrfFieldFunc,
		"nonce":        nonceFunc,
		"icon":         iconFunc,
		"activeNav":    activeNavFunc,
		"json":         jsonFunc,
		"pathSegments": pathSegmentsFunc,
		"pathID":       pathToID,
	}

	// Parse shared templates (layout, sidebar, topbar) and all fragments.
	// Fragment files are included here so page templates can reference them.
	sharedFiles := []string{
		"templates/layout.html",
		"templates/sidebar.html",
		"templates/topbar.html",
	}

	// Discover all fragment files to include in the shared set.
	fragFiles, err := fs.Glob(templateFS, "templates/fragments/*.html")
	if err != nil {
		return nil, fmt.Errorf("globbing fragments: %w", err)
	}
	sharedFiles = append(sharedFiles, fragFiles...)

	e := &templateEngine{
		pages:     make(map[string]*template.Template),
		fragments: make(map[string]*template.Template),
	}

	// Parse the base layout with shared templates.
	base, err := template.New("layout.html").Funcs(funcMap).ParseFS(templateFS, sharedFiles...)
	if err != nil {
		return nil, fmt.Errorf("parsing layout templates: %w", err)
	}
	e.layout = base

	// Parse each page template by cloning the base and adding the page.
	pageFiles, err := fs.Glob(templateFS, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("globbing pages: %w", err)
	}
	for _, pf := range pageFiles {
		name := strings.TrimPrefix(pf, "templates/pages/")
		name = strings.TrimSuffix(name, ".html")
		clone, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("cloning base for %s: %w", name, err)
		}
		_, err = clone.ParseFS(templateFS, pf)
		if err != nil {
			return nil, fmt.Errorf("parsing page %s: %w", name, err)
		}
		e.pages[name] = clone
	}

	// Parse all fragment templates together so they can reference each
	// other (e.g. tree_content references tree_node).
	if len(fragFiles) > 0 {
		allFrags, err := template.New("fragments").Funcs(funcMap).ParseFS(templateFS, fragFiles...)
		if err != nil {
			return nil, fmt.Errorf("parsing fragments: %w", err)
		}
		for _, ff := range fragFiles {
			name := strings.TrimPrefix(ff, "templates/fragments/")
			name = strings.TrimSuffix(name, ".html")
			e.fragments[name] = allFrags
		}
	}

	return e, nil
}

// renderPage renders a full page (layout + page content).
func (e *templateEngine) renderPage(w http.ResponseWriter, name string, data any) error {
	tmpl, ok := e.pages[name]
	if !ok {
		return fmt.Errorf("page template %q not found", name)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		return fmt.Errorf("executing page %s: %w", name, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(buf.Bytes())
	return err
}

// renderFragment renders only a named template block (for HTMX partial updates).
func (e *templateEngine) renderFragment(w http.ResponseWriter, name string, data any) error {
	tmpl, ok := e.fragments[name]
	if !ok {
		return fmt.Errorf("fragment template %q not found", name)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("executing fragment %s: %w", name, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(buf.Bytes())
	return err
}

// isHTMX checks whether the request was made by HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// nonceFromCtx retrieves the CSP nonce from the request context.
func nonceFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(nonceCtxKey).(string); ok {
		return v
	}
	return ""
}

// csrfFromCtx retrieves the CSRF token from the request context.
func csrfFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(csrfCtxKey).(string); ok {
		return v
	}
	return ""
}

// Template function implementations. Each receives the page data which
// carries context values via the PageData struct.

func csrfTokenFunc(data any) string {
	if pd, ok := data.(pageData); ok {
		return pd.CSRFToken
	}
	return ""
}

func csrfFieldFunc(data any) template.HTML {
	token := csrfTokenFunc(data)
	//nolint:gosec // This is a generated CSRF token embedded in a hidden field.
	return template.HTML(fmt.Sprintf(`<input type="hidden" name="_csrf" value="%s">`, template.HTMLEscapeString(token)))
}

func nonceFunc(data any) string {
	if pd, ok := data.(pageData); ok {
		return pd.Nonce
	}
	return ""
}

func iconFunc(name string) template.HTML {
	icons := map[string]string{
		"folder":   `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>`,
		"file":     `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>`,
		"sun":      `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>`,
		"search":   `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>`,
		"alert":    `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>`,
		"check":    `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`,
		"grid":     `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>`,
		"save":     `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>`,
		"keyboard": `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="20" height="16" rx="2" ry="2"/><path d="M6 8h.001M10 8h.001M14 8h.001M18 8h.001M8 12h.001M12 12h.001M16 12h.001M7 16h10"/></svg>`,
		"play":     `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>`,
	}
	if svg, ok := icons[name]; ok {
		//nolint:gosec // SVG icons are static, developer-controlled content.
		return template.HTML(svg)
	}
	return ""
}

func activeNavFunc(data any, name string) string {
	if pd, ok := data.(pageData); ok {
		if pd.ActiveNav == name {
			return "active"
		}
	}
	return ""
}

func jsonFunc(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func pathSegmentsFunc(path string) []string {
	return strings.Split(path, "/")
}

// pageData is the common data structure passed to all page templates.
type pageData struct {
	WorkspaceName string
	ActiveNav     string
	Nonce         string
	CSRFToken     string
	Breadcrumbs   []breadcrumb

	// Tree page data.
	TreeNodes []*treeNode
	Filter    string

	// Validation page data.
	ValidationGroups []validationGroup
	ValidationCount  int

	// Components page data.
	Components []componentViewData

	// Export page data.
	ExportTemplates []exportTemplateData
	ExportPreview   *exportPreviewData
	ExportResult    *exportResultData

	// Apply page data.
	ApplyTargets []applyTargetData

	// Error page data.
	StatusCode int
	StatusText string
	Message    string
}

type breadcrumb struct {
	Label string
	Href  string
}
