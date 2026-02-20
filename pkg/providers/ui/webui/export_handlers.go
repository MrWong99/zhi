package webui

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// handleExportPage renders the export template list page.
// GET /export
func (s *Server) handleExportPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	templates, err := s.ctrl.ExportTemplates(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load export templates: "+err.Error())
		return
	}

	wsName, _ := s.ctrl.WorkspaceName(ctx)

	data := pageData{
		WorkspaceName:   wsName,
		ActiveNav:       "export",
		Nonce:           nonceFromCtx(ctx),
		CSRFToken:       csrfFromCtx(ctx),
		Authenticated:   authenticatedFromCtx(ctx),
		ExportTemplates: toExportTemplateData(templates),
		Breadcrumbs: []breadcrumb{
			{Label: "Export", Href: "/export"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "export_content", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := s.engine.renderPage(w, "export", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleExportPreview returns a dry-run preview of an export.
// POST /export/preview
func (s *Server) handleExportPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	req := buildExportRequest(r)
	req.DryRun = true

	result, err := s.ctrl.Export(ctx, req)
	if err != nil {
		preview := exportPreviewData{
			Error: "Export preview failed: " + err.Error(),
		}
		if renderErr := s.engine.renderFragment(w, "export_preview", preview); renderErr != nil {
			http.Error(w, renderErr.Error(), http.StatusInternalServerError)
		}
		return
	}

	preview := exportPreviewData{
		Name:    result.Name,
		Content: result.Content,
		Format:  formatForCSS(req.Format),
	}

	if renderErr := s.engine.renderFragment(w, "export_preview", preview); renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

// handleExport executes an export and writes to the configured output path.
// POST /export
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	req := buildExportRequest(r)
	req.DryRun = false

	result, err := s.ctrl.Export(ctx, req)
	if err != nil {
		name := req.Format
		if result != nil && result.Name != "" {
			name = result.Name
		}
		notif := notificationEvent{
			Type:    "error",
			Message: fmt.Sprintf("Export %s failed: %s", name, err.Error()),
		}
		b, _ := json.Marshal(map[string]any{"showNotification": notif})
		w.Header().Set("HX-Trigger", string(b))
		w.WriteHeader(http.StatusOK)
		return
	}

	notif := notificationEvent{
		Type:    "success",
		Message: fmt.Sprintf("Exported %s to %s", result.Name, result.OutputPath),
	}
	b, _ := json.Marshal(map[string]any{"showNotification": notif})
	w.Header().Set("HX-Trigger", string(b))
	w.WriteHeader(http.StatusOK)
}

// handleExportAll exports all configured templates.
// POST /export/all
func (s *Server) handleExportAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	templates, err := s.ctrl.ExportTemplates(ctx)
	if err != nil {
		notif := notificationEvent{
			Type:    "error",
			Message: "Failed to load export templates: " + err.Error(),
		}
		b, _ := json.Marshal(map[string]any{"showNotification": notif})
		w.Header().Set("HX-Trigger", string(b))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(templates) == 0 {
		notif := notificationEvent{
			Type:    "warning",
			Message: "No export templates configured",
		}
		b, _ := json.Marshal(map[string]any{"showNotification": notif})
		w.Header().Set("HX-Trigger", string(b))
		w.WriteHeader(http.StatusOK)
		return
	}

	var exported int
	var lastErr error
	for _, tmpl := range templates {
		req := ui.ExportRequest{
			TemplatePath: tmpl.Template,
			Format:       tmpl.Format,
			OutputPath:   tmpl.Output,
			Prefix:       tmpl.Prefix,
			DryRun:       false,
		}
		_, err := s.ctrl.Export(ctx, req)
		if err != nil {
			lastErr = err
		} else {
			exported++
		}
	}

	notif := notificationEvent{Type: "success"}
	if lastErr != nil {
		notif.Type = "warning"
		notif.Message = fmt.Sprintf("Exported %d/%d templates (last error: %s)", exported, len(templates), lastErr.Error())
	} else {
		notif.Message = fmt.Sprintf("Exported all %d templates", exported)
	}

	b, _ := json.Marshal(map[string]any{"showNotification": notif})
	w.Header().Set("HX-Trigger", string(b))
	w.WriteHeader(http.StatusOK)
}

// buildExportRequest creates an ExportRequest from form values.
func buildExportRequest(r *http.Request) ui.ExportRequest {
	return ui.ExportRequest{
		TemplatePath: r.FormValue("template"),
		Format:       r.FormValue("format"),
		OutputPath:   r.FormValue("output"),
		Prefix:       r.FormValue("prefix"),
	}
}

// formatForCSS returns a CSS-friendly format name for syntax highlighting hints.
func formatForCSS(format string) string {
	switch format {
	case "json", "yaml", "toml", "dotenv":
		return format
	default:
		return "text"
	}
}

// exportTemplateData holds data for rendering export template cards.
type exportTemplateData struct {
	Name     string
	Template string
	Format   string
	Output   string
	Prefix   string
}

// exportPreviewData holds data for the export preview fragment.
type exportPreviewData struct {
	Name    string
	Content string
	Format  string
	Error   string
}

// exportResultData holds data for the export result fragment.
type exportResultData struct {
	Name       string
	OutputPath string
	Error      string
}

// toExportTemplateData converts ui.ExportTemplate to exportTemplateData.
func toExportTemplateData(templates []ui.ExportTemplate) []exportTemplateData {
	result := make([]exportTemplateData, len(templates))
	for i, t := range templates {
		result[i] = exportTemplateData{
			Name:     t.Name,
			Template: t.Template,
			Format:   t.Format,
			Output:   t.Output,
			Prefix:   t.Prefix,
		}
	}
	return result
}
