package server

import (
	"net/http"
	"strings"
)

// NewMux creates the HTTP mux with all mirror routes.
func NewMux(oci *OCIHandler, marketplace *MarketplaceProxy) *http.ServeMux {
	mux := http.NewServeMux()

	// OCI Distribution API.
	mux.HandleFunc("GET /v2/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case path == "/v2/" || path == "/v2":
			oci.HandleV2Check(w, r)

		case strings.Contains(path, "/manifests/"):
			oci.HandleManifest(w, r)

		case strings.Contains(path, "/blobs/"):
			oci.HandleBlob(w, r)

		case strings.HasSuffix(path, "/tags/list"):
			oci.HandleTagsList(w, r)

		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("HEAD /v2/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/manifests/"):
			oci.HandleManifest(w, r)
		case strings.Contains(path, "/blobs/"):
			oci.HandleBlob(w, r)
		default:
			oci.HandleV2Check(w, r)
		}
	})

	// Marketplace API proxy.
	mux.HandleFunc("GET /.well-known/zhi-marketplace.json", marketplace.HandleWellKnown)
	mux.HandleFunc("GET /api/v1/search", marketplace.HandleSearch)
	mux.HandleFunc("/api/v1/plugins/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Path
		if strings.HasSuffix(path, "/versions") {
			marketplace.HandleVersions(w, r)
		} else {
			marketplace.HandlePlugin(w, r)
		}
	})

	return mux
}
