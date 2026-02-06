// Package auth provides authentication for the marketplace server.
// It supports API key authentication for CLI operations.
package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// Authenticator validates requests.
type Authenticator struct {
	// apiKeys maps API key strings to publisher IDs.
	apiKeys map[string]string
}

// NewAuthenticator creates an authenticator with the given API key -> publisher mappings.
func NewAuthenticator(keys map[string]string) *Authenticator {
	return &Authenticator{apiKeys: keys}
}

// ValidateRequest extracts and validates the Bearer token from the Authorization header.
// Returns the publisher ID associated with the API key, or an error.
func (a *Authenticator) ValidateRequest(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return "", fmt.Errorf("invalid Authorization scheme (expected Bearer)")
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	return a.ValidateAPIKey(token)
}

// ValidateAPIKey checks an API key and returns the associated publisher ID.
func (a *Authenticator) ValidateAPIKey(key string) (string, error) {
	for storedKey, publisherID := range a.apiKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(storedKey)) == 1 {
			return publisherID, nil
		}
	}
	return "", fmt.Errorf("invalid API key")
}

// RequireAuth is middleware that validates the request has a valid API key.
func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		publisherID, err := a.ValidateRequest(r)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnauthorized)
			return
		}
		// Store publisher ID in the request header for downstream handlers.
		r.Header.Set("X-Publisher-ID", publisherID)
		next.ServeHTTP(w, r)
	})
}
