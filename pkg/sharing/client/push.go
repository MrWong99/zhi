package client

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/shibumi/go-pathspec"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
)

// PushOptions controls how an artifact is pushed.
type PushOptions struct {
	// Tag overrides the default OCI tag (default: "v{version}").
	Tag string
	// AdditionalTags lists extra OCI tags to apply to the same artifact
	// (e.g. "latest"). They are pushed alongside Tag and reference the
	// identical manifest digest.
	AdditionalTags []string
}

// PushResult describes the outcome of a push operation.
type PushResult struct {
	// Reference is the full OCI reference that was pushed.
	Reference string
	// Digest is the OCI manifest digest.
	Digest string
	// Tag is the OCI tag that was applied.
	Tag string
}

// PushPlugin builds and pushes an OCI artifact for a plugin. The binaries map
// contains platform strings (e.g. "linux/amd64") mapped to local binary paths.
// If multiple platforms are provided, an OCI Image Index is created.
func (c *Client) PushPlugin(ctx context.Context, m *manifest.PluginManifest, binaries map[string]string, registry string, opts PushOptions) (*PushResult, error) {
	if len(binaries) == 0 {
		return nil, fmt.Errorf("no binaries provided")
	}

	tag := opts.Tag
	if tag == "" {
		tag = "v" + m.Version
	}

	// Serialise the plugin manifest as the OCI config blob.
	configData, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshalling plugin config: %w", err)
	}

	repo := registry + "/" + m.BinaryName()
	parsed, err := ParseReference(repo + ":" + tag)
	if err != nil {
		return nil, fmt.Errorf("parsing target reference: %w", err)
	}

	core.Logger().Debug("repository config", "host", parsed.Host, "repository", parsed.Repository, "tag", parsed.Tag)
	remote, err := c.newRepository(parsed)
	if err != nil {
		return nil, fmt.Errorf("connecting to registry: %w", err)
	}

	if len(binaries) == 1 {
		// Single-platform push: create a single manifest.
		var platform string
		var binaryPath string
		for p, bp := range binaries {
			platform = p
			binaryPath = bp
		}

		desc, err := c.pushSinglePlatformPlugin(ctx, remote, configData, binaryPath, platform, tag, opts.AdditionalTags)
		if err != nil {
			return nil, err
		}

		return &PushResult{
			Reference: parsed.Reference(),
			Digest:    string(desc.Digest),
			Tag:       tag,
		}, nil
	}

	// Multi-platform push: create per-platform manifests and an index.
	desc, err := c.pushMultiPlatformPlugin(ctx, remote, configData, binaries, tag, opts.AdditionalTags)
	if err != nil {
		return nil, err
	}

	return &PushResult{
		Reference: parsed.Reference(),
		Digest:    string(desc.Digest),
		Tag:       tag,
	}, nil
}

// pushSinglePlatformPlugin creates and pushes a single OCI manifest with the
// plugin binary as a layer.
func (c *Client) pushSinglePlatformPlugin(ctx context.Context, target oras.Target, configData []byte, binaryPath, platform, tag string, additionalTags []string) (ocispec.Descriptor, error) {
	store := memory.New()

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("reading binary %s: %w", binaryPath, err)
	}

	// Push config blob.
	configDesc, err := pushBlob(ctx, store, MediaTypePluginConfig, configData)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing config blob: %w", err)
	}

	// Push binary layer.
	binaryDesc, err := pushBlob(ctx, store, MediaTypePluginBinary, binaryData)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing binary blob: %w", err)
	}

	// Build and push the OCI manifest.
	ociManifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{binaryDesc},
	}

	manifestData, err := json.Marshal(ociManifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshalling manifest: %w", err)
	}

	manifestDesc, err := pushBlob(ctx, store, ocispec.MediaTypeImageManifest, manifestData)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing manifest: %w", err)
	}

	// Tag the manifest.
	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("tagging manifest: %w", err)
	}

	// Copy from memory store to remote. Wrap target to handle registries
	// that return 403 on Exists for new packages (e.g. GHCR).
	wrapped := &lenientExistsTarget{target}
	desc, err := oras.Copy(ctx, store, tag, wrapped, tag, oras.DefaultCopyOptions)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing to registry: %w", err)
	}
	if err := pushAdditionalTags(ctx, store, wrapped, manifestDesc, additionalTags); err != nil {
		return ocispec.Descriptor{}, err
	}
	_ = platform // platform is informational for single-platform
	return desc, nil
}

