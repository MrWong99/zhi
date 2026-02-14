// Package airgap provides export and import functionality for air-gapped
// environments. Artifacts can be exported to a tarball on an internet-connected
// machine and imported into a mirror on an air-gapped network.
package airgap

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportOptions controls artifact export.
type ExportOptions struct {
	// Artifacts is a list of OCI references to export
	// (e.g. "zhi-project/zhi-config-ansible:v1.2.0").
	Artifacts []string
	// AllPlatforms exports all platform variants.
	AllPlatforms bool
	// Output is the path to the output tarball.
	Output string
	// UpstreamRegistry is the upstream OCI registry host (e.g. "ghcr.io").
	UpstreamRegistry string
	// HTTPClient is an optional HTTP client for upstream connections.
	// When nil, a default client with a 5-minute timeout is used.
	HTTPClient *http.Client
}

// BundleManifest describes the contents of an export bundle.
type BundleManifest struct {
	Version   string        `json:"version"`
	CreatedAt string        `json:"createdAt"`
	Artifacts []BundleEntry `json:"artifacts"`
}

// BundleEntry describes a single artifact in the bundle.
type BundleEntry struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

// Export downloads the specified artifacts and packages them into a tarball
// for transfer to an air-gapped environment.
func Export(opts ExportOptions) error {
	if len(opts.Artifacts) == 0 {
		return fmt.Errorf("no artifacts specified")
	}
	if opts.Output == "" {
		return fmt.Errorf("output path is required")
	}
	if opts.UpstreamRegistry == "" {
		opts.UpstreamRegistry = "ghcr.io"
	}

	// Create a temporary directory for staging.
	tmpDir, err := os.MkdirTemp("", "zhi-export-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	blobsDir := filepath.Join(tmpDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		return fmt.Errorf("creating blobs directory: %w", err)
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	var entries []BundleEntry

	for _, ref := range opts.Artifacts {
		entry, err := exportArtifact(client, opts.UpstreamRegistry, ref, blobsDir)
		if err != nil {
			return fmt.Errorf("exporting %s: %w", ref, err)
		}
		entries = append(entries, *entry)
	}

	// Write bundle manifest.
	manifest := BundleManifest{
		Version:   "1",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Artifacts: entries,
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling bundle manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "bundle.json"), manifestData, 0o644); err != nil {
		return fmt.Errorf("writing bundle manifest: %w", err)
	}

	// Write OCI layout marker.
	if err := os.WriteFile(filepath.Join(tmpDir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644); err != nil {
		return fmt.Errorf("writing oci-layout: %w", err)
	}

	// Package as tarball.
	if err := createTarball(opts.Output, tmpDir); err != nil {
		return fmt.Errorf("creating tarball: %w", err)
	}

	return nil
}

// exportArtifact fetches a single artifact from upstream and stores it in
// the staging directory.
func exportArtifact(client *http.Client, registry, ref, blobsDir string) (*BundleEntry, error) {
	// Parse ref: "publisher/name:tag" or "publisher/name@sha256:..."
	repo := ref
	tag := "latest"
	if idx := strings.LastIndex(ref, ":"); idx >= 0 && !strings.Contains(ref[idx:], "@") {
		repo = ref[:idx]
		tag = ref[idx+1:]
	}

	// Fetch manifest.
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repo, tag)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned %d for %s", resp.StatusCode, ref)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	dgst := resp.Header.Get("Docker-Content-Digest")
	if dgst == "" {
		dgst = digestSHA256(data)
	}

	// Store the manifest blob.
	hexStr := strings.TrimPrefix(dgst, "sha256:")
	if err := os.WriteFile(filepath.Join(blobsDir, hexStr), data, 0o644); err != nil {
		return nil, fmt.Errorf("writing manifest blob: %w", err)
	}

	return &BundleEntry{
		Ref:    ref,
		Digest: dgst,
	}, nil
}

// createTarball creates a gzipped tarball of the source directory.
func createTarball(output, srcDir string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}

	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
}

// digestSHA256 computes a "sha256:<hex>" digest for the given data.
func digestSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}
