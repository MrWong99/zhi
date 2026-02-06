// Package marketplace provides an HTTP client for the zhi marketplace API.
// The marketplace is a metadata and discovery service that indexes plugins
// published to OCI registries. It provides search, plugin detail, and
// short-name resolution for the CLI.
package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultURL is the default marketplace API base URL.
const DefaultURL = "https://marketplace.zhi.dev"

// SearchResult represents a single plugin in search results.
type SearchResult struct {
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

// SearchResponse is the response from the search endpoint.
type SearchResponse struct {
	Total   int            `json:"total"`
	Results []SearchResult `json:"results"`
}

// SearchOptions controls search behaviour.
type SearchOptions struct {
	// Type filters results by plugin type (config, transform, store, ui).
	Type string
	// Sort orders results: relevance, downloads, rating, updated.
	Sort string
	// Verified limits results to verified publishers only.
	Verified bool
	// Limit is the maximum number of results.
	Limit int
	// Page is the result page number (1-based).
	Page int
}

// PluginVersion describes a single version of a plugin.
type PluginVersion struct {
	Version            string   `json:"version"`
	Digest             string   `json:"digest"`
	ZhiProtocolVersion string   `json:"zhiProtocolVersion,omitempty"`
	MinZhiVersion      string   `json:"minZhiVersion,omitempty"`
	Platforms          []string `json:"platforms,omitempty"`
	PublishedAt        string   `json:"publishedAt"`
	Signed             bool     `json:"signed"`
	Downloads          int64    `json:"downloads,omitempty"`
	Changelog          string   `json:"changelog,omitempty"`
}

// RatingDistribution holds the breakdown of ratings by star count.
type RatingDistribution struct {
	Five  int `json:"5"`
	Four  int `json:"4"`
	Three int `json:"3"`
	Two   int `json:"2"`
	One   int `json:"1"`
}

// PluginRating holds aggregate rating information.
type PluginRating struct {
	Average      float64            `json:"average"`
	Count        int                `json:"count"`
	Distribution RatingDistribution `json:"distribution"`
}

// PluginStatistics holds download and usage stats.
type PluginStatistics struct {
	TotalDownloads   int64 `json:"totalDownloads"`
	MonthlyDownloads int64 `json:"monthlyDownloads"`
	Dependents       int   `json:"dependents"`
}

// PluginDetail is the full plugin information from the marketplace.
type PluginDetail struct {
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	Author          string           `json:"author"`
	Verified        bool             `json:"verified"`
	Description     string           `json:"description"`
	LongDescription string           `json:"longDescription,omitempty"`
	License         string           `json:"license,omitempty"`
	Homepage        string           `json:"homepage,omitempty"`
	Repository      string           `json:"repository,omitempty"`
	OCIRef          string           `json:"ociRef"`
	Versions        []PluginVersion  `json:"versions"`
	Rating          PluginRating     `json:"rating"`
	Statistics      PluginStatistics `json:"statistics"`
}

// ResolveResponse is returned by the download resolution endpoint.
type ResolveResponse struct {
	OCIRef          string `json:"ociRef"`
	Digest          string `json:"digest"`
	Signed          bool   `json:"signed"`
	SigningIdentity string `json:"signingIdentity,omitempty"`
}

// VersionListResponse is the response from the version listing endpoint.
type VersionListResponse struct {
	Versions []PluginVersion `json:"versions"`
}

// RegisterPluginRequest is the body for plugin registration.
type RegisterPluginRequest struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	OCIRef      string   `json:"ociRef"`
	Description string   `json:"description"`
	Homepage    string   `json:"homepage,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// RegisterVersionRequest notifies the marketplace of a new version.
type RegisterVersionRequest struct {
	Version            string   `json:"version"`
	Digest             string   `json:"digest"`
	Platforms          []string `json:"platforms"`
	ZhiProtocolVersion string   `json:"zhiProtocolVersion,omitempty"`
	Changelog          string   `json:"changelog,omitempty"`
}

// WellKnown is the service discovery response.
type WellKnown struct {
	API        string   `json:"api"`
	Version    string   `json:"version"`
	Registries []string `json:"registries,omitempty"`
	SigningKey string   `json:"signingKey,omitempty"`
}

// APIError represents an error response from the marketplace.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("marketplace API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("marketplace API error %d", e.StatusCode)
}

// Client communicates with a zhi marketplace server.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithAPIKey sets the API key for authenticated requests.
func WithAPIKey(key string) ClientOption {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClient creates a marketplace API client.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Discover fetches the well-known service discovery document.
func (c *Client) Discover(ctx context.Context) (*WellKnown, error) {
	u := c.baseURL + "/.well-known/zhi-marketplace.json"
	var wk WellKnown
	if err := c.get(ctx, u, &wk); err != nil {
		return nil, fmt.Errorf("discovering marketplace: %w", err)
	}
	return &wk, nil
}

// Search queries the marketplace for plugins matching the given query.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResponse, error) {
	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	if opts.Type != "" {
		params.Set("type", opts.Type)
	}
	if opts.Sort != "" {
		params.Set("sort", opts.Sort)
	}
	if opts.Verified {
		params.Set("verified", "true")
	}
	if opts.Limit > 0 {
		params.Set("per_page", strconv.Itoa(opts.Limit))
	}
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}

	u := c.baseURL + "/api/v1/search?" + params.Encode()
	var resp SearchResponse
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, fmt.Errorf("searching marketplace: %w", err)
	}
	return &resp, nil
}

// GetPlugin fetches detailed information about a specific plugin.
func (c *Client) GetPlugin(ctx context.Context, publisher, name string) (*PluginDetail, error) {
	u := fmt.Sprintf("%s/api/v1/plugins/%s/%s", c.baseURL, url.PathEscape(publisher), url.PathEscape(name))
	var detail PluginDetail
	if err := c.get(ctx, u, &detail); err != nil {
		return nil, fmt.Errorf("getting plugin detail: %w", err)
	}
	return &detail, nil
}

// ListVersions lists all versions of a plugin.
func (c *Client) ListVersions(ctx context.Context, publisher, name string) (*VersionListResponse, error) {
	u := fmt.Sprintf("%s/api/v1/plugins/%s/%s/versions", c.baseURL, url.PathEscape(publisher), url.PathEscape(name))
	var resp VersionListResponse
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, fmt.Errorf("listing versions: %w", err)
	}
	return &resp, nil
}

// Resolve returns the OCI reference and digest for a specific version and platform.
func (c *Client) Resolve(ctx context.Context, publisher, name, version, os, arch string) (*ResolveResponse, error) {
	params := url.Values{}
	if os != "" {
		params.Set("os", os)
	}
	if arch != "" {
		params.Set("arch", arch)
	}
	u := fmt.Sprintf("%s/api/v1/plugins/%s/%s/%s/resolve?%s",
		c.baseURL,
		url.PathEscape(publisher),
		url.PathEscape(name),
		url.PathEscape(version),
		params.Encode(),
	)
	var resp ResolveResponse
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, fmt.Errorf("resolving plugin: %w", err)
	}
	return &resp, nil
}

// RegisterPlugin registers a new plugin with the marketplace. Requires API key.
func (c *Client) RegisterPlugin(ctx context.Context, req RegisterPluginRequest) error {
	u := c.baseURL + "/api/v1/plugins"
	if err := c.post(ctx, u, req, nil); err != nil {
		return fmt.Errorf("registering plugin: %w", err)
	}
	return nil
}

// RegisterVersion notifies the marketplace of a new plugin version. Requires API key.
func (c *Client) RegisterVersion(ctx context.Context, publisher, name string, req RegisterVersionRequest) error {
	u := fmt.Sprintf("%s/api/v1/plugins/%s/%s/versions",
		c.baseURL,
		url.PathEscape(publisher),
		url.PathEscape(name),
	)
	if err := c.post(ctx, u, req, nil); err != nil {
		return fmt.Errorf("registering version: %w", err)
	}
	return nil
}

// ResolveShortName attempts to resolve a short plugin name (e.g. "ansible-config")
// to a full OCI reference via the marketplace. It searches for the exact name
// and returns the OCI reference of the best match.
func (c *Client) ResolveShortName(ctx context.Context, shortName string) (string, error) {
	// Split version if present: name@version
	name := shortName
	version := ""
	if idx := strings.LastIndex(shortName, "@"); idx >= 0 {
		name = shortName[:idx]
		version = shortName[idx+1:]
	}

	resp, err := c.Search(ctx, name, SearchOptions{Limit: 10})
	if err != nil {
		return "", err
	}

	// Find exact name match.
	for _, r := range resp.Results {
		if r.Name == name {
			ref := r.OCIRef
			if version != "" {
				ref += ":v" + strings.TrimPrefix(version, "v")
			}
			return ref, nil
		}
	}

	return "", fmt.Errorf("plugin %q not found in marketplace", name)
}

// get performs an authenticated GET request and decodes the JSON response.
func (c *Client) get(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.readError(resp)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// post performs an authenticated POST request with a JSON body.
func (c *Client) post(ctx context.Context, rawURL string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bodyReader)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.readError(resp)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// setHeaders adds common headers to a request.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "zhi-cli/1.0")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// readError reads an error response body and returns an APIError.
func (c *Client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	apiErr := &APIError{StatusCode: resp.StatusCode}
	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errResp) == nil {
		if errResp.Error != "" {
			apiErr.Message = errResp.Error
		} else if errResp.Message != "" {
			apiErr.Message = errResp.Message
		}
	}
	if apiErr.Message == "" && len(body) > 0 {
		apiErr.Message = strings.TrimSpace(string(body))
	}
	return apiErr
}