// pushMultiPlatformPlugin creates per-platform manifests and wraps them in an
// OCI Image Index.
func (c *Client) pushMultiPlatformPlugin(ctx context.Context, target oras.Target, configData []byte, binaries map[string]string, tag string, additionalTags []string) (ocispec.Descriptor, error) {
	store := memory.New()

	// Push config blob once — it is shared across all platform manifests.
	configDesc, err := pushBlob(ctx, store, MediaTypePluginConfig, configData)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing config: %w", err)
	}

	var platformManifests []ocispec.Descriptor

	for platform, binaryPath := range binaries {
		osName, arch, err := parsePlatform(platform)
		if err != nil {
			return ocispec.Descriptor{}, err
		}

		binaryData, err := os.ReadFile(binaryPath)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("reading binary for %s: %w", platform, err)
		}

		// Push binary layer.
		binaryDesc, err := pushBlob(ctx, store, MediaTypePluginBinary, binaryData)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("pushing binary for %s: %w", platform, err)
		}

		// Build the platform-specific manifest.
		m := ocispec.Manifest{
			Versioned: specs.Versioned{SchemaVersion: 2},
			MediaType: ocispec.MediaTypeImageManifest,
			Config:    configDesc,
			Layers:    []ocispec.Descriptor{binaryDesc},
		}

		mData, err := json.Marshal(m)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("marshalling manifest for %s: %w", platform, err)
		}

		mDesc, err := pushBlob(ctx, store, ocispec.MediaTypeImageManifest, mData)
		if err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("pushing manifest for %s: %w", platform, err)
		}

		mDesc.Platform = &ocispec.Platform{
			OS:           osName,
			Architecture: arch,
		}
		platformManifests = append(platformManifests, mDesc)
	}

	// Build OCI Image Index.
	index := ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: platformManifests,
	}

	indexData, err := json.Marshal(index)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshalling index: %w", err)
	}

	indexDesc, err := pushBlob(ctx, store, ocispec.MediaTypeImageIndex, indexData)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing index: %w", err)
	}

	if err := store.Tag(ctx, indexDesc, tag); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("tagging index: %w", err)
	}

	wrapped := &lenientExistsTarget{target}
	desc, err := oras.Copy(ctx, store, tag, wrapped, tag, oras.DefaultCopyOptions)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("pushing to registry: %w", err)
	}
	if err := pushAdditionalTags(ctx, store, wrapped, indexDesc, additionalTags); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// PushWorkspace bundles a workspace directory and pushes it as an OCI artifact.
func (c *Client) PushWorkspace(ctx context.Context, m *manifest.WorkspaceManifest, dir, registryHost string, opts PushOptions) (*PushResult, error) {
	tag := opts.Tag
	if tag == "" {
		tag = "v" + m.Version
	}

	// Serialise the workspace manifest as the OCI config blob.
	configData, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshalling workspace config: %w", err)
	}

	// Create the workspace bundle tarball.
	bundleData, err := createWorkspaceBundle(dir)
	if err != nil {
		return nil, fmt.Errorf("creating workspace bundle: %w", err)
	}

	repoName := registryHost + "/zhi-workspace-" + m.Name
	parsed, err := ParseReference(repoName + ":" + tag)
	if err != nil {
		return nil, fmt.Errorf("parsing target reference: %w", err)
	}

	core.Logger().Debug("repository config", "host", parsed.Host, "repository", parsed.Repository, "tag", parsed.Tag)
	remote, err := c.newRepository(parsed)
	if err != nil {
		return nil, fmt.Errorf("connecting to registry: %w", err)
	}

	store := memory.New()

	// Push config blob.
	configDesc, err := pushBlob(ctx, store, MediaTypeWorkspaceConfig, configData)
	if err != nil {
		return nil, fmt.Errorf("pushing workspace config: %w", err)
	}

	// Push bundle layer.
	bundleDesc, err := pushBlob(ctx, store, MediaTypeWorkspaceBundle, bundleData)
	if err != nil {
		return nil, fmt.Errorf("pushing workspace bundle: %w", err)
	}

	// Build OCI manifest.
	ociManifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{bundleDesc},
	}

	manifestData, err := json.Marshal(ociManifest)
	if err != nil {
		return nil, fmt.Errorf("marshalling manifest: %w", err)
	}

	manifestDesc, err := pushBlob(ctx, store, ocispec.MediaTypeImageManifest, manifestData)
	if err != nil {
		return nil, fmt.Errorf("pushing manifest: %w", err)
	}

	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		return nil, fmt.Errorf("tagging manifest: %w", err)
	}

	wrapped := &lenientExistsTarget{remote}
	desc, err := oras.Copy(ctx, store, tag, wrapped, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("pushing to registry: %w", err)
	}
	if err := pushAdditionalTags(ctx, store, wrapped, manifestDesc, opts.AdditionalTags); err != nil {
		return nil, err
	}

	return &PushResult{
		Reference: parsed.Reference(),
		Digest:    string(desc.Digest),
		Tag:       tag,
	}, nil
}

// pushAdditionalTags re-tags an already-pushed artifact with extra tags. Each
// tag points to the same manifest digest as the primary push.
func pushAdditionalTags(ctx context.Context, store *memory.Store, target oras.Target, desc ocispec.Descriptor, tags []string) error {
	for _, t := range tags {
		if t == "" {
			continue
		}
		if err := store.Tag(ctx, desc, t); err != nil {
			return fmt.Errorf("tagging %q in memory store: %w", t, err)
		}
		if _, err := oras.Copy(ctx, store, t, target, t, oras.DefaultCopyOptions); err != nil {
			return fmt.Errorf("pushing additional tag %q: %w", t, err)
		}
	}
	return nil
}

