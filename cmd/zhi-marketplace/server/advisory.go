package server

import (
	"encoding/json"
	"net/http"

	"github.com/MrWong99/zhi/cmd/zhi-marketplace/storage"
)

// --- Advisory types ---

type createAdvisoryRequest struct {
	Publisher        string `json:"publisher"`
	Plugin           string `json:"plugin"`
	Severity         string `json:"severity"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	AffectedVersions string `json:"affectedVersions"`
	FixedVersion     string `json:"fixedVersion,omitempty"`
	CVE              string `json:"cve,omitempty"`
}

type advisoryResponse struct {
	ID               string `json:"id"`
	Publisher        string `json:"publisherName"`
	Plugin           string `json:"pluginName"`
	Severity         string `json:"severity"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	AffectedVersions string `json:"affectedVersions"`
	FixedVersion     string `json:"fixedVersion,omitempty"`
	CVE              string `json:"cve,omitempty"`
	CreatedAt        string `json:"createdAt"`
}

// HandleCreateAdvisory handles POST /api/v1/advisories.
func (h *Handler) HandleCreateAdvisory(w http.ResponseWriter, r *http.Request) {
	publisherID := r.Header.Get("X-Publisher-ID")
	if publisherID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createAdvisoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Publisher == "" || req.Plugin == "" || req.Severity == "" || req.Title == "" || req.Description == "" || req.AffectedVersions == "" {
		writeError(w, http.StatusBadRequest, "publisher, plugin, severity, title, description, and affectedVersions are required")
		return
	}

	validSeverities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
	if !validSeverities[req.Severity] {
		writeError(w, http.StatusBadRequest, "severity must be one of: low, medium, high, critical")
		return
	}

	// Look up the artifact to link the advisory.
	art, err := h.store.GetArtifact(req.Publisher, req.Plugin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var artifactID string
	if art != nil {
		artifactID = art.ID
	}

	adv := &storage.Advisory{
		ArtifactID:       artifactID,
		PublisherName:    req.Publisher,
		PluginName:       req.Plugin,
		Severity:         req.Severity,
		Title:            req.Title,
		Description:      req.Description,
		AffectedVersions: req.AffectedVersions,
		FixedVersion:     req.FixedVersion,
		CVE:              req.CVE,
	}

	if err := h.store.CreateAdvisory(adv); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": adv.ID, "status": "published"})
}

// HandleListAdvisories handles GET /api/v1/advisories.
func (h *Handler) HandleListAdvisories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	publisher := q.Get("publisher")
	plugin := q.Get("plugin")
	severity := q.Get("severity")

	// If publisher and plugin are given, resolve to artifact ID.
	var artifactID string
	if publisher != "" && plugin != "" {
		art, err := h.store.GetArtifact(publisher, plugin)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if art != nil {
			artifactID = art.ID
		}
	}

	advisories, err := h.store.ListAdvisories(artifactID, severity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]advisoryResponse, len(advisories))
	for i, a := range advisories {
		resp[i] = advisoryResponse{
			ID:               a.ID,
			Publisher:        a.PublisherName,
			Plugin:           a.PluginName,
			Severity:         a.Severity,
			Title:            a.Title,
			Description:      a.Description,
			AffectedVersions: a.AffectedVersions,
			FixedVersion:     a.FixedVersion,
			CVE:              a.CVE,
			CreatedAt:        a.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"advisories": resp})
}
