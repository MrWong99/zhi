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
		data, err := os.ReadFile(filepath.Join(blobsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading blob %s: %w", entry.Name(), err)
		}
		if _, err := store.PutBlob(data); err != nil {
			return nil, fmt.Errorf("storing blob %s: %w", entry.Name(), err)
		}
	}

	// Create tag mappings for each artifact.
	for _, art := range manifest.Artifacts {
		repo, tag := parseRefParts(art.Ref)
		if repo != "" && tag != "" && art.Digest != "" {
			if err := store.PutTag(repo, tag, art.Digest); err != nil {
				return nil, fmt.Errorf("storing tag for %s: %w", art.Ref, err)
			}
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
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		target := filepath.Join(dst, header.Name)

		// Prevent path traversal.
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
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}