// PullWorkspace downloads a workspace artifact, extracts it, and installs
// plugin dependencies.
func (c *Client) PullWorkspace(ctx context.Context, ref, targetDir string, opts PullOptions) (*manifest.WorkspaceManifest, error) {
	parsed, err := ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("parsing reference: %w", err)
	}

	core.Logger().Debug("repository config", "host", parsed.Host, "repository", parsed.Repository, "tag", parsed.Tag)
	repo, err := c.newRepository(parsed)
	if err != nil {
		return nil, fmt.Errorf("connecting to registry: %w", err)
	}

	pullRef := parsed.Tag
	if parsed.Digest != "" {
		pullRef = parsed.Digest
	}
	if pullRef == "" {
		pullRef = "latest"
	}

	memStore := memory.New()
	desc, err := oras.Copy(ctx, repo, pullRef, memStore, pullRef, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("pulling workspace artifact: %w", err)
	}

	// Read the manifest.
	manifestData, err := readBlob(ctx, memStore, desc)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var ociManifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
		return nil, fmt.Errorf("parsing OCI manifest: %w", err)
	}

	// Parse config blob as workspace manifest.
	configData, err := readBlob(ctx, memStore, ociManifest.Config)
	if err != nil {
		return nil, fmt.Errorf("reading config blob: %w", err)
	}

	var wm manifest.WorkspaceManifest
	if err := json.Unmarshal(configData, &wm); err != nil {
		return nil, fmt.Errorf("parsing workspace config: %w", err)
	}

	// Extract workspace bundle.
	for _, layer := range ociManifest.Layers {
		if layer.MediaType == MediaTypeWorkspaceBundle {
			bundleData, err := readBlob(ctx, memStore, layer)
			if err != nil {
				return nil, fmt.Errorf("reading workspace bundle: %w", err)
			}
			if err := extractTarGz(bundleData, targetDir); err != nil {
				return nil, fmt.Errorf("extracting workspace bundle: %w", err)
			}
			break
		}
	}

	// Install plugin dependencies unless skipped.
	if !opts.SkipDependencies {
		for _, dep := range wm.Dependencies {
			if dep.Ref == "" {
				continue
			}
			_, err := c.PullPlugin(ctx, dep.Ref, PullOptions{Force: opts.Force})
			if err != nil {
				if dep.Optional {
					continue
				}
				return nil, fmt.Errorf("installing dependency %s: %w", dep.Ref, err)
			}
		}
	}

	return &wm, nil
}

// pushBlob pushes a blob to the given target and returns its descriptor.
func pushBlob(ctx context.Context, store *memory.Store, mediaType string, data []byte) (ocispec.Descriptor, error) {
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digestOf(data),
		Size:      int64(len(data)),
	}
	if err := store.Push(ctx, desc, bytes.NewReader(data)); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// digestOf returns the sha256 digest of the data.
func digestOf(data []byte) digest.Digest {
	return digest.FromBytes(data)
}

// parsePlatform splits a "os/arch" string.
func parsePlatform(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid platform %q: expected os/arch format", s)
	}
	return parts[0], parts[1], nil
}

// createWorkspaceBundle creates a tar.gz archive of workspace files in dir.
// It respects a .gitignore file in the workspace root (if present) and always
// skips hidden directories (e.g. .git, .zhi).
func createWorkspaceBundle(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	patterns := loadGitignorePatterns(dir)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Skip hidden directories (e.g. .git, .zhi).
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && rel != "." {
			return filepath.SkipDir
		}

		// Skip the root directory entry and the .gitignore itself.
		if rel == "." || rel == ".gitignore" {
			return nil
		}

		// Skip files matched by .gitignore patterns.
		if len(patterns) > 0 {
			ignored, _ := pathspec.GitIgnore(patterns, rel)
			if ignored {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// loadGitignorePatterns reads a .gitignore file from the given directory and
// returns the non-empty, non-comment lines. Returns nil if no .gitignore exists.
func loadGitignorePatterns(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// Decompression limits guard against gzip/zip bombs. They are package
// variables (rather than constants) only so tests can lower them; the
// production values are a generous ceiling for real workspace bundles.
var (
	// maxExtractedBytes caps the total decompressed size of a bundle.
	maxExtractedBytes int64 = 1 << 30
	// maxExtractedEntries caps the number of archive entries to guard
	// against archives with an enormous number of tiny files.
	maxExtractedEntries = 50_000
)

// extractTarGz extracts a tar.gz archive to the target directory.
func extractTarGz(data []byte, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	// Resolve to absolute path so the traversal check works with relative
	// target directories (e.g. ".").
	targetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolving target directory: %w", err)
	}

	gr, err := gzip.NewReader(bytes.NewReader(data))
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

		target := filepath.Join(targetDir, header.Name)

		// Defense-in-depth: verify the joined path is still inside targetDir.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(targetDir)+string(os.PathSeparator)) &&
			filepath.Clean(target) != filepath.Clean(targetDir) {
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
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			// Cap decompression to guard against gzip bombs. LimitReader is
			// given one extra byte so an over-budget entry is detectable.
			remaining := maxExtractedBytes - extractedSize
			n, err := io.Copy(f, io.LimitReader(tr, remaining+1))
			f.Close()
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
