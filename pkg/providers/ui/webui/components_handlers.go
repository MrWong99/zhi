package webui

import (
	"fmt"
	"net/http"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// handleComponentsPage renders the component management page.
// GET /components
func (s *Server) handleComponentsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	components, err := s.ctrl.ListComponents(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load components: "+err.Error())
		return
	}

	wsName, _ := s.ctrl.WorkspaceName(ctx)

	data := pageData{
		WorkspaceName: wsName,
		ActiveNav:     "components",
		Nonce:         nonceFromCtx(ctx),
		CSRFToken:     csrfFromCtx(ctx),
		Components:    toComponentViewData(components),
		Breadcrumbs: []breadcrumb{
			{Label: "Components", Href: "/components"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "components_content", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := s.engine.renderPage(w, "components", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleComponentToggle enables or disables a component.
// POST /components/{name}/toggle
func (s *Server) handleComponentToggle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "component name required", http.StatusBadRequest)
		return
	}

	// Get current component state to determine action.
	components, err := s.ctrl.ListComponents(ctx)
	if err != nil {
		http.Error(w, "failed to load components", http.StatusInternalServerError)
		return
	}

	var target *ui.ComponentInfo
	for i := range components {
		if components[i].Name == name {
			target = &components[i]
			break
		}
	}

	if target == nil {
		http.Error(w, "component not found", http.StatusNotFound)
		return
	}

	if target.Mandatory {
		// Return the card with an error message.
		card := componentViewData{
			Name:         target.Name,
			Description:  target.Description,
			Enabled:      target.Enabled,
			Mandatory:    target.Mandatory,
			Paths:        target.Paths,
			Dependencies: target.Dependencies,
			Error:        "Cannot toggle mandatory component",
		}
		if err := s.engine.renderFragment(w, "component_card", card); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var toggleErr error
	var enabled []string
	if target.Enabled {
		toggleErr = s.ctrl.DisableComponent(ctx, name)
	} else {
		enabled, toggleErr = s.ctrl.EnableComponent(ctx, name)
	}

	// Reload components to get updated state.
	updatedComponents, listErr := s.ctrl.ListComponents(ctx)
	if listErr != nil {
		http.Error(w, "failed to reload components", http.StatusInternalServerError)
		return
	}

	// Find the updated component.
	var updated *ui.ComponentInfo
	for i := range updatedComponents {
		if updatedComponents[i].Name == name {
			updated = &updatedComponents[i]
			break
		}
	}

	if updated == nil {
		http.Error(w, "component not found after toggle", http.StatusInternalServerError)
		return
	}

	card := componentViewData{
		Name:         updated.Name,
		Description:  updated.Description,
		Enabled:      updated.Enabled,
		Mandatory:    updated.Mandatory,
		Paths:        updated.Paths,
		Dependencies: updated.Dependencies,
	}

	if toggleErr != nil {
		card.Error = toggleErr.Error()
	} else if len(enabled) > 1 {
		// Show which dependencies were auto-enabled.
		card.Info = fmt.Sprintf("Also enabled: %v", enabled[1:])
	}

	if err := s.engine.renderFragment(w, "component_card", card); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// componentViewData holds data for the component_card fragment.
type componentViewData struct {
	Name         string
	Description  string
	Enabled      bool
	Mandatory    bool
	Paths        []string
	Dependencies []string
	Error        string
	Info         string
}

// toComponentViewData converts ui.ComponentInfo to componentViewData.
func toComponentViewData(components []ui.ComponentInfo) []componentViewData {
	result := make([]componentViewData, len(components))
	for i, c := range components {
		result[i] = componentViewData{
			Name:         c.Name,
			Description:  c.Description,
			Enabled:      c.Enabled,
			Mandatory:    c.Mandatory,
			Paths:        c.Paths,
			Dependencies: c.Dependencies,
		}
	}
	return result
}
