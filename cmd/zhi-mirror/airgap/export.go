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
	"runtime"
	"strings"
	"time"

	"github.com/MrWong99/zhi/cmd/zhi-mirror/storage"
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
		entry, err := exportArtifact(client, opts.UpstreamRegistry, ref, blobsDir, opts.AllPlatforms)
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

// maxManifestDepth bounds recursion into nested image indexes to guard against
// a malicious or malformed upstream returning a cyclic/deeply-nested tree.
const maxManifestDepth = 10

// ociPlatform is the platform selector of an image-index manifest entry.
type ociPlatform struct {
	OS   string `json:"os"`
	Arch string `json:"architecture"`
}

// ociDescriptor is a content descriptor referencing another blob (a child
// manifest, a config, or a layer) by digest.
type ociDescriptor struct {
	MediaType string       `json:"mediaType"`
	Digest    string       `json:"digest"`
	Size      int64        `json:"size"`
	Platform  *ociPlatform `json:"platform,omitempty"`
}

// ociManifest is a permissive view over both image manifests and image indexes.
// An index populates Manifests; an image manifest populates Config and Layers.
type ociManifest struct {
	MediaType string          `json:"mediaType"`
	Config    ociDescriptor   `json:"config"`
	Layers    []ociDescriptor `json:"layers"`
	Manifests []ociDescriptor `json:"manifests"`
}

// exportArtifact fetches a single artifact from upstream and stores it — along
// with every blob it transitively references — into the staging directory so
// the bundle is self-contained and usable on the air-gapped side.
func exportArtifact(client *http.Client, registry, ref, blobsDir string, allPlatforms bool) (*BundleEntry, error) {
	// Parse ref: "publisher/name:tag" or "publisher/name@sha256:..."
	repo := ref
	tag := "latest"
	if idx := strings.LastIndex(ref, ":"); idx >= 0 && !strings.Contains(ref[idx:], "@") {
		repo = ref[:idx]
		tag = ref[idx+1:]
	}

	seen := make(map[string]bool)
	topDigest, err := exportManifestTree(client, registry, repo, tag, blobsDir, allPlatforms, seen, 0)
	if err != nil {
		return nil, err
	}
	return &BundleEntry{Ref: ref, Digest: topDigest}, nil
}

// exportManifestTree fetches the manifest identified by reference (a tag or a
// digest), writes it into blobsDir, and recursively downloads every blob it
// references: child manifests for an image index, and the config + layer blobs
// for an image manifest. It returns the content digest of the fetched manifest.
func exportManifestTree(client *http.Client, registry, repo, reference, blobsDir string, allPlatforms bool, seen map[string]bool, depth int) (string, error) {
	if depth > maxManifestDepth {
		return "", fmt.Errorf("manifest nesting too deep for %s/%s", repo, reference)
	}

	data, dgst, err := fetchManifest(client, registry, repo, reference)
	if err != nil {
		return "", err
	}
	if err := writeBlob(blobsDir, dgst, data, seen); err != nil {
		return "", err
	}

	var m ociManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("parsing manifest %s/%s: %w", repo, reference, err)
	}

	if len(m.Manifests) > 0 {
		// Image index / manifest list: recurse into the selected child manifests.
		for _, child := range selectPlatforms(m.Manifests, allPlatforms) {
			if !storage.ValidDigest(child.Digest) {
				return "", fmt.Errorf("invalid child manifest digest %q in %s", child.Digest, repo)
			}
			if _, err := exportManifestTree(client, registry, repo, child.Digest, blobsDir, allPlatforms, seen, depth+1); err != nil {
				return "", err
			}
		}
		return dgst, nil
	}

	// Image manifest: download the config descriptor and every layer.
	descs := make([]ociDescriptor, 0, len(m.Layers)+1)
	if m.Config.Digest != "" {
		descs = append(descs, m.Config)
	}
	descs = append(descs, m.Layers...)
	for _, d := range descs {
		if err := fetchAndStoreBlob(client, registry, repo, d.Digest, blobsDir, seen); err != nil {
			return "", err
		}
	}
	return dgst, nil
}

