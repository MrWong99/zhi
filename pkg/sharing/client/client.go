package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/registry"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
)

// Platform represents an OS/architecture combination.
type Platform struct {
	OS   string
	Arch string
}

// String returns the platform in "os/arch" format.
func (p Platform) String() string {
	return p.OS + "/" + p.Arch
}

// CurrentPlatform returns the runtime OS and architecture.
func CurrentPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

// PullOptions controls how a plugin is pulled and installed.
type PullOptions struct {
	// Platform overrides automatic platform detection.
	Platform *Platform
	// SkipVerify skips signature verification (not recommended).
	SkipVerify bool
	// Force overwrites an existing plugin even if the same version is installed.
	Force bool
	// SkipDependencies skips installing plugin dependencies when pulling a workspace.
	SkipDependencies bool
}

// PullResult describes the outcome of a pull operation.
type PullResult struct {
	// Manifest is the plugin metadata from the OCI config blob.
	Manifest *manifest.PluginManifest
	// BinaryPath is the absolute path where the binary was installed.
	BinaryPath string
	// Digest is the OCI manifest digest.
	Digest string
	// Platform is the resolved platform.
	Platform string
	// Verification holds the signature verification result.
	Verification *verify.Result
}

// Client manages OCI artifact operations for zhi plugins.
type Client struct {
	// pluginDir is where plugin binaries are installed.
	pluginDir string
	// cacheDir is the local OCI blob cache directory.
	cacheDir string
	// registryStore provides registry credentials.
	registryStore *registry.Store
	// metadataStore tracks installed plugins.
	metadataStore *metadata.Store
	// httpClient is an optional HTTP client used for OCI registry
	// connections (e.g. with client TLS configured).
	httpClient *http.Client
	// verifier enforces the local security policy (blocked plugins,
	// allowed registries, signature requirements). When nil, no policy
	// enforcement is performed.
	verifier *verify.Verifier
}

// ClientOption configures a [Client].
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client for OCI registry connections.
// This is used to configure client TLS (mTLS / custom CA).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithVerifier sets the security-policy verifier. When set, every pull
// (including workspace dependency auto-installs) is checked against the
// policy before any content is downloaded, so that blocked plugins,
// disallowed registries, and signature requirements are enforced
// consistently across install, update, and workspace flows.
func WithVerifier(v *verify.Verifier) ClientOption {
	return func(c *Client) {
		c.verifier = v
	}
}

