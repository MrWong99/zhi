package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MrWong99/zhi/cmd/zhi-mirror/storage"
)

// OCIHandler implements the OCI Distribution API endpoints for the
// pull-through cache. It handles:
//   - GET /v2/                                  (API version check)
//   - GET /v2/{name}/manifests/{reference}      (manifest pull)
//   - HEAD /v2/{name}/manifests/{reference}     (manifest existence check)
//   - GET /v2/{name}/blobs/{digest}             (blob pull)
//   - HEAD /v2/{name}/blobs/{digest}            (blob existence check)
//   - GET /v2/{name}/tags/list                  (tag listing)
type OCIHandler struct {
	store    *storage.OCILayout
	upstream string
	policy   *Policy
	audit    *AuditLogger
	tagTTL   time.Duration
	client   *http.Client
}

// NewOCIHandler creates an OCI Distribution API handler.
func NewOCIHandler(store *storage.OCILayout, upstream string, policy *Policy, audit *AuditLogger) *OCIHandler {
	return &OCIHandler{
		store:    store,
		upstream: strings.TrimRight(upstream, "/"),
		policy:   policy,
		audit:    audit,
		tagTTL:   5 * time.Minute,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// HandleV2Check handles GET /v2/ — the OCI Distribution API version check.
func (h *OCIHandler) HandleV2Check(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

// HandleManifest handles GET and HEAD /v2/{name}/manifests/{reference}.
func (h *OCIHandler) HandleManifest(w http.ResponseWriter, r *http.Request) {
	repo, ref := parseManifestPath(r.URL.Path)
	if repo == "" || ref == "" {
		http.Error(w, `{"errors":[{"code":"NAME_UNKNOWN","message":"invalid path"}]}`, http.StatusNotFound)
		return
	}

	// Check policy.
	if err := h.policy.CheckRepository(repo); err != nil {
		http.Error(w, fmt.Sprintf(`{"errors":[{"code":"DENIED","message":%q}]}`, err.Error()), http.StatusForbidden)
		return
	}

	isDigest := strings.HasPrefix(ref, "sha256:")

	// For tag references, check if we have a cached tag mapping.
	dgst := ref
	if !isDigest {
		cached, ok := h.store.GetTag(repo, ref)
		if ok {
			// Check TTL — tags need periodic revalidation.
			entry, _ := h.store.GetTagEntry(repo, ref)
			if entry != nil && time.Since(entry.UpdatedAt) < h.tagTTL {
				dgst = cached
			} else {
				// Stale tag — fetch from upstream.
				upstreamDigest, data, mediaType, err := h.fetchManifestFromUpstream(repo, ref)
				if err != nil {
					// Fall back to stale cache.
					dgst = cached
				} else {
					dgst = upstreamDigest
					if _, err := h.store.PutBlob(data); err != nil {
						log.Printf("mirror: failed to cache manifest blob: %v", err)
					}
					if err := h.store.PutTag(repo, ref, dgst); err != nil {
						log.Printf("mirror: failed to update tag: %v", err)
					}
					_ = mediaType
				}
			}
		} else {
			// Tag not cached — fetch from upstream.
			upstreamDigest, data, mediaType, err := h.fetchManifestFromUpstream(repo, ref)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"errors":[{"code":"MANIFEST_UNKNOWN","message":%q}]}`, err.Error()), http.StatusNotFound)
				return
			}
			dgst = upstreamDigest
			if _, err := h.store.PutBlob(data); err != nil {
				log.Printf("mirror: failed to cache manifest blob: %v", err)
			}
			if err := h.store.PutTag(repo, ref, dgst); err != nil {
				log.Printf("mirror: failed to cache tag: %v", err)
			}
			_ = mediaType
		}
	}

	// Serve the manifest from the store.
	data, ok, err := h.store.GetBlob(dgst)
	if err != nil {
		http.Error(w, `{"errors":[{"code":"INTERNAL_ERROR","message":"storage error"}]}`, http.StatusInternalServerError)
		return
	}
	if !ok {
		// Try to fetch from upstream for digest references.
		_, fetchedData, _, fetchErr := h.fetchManifestFromUpstream(repo, dgst)
		if fetchErr != nil {
			http.Error(w, `{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest not found"}]}`, http.StatusNotFound)
			return
		}
		if _, err := h.store.PutBlob(fetchedData); err != nil {
			log.Printf("mirror: failed to cache manifest: %v", err)
		}
		data = fetchedData
	}

	// Detect media type from the manifest JSON.
	mediaType := detectMediaType(data)

	h.audit.LogPull(clientIP(r), repo, ref, "", dgst, true)

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", dgst)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if r.Method == http.MethodHead {
		return
	}
	w.Write(data) //nolint:errcheck
}

// HandleBlob handles GET and HEAD /v2/{name}/blobs/{digest}.
func (h *OCIHandler) HandleBlob(w http.ResponseWriter, r *http.Request) {
	repo, dgst := parseBlobPath(r.URL.Path)
	if repo == "" || dgst == "" {
		http.Error(w, `{"errors":[{"code":"BLOB_UNKNOWN","message":"invalid path"}]}`, http.StatusNotFound)
		return
	}

	// Check policy.
	if err := h.policy.CheckRepository(repo); err != nil {
		http.Error(w, fmt.Sprintf(`{"errors":[{"code":"DENIED","message":%q}]}`, err.Error()), http.StatusForbidden)
		return
	}

	// Try local cache first.
	data, ok, err := h.store.GetBlob(dgst)
	if err != nil {
		http.Error(w, `{"errors":[{"code":"INTERNAL_ERROR","message":"storage error"}]}`, http.StatusInternalServerError)
		return
	}

	if !ok {
		// Fetch from upstream.
		data, err = h.fetchBlobFromUpstream(repo, dgst)
		if err != nil {
			http.Error(w, `{"errors":[{"code":"BLOB_UNKNOWN","message":"blob not found"}]}`, http.StatusNotFound)
			return
		}
		// Cache the blob.
		if _, err := h.store.PutBlob(data); err != nil {
			log.Printf("mirror: failed to cache blob: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", dgst)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if r.Method == http.MethodHead {
		return
	}
	w.Write(data) //nolint:errcheck
}

// HandleTagsList handles GET /v2/{name}/tags/list.
func (h *OCIHandler) HandleTagsList(w http.ResponseWriter, r *http.Request) {
	repo := parseTagsListPath(r.URL.Path)
	if repo == "" {
		http.Error(w, `{"errors":[{"code":"NAME_UNKNOWN","message":"invalid path"}]}`, http.StatusNotFound)
		return
	}

	tags, err := h.store.ListTags(repo)
	if err != nil {
		http.Error(w, `{"errors":[{"code":"INTERNAL_ERROR","message":"storage error"}]}`, http.StatusInternalServerError)
		return
	}
	if tags == nil {
		tags = []string{}
	}

	resp := map[string]any{
		"name": repo,
		"tags": tags,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// fetchManifestFromUpstream fetches a manifest from the upstream registry.
func (h *OCIHandler) fetchManifestFromUpstream(repo, ref string) (digest string, data []byte, mediaType string, err error) {
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", h.upstream, repo, ref)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", nil, "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
	}, ", "))

	resp, err := h.client.Do(req)
	if err != nil {
		return "", nil, "", fmt.Errorf("fetching from upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, "", fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, "", fmt.Errorf("reading response: %w", err)
	}

	digest = resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		digest = storage.ComputeSHA256(data)
	}
	mediaType = resp.Header.Get("Content-Type")

	return digest, data, mediaType, nil
}

// fetchBlobFromUpstream fetches a blob from the upstream registry.
func (h *OCIHandler) fetchBlobFromUpstream(repo, dgst string) ([]byte, error) {
	url := fmt.Sprintf("https://%s/v2/%s/blobs/%s", h.upstream, repo, dgst)
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching blob from upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading blob: %w", err)
	}
	return data, nil
}

// parseManifestPath extracts the repository name and reference from a path
// like /v2/{name}/manifests/{reference}. The name may contain slashes.
func parseManifestPath(path string) (repo, ref string) {
	const prefix = "/v2/"
	const segment = "/manifests/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	rest := path[len(prefix):]
	idx := strings.LastIndex(rest, segment)
	if idx < 0 {
		return "", ""
	}
	return rest[:idx], rest[idx+len(segment):]
}

// parseBlobPath extracts the repository name and digest from a path like
// /v2/{name}/blobs/{digest}.
func parseBlobPath(path string) (repo, dgst string) {
	const prefix = "/v2/"
	const segment = "/blobs/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	rest := path[len(prefix):]
	idx := strings.LastIndex(rest, segment)
	if idx < 0 {
		return "", ""
	}
	return rest[:idx], rest[idx+len(segment):]
}

// parseTagsListPath extracts the repository name from /v2/{name}/tags/list.
func parseTagsListPath(path string) string {
	const prefix = "/v2/"
	const suffix = "/tags/list"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	end := len(path) - len(suffix)
	if end <= len(prefix) {
		return ""
	}
	return path[len(prefix):end]
}

// detectMediaType tries to determine the OCI media type from manifest JSON.
func detectMediaType(data []byte) string {
	var probe struct {
		MediaType string `json:"mediaType"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.MediaType != "" {
		return probe.MediaType
	}
	// Default to OCI manifest.
	return "application/vnd.oci.image.manifest.v1+json"
}

// clientIP extracts the client IP from the request.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.SplitN(fwd, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[:idx]
	}
	return addr
}
