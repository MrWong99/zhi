package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MetadataStore caches marketplace API responses on the local filesystem.
// It uses JSON files for simplicity, avoiding the need for a SQLite dependency.
type MetadataStore struct {
	mu   sync.RWMutex
	root string
}

// CachedResponse holds a cached API response with its timestamp.
type CachedResponse struct {
	Data      json.RawMessage `json:"data"`
	CachedAt  time.Time       `json:"cachedAt"`
	SourceURL string          `json:"sourceUrl,omitempty"`
}

// NewMetadataStore creates or opens a metadata store at the given root directory.
func NewMetadataStore(root string) (*MetadataStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating metadata directory: %w", err)
	}
	return &MetadataStore{root: root}, nil
}

// Get retrieves a cached response for the given cache key. Returns nil if
// not found or if the entry is older than maxAge.
func (s *MetadataStore) Get(key string, maxAge time.Duration) (*CachedResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	var entry CachedResponse
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parsing cache entry: %w", err)
	}

	if maxAge > 0 && time.Since(entry.CachedAt) > maxAge {
		return nil, nil // expired
	}
	return &entry, nil
}

// Put stores a response in the cache.
func (s *MetadataStore) Put(key string, responseData json.RawMessage, sourceURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := CachedResponse{
		Data:      responseData,
		CachedAt:  time.Now().UTC(),
		SourceURL: sourceURL,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshalling cache entry: %w", err)
	}

	path := s.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing cache entry: %w", err)
	}
	return nil
}

// Delete removes a cached entry.
func (s *MetadataStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.path(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting cache entry: %w", err)
	}
	return nil
}

// path returns the filesystem path for a cache key.
func (s *MetadataStore) path(key string) string {
	// Use SHA-256 of the key to avoid filesystem path issues.
	hash := ComputeSHA256([]byte(key))
	// Strip "sha256:" prefix for filename.
	hex := hash[7:]
	return filepath.Join(s.root, hex+".json")
}
