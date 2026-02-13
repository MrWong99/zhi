package server

import (
	"net/http"
	"strings"
)

// HandleRecordDownload handles POST /api/v1/plugins/{type}/{publisher}/{name}/{version}/download.
func (h *Handler) HandleRecordDownload(w http.ResponseWriter, r *http.Request) {
	pluginType, publisher, name, version := extractDownloadPath(r.URL.Path)
	if publisher == "" || name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "invalid download path")
		return
	}

	art, err := h.store.GetArtifact(publisher, name, pluginType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if art == nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	v, err := h.store.GetVersion(art.ID, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if v == nil {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	platform := r.URL.Query().Get("platform")
	if err := h.store.RecordDownload(v.ID, platform); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// HandleGetStats handles GET /api/v1/plugins/{type}/{publisher}/{name}/stats.
func (h *Handler) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	pluginType, publisher, name := extractStatsPath(r.URL.Path)
	if publisher == "" || name == "" {
		writeError(w, http.StatusBadRequest, "invalid stats path")
		return
	}

	art, err := h.store.GetArtifact(publisher, name, pluginType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if art == nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	total, monthly, err := h.store.GetDownloadStats(art.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	avg, count, dist, err := h.store.GetRatingAggregation(art.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"downloads": map[string]any{
			"total":   total,
			"monthly": monthly,
		},
		"rating": map[string]any{
			"average":      avg,
			"count":        count,
			"distribution": dist,
		},
	})
}

// --- Path helpers ---

// extractDownloadPath parses /api/v1/plugins/{type}/{publisher}/{name}/{version}/download.
func extractDownloadPath(path string) (pluginType, publisher, name, version string) {
	const prefix = "/api/v1/plugins/"
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, "/download")
	parts := strings.SplitN(path, "/", 5)
	if len(parts) < 4 {
		return "", "", "", ""
	}
	return parts[0], parts[1], parts[2], parts[3]
}

// extractStatsPath parses /api/v1/plugins/{type}/{publisher}/{name}/stats.
func extractStatsPath(path string) (pluginType, publisher, name string) {
	const prefix = "/api/v1/plugins/"
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, "/stats")
	parts := strings.SplitN(path, "/", 4)
	if len(parts) < 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}