// NewClient creates a new OCI client with the given directories and stores.
func NewClient(pluginDir, cacheDir string, regStore *registry.Store, metaStore *metadata.Store, opts ...ClientOption) *Client {
	c := &Client{
		pluginDir:     pluginDir,
		cacheDir:      cacheDir,
		registryStore: regStore,
		metadataStore: metaStore,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// PullPlugin downloads and installs a plugin from an OCI reference.
func (c *Client) PullPlugin(ctx context.Context, ref string, opts PullOptions) (*PullResult, error) {
	parsed, err := ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("parsing reference: %w", err)
	}

	// Enforce the security policy at the pull chokepoint so that every
	// path — direct install, update, workspace setup, and workspace
	// dependency auto-install — is subject to the same checks (blocked
	// plugins, allowed registries, signature requirements).
	if c.verifier != nil {
		if vResult := c.verifier.VerifyArtifact(ref, opts.SkipVerify); !vResult.OK() {
			return nil, fmt.Errorf("verification failed: %w", vResult.Error)
		}
	}

	// Set up remote repository.
	core.Logger().Debug("repository config", "host", parsed.Host, "repository", parsed.Repository, "tag", parsed.Tag)
	repo, err := c.newRepository(parsed)
	if err != nil {
		return nil, fmt.Errorf("connecting to registry: %w", err)
	}

	// Determine tag or digest to pull.
	pullRef := parsed.Tag
	if parsed.Digest != "" {
		pullRef = parsed.Digest
	}
	if pullRef == "" {
		pullRef = "latest"
	}

	// Resolve the artifact root first so we can record the true root
	// (index) digest independently of the platform pruning below.
	rootDesc, err := repo.Resolve(ctx, pullRef)
	if err != nil {
		return nil, fmt.Errorf("resolving reference: %w", err)
	}

	// Pull into an in-memory store, copying only the subgraph for the
	// target platform. For a multi-platform index this avoids downloading
	// the manifests and binary layers for every other platform (which the
	// old code fetched in full and then discarded all but one of).
	memStore := memory.New()
	copyOpts := oras.DefaultCopyOptions
	copyOpts.MapRoot = func(ctx context.Context, src content.ReadOnlyStorage, root ocispec.Descriptor) (ocispec.Descriptor, error) {
		return c.resolvePlatform(ctx, src, root, opts.Platform)
	}
	manifestDesc, err := oras.Copy(ctx, repo, pullRef, memStore, pullRef, copyOpts)
	if err != nil {
		return nil, fmt.Errorf("pulling artifact: %w", err)
	}

	// desc identifies the artifact for metadata/lockfile purposes: the
	// index digest for a multi-platform artifact, or the manifest digest
	// for a single-platform one.
	desc := rootDesc

	// Fetch and parse the manifest.
	manifestData, err := readBlob(ctx, memStore, manifestDesc)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var ociManifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
		return nil, fmt.Errorf("parsing OCI manifest: %w", err)
	}

	// Parse the config blob for plugin metadata.
	configData, err := readBlob(ctx, memStore, ociManifest.Config)
	if err != nil {
		return nil, fmt.Errorf("reading config blob: %w", err)
	}

	pluginManifest, err := parseConfigBlob(configData)
	if err != nil {
		return nil, fmt.Errorf("parsing plugin config: %w", err)
	}

	// Check if already installed at same version.
	if !opts.Force {
		existing, _ := c.metadataStore.Load(pluginManifest.Name)
		if existing != nil && existing.Version == pluginManifest.Version {
			return &PullResult{
				Manifest:   pluginManifest,
				BinaryPath: filepath.Join(c.pluginDir, pluginManifest.BinaryName()),
				Digest:     string(desc.Digest),
				Platform:   c.effectivePlatform(opts.Platform).String(),
			}, nil
		}
	}

	// Find and extract the binary layer.
	binaryData, err := c.extractBinaryLayer(ctx, memStore, ociManifest.Layers)
	if err != nil {
		return nil, fmt.Errorf("extracting binary: %w", err)
	}

	// Install binary.
	binaryPath := filepath.Join(c.pluginDir, pluginManifest.BinaryName())
	if err := c.installBinary(binaryPath, binaryData); err != nil {
		return nil, fmt.Errorf("installing binary: %w", err)
	}

	// Compute the launch-time integrity baseline from the verified
	// in-memory bytes rather than by re-reading the file from disk. This
	// closes a TOCTOU window where a concurrent write between install and
	// hashing could enshrine tampered content as the trusted baseline.
	binaryDigest := verify.ComputeBinaryDigestBytes(binaryData)

	// Save metadata including binary digest for launch-time integrity checks.
	platform := c.effectivePlatform(opts.Platform)
	meta := &metadata.InstalledPlugin{
		Name:         pluginManifest.Name,
		Type:         pluginManifest.Type,
		Version:      pluginManifest.Version,
		Ref:          ref,
		Digest:       string(desc.Digest),
		Platform:     platform.String(),
		Publisher:    pluginManifest.Author,
		BinaryDigest: binaryDigest,
	}
	if err := c.metadataStore.Save(meta); err != nil {
		return nil, fmt.Errorf("saving metadata: %w", err)
	}

	return &PullResult{
		Manifest:   pluginManifest,
		BinaryPath: binaryPath,
		Digest:     string(desc.Digest),
		Platform:   platform.String(),
	}, nil
}

