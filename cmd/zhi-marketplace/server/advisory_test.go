package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/cmd/zhi-marketplace/storage"
)

func TestHandleCreateAdvisory(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{
		"publisher":"zhi-project",
		"plugin":"ansible-config",
		"severity":"high",
		"title":"Path traversal in config loading",
		"description":"A path traversal vulnerability was found.",
		"affectedVersions":"<1.2.0",
		"fixedVersion":"1.2.0",
		"cve":"CVE-2026-0001"
	}`
	req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "published" {
		t.Errorf("status = %q", resp["status"])
	}
}

func TestHandleCreateAdvisoryInvalidSeverity(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{
		"publisher":"zhi-project",
		"plugin":"ansible-config",
		"severity":"extreme",
		"title":"Bad severity",
		"description":"Testing invalid severity",
		"affectedVersions":"<1.0"
	}`
	req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateAdvisoryUnauthorized(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	body := `{"publisher":"zhi-project","plugin":"test","severity":"low","title":"t","description":"d","affectedVersions":"<1.0"}`
	req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleCreateAdvisoryForeignPluginForbidden(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)

	// A second publisher owns a different plugin.
	other := &storage.Publisher{ID: "pub-2", Name: "other-org", Email: "o@b.com"}
	if err := store.CreatePublisher(other); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateArtifact(&storage.Artifact{
		PublisherID: "pub-2", Name: "other-plugin", Type: "config", OCIRef: "oci://x/y",
	}); err != nil {
		t.Fatal(err)
	}
	mux := NewMux(h)

	// pub-1 (test-key) tries to file an advisory against pub-2's plugin.
	body := `{"publisher":"other-org","plugin":"other-plugin","severity":"critical","title":"RCE","description":"forged","affectedVersions":"<9.9"}`
	req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateAdvisoryUnknownPluginNotFound(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{"publisher":"zhi-project","plugin":"does-not-exist","severity":"high","title":"t","description":"d","affectedVersions":"<1.0"}`
	req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleListAdvisoriesUnknownPluginReturnsEmpty(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	// File a real advisory against the owned plugin.
	body := `{"publisher":"zhi-project","plugin":"ansible-config","severity":"critical","title":"RCE","description":"d","affectedVersions":"<1.1.0"}`
	req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d", w.Code)
	}

	// Filtering by a plugin that does not exist must not leak the advisory.
	req = httptest.NewRequest("GET", "/api/v1/advisories?publisher=zhi-project&plugin=nonexistent", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp map[string][]advisoryResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp["advisories"]) != 0 {
		t.Errorf("len = %d, want 0 (filter must not fall open)", len(resp["advisories"]))
	}
}

func TestHandleListAdvisories(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	// Create an advisory first.
	body := `{
		"publisher":"zhi-project",
		"plugin":"ansible-config",
		"severity":"critical",
		"title":"Remote code execution",
		"description":"A remote code execution vulnerability was found.",
		"affectedVersions":"<1.1.0"
	}`
	req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d", w.Code)
	}

	// List advisories.
	req = httptest.NewRequest("GET", "/api/v1/advisories", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	var resp map[string][]advisoryResponse
	json.NewDecoder(w.Body).Decode(&resp)
	advisories := resp["advisories"]
	if len(advisories) != 1 {
		t.Fatalf("len(advisories) = %d, want 1", len(advisories))
	}
	if advisories[0].Severity != "critical" {
		t.Errorf("severity = %q", advisories[0].Severity)
	}
	if advisories[0].Title != "Remote code execution" {
		t.Errorf("title = %q", advisories[0].Title)
	}
}

func TestHandleListAdvisoriesFilterBySeverity(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	// Create two advisories with different severities.
	for _, sev := range []string{"low", "critical"} {
		body := `{"publisher":"zhi-project","plugin":"ansible-config","severity":"` + sev + `","title":"Advisory ` + sev + `","description":"Test","affectedVersions":"<1.0"}`
		req := httptest.NewRequest("POST", "/api/v1/advisories", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: status = %d", sev, w.Code)
		}
	}

	// Filter by critical.
	req := httptest.NewRequest("GET", "/api/v1/advisories?severity=critical", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	var resp map[string][]advisoryResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp["advisories"]) != 1 {
		t.Errorf("len = %d, want 1 (only critical)", len(resp["advisories"]))
	}
}
