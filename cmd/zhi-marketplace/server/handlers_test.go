package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/cmd/zhi-marketplace/auth"
	"github.com/MrWong99/zhi/cmd/zhi-marketplace/storage"
)

func newTestHandler(t *testing.T) (*Handler, storage.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.json")
	store, err := storage.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	keys := map[string]string{"test-key": "pub-1"}
	authenticator := auth.NewAuthenticator(keys)
	h := NewHandler(store, authenticator, []string{"ghcr.io"})
	return h, store
}

func seedTestData(t *testing.T, store storage.Store) {
	t.Helper()
	pub := &storage.Publisher{ID: "pub-1", Name: "zhi-project", Email: "a@b.com", Verified: true}
	if err := store.CreatePublisher(pub); err != nil {
		t.Fatal(err)
	}
	art := &storage.Artifact{
		ID:          "art-1",
		PublisherID: "pub-1",
		Name:        "ansible-config",
		Type:        "config",
		OCIRef:      "oci://ghcr.io/zhi-project/zhi-config-ansible",
		Description: "Ansible config provider",
		Keywords:    []string{"ansible", "config"},
	}
	if err := store.CreateArtifact(art); err != nil {
		t.Fatal(err)
	}
	v := &storage.Version{
		ID:         "v-1",
		ArtifactID: "art-1",
		Version:    "1.2.0",
		Digest:     "sha256:abc123",
		Platforms:  []string{"linux/amd64", "darwin/arm64"},
		Signed:     true,
	}
	if err := store.CreateVersion(v); err != nil {
		t.Fatal(err)
	}
}

func TestHandleWellKnown(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/.well-known/zhi-marketplace.json", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["version"] != "1" {
		t.Errorf("version = %v", resp["version"])
	}
}

func TestHandleSearch(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/search?q=ansible", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	var resp searchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results")
	}
	if resp.Results[0].Name != "ansible-config" {
		t.Errorf("name = %q", resp.Results[0].Name)
	}
}

func TestHandleSearchNoResults(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/search?q=nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	var resp searchResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 0 {
		t.Errorf("total = %d", resp.Total)
	}
}

func TestHandleGetPlugin(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/plugins/config/zhi-project/ansible-config", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "ansible-config" {
		t.Errorf("name = %v", resp["name"])
	}
	if resp["type"] != "config" {
		t.Errorf("type = %v", resp["type"])
	}
}

func TestHandleGetPluginNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/plugins/config/nobody/missing", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleListVersions(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/plugins/config/zhi-project/ansible-config/versions", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	versions, ok := resp["versions"].([]any)
	if !ok || len(versions) != 1 {
		t.Errorf("versions = %v", resp["versions"])
	}
}

func TestHandleResolve(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/plugins/config/zhi-project/ansible-config/1.2.0/resolve?os=linux&arch=amd64", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["signed"] != true {
		t.Errorf("signed = %v", resp["signed"])
	}
	if resp["digest"] != "sha256:abc123" {
		t.Errorf("digest = %v", resp["digest"])
	}
}

func TestHandleRegisterPlugin(t *testing.T) {
	h, store := newTestHandler(t)
	mux := NewMux(h)

	// Create publisher first.
	pub := &storage.Publisher{ID: "pub-1", Name: "org", Email: "a@b.com"}
	store.CreatePublisher(pub)

	body := `{"name":"my-plugin","type":"config","ociRef":"oci://ghcr.io/org/plugin","description":"Test"}`
	req := httptest.NewRequest("POST", "/api/v1/plugins", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	// Verify it was created.
	art, _ := store.GetArtifact("org", "my-plugin", "config")
	if art == nil {
		t.Fatal("expected artifact to be created")
	}
	if art.Description != "Test" {
		t.Errorf("Description = %q", art.Description)
	}
}

func TestHandleRegisterPluginUnauthorized(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	body := `{"name":"my-plugin","type":"config","ociRef":"oci://ref"}`
	req := httptest.NewRequest("POST", "/api/v1/plugins", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleRegisterVersion(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{"version":"2.0.0","digest":"sha256:def","platforms":["linux/amd64"]}`
	req := httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/versions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	// Verify version was created.
	versions, _ := store.ListVersions("art-1")
	if len(versions) != 2 {
		t.Errorf("len(versions) = %d, want 2", len(versions))
	}
}

func TestExtractPluginPath(t *testing.T) {
	tests := []struct {
		path       string
		pluginType string
		publisher  string
		name       string
	}{
		{"/api/v1/plugins/config/org/plugin", "config", "org", "plugin"},
		{"/api/v1/plugins/store/org/plugin/extra", "store", "org", "plugin"},
		{"/api/v1/plugins/", "", "", ""},
	}
	for _, tt := range tests {
		typ, pub, name := extractPluginPath(tt.path)
		if typ != tt.pluginType || pub != tt.publisher || name != tt.name {
			t.Errorf("extractPluginPath(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.path, typ, pub, name, tt.pluginType, tt.publisher, tt.name)
		}
	}
}

func TestExtractVersionsPath(t *testing.T) {
	tests := []struct {
		path       string
		pluginType string
		publisher  string
		name       string
	}{
		{"/api/v1/plugins/config/org/plugin/versions", "config", "org", "plugin"},
		{"/api/v1/plugins/store/org/plugin", "store", "org", "plugin"},
	}
	for _, tt := range tests {
		typ, pub, name := extractVersionsPath(tt.path)
		if typ != tt.pluginType || pub != tt.publisher || name != tt.name {
			t.Errorf("extractVersionsPath(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.path, typ, pub, name, tt.pluginType, tt.publisher, tt.name)
		}
	}
}

func TestExtractResolvePath(t *testing.T) {
	typ, pub, name, ver := extractResolvePath("/api/v1/plugins/config/org/plugin/1.0.0/resolve")
	if typ != "config" || pub != "org" || name != "plugin" || ver != "1.0.0" {
		t.Errorf("got (%q, %q, %q, %q)", typ, pub, name, ver)
	}
}