// newRepository creates an authenticated remote repository.
func (c *Client) newRepository(parsed *ParsedRef) (*remote.Repository, error) {
	repo, err := remote.NewRepository(parsed.Reference())
	if err != nil {
		return nil, err
	}

	authClient := &auth.Client{}
	if c.registryStore != nil {
		username, password, ok := c.registryStore.GetCredentials(parsed.Host)
		if ok {
			authClient.Credential = auth.StaticCredential(parsed.Host, auth.Credential{
				Username: username,
				Password: password,
			})
		}
	}
	if c.httpClient != nil {
		authClient.Client = c.httpClient
	}
	if authClient.Credential != nil || authClient.Client != nil {
		repo.Client = authClient
	}

	return repo, nil
}

// resolvePlatform selects the platform-specific manifest from an OCI Image
// Index, or returns the descriptor as-is if it's already a single manifest.
func (c *Client) resolvePlatform(ctx context.Context, store content.ReadOnlyStorage, desc ocispec.Descriptor, platform *Platform) (ocispec.Descriptor, error) {
	// If it's already a manifest, return it directly.
	if desc.MediaType == ocispec.MediaTypeImageManifest {
		return desc, nil
	}

	// For an index, select the matching platform.
	if desc.MediaType != ocispec.MediaTypeImageIndex {
		return desc, nil
	}

	data, err := readBlob(ctx, store, desc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("reading index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("parsing index: %w", err)
	}

	target := c.effectivePlatform(platform)

	for _, m := range index.Manifests {
		if m.Platform != nil && m.Platform.OS == target.OS && m.Platform.Architecture == target.Arch {
			// Fetch the actual manifest data so we can work with it.
			return m, nil
		}
	}

	return ocispec.Descriptor{}, fmt.Errorf("no manifest found for platform %s", target)
}

// effectivePlatform returns the override platform if set, else current platform.
func (c *Client) effectivePlatform(override *Platform) Platform {
	if override != nil {
		return *override
	}
	return CurrentPlatform()
}

// extractBinaryLayer finds and reads the plugin binary layer from OCI layers.
func (c *Client) extractBinaryLayer(ctx context.Context, store content.Fetcher, layers []ocispec.Descriptor) ([]byte, error) {
	for _, layer := range layers {
		if layer.MediaType == MediaTypePluginBinary {
			return readBlob(ctx, store, layer)
		}
	}
	return nil, fmt.Errorf("no binary layer found (expected media type %s)", MediaTypePluginBinary)
}

// installBinary writes the plugin binary to disk with executable
// permissions. It writes to a temporary file in the same directory and
// renames it over the final path, so the install is atomic: a crash
// mid-write never leaves a truncated executable at the final path, and
// replacing a currently-running plugin binary avoids ETXTBSY on Linux.
func (c *Client) installBinary(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temporary binary: %w", err)
	}
	tmpName := tmp.Name()
	// Ensure the temp file is removed on any failure before the rename.
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting binary permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing binary: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("finalizing binary: %w", err)
	}
	tmpName = "" // rename succeeded; nothing to clean up.
	return nil
}

// parseConfigBlob parses an OCI config blob as plugin manifest JSON.
func parseConfigBlob(data []byte) (*manifest.PluginManifest, error) {
	var m manifest.PluginManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// readBlob fetches the content of a descriptor from a store.
func readBlob(ctx context.Context, store content.Fetcher, desc ocispec.Descriptor) ([]byte, error) {
	rc, err := store.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// lenientExistsTarget wraps an oras.Target so that Exists treats HTTP 403
// responses as "not found". Some registries (notably GHCR) return 403 Forbidden
// on HEAD requests for repositories that do not yet exist, instead of the
// expected 404. Without this wrapper, oras.Copy fails on the Exists pre-check
// when pushing to a brand-new package.
type lenientExistsTarget struct {
	oras.Target
}

func (t *lenientExistsTarget) Exists(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	exists, err := t.Target.Exists(ctx, desc)
	if err != nil {
		var errResp *errcode.ErrorResponse
		if errors.As(err, &errResp) && errResp.StatusCode == http.StatusForbidden {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}
