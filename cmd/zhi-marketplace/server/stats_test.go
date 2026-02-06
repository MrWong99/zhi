package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRecordDownload(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	req := httptest.NewRequest("POST", "/api/v1/plugins/zhi-project/ansible-config/1.2.0/download?platform=linux/amd64", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	// Verify download was recorded.
	total, monthly, err := store.GetDownloadStats("art-1")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if monthly != 1 {
		t.Errorf("monthly = %d, want 1", monthly)
	}
}

func TestHandleRecordDownloadNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	req := httptest.NewRequest("POST", "/api/v1/plugins/nobody/missing/1.0.0/download", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleGetStats(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	// Record a download.
	req := httptest.NewRequest("POST", "/api/v1/plugins/zhi-project/ansible-config/1.2.0/download?platform=linux/amd64", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("download: status = %d", w.Code)
	}

	// Submit a rating.
	body := `{"score":4}`
	req = httptest.NewRequest("POST", "/api/v1/plugins/zhi-project/ansible-config/ratings",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Get stats.
	req = httptest.NewRequest("GET", "/api/v1/plugins/zhi-project/ansible-config/stats", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	downloads, ok := resp["downloads"].(map[string]any)
	if !ok {
		t.Fatal("expected downloads in response")
	}
	if downloads["total"].(float64) < 1 {
		t.Errorf("total downloads = %v", downloads["total"])
	}
}

func TestHandleGetStatsNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/plugins/nobody/missing/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
