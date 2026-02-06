// Package storage implements the local OCI Layout storage backend for the
// zhi-mirror registry proxy.
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OCILayout manages OCI artifacts stored in OCI Image Layout format on disk.
// The directory structure follows the OCI Image Layout Specification:
//
//	<root>/
//	  oci-layout          (required, contains {"imageLayoutVersion":"1.0.0"})
//	  index.json          (index of all stored manifests)
//	  blobs/sha256/<hex>  (content-addressed blob storage)
type OCILayout struct {
	mu   sync.RWMutex
	root string
}

// layoutFile is the content of the oci-layout marker file.
const layoutFile = `{"imageLayoutVersion":"1.0.0"}`

// TagEntry tracks a tag-to-digest mapping with a timestamp.
type TagEntry struct {
	Digest    string    `json:"digest"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NewOCILayout creates or opens an OCI Layout storage at the given root directory.
func NewOCILayout(root string) (*OCILayout, error) {
	s := &OCILayout{root: root}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

// init creates the directory structure and oci-layout file if they don't exist.
func (s *OCILayout) init() error {
	blobDir := filepath.Join(s.root, "blobs", "sha256")
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		return fmt.Errorf("creating blob directory: %w", err)
	}

	layoutPath := filepath.Join(s.root, "oci-layout")
	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		if err := os.WriteFile(layoutPath, []byte(layoutFile), 0o644); err != nil {
			return fmt.Errorf("writing oci-layout: %w", err)
		}
	}

	indexPath := filepath.Join(s.root, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		empty := &ociIndex{SchemaVersion: 2, Manifests: []ociIndexEntry{}}
		data, err := json.MarshalIndent(empty, "", "  ")
		if err != nil {
			return fmt.Errorf("marshalling empty index: %w", err)
		}
		if err := os.WriteFile(indexPath, data, 0o644); err != nil {
			return fmt.Errorf("writing index.json: %w", err)
		}
	}

	return nil
}

// ociIndex is a minimal representation of the OCI index.json.
type ociIndex struct {
	SchemaVersion int             `json:"schemaVersion"`
	Manifests     []ociIndexEntry `json:"manifests"`
}

// ociIndexEntry is an entry in index.json.
type ociIndexEntry struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// HasBlob checks whether a blob with the given digest exists.
func (s *OCILayout) HasBlob(dgst string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := os.Stat(s.blobPath(dgst))
	return err == nil
}

// GetBlob reads a blob by digest. Returns the content and true if found,
// or nil and false if not found.
func (s *OCILayout) GetBlob(dgst string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.blobPath(dgst))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reading blob: %w", err)
	}
	return data, true, nil
}

// PutBlob stores a blob. The digest is computed from the data and returned.
// If the blob already exists, this is a no-op.
func (s *OCILayout) PutBlob(data []byte) (string, error) {
	dgst := ComputeSHA256(data)
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.blobPath(dgst)
	if _, err := os.Stat(path); err == nil {
		return dgst, nil // already exists
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing blob: %w", err)
	}
	return dgst, nil
}

// PutBlobFromReader stores a blob from a reader. Returns the digest and size.
func (s *OCILayout) PutBlobFromReader(r io.Reader) (string, int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", 0, fmt.Errorf("reading blob data: %w", err)
	}
	dgst, err := s.PutBlob(data)
	if err != nil {
		return "", 0, err
	}
	return dgst, int64(len(data)), nil
}

// DeleteBlob removes a blob by digest.
func (s *OCILayout) DeleteBlob(dgst string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.blobPath(dgst)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting blob: %w", err)
	}
	return nil
}

// PutTag records a tag-to-digest mapping for a repository.
func (s *OCILayout) PutTag(repo, tag, dgst string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putTagLocked(repo, tag, dgst)
}

func (s *OCILayout) putTagLocked(repo, tag, dgst string) error {
	tagsDir := filepath.Join(s.root, "tags", sanitizePath(repo))
	if err := os.MkdirAll(tagsDir, 0o755); err != nil {
		return fmt.Errorf("creating tags directory: %w", err)
	}
	entry := TagEntry{Digest: dgst, UpdatedAt: time.Now().UTC()}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshalling tag entry: %w", err)
	}
	tagFile := filepath.Join(tagsDir, sanitizeTag(tag)+".json")
	if err := os.WriteFile(tagFile, data, 0o644); err != nil {
		return fmt.Errorf("writing tag file: %w", err)
	}
	return nil
}

// GetTag resolves a tag to a digest for a repository. Returns the digest and
// true if found, or empty string and false if not found.
func (s *OCILayout) GetTag(repo, tag string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tagFile := filepath.Join(s.root, "tags", sanitizePath(repo), sanitizeTag(tag)+".json")
	data, err := os.ReadFile(tagFile)
	if err != nil {
		return "", false
	}
	var entry TagEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}
	return entry.Digest, true
}

// GetTagEntry returns the full tag entry including timestamps.
func (s *OCILayout) GetTagEntry(repo, tag string) (*TagEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tagFile := filepath.Join(s.root, "tags", sanitizePath(repo), sanitizeTag(tag)+".json")
	data, err := os.ReadFile(tagFile)
	if err != nil {
		return nil, false
	}
	var entry TagEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	return &entry, true
}

// ListTags returns all tags for a repository.
func (s *OCILayout) ListTags(repo string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tagsDir := filepath.Join(s.root, "tags", sanitizePath(repo))
	entries, err := os.ReadDir(tagsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading tags directory: %w", err)
	}
	var tags []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if before, ok := strings.CutSuffix(name, ".json"); ok {
			tags = append(tags, before)
		}
	}
	return tags, nil
}

// Root returns the storage root directory.
func (s *OCILayout) Root() string {
	return s.root
}

// blobPath returns the filesystem path for a blob with the given digest.
func (s *OCILayout) blobPath(dgst string) string {
	// Accept both "sha256:hex" and bare "hex" formats.
	hex := strings.TrimPrefix(dgst, "sha256:")
	return filepath.Join(s.root, "blobs", "sha256", hex)
}

// ComputeSHA256 returns "sha256:<hex>" for the given data.
func ComputeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// sanitizePath replaces slashes and other problematic chars in repository
// names for use in filesystem paths.
func sanitizePath(repo string) string {
	return strings.ReplaceAll(repo, "/", string(filepath.Separator))
}

// sanitizeTag ensures a tag is safe for use as a filename.
func sanitizeTag(tag string) string {
	// Replace characters not safe for filenames.
	r := strings.NewReplacer(":", "_", "/", "_")
	return r.Replace(tag)
}
