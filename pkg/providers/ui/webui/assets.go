package webui

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// assetManifest maps original static asset paths to content-hashed paths
// (e.g. "css/tokens.css" → "css/tokens.a1b2c3d4.css"). Hash-versioned
// paths allow aggressive immutable caching.
type assetManifest struct {
	// hashed maps original path → hashed path (both relative to static/).
	hashed map[string]string
}

// newAssetManifest walks the embedded static FS and computes an 8-char
// SHA-256 prefix hash for each file.
func newAssetManifest() *assetManifest {
	m := &assetManifest{hashed: make(map[string]string)}
	_ = fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(staticFS, p)
		if err != nil {
			return nil
		}
		// p is "static/css/tokens.css", we want "css/tokens.css"
		rel := strings.TrimPrefix(p, "static/")
		hash := fmt.Sprintf("%x", sha256.Sum256(data))[:8]
		ext := path.Ext(rel)
		base := strings.TrimSuffix(rel, ext)
		m.hashed[rel] = base + "." + hash + ext
		return nil
	})
	return m
}

// hashedPath returns the hashed path for an original asset path.
// If not found, returns the original path unchanged.
func (m *assetManifest) hashedPath(original string) string {
	if h, ok := m.hashed[original]; ok {
		return "/static/" + h
	}
	return "/static/" + original
}

// isHashedPath checks whether a URL path (relative to /static/) contains
// a content hash and returns the original path with the hash stripped.
// For example "css/tokens.a1b2c3d4.css" → "css/tokens.css", true.
func isHashedPath(urlPath string) (string, bool) {
	ext := path.Ext(urlPath)
	if ext == "" {
		return urlPath, false
	}
	base := strings.TrimSuffix(urlPath, ext)
	// Look for the pattern "name.HASH" where HASH is exactly 8 hex chars.
	lastDot := strings.LastIndex(base, ".")
	if lastDot < 0 || lastDot == len(base)-1 {
		return urlPath, false
	}
	hash := base[lastDot+1:]
	if len(hash) != 8 || !isHex(hash) {
		return urlPath, false
	}
	return base[:lastDot] + ext, true
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// assetPathFunc is the template function exposed as "assetPath".
// It returns the content-hashed URL for a static asset.
func assetPathFunc(original string) string {
	return globalAssetManifest.hashedPath(original)
}

// globalAssetManifest is initialized at package init time from embedded
// static files.
var globalAssetManifest = newAssetManifest()
