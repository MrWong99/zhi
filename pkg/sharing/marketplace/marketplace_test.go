package marketplace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.com")
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want https://example.com", c.baseURL)
	}
	if c.apiKey != "" {
		t.Error("apiKey should be empty")
	}
}

func TestNewClientWithOptions(t *testing.T) {
	c := NewClient("https://example.com/", WithAPIKey("test-key"))
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want trailing slash stripped", c.baseURL)
	}
	if c.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want test-key", c.apiKey)
	}
}

func TestClientSearch(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("path = %s, want /api/v1/search", r.URL.Path)
		}
		if q := r.URL.Query().Get("q"); q != "ansible" {
			t.Errorf("q = %q, want ansible", q)
		}
		if typ := r.URL.Query().Get("type"); typ != "config" {
			t.Errorf("type = %q, want config", typ)
		}
		if pp := r.URL.Query().Get("per_page"); pp != "10" {
			t.Errorf("per_page = %q, want 10", pp)
		}

		resp := SearchResponse{
			Total: 1,
			Results: []SearchResult{
				{
					Name:          "ansible-config",
					Type:          "config",
					Description:   "Ansible config provider",
					Author:        "zhi-project",
					Verified:      true,
					LatestVersion: "1.2.0",
					OCIRef:        "oci://ghcr.io/zhi-project/zhi-config-ansible",
					Downloads:     12450,
					Rating:        4.7,
					RatingCount:   89,
					Keywords:      []string{"ansible"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.Search(context.Background(), "ansible", SearchOptions{
		Type:  "config",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(resp.Results))
	}
	r := resp.Results[0]
	if r.Name != "ansible-config" {
		t.Errorf("Name = %q", r.Name)
	}
	if r.Type != "config" {
		t.Errorf("Type = %q", r.Type)
	}
	if !r.Verified {
		t.Error("Verified should be true")
	}
	if r.LatestVersion != "1.2.0" {
		t.Errorf("LatestVersion = %q", r.LatestVersion)
	}
}

func TestClientSearchEmpty(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := SearchResponse{Total: 0, Results: []SearchResult{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.Search(context.Background(), "nonexistent", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("Total = %d", resp.Total)
	}
}

func TestClientGetPlugin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/plugins/config/zhi-project/ansible-config" {
			t.Errorf("path = %s", r.URL.Path)
		}
		detail := PluginDetail{
			Name:        "ansible-config",
			Type:        "config",
			Author:      "zhi-project",
			Verified:    true,
			Description: "Ansible config",
			OCIRef:      "oci://ghcr.io/zhi-project/zhi-config-ansible",
			Versions: []PluginVersion{
				{Version: "1.2.0", Digest: "sha256:abc"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)
	detail, err := c.GetPlugin(context.Background(), "config", "zhi-project", "ansible-config")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	if detail.Name != "ansible-config" {
		t.Errorf("Name = %q", detail.Name)
	}
	if len(detail.Versions) != 1 {
		t.Fatalf("len(Versions) = %d", len(detail.Versions))
	}
}

func TestClientResolve(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/plugins/config/zhi-project/ansible-config/1.2.0/resolve" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("os") != "linux" {
			t.Errorf("os = %q", r.URL.Query().Get("os"))
		}
		if r.URL.Query().Get("arch") != "amd64" {
			t.Errorf("arch = %q", r.URL.Query().Get("arch"))
		}
		resp := ResolveResponse{
			OCIRef: "oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
			Digest: "sha256:abc123",
			Signed: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.Resolve(context.Background(), "config", "zhi-project", "ansible-config", "1.2.0", "linux", "amd64")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !resp.Signed {
		t.Error("expected Signed=true")
	}
	if resp.Digest != "sha256:abc123" {
		t.Errorf("Digest = %q", resp.Digest)
	}
}

func TestClientResolveShortName(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		resp := SearchResponse{
			Total: 1,
			Results: []SearchResult{
				{
					Name:          q,
					OCIRef:        "oci://ghcr.io/zhi-project/zhi-config-" + q,
					LatestVersion: "1.0.0",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)

	// Without version.
	ref, err := c.ResolveShortName(context.Background(), "ansible-config")
	if err != nil {
		t.Fatalf("ResolveShortName: %v", err)
	}
	if ref != "oci://ghcr.io/zhi-project/zhi-config-ansible-config" {
		t.Errorf("ref = %q", ref)
	}

	// With version.
	ref, err = c.ResolveShortName(context.Background(), "ansible-config@1.2.0")
	if err != nil {
		t.Fatalf("ResolveShortName: %v", err)
	}
	if ref != "oci://ghcr.io/zhi-project/zhi-config-ansible-config:v1.2.0" {
		t.Errorf("ref = %q", ref)
	}
}

func TestClientResolveShortNameNotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := SearchResponse{Total: 0, Results: []SearchResult{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.ResolveShortName(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestClientAPIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "plugin not found"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetPlugin(context.Background(), "config", "nobody", "missing")
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify it's an APIError.
	apiErr, ok := err.(*APIError)
	// The error is wrapped by fmt.Errorf, so unwrap it.
	if !ok {
		// Try unwrapping.
		var inner *APIError
		if wrappedErr, ok2 := err.(interface{ Unwrap() error }); ok2 {
			inner, _ = wrappedErr.Unwrap().(*APIError)
		}
		if inner == nil {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		apiErr = inner
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
}

func TestClientAuthHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("Authorization = %q, want Bearer test-api-key", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResponse{Total: 0, Results: []SearchResult{}})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("test-api-key"))
	_, err := c.Search(context.Background(), "", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
}

func TestClientDiscover(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/zhi-marketplace.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		resp := WellKnown{
			API:        "https://marketplace.zhi.dev/api/v1",
			Version:    "1",
			Registries: []string{"ghcr.io"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL)
	wk, err := c.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if wk.Version != "1" {
		t.Errorf("Version = %q", wk.Version)
	}
	if len(wk.Registries) != 1 || wk.Registries[0] != "ghcr.io" {
		t.Errorf("Registries = %v", wk.Registries)
	}
}

func TestClientRegisterPlugin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/plugins" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}

		var req RegisterPluginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Name != "my-plugin" {
			t.Errorf("Name = %q", req.Name)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("key"))
	err := c.RegisterPlugin(context.Background(), RegisterPluginRequest{
		Name:   "my-plugin",
		Type:   "config",
		OCIRef: "oci://ghcr.io/org/plugin",
	})
	if err != nil {
		t.Fatalf("RegisterPlugin: %v", err)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	e := &APIError{StatusCode: 404, Message: "not found"}
	if e.Error() != "marketplace API error 404: not found" {
		t.Errorf("Error() = %q", e.Error())
	}

	e2 := &APIError{StatusCode: 500}
	if e2.Error() != "marketplace API error 500" {
		t.Errorf("Error() = %q", e2.Error())
	}
}

func TestDefaultURL(t *testing.T) {
	if DefaultURL != "https://marketplace.zhi.dev" {
		t.Errorf("DefaultURL = %q", DefaultURL)
	}
}
