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

// MarketplaceProxy proxies and caches marketplace API responses, filtering
// results according to the mirror policy.
type MarketplaceProxy struct {
	upstream string
	meta     *storage.MetadataStore
	policy   *Policy
	client   *http.Client
	cacheTTL time.Duration
}

// NewMarketplaceProxy creates a marketplace API proxy. If httpClient is
// nil, a default client with a 30-second timeout is used.
func NewMarketplaceProxy(upstream string, meta *storage.MetadataStore, policy *Policy, httpClient *http.Client) *MarketplaceProxy {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &MarketplaceProxy{
		upstream: strings.TrimRight(upstream, "/"),
		meta:     meta,
		policy:   policy,
		client:   httpClient,
		cacheTTL: 5 * time.Minute,
	}
}

// HandleSearch proxies GET /api/v1/search with policy filtering.
func (m *MarketplaceProxy) HandleSearch(w http.ResponseWriter, r *http.Request) {
	cacheKey := "search:" + r.URL.RawQuery

	// Try cache.
	cached, err := m.meta.Get(cacheKey, m.cacheTTL)
	if err == nil && cached != nil {
		data := m.filterSearchResults(cached.Data)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(data) //nolint:errcheck
		return
	}

	// Proxy to upstream.
	upstreamURL := m.upstream + "/api/v1/search?" + r.URL.RawQuery
	data, fetchErr := m.fetchUpstream(upstreamURL)
	if fetchErr != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, fetchErr.Error()), http.StatusBadGateway)
		return
	}

	// Cache the raw response.
	if err := m.meta.Put(cacheKey, data, upstreamURL); err != nil {
		log.Printf("mirror: failed to cache search results: %v", err)
	}

	// Filter and return.
	filtered := m.filterSearchResults(data)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(filtered) //nolint:errcheck
}

// HandlePlugin proxies GET /api/v1/plugins/{publisher}/{name}.
func (m *MarketplaceProxy) HandlePlugin(w http.ResponseWriter, r *http.Request) {
	cacheKey := "plugin:" + r.URL.Path

	cached, err := m.meta.Get(cacheKey, m.cacheTTL)
	if err == nil && cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached.Data) //nolint:errcheck
		return
	}

	upstreamURL := m.upstream + r.URL.Path
	data, fetchErr := m.fetchUpstream(upstreamURL)
	if fetchErr != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, fetchErr.Error()), http.StatusBadGateway)
		return
	}

	if err := m.meta.Put(cacheKey, data, upstreamURL); err != nil {
		log.Printf("mirror: failed to cache plugin detail: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data) //nolint:errcheck
}

// HandleVersions proxies GET /api/v1/plugins/{publisher}/{name}/versions.
func (m *MarketplaceProxy) HandleVersions(w http.ResponseWriter, r *http.Request) {
	cacheKey := "versions:" + r.URL.Path

	cached, err := m.meta.Get(cacheKey, m.cacheTTL)
	if err == nil && cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached.Data) //nolint:errcheck
		return
	}

	upstreamURL := m.upstream + r.URL.Path
	data, fetchErr := m.fetchUpstream(upstreamURL)
	if fetchErr != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, fetchErr.Error()), http.StatusBadGateway)
		return
	}

	if err := m.meta.Put(cacheKey, data, upstreamURL); err != nil {
		log.Printf("mirror: failed to cache versions: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data) //nolint:errcheck
}

// HandleWellKnown serves the service discovery document for this mirror.
func (m *MarketplaceProxy) HandleWellKnown(w http.ResponseWriter, r *http.Request) {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	resp := map[string]any{
		"api":     fmt.Sprintf("%s://%s/api/v1", scheme, r.Host),
		"version": "1",
		"mirror":  true,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// fetchUpstream performs a GET request to the upstream marketplace.
func (m *MarketplaceProxy) fetchUpstream(url string) (json.RawMessage, error) {
	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading upstream response: %w", err)
	}
	return json.RawMessage(data), nil
}

// filterSearchResults removes blocked publishers and artifacts from search results.
func (m *MarketplaceProxy) filterSearchResults(data json.RawMessage) json.RawMessage {
	var resp struct {
		Total   int               `json:"total"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return data // return unfiltered on parse error
	}

	var filtered []json.RawMessage
	for _, raw := range resp.Results {
		var result struct {
			Name   string `json:"name"`
			Author string `json:"author"`
			Type   string `json:"type"`
			OCIRef string `json:"ociRef"`
		}
		if json.Unmarshal(raw, &result) != nil {
			continue
		}

		if !m.policy.IsPublisherAllowed(result.Author) {
			continue
		}
		if m.policy.IsArtifactBlocked(result.OCIRef) {
			continue
		}
		if !m.policy.IsTypeAllowed(result.Type) {
			continue
		}
		filtered = append(filtered, raw)
	}

	out := map[string]any{
		"total":   len(filtered),
		"results": filtered,
	}
	result, err := json.Marshal(out)
	if err != nil {
		return data
	}
	return result
}