// selectPlatforms returns the index entries to export. With allPlatforms it
// returns every entry; otherwise it filters to the host platform, falling back
// to all entries when no host-specific match exists so the bundle stays usable.
func selectPlatforms(manifests []ociDescriptor, allPlatforms bool) []ociDescriptor {
	if allPlatforms {
		return manifests
	}
	host := runtime.GOOS + "/" + runtime.GOARCH
	var matched []ociDescriptor
	for _, d := range manifests {
		if d.Platform == nil || d.Platform.OS == "unknown" || d.Platform.Arch == "unknown" {
			continue
		}
		if d.Platform.OS+"/"+d.Platform.Arch == host {
			matched = append(matched, d)
		}
	}
	if len(matched) == 0 {
		return manifests
	}
	return matched
}

// fetchManifest downloads a manifest by reference (tag or digest) and returns
// its bytes and verified content digest.
func fetchManifest(client *http.Client, registry, repo, reference string) ([]byte, string, error) {
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repo, reference)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
	}, ", "))

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("upstream returned %d for %s/%s", resp.StatusCode, repo, reference)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading manifest: %w", err)
	}

	dgst, err := resolveDigest(resp.Header.Get("Docker-Content-Digest"), data)
	if err != nil {
		return nil, "", fmt.Errorf("manifest %s/%s: %w", repo, reference, err)
	}
	// When fetched by digest, the content must match the requested digest.
	if storage.ValidDigest(reference) && reference != dgst {
		return nil, "", fmt.Errorf("manifest %s/%s content digest %s does not match requested %s", repo, reference, dgst, reference)
	}
	return data, dgst, nil
}

// fetchAndStoreBlob downloads a non-manifest blob (config or layer) by digest,
// verifies its content against the declared digest, and writes it to blobsDir.
func fetchAndStoreBlob(client *http.Client, registry, repo, dgst, blobsDir string, seen map[string]bool) error {
	if !storage.ValidDigest(dgst) {
		return fmt.Errorf("invalid blob digest %q in %s", dgst, repo)
	}
	if seen[dgst] {
		return nil
	}
	url := fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry, repo, dgst)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetching blob %s: %w", dgst, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned %d for blob %s", resp.StatusCode, dgst)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading blob %s: %w", dgst, err)
	}
	if got := digestSHA256(data); got != dgst {
		return fmt.Errorf("blob %s content digest is %s", dgst, got)
	}
	return writeBlob(blobsDir, dgst, data, seen)
}

// writeBlob writes data to blobsDir keyed by its (already validated) digest,
// using the bare hex as the filename. It deduplicates via seen so shared layers
// are downloaded and written once.
func writeBlob(blobsDir, dgst string, data []byte, seen map[string]bool) error {
	if seen[dgst] {
		return nil
	}
	hexStr := strings.TrimPrefix(dgst, "sha256:")
	if err := os.WriteFile(filepath.Join(blobsDir, hexStr), data, 0o644); err != nil {
		return fmt.Errorf("writing blob %s: %w", dgst, err)
	}
	seen[dgst] = true
	return nil
}

// resolveDigest returns the trustworthy sha256 digest for data. An upstream
// declared digest is honored only when it is well-formed AND matches the
// content: a malformed declaration falls back to the locally computed digest
// (never used to build a path), and a well-formed but mismatching one is
// rejected so the bundle's content-addressing stays trustworthy air-gapped.
func resolveDigest(declared string, data []byte) (string, error) {
	computed := digestSHA256(data)
	if declared == "" || !storage.ValidDigest(declared) {
		return computed, nil
	}
	if declared != computed {
		return "", fmt.Errorf("declared digest %s does not match content digest %s", declared, computed)
	}
	return declared, nil
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

	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
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
	}); err != nil {
		return err
	}

	// Close writers explicitly, innermost first, and propagate errors: tar and
	// gzip Close write the buffered tail of the archive and routinely fail on
	// disk-full/quota — discarding those errors would report success while
	// producing a truncated bundle that only fails later on the air-gapped side.
	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing bundle: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing bundle: %w", err)
	}
	return nil
}

// digestSHA256 computes a "sha256:<hex>" digest for the given data.
func digestSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}
