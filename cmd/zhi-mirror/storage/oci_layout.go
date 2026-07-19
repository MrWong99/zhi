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
	path, err := s.blobPath(dgst)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// GetBlob reads a blob by digest. Returns the content and true if found,
// or nil and false if not found. A malformed digest is treated as not found so
// a client-supplied value can never be joined into a filesystem path.
func (s *OCILayout) GetBlob(dgst string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := s.blobPath(dgst)
	if err != nil {
		return nil, false, nil
	}
	data, err := os.ReadFile(path)
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

	path, err := s.blobPath(dgst)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return dgst, nil // already exists
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
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
	path, err := s.blobPath(dgst)
	if err != nil {
		return nil // malformed digest: nothing to delete
	}
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
	if err := writeFileAtomic(tagFile, data, 0o644); err != nil {
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
// Accepts both "sha256:hex" and bare "hex" formats, but rejects any value whose
// hex component is not exactly 64 lowercase hex characters so a client-supplied
// digest can never contain ".." or other path separators that escape the
// content-addressed blob directory.
func (s *OCILayout) blobPath(dgst string) (string, error) {
	hexStr := strings.TrimPrefix(dgst, "sha256:")
	if !validHex64(hexStr) {
		return "", fmt.Errorf("invalid digest %q", dgst)
	}
	return filepath.Join(s.root, "blobs", "sha256", hexStr), nil
}

// ValidDigest reports whether dgst is a canonical OCI digest of the form
// "sha256:<64 lowercase hex chars>".
func ValidDigest(dgst string) bool {
	hexStr, ok := strings.CutPrefix(dgst, "sha256:")
	return ok && validHex64(hexStr)
}

// validHex64 reports whether s is exactly 64 lowercase hexadecimal characters.
func validHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// writeFileAtomic writes data to a temporary file in the same directory as
// path, fsyncs it, and atomically renames it into place. This ensures a crash
// or disk-full condition mid-write can never leave a truncated file at a
// content-addressed path (where its wrong contents would be permanent).
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// Best-effort fsync of the directory so the rename itself is durable.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
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
