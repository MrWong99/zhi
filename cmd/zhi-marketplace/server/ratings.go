package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/MrWong99/zhi/cmd/zhi-marketplace/storage"
)

// --- Rating types ---

type submitRatingRequest struct {
	Score   int    `json:"score"`
	Comment string `json:"comment,omitempty"`
}

type ratingResponse struct {
	ID        string `json:"id"`
	User      string `json:"user"`
	Score     int    `json:"score"`
	Comment   string `json:"comment,omitempty"`
	Helpful   int    `json:"helpful"`
	CreatedAt string `json:"createdAt"`
}

type listRatingsResponse struct {
	Ratings []ratingResponse `json:"ratings"`
	Total   int              `json:"total"`
}

// HandleListRatings handles GET /api/v1/plugins/{type}/{publisher}/{name}/ratings.
func (h *Handler) HandleListRatings(w http.ResponseWriter, r *http.Request) {
	pluginType, publisher, name := extractRatingsPath(r.URL.Path)
	if publisher == "" || name == "" {
		writeError(w, http.StatusBadRequest, "invalid plugin path")
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

	q := r.URL.Query()
	page := 1
	perPage := 20
	if p := q.Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	if pp := q.Get("per_page"); pp != "" {
		perPage, _ = strconv.Atoi(pp)
	}
	sortBy := q.Get("sort")

	ratings, total, err := h.store.ListRatings(art.ID, page, perPage, sortBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := listRatingsResponse{
		Ratings: make([]ratingResponse, len(ratings)),
		Total:   total,
	}
	for i, r := range ratings {
		resp.Ratings[i] = ratingResponse{
			ID:        r.ID,
			User:      r.UserName,
			Score:     r.Score,
			Comment:   r.Comment,
			Helpful:   r.Helpful,
			CreatedAt: r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleSubmitRating handles POST /api/v1/plugins/{type}/{publisher}/{name}/ratings.
func (h *Handler) HandleSubmitRating(w http.ResponseWriter, r *http.Request) {
	publisherID := r.Header.Get("X-Publisher-ID")
	if publisherID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	pluginType, publisher, name := extractRatingsPath(r.URL.Path)
	if publisher == "" || name == "" {
		writeError(w, http.StatusBadRequest, "invalid plugin path")
		return
	}

	var req submitRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Score < 1 || req.Score > 5 {
		writeError(w, http.StatusBadRequest, "score must be between 1 and 5")
		return
	}
	if len(req.Comment) > 2000 {
		writeError(w, http.StatusBadRequest, "comment must be at most 2000 characters")
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

	pub, err := h.store.GetPublisherByID(publisherID)
	if err != nil || pub == nil {
		writeError(w, http.StatusUnauthorized, "publisher not found")
		return
	}

	rating := &storage.Rating{
		ArtifactID: art.ID,
		UserID:     publisherID,
		UserName:   pub.Name,
		Score:      req.Score,
		Comment:    req.Comment,
	}

	if err := h.store.CreateOrUpdateRating(rating); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("saving rating: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "submitted"})
}

// HandleRatingHelpful handles POST /api/v1/plugins/{publisher}/{name}/ratings/{id}/helpful.
func (h *Handler) HandleRatingHelpful(w http.ResponseWriter, r *http.Request) {
	ratingID := extractRatingHelpfulID(r.URL.Path)
	if ratingID == "" {
		writeError(w, http.StatusBadRequest, "invalid rating path")
		return
	}

	if err := h.store.IncrementHelpful(ratingID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Path helpers ---

// extractRatingsPath parses /api/v1/plugins/{type}/{publisher}/{name}/ratings...
func extractRatingsPath(path string) (pluginType, publisher, name string) {
	const prefix = "/api/v1/plugins/"
	path = strings.TrimPrefix(path, prefix)
	// Remove /ratings suffix (and anything after it).
	if idx := strings.Index(path, "/ratings"); idx >= 0 {
		path = path[:idx]
	}
	parts := strings.SplitN(path, "/", 4)
	if len(parts) < 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

// extractRatingHelpfulID parses /api/v1/plugins/.../ratings/{id}/helpful.
func extractRatingHelpfulID(path string) string {
	const suffix = "/helpful"
	path = strings.TrimSuffix(path, suffix)
	idx := strings.LastIndex(path, "/ratings/")
	if idx < 0 {
		return ""
	}
	return path[idx+len("/ratings/"):]
}
