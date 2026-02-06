package server

import (
	"net/http"
	"strings"
	"sync/atomic"
)

// uniqueCounter provides unique IDs for new resources.
var uniqueCounter atomic.Int64

// NewMux creates the HTTP mux with all marketplace routes.
func NewMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()

	// Service discovery.
	mux.HandleFunc("GET /.well-known/zhi-marketplace.json", h.HandleWellKnown)

	// Search.
	mux.HandleFunc("GET /api/v1/search", h.HandleSearch)

	// Plugin registration (authenticated).
	mux.Handle("POST /api/v1/plugins", h.authenticator.RequireAuth(http.HandlerFunc(h.HandleRegisterPlugin)))

	// Plugin and version routes need pattern matching.
	// Go 1.22+ ServeMux supports patterns, but we handle sub-routing manually
	// for version and resolve paths since they have variable depth.
	mux.HandleFunc("/api/v1/plugins/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/resolve"):
			h.HandleResolve(w, r)

		case strings.HasSuffix(path, "/versions"):
			switch r.Method {
			case http.MethodGet:
				h.HandleListVersions(w, r)
			case http.MethodPost:
				h.authenticator.RequireAuth(http.HandlerFunc(h.HandleRegisterVersion)).ServeHTTP(w, r)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}

		case r.Method == http.MethodGet:
			// GET /api/v1/plugins/{publisher}/{name}
			h.HandleGetPlugin(w, r)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return mux
}
