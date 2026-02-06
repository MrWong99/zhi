package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// noDeadline is the zero time used to disable write deadlines for SSE streams.
var noDeadline time.Time

// handleApplyPage renders the apply targets page.
// GET /apply
func (s *Server) handleApplyPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsName, _ := s.ctrl.WorkspaceName(ctx)

	templates, _ := s.ctrl.ExportTemplates(ctx)

	data := pageData{
		WorkspaceName:   wsName,
		ActiveNav:       "apply",
		Nonce:           nonceFromCtx(ctx),
		CSRFToken:       csrfFromCtx(ctx),
		ApplyTargets:    toApplyTargetData(templates),
		ExportTemplates: toExportTemplateData(templates),
		Breadcrumbs: []breadcrumb{
			{Label: "Apply", Href: "/apply"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "apply_targets", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := s.engine.renderPage(w, "apply", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleApplyRun executes an apply target with SSE streaming output.
// POST /apply/run
func (s *Server) handleApplyRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	target := r.FormValue("target")

	// Check if pre-export is requested.
	if r.FormValue("pre_export") == "true" {
		templates, err := s.ctrl.ExportTemplates(ctx)
		if err != nil {
			http.Error(w, "failed to load export templates: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for _, tmpl := range templates {
			req := ui.ExportRequest{
				TemplatePath: tmpl.Template,
				Format:       tmpl.Format,
				OutputPath:   tmpl.Output,
				Prefix:       tmpl.Prefix,
				DryRun:       false,
			}
			if _, err := s.ctrl.Export(ctx, req); err != nil {
				http.Error(w, "pre-export failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Set SSE headers. These must be set before any writes.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Disable buffering for the compress middleware; SSE must not be gzipped.
	w.Header().Del("Content-Encoding")

	// Use http.ResponseController to set an unlimited write deadline for
	// long-running apply commands.
	rc := http.NewResponseController(w)
	//nolint:errcheck // Best-effort; not all ResponseWriters support this.
	_ = rc.SetWriteDeadline(noDeadline)

	flusher, ok := w.(http.Flusher)
	if !ok {
		// Unwrap through middleware wrappers to find a Flusher.
		if uw, ok2 := w.(interface{ Unwrap() http.ResponseWriter }); ok2 {
			flusher, ok = uw.Unwrap().(http.Flusher)
		}
	}

	handler := func(event ui.ApplyEvent) {
		data, _ := json.Marshal(map[string]string{
			"line":   event.Line,
			"stream": event.Stream,
		})
		fmt.Fprintf(w, "event: output\ndata: %s\n\n", data)
		if ok {
			flusher.Flush()
		}
	}

	result, err := s.ctrl.Apply(ctx, target, handler)

	// Send completion event.
	exitCode := -1
	errStr := ""
	if result != nil {
		exitCode = result.ExitCode
		if result.Error != "" {
			errStr = result.Error
		}
	}
	if err != nil {
		errStr = err.Error()
	}

	doneData, _ := json.Marshal(map[string]any{
		"exit_code": exitCode,
		"error":     errStr,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	if ok {
		flusher.Flush()
	}
}

// applyTargetData holds data for rendering apply target UI elements.
type applyTargetData struct {
	Name        string
	Description string
}

// toApplyTargetData builds apply target display data. For now it exposes
// a "default" target. If the workspace has export templates it hints that
// pre-export is available.
func toApplyTargetData(templates []ui.ExportTemplate) []applyTargetData {
	targets := []applyTargetData{
		{Name: "default", Description: "Default apply target"},
	}
	_ = templates // May be used for pre-export hints in future.
	return targets
}
