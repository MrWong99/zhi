// Package server implements the HTTP handlers for the zhi marketplace API.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/MrWong99/zhi/cmd/zhi-marketplace/auth"
	"github.com/MrWong99/zhi/cmd/zhi-marketplace/storage"
)

// Handler holds the dependencies for HTTP handlers.
type Handler struct {
	store         storage.Store
	authenticator *auth.Authenticator
	registries    []string
}

// NewHandler creates a new handler with the given dependencies.
func NewHandler(store storage.Store, authenticator *auth.Authenticator, registries []string) *Handler {
	return &Handler{
		store:         store,
		authenticator: authenticator,
		registries:    registries,
	}
}

// wellKnownResponse is the /.well-known/zhi-marketplace.json body.
type wellKnownResponse struct {
	API        string   `json:"api"`
	Version    string   `json:"version"`
	Registries []string `json:"registries,omitempty"`
}

// HandleWellKnown serves the service discovery document.
func (h *Handler) HandleWellKnown(w http.ResponseWriter, r *http.Request) {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	resp := wellKnownResponse{
		API:        fmt.Sprintf("%s://%s/api/v1", scheme, r.Host),
		Version:    "1",
		Registries: h.registries,
	}
	writeJSON(w, http.StatusOK, resp)
}

// searchResponse is the GET /api/v1/search response body.
type searchResponse struct {
	Total   int            `json:"total"`
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	Author        string   `json:"author"`
	Verified      bool     `json:"verified"`
	LatestVersion string   `json:"latestVersion"`
	OCIRef        string   `json:"ociRef"`
	Downloads     int64    `json:"downloads"`
	Rating        float64  `json:"rating"`
	RatingCount   int      `json:"ratingCount"`
	Keywords      []string `json:"keywords"`
	UpdatedAt     string   `json:"updatedAt"`
	Platforms     []string `json:"platforms"`
}

