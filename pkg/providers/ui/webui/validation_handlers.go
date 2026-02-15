package webui

import (
	"net/http"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// handleValidationPage renders the full validation results page.
// GET /validation
func (s *Server) handleValidationPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	results, err := s.ctrl.Validate(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to validate: "+err.Error())
		return
	}

	wsName, _ := s.ctrl.WorkspaceName(ctx)
	groups := groupValidationResults(results)
	count := len(results)

	data := pageData{
		WorkspaceName:    wsName,
		ActiveNav:        "validation",
		Nonce:            nonceFromCtx(ctx),
		CSRFToken:        csrfFromCtx(ctx),
		Authenticated:    authenticatedFromCtx(ctx),
		ValidationGroups: groups,
		ValidationCount:  count,
		Breadcrumbs: []breadcrumb{
			{Label: "Validation", Href: "/validation"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "validation_content", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := s.engine.renderPage(w, "validation", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleFullValidation runs a full validation and returns results.
// POST /validate
func (s *Server) handleFullValidation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	results, err := s.ctrl.Validate(ctx)
	if err != nil {
		http.Error(w, "validation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	groups := groupValidationResults(results)
	count := len(results)

	wsName, _ := s.ctrl.WorkspaceName(ctx)
	data := pageData{
		WorkspaceName:    wsName,
		ActiveNav:        "validation",
		Nonce:            nonceFromCtx(ctx),
		CSRFToken:        csrfFromCtx(ctx),
		Authenticated:    authenticatedFromCtx(ctx),
		ValidationGroups: groups,
		ValidationCount:  count,
		Breadcrumbs: []breadcrumb{
			{Label: "Validation", Href: "/validation"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "validation_content", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := s.engine.renderPage(w, "validation", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// validationGroup groups validation results by severity.
type validationGroup struct {
	Severity string
	Icon     string
	CSSClass string
	Results  []validationItem
}

// validationItem represents a single validation result for display.
type validationItem struct {
	Path     string
	PathID   string
	Message  string
	Severity string
}

// groupValidationResults groups results by severity in display order:
// Blocking, Warning, Info.
func groupValidationResults(results []config.ValidationResult) []validationGroup {
	blocking := validationGroup{Severity: "Blocking", Icon: "alert-circle", CSSClass: "danger"}
	warning := validationGroup{Severity: "Warning", Icon: "alert-triangle", CSSClass: "warning"}
	info := validationGroup{Severity: "Info", Icon: "info", CSSClass: "info"}

	for _, r := range results {
		item := validationItem{
			Message:  r.Message,
			Severity: r.Severity.String(),
		}
		// Extract path from metadata if present.
		if r.Metadata != nil {
			if p, ok := r.Metadata["path"].(string); ok {
				item.Path = p
				item.PathID = pathToID(p)
			}
		}
		switch r.Severity {
		case config.Blocking:
			blocking.Results = append(blocking.Results, item)
		case config.Warning:
			warning.Results = append(warning.Results, item)
		default:
			info.Results = append(info.Results, item)
		}
	}

	var groups []validationGroup
	if len(blocking.Results) > 0 {
		groups = append(groups, blocking)
	}
	if len(warning.Results) > 0 {
		groups = append(groups, warning)
	}
	if len(info.Results) > 0 {
		groups = append(groups, info)
	}
	return groups
}

// filterValidationForPath returns only results relevant to the given path.
func filterValidationForPath(results []config.ValidationResult, path string) []config.ValidationResult {
	var filtered []config.ValidationResult
	for _, r := range results {
		if r.Metadata == nil {
			continue
		}
		p, ok := r.Metadata["path"].(string)
		if !ok {
			continue
		}
		if p == path || strings.HasPrefix(p, path+"/") {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// hasBlockingResults checks if any result has Blocking severity.
func hasBlockingResults(results []config.ValidationResult) bool {
	for _, r := range results {
		if r.Severity == config.Blocking {
			return true
		}
	}
	return false
}

// toValidationItems converts config.ValidationResult to display items.
func toValidationItems(results []config.ValidationResult) []validationItem {
	items := make([]validationItem, 0, len(results))
	for _, r := range results {
		item := validationItem{
			Message:  r.Message,
			Severity: r.Severity.String(),
		}
		if r.Metadata != nil {
			if p, ok := r.Metadata["path"].(string); ok {
				item.Path = p
				item.PathID = pathToID(p)
			}
		}
		items = append(items, item)
	}
	return items
}
