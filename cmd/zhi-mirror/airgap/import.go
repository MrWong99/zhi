package airgap

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MrWong99/zhi/cmd/zhi-mirror/storage"
)

// ImportOptions controls artifact import.
type ImportOptions struct {
	// Input is the path to the bundle tarball.
	Input string
}

// Import seeds a mirror's local storage from an exported bundle.
func Import(store *storage.OCILayout, opts ImportOptions) (*BundleManifest, error) {
	if opts.Input == "" {
		return nil, fmt.Errorf("input path is required")
	}

	// Extract tarball to a temporary directory.
	tmpDir, err := os.MkdirTemp("", "zhi-import-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarball(opts.Input, tmpDir); err != nil {
		return nil, fmt.Errorf("extracting bundle: %w", err)
	}

	// Read bundle manifest.
	manifestData, err := os.ReadFile(filepath.Join(tmpDir, "bundle.json"))
	if err != nil {
		return nil, fmt.Errorf("reading bundle manifest: %w", err)
	}

	var manifest BundleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parsing bundle manifest: %w", err)
	}

	// Import all blobs from the bundle into the store.
	blobsDir := filepath.Join(tmpDir, "blobs", "sha256")
	entries, err := os.ReadDir(blobsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading blobs directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join(blobsDir, name))
		if err != nil {
			return nil, fmt.Errorf("reading blob %s: %w", name, err)
		}
		// The filename is the blob's content digest (bare hex). Verify the bytes
		// hash to it so bit-rot or truncation on the transfer medium is caught
		// loudly here rather than as an opaque pull failure later. PutBlob stores
		// under the recomputed digest, so a mismatch would otherwise silently
		// store the blob under the wrong key.
		expected := "sha256:" + name
		if got := storage.ComputeSHA256(data); got != expected {
			return nil, fmt.Errorf("blob %s failed integrity check: content digest is %s", name, got)
		}
		if _, err := store.PutBlob(data); err != nil {
			return nil, fmt.Errorf("storing blob %s: %w", name, err)
		}
	}

	// Create tag mappings for each artifact. Verify the referenced digest is
	// actually present so an incomplete bundle is rejected atomically rather
	// than surfacing as a missing-blob pull failure with no upstream to recover.
	for _, art := range manifest.Artifacts {
		repo, tag := parseRefParts(art.Ref)
		if repo == "" || tag == "" || art.Digest == "" {
			continue
		}
		if !store.HasBlob(art.Digest) {
			return nil, fmt.Errorf("bundle references digest %s for %s but it is missing from the bundle", art.Digest, art.Ref)
		}
		if err := store.PutTag(repo, tag, art.Digest); err != nil {
			return nil, fmt.Errorf("storing tag for %s: %w", art.Ref, err)
		}
	}

	return &manifest, nil
}

// parseRefParts extracts repository and tag from a reference like
// "zhi-project/zhi-config-ansible:v1.2.0".
func parseRefParts(ref string) (repo, tag string) {
	if idx := strings.LastIndex(ref, ":"); idx >= 0 && !strings.Contains(ref[idx:], "@") {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}

const (
	// maxExtractedBytes caps the total decompressed size of a bundle to
	// guard against gzip/zip bombs.
	maxExtractedBytes int64 = 1 << 30
	// maxExtractedEntries caps the number of archive entries.
	maxExtractedEntries = 50_000
)

// extractTarball extracts a gzipped tarball to the target directory.
func extractTarball(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("opening gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var (
		entries       int
		extractedSize int64
	)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		entries++
		if entries > maxExtractedEntries {
			return fmt.Errorf("archive has too many entries (limit %d)", maxExtractedEntries)
		}

		// Reject archive entries with absolute paths or ".." components before
		// joining, to prevent zip-slip directory traversal attacks.
		for _, part := range strings.Split(filepath.ToSlash(header.Name), "/") {
			if part == ".." {
				return fmt.Errorf("invalid tar entry path: %s", header.Name)
			}
		}
		if filepath.IsAbs(header.Name) {
			return fmt.Errorf("invalid tar entry path (absolute): %s", header.Name)
		}

		target := filepath.Join(dst, header.Name)

		// Defense-in-depth: verify the joined path is still inside dst.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)+string(os.PathSeparator)) &&
			filepath.Clean(target) != filepath.Clean(dst) {
			return fmt.Errorf("invalid tar entry path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			// Cap decompression to guard against gzip bombs. LimitReader is
			// given one extra byte so an over-budget entry is detectable.
			remaining := maxExtractedBytes - extractedSize
			n, err := io.Copy(out, io.LimitReader(tr, remaining+1))
			out.Close()
			if err != nil {
				return err
			}
			extractedSize += n
			if extractedSize > maxExtractedBytes {
				return fmt.Errorf("archive exceeds maximum extracted size (limit %d bytes)", maxExtractedBytes)
			}
		}
	}
	return nil
}
