package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MrWong99/zhi/cmd/zhi-marketplace/storage"
)

// --- Publisher profile and verification ---

type publisherProfileResponse struct {
	Name             string `json:"name"`
	Verified         bool   `json:"verified"`
	VerificationTier int    `json:"verificationTier"`
	PluginCount      int    `json:"pluginCount"`
	MemberSince      string `json:"memberSince"`
}

type verifyRequest struct {
	Repository string `json:"repository"`
	License    string `json:"license"`
}

// HandleGetPublisher handles GET /api/v1/publishers/{name}.
func (h *Handler) HandleGetPublisher(w http.ResponseWriter, r *http.Request) {
	name := extractPublisherName(r.URL.Path)
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid publisher path")
		return
	}

	pub, err := h.store.GetPublisher(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pub == nil {
		writeError(w, http.StatusNotFound, "publisher not found")
		return
	}

	// Count plugins owned by this publisher.
	results, _, err := h.store.Search(storage.SearchParams{PerPage: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pluginCount := 0
	for _, sr := range results {
		if sr.Artifact.PublisherID == pub.ID {
			pluginCount++
		}
	}

	resp := publisherProfileResponse{
		Name:             pub.Name,
		Verified:         pub.Verified,
		VerificationTier: pub.VerificationTier,
		PluginCount:      pluginCount,
		MemberSince:      pub.CreatedAt.Format("2006-01-02"),
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleVerifyRequest handles POST /api/v1/publishers/{name}/verify-request.
func (h *Handler) HandleVerifyRequest(w http.ResponseWriter, r *http.Request) {
	publisherID := r.Header.Get("X-Publisher-ID")
	if publisherID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	name := extractPublisherVerifyName(r.URL.Path)
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid publisher path")
		return
	}

	pub, err := h.store.GetPublisher(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pub == nil {
		writeError(w, http.StatusNotFound, "publisher not found")
		return
	}

	// Only the publisher themselves can request verification.
	if pub.ID != publisherID {
		writeError(w, http.StatusForbidden, "you can only request verification for your own account")
		return
	}

	if pub.VerificationTier >= 2 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already verified"})
		return
	}

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Set tier to 1 (authenticated) pending manual review for tier 2.
	if pub.VerificationTier < 1 {
		pub.VerificationTier = 1
		pub.Verified = false
		if err := h.store.UpdatePublisher(pub); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "pending",
		"message": "Verification request submitted. Automated checks will run and a maintainer will review.",
	})
}

// --- Path helpers ---

// extractPublisherName parses /api/v1/publishers/{name} from a URL path.
func extractPublisherName(path string) string {
	const prefix = "/api/v1/publishers/"
	path = strings.TrimPrefix(path, prefix)
	// Remove any trailing path segments.
	if idx := strings.Index(path, "/"); idx >= 0 {
		path = path[:idx]
	}
	return path
}

// extractPublisherVerifyName parses /api/v1/publishers/{name}/verify-request.
func extractPublisherVerifyName(path string) string {
	const prefix = "/api/v1/publishers/"
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, "/verify-request")
	if idx := strings.Index(path, "/"); idx >= 0 {
		path = path[:idx]
	}
	return path
}
