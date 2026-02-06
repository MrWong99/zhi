package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGetPublisher(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/publishers/zhi-project", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp publisherProfileResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "zhi-project" {
		t.Errorf("name = %q", resp.Name)
	}
	if !resp.Verified {
		t.Error("expected verified = true")
	}
	if resp.PluginCount != 1 {
		t.Errorf("pluginCount = %d, want 1", resp.PluginCount)
	}
}

func TestHandleGetPublisherNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/publishers/nobody", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleVerifyRequest(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{"repository":"https://github.com/zhi-project/ansible-config","license":"MIT"}`
	req := httptest.NewRequest("POST", "/api/v1/publishers/zhi-project/verify-request", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// The publisher is already verified (tier default 0 in test data, but verified=true),
	// so let's check it doesn't error.
	if w.Code != http.StatusAccepted && w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleVerifyRequestUnauthorized(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{"repository":"https://example.com"}`
	req := httptest.NewRequest("POST", "/api/v1/publishers/zhi-project/verify-request", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
