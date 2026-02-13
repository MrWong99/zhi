package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSubmitRating(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{"score":5,"comment":"Excellent plugin!"}`
	req := httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	// Verify rating was stored.
	ratings, total, err := store.ListRatings("art-1", 1, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if ratings[0].Score != 5 {
		t.Errorf("score = %d, want 5", ratings[0].Score)
	}
	if ratings[0].Comment != "Excellent plugin!" {
		t.Errorf("comment = %q", ratings[0].Comment)
	}
}

func TestHandleSubmitRatingInvalidScore(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{"score":6}`
	req := httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSubmitRatingUnauthorized(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	body := `{"score":4}`
	req := httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleListRatings(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	// Submit a rating first.
	body := `{"score":4,"comment":"Good"}`
	req := httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("submit status = %d", w.Code)
	}

	// Now list ratings.
	req = httptest.NewRequest("GET", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	var resp listRatingsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
	if len(resp.Ratings) == 0 {
		t.Fatal("expected ratings")
	}
	if resp.Ratings[0].Score != 4 {
		t.Errorf("score = %d, want 4", resp.Ratings[0].Score)
	}
}

func TestHandleListRatingsEmpty(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	req := httptest.NewRequest("GET", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}

	var resp listRatingsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
}

func TestHandleRatingUpdate(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	// Submit first rating.
	body := `{"score":3,"comment":"OK"}`
	req := httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first submit: status = %d", w.Code)
	}

	// Update the rating.
	body = `{"score":5,"comment":"Much better now"}`
	req = httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("second submit: status = %d", w.Code)
	}

	// Verify only one rating exists (updated).
	ratings, total, _ := store.ListRatings("art-1", 1, 20, "")
	if total != 1 {
		t.Errorf("total = %d, want 1 (should update, not create)", total)
	}
	if len(ratings) > 0 && ratings[0].Score != 5 {
		t.Errorf("score = %d, want 5 (updated)", ratings[0].Score)
	}
}

func TestHandleRatingHelpful(t *testing.T) {
	h, store := newTestHandler(t)
	seedTestData(t, store)
	mux := NewMux(h)

	// Submit a rating first.
	body := `{"score":4}`
	req := httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("submit: status = %d", w.Code)
	}

	// Get the rating ID.
	ratings, _, _ := store.ListRatings("art-1", 1, 20, "")
	if len(ratings) == 0 {
		t.Fatal("no ratings found")
	}
	ratingID := ratings[0].ID

	// Mark as helpful.
	req = httptest.NewRequest("POST", "/api/v1/plugins/config/zhi-project/ansible-config/ratings/"+ratingID+"/helpful", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}

	// Verify helpful count.
	ratings, _, _ = store.ListRatings("art-1", 1, 20, "")
	if len(ratings) > 0 && ratings[0].Helpful != 1 {
		t.Errorf("helpful = %d, want 1", ratings[0].Helpful)
	}
}