// HandleSearch handles GET /api/v1/search.
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := storage.SearchParams{
		Query:    q.Get("q"),
		Type:     q.Get("type"),
		Sort:     q.Get("sort"),
		Verified: q.Get("verified") == "true",
	}
	if pp := q.Get("per_page"); pp != "" {
		params.PerPage, _ = strconv.Atoi(pp)
	}
	if p := q.Get("page"); p != "" {
		params.Page, _ = strconv.Atoi(p)
	}

	results, total, err := h.store.Search(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	resp := searchResponse{
		Total:   total,
		Results: make([]searchResult, len(results)),
	}
	for i, sr := range results {
		resp.Results[i] = searchResult{
			Name:          sr.Artifact.Name,
			Type:          sr.Artifact.Type,
			Description:   sr.Artifact.Description,
			Author:        sr.Artifact.PublisherName,
			Verified:      sr.Artifact.Verified,
			LatestVersion: sr.LatestVersion,
			OCIRef:        sr.Artifact.OCIRef,
			Downloads:     sr.TotalDownloads,
			Rating:        sr.Rating,
			RatingCount:   sr.RatingCount,
			Keywords:      sr.Artifact.Keywords,
			UpdatedAt:     sr.Artifact.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Platforms:     sr.Platforms,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleGetPlugin handles GET /api/v1/plugins/{publisher}/{name}.
func (h *Handler) HandleGetPlugin(w http.ResponseWriter, r *http.Request) {
	publisher, name := extractPluginPath(r.URL.Path)
	if publisher == "" || name == "" {
		writeError(w, http.StatusBadRequest, "invalid plugin path")
		return
	}

	art, err := h.store.GetArtifact(publisher, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if art == nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	versions, err := h.store.ListVersions(art.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type versionEntry struct {
		Version            string   `json:"version"`
		Digest             string   `json:"digest"`
		ZhiProtocolVersion string   `json:"zhiProtocolVersion,omitempty"`
		MinZhiVersion      string   `json:"minZhiVersion,omitempty"`
		Platforms          []string `json:"platforms,omitempty"`
		PublishedAt        string   `json:"publishedAt"`
		Signed             bool     `json:"signed"`
		Downloads          int64    `json:"downloads,omitempty"`
	}

	vEntries := make([]versionEntry, len(versions))
	var totalDl int64
	for i, v := range versions {
		vEntries[i] = versionEntry{
			Version:            v.Version,
			Digest:             v.Digest,
			ZhiProtocolVersion: v.ZhiProtocolVersion,
			MinZhiVersion:      v.MinZhiVersion,
			Platforms:          v.Platforms,
			PublishedAt:        v.PublishedAt.Format("2006-01-02T15:04:05Z"),
			Signed:             v.Signed,
			Downloads:          v.Downloads,
		}
		totalDl += v.Downloads
	}

	resp := map[string]any{
		"name":        art.Name,
		"type":        art.Type,
		"author":      art.PublisherName,
		"verified":    art.Verified,
		"description": art.Description,
		"license":     art.License,
		"homepage":    art.Homepage,
		"repository":  art.Repository,
		"ociRef":      art.OCIRef,
		"versions":    vEntries,
		"statistics": map[string]any{
			"totalDownloads": totalDl,
		},
	}
	if art.LongDescription != "" {
		resp["longDescription"] = art.LongDescription
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleListVersions handles GET /api/v1/plugins/{publisher}/{name}/versions.
func (h *Handler) HandleListVersions(w http.ResponseWriter, r *http.Request) {
	publisher, name := extractVersionsPath(r.URL.Path)
	if publisher == "" || name == "" {
		writeError(w, http.StatusBadRequest, "invalid plugin path")
		return
	}

	art, err := h.store.GetArtifact(publisher, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if art == nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	versions, err := h.store.ListVersions(art.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type versionEntry struct {
		Version     string   `json:"version"`
		Platforms   []string `json:"platforms,omitempty"`
		Digest      string   `json:"digest"`
		Signed      bool     `json:"signed"`
		PublishedAt string   `json:"publishedAt"`
	}

	entries := make([]versionEntry, len(versions))
	for i, v := range versions {
		entries[i] = versionEntry{
			Version:     v.Version,
			Platforms:   v.Platforms,
			Digest:      v.Digest,
			Signed:      v.Signed,
			PublishedAt: v.PublishedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"versions": entries})
}

// HandleResolve handles GET /api/v1/plugins/{publisher}/{name}/{version}/resolve.
func (h *Handler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	publisher, name, version := extractResolvePath(r.URL.Path)
	if publisher == "" || name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "invalid resolve path")
		return
	}

	art, err := h.store.GetArtifact(publisher, name)
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

	resp := map[string]any{
		"ociRef": art.OCIRef + ":v" + v.Version,
		"digest": v.Digest,
		"signed": v.Signed,
	}
	if v.SigningIdentity != "" {
		resp["signingIdentity"] = v.SigningIdentity
	}

	writeJSON(w, http.StatusOK, resp)
}

// registerPluginRequest is the POST /api/v1/plugins body.
type registerPluginRequest struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	OCIRef      string   `json:"ociRef"`
	Description string   `json:"description"`
	Homepage    string   `json:"homepage"`
	Keywords    []string `json:"keywords"`
}

// HandleRegisterPlugin handles POST /api/v1/plugins.
func (h *Handler) HandleRegisterPlugin(w http.ResponseWriter, r *http.Request) {
	publisherID := r.Header.Get("X-Publisher-ID")
	if publisherID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req registerPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" || req.OCIRef == "" {
		writeError(w, http.StatusBadRequest, "name, type, and ociRef are required")
		return
	}

	pub, err := h.store.GetPublisherByID(publisherID)
	if err != nil || pub == nil {
		writeError(w, http.StatusUnauthorized, "publisher not found")
		return
	}

	art := &storage.Artifact{
		ID:          generateID(),
		PublisherID: publisherID,
		Name:        req.Name,
		Type:        req.Type,
		OCIRef:      req.OCIRef,
		Description: req.Description,
		Homepage:    req.Homepage,
		Keywords:    req.Keywords,
		Verified:    pub.Verified,
	}

	if err := h.store.CreateArtifact(art); err != nil {
		writeError(w, http.StatusConflict, "plugin already registered or error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": art.ID, "status": "registered"})
}

// registerVersionRequest is the POST /api/v1/plugins/{publisher}/{name}/versions body.
type registerVersionRequest struct {
	Version            string   `json:"version"`
	Digest             string   `json:"digest"`
	Platforms          []string `json:"platforms"`
	ZhiProtocolVersion string   `json:"zhiProtocolVersion"`
	Changelog          string   `json:"changelog"`
}

// HandleRegisterVersion handles POST /api/v1/plugins/{publisher}/{name}/versions.
func (h *Handler) HandleRegisterVersion(w http.ResponseWriter, r *http.Request) {
	publisherID := r.Header.Get("X-Publisher-ID")
	if publisherID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	publisher, name := extractVersionsPath(r.URL.Path)
	if publisher == "" || name == "" {
		writeError(w, http.StatusBadRequest, "invalid plugin path")
		return
	}

	var req registerVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Version == "" {
		writeError(w, http.StatusBadRequest, "version is required")
		return
	}

	art, err := h.store.GetArtifact(publisher, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if art == nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	// Verify the caller owns this plugin.
	if art.PublisherID != publisherID {
		writeError(w, http.StatusForbidden, "you do not own this plugin")
		return
	}

	v := &storage.Version{
		ID:                 generateID(),
		ArtifactID:         art.ID,
		Version:            req.Version,
		Digest:             req.Digest,
		Platforms:          req.Platforms,
		ZhiProtocolVersion: req.ZhiProtocolVersion,
		Changelog:          req.Changelog,
	}

	if err := h.store.CreateVersion(v); err != nil {
		writeError(w, http.StatusConflict, "version already exists or error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": v.ID, "status": "registered"})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// extractPluginPath parses /api/v1/plugins/{publisher}/{name} from a URL path.
func extractPluginPath(path string) (publisher, name string) {
	const prefix = "/api/v1/plugins/"
	path = strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// extractVersionsPath parses /api/v1/plugins/{publisher}/{name}/versions from a URL path.
func extractVersionsPath(path string) (publisher, name string) {
	const prefix = "/api/v1/plugins/"
	path = strings.TrimPrefix(path, prefix)
	// Remove /versions suffix if present
	path = strings.TrimSuffix(path, "/versions")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// extractResolvePath parses /api/v1/plugins/{publisher}/{name}/{version}/resolve.
func extractResolvePath(path string) (publisher, name, version string) {
	const prefix = "/api/v1/plugins/"
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, "/resolve")
	parts := strings.SplitN(path, "/", 4)
	if len(parts) < 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

// generateID produces a simple unique ID. In production this would be a UUID.
func generateID() string {
	return fmt.Sprintf("%d", uniqueCounter.Add(1))
}
