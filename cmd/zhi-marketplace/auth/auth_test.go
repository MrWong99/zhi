package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateAPIKey(t *testing.T) {
	a := NewAuthenticator(map[string]string{
		"zhk_abc123": "pub-1",
		"zhk_def456": "pub-2",
	})

	id, err := a.ValidateAPIKey("zhk_abc123")
	if err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}
	if id != "pub-1" {
		t.Errorf("id = %q, want pub-1", id)
	}

	_, err = a.ValidateAPIKey("invalid")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestValidateRequest(t *testing.T) {
	a := NewAuthenticator(map[string]string{"key1": "pub-1"})

	// Valid request.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer key1")
	id, err := a.ValidateRequest(req)
	if err != nil {
		t.Fatalf("ValidateRequest: %v", err)
	}
	if id != "pub-1" {
		t.Errorf("id = %q", id)
	}

	// Missing header.
	req2 := httptest.NewRequest("GET", "/", nil)
	_, err = a.ValidateRequest(req2)
	if err == nil {
		t.Error("expected error for missing auth")
	}

	// Wrong scheme.
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.Header.Set("Authorization", "Basic abc")
	_, err = a.ValidateRequest(req3)
	if err == nil {
		t.Error("expected error for wrong scheme")
	}
}

func TestRequireAuthMiddleware(t *testing.T) {
	a := NewAuthenticator(map[string]string{"key1": "pub-1"})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pubID := r.Header.Get("X-Publisher-ID")
		if pubID != "pub-1" {
			t.Errorf("X-Publisher-ID = %q", pubID)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := a.RequireAuth(inner)

	// Authorized request.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer key1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	// Unauthorized request.
	req2 := httptest.NewRequest("GET", "/", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w2.Code)
	}
}

func TestEmptyKeys(t *testing.T) {
	a := NewAuthenticator(map[string]string{})
	_, err := a.ValidateAPIKey("anything")
	if err == nil {
		t.Error("expected error with no keys configured")
	}
}
