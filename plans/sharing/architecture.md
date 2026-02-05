# Architecture Overview

## System Components

```
┌──────────────────────────────────────────────────────────────────┐
│                        User's Machine                            │
│                                                                  │
│  ┌─────────┐    ┌──────────────┐    ┌─────────────────────────┐ │
│  │ zhi CLI  │───▶│ Share Client │───▶│ ~/.zhi/plugins/         │ │
│  │          │    │  (oras-go)   │    │   zhi-config-ansible    │ │
│  └─────────┘    └──────┬───────┘    │   zhi-store-vault       │ │
│       │                │            │   ...                    │ │
│  ┌─────────┐           │            └─────────────────────────┘ │
│  │ UI      │           │            ┌─────────────────────────┐ │
│  │ Plugin  │           │            │ ~/.zhi/runtimes/         │ │
│  │ (TUI/   │           │            │   java-21/              │ │
│  │  Web/   │           │            │   python-3.12/           │ │
│  │  API)   │           │            └─────────────────────────┘ │
│  └─────────┘           │            ┌─────────────────────────┐ │
│                        │            │ ~/.zhi/cache/            │ │
│                        │            │   oci/  (blob cache)     │ │
│                        │            │   metadata/              │ │
│                        │            └─────────────────────────┘ │
└────────────────────────┼────────────────────────────────────────┘
                         │
            ┌────────────┴────────────┐
            │                         │
            ▼                         ▼
┌───────────────────┐    ┌──────────────────────┐
│  OCI Registry     │    │  Marketplace API     │
│  (artifact store) │    │  (discovery + meta)  │
│                   │    │                      │
│  - GHCR           │    │  - Search            │
│  - Docker Hub     │    │  - Ratings           │
│  - Harbor         │    │  - Verified badges   │
│  - ECR/ACR/GAR    │    │  - Version index     │
│  - Self-hosted    │    │  - Statistics        │
│  - Local proxy    │    │                      │
└───────────────────┘    └──────────────────────┘
```

## Data Flow

### Installing a Plugin

```
1. User runs: zhi plugin install ansible-config@1.2.0

2. CLI resolves short name to OCI reference:
   → Query marketplace API (or local config) for mapping
   → ansible-config@1.2.0 → oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0

3. Share Client pulls artifact:
   → Fetch OCI Image Index (fat manifest)
   → Select platform-specific manifest (linux/amd64)
   → Download config blob → parse plugin metadata
   → Download binary layer → write to temp file
   → Download runtime layer (if present) → extract to ~/.zhi/runtimes/

4. Verify artifact:
   → Check cosign signature via Sigstore
   → Verify binary checksum matches manifest digest
   → Run existing auditPluginBinary() (SHA-256 hash logging)

5. Install:
   → Move binary to ~/.zhi/plugins/zhi-config-ansible
   → Set executable permissions (0755)
   → Update zhi-plugins.lock with digest

6. Registry refreshes:
   → reg.RefreshExternal() picks up the new binary
   → Plugin available by name in workspace configs
```

### Installing a Workspace

```
1. User runs: zhi workspace install oci://ghcr.io/org/k8s-workspace:v1.0

2. Share Client pulls workspace artifact:
   → Download config blob → parse workspace metadata + dependencies
   → Download workspace bundle layer → extract to temp dir

3. Resolve dependencies:
   → For each plugin dependency in the config:
     → Check if already installed at compatible version
     → If not, pull and install (same flow as plugin install)

4. Check tools:
   → For each external tool requirement (kubectl, helm, etc.):
     → Check if available on PATH at required version
     → Warn user about missing tools with install instructions

5. Install workspace:
   → Copy zhi.yaml and templates to target directory
   → Update zhi-plugins.lock with all plugin digests

6. User can now: cd my-workspace && zhi run
```

### Publishing a Plugin

```
1. Developer builds plugin binary for all platforms:
   → make build-all  (produces linux/amd64, darwin/arm64, etc.)

2. Developer creates plugin manifest:
   → zhi plugin init  (generates zhi-plugin.yaml with metadata)

3. Developer publishes:
   → zhi plugin publish --registry ghcr.io/myorg
   → Share Client creates OCI Image Index with platform manifests
   → Pushes to OCI registry
   → Signs artifact with cosign (if key configured)

4. Optionally register with marketplace:
   → zhi plugin register  (submits to central marketplace API)
   → Or: submit PR to plugin index repository
```

## Core Components (Go packages)

### `pkg/sharing/client` — Share Client

The central library for pulling and pushing OCI artifacts.

```go
package client

// Client manages OCI artifact operations for plugins and workspaces.
type Client struct {
    registries []RegistryConfig  // configured registries with auth
    cache      *BlobCache        // local content-addressed cache
    verifier   *Verifier         // signature verification
    platform   Platform          // current OS/arch
}

// PullPlugin downloads and installs a plugin from an OCI reference.
func (c *Client) PullPlugin(ctx context.Context, ref string, opts PullOptions) (*InstalledPlugin, error)

// PullWorkspace downloads a workspace bundle and resolves plugin dependencies.
func (c *Client) PullWorkspace(ctx context.Context, ref string, targetDir string, opts PullOptions) (*InstalledWorkspace, error)

// PushPlugin builds and pushes an OCI artifact for a plugin.
func (c *Client) PushPlugin(ctx context.Context, manifest PluginManifest, binaries map[Platform]string, opts PushOptions) (string, error)

// PushWorkspace bundles and pushes a workspace artifact.
func (c *Client) PushWorkspace(ctx context.Context, manifest WorkspaceManifest, dir string, opts PushOptions) (string, error)

// Search queries the marketplace API for plugins/workspaces.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)

// ListVersions returns available versions for an artifact.
func (c *Client) ListVersions(ctx context.Context, ref string) ([]VersionInfo, error)

// CheckUpdates reports available updates for installed plugins.
func (c *Client) CheckUpdates(ctx context.Context) ([]UpdateAvailable, error)
```

### `pkg/sharing/manifest` — Artifact Metadata

```go
package manifest

// PluginManifest describes a published plugin.
type PluginManifest struct {
    Name               string            `json:"name"`
    Type               string            `json:"type"` // config, transform, store, ui
    Version            string            `json:"version"`
    ZhiProtocolVersion string            `json:"zhiProtocolVersion"`
    Description        string            `json:"description"`
    Author             string            `json:"author"`
    License            string            `json:"license"`
    Homepage           string            `json:"homepage,omitempty"`
    Keywords           []string          `json:"keywords,omitempty"`
    Runtime            *RuntimeRequirement `json:"runtime,omitempty"`
    MinZhiVersion      string            `json:"minZhiVersion,omitempty"`
}

// WorkspaceManifest describes a published workspace.
type WorkspaceManifest struct {
    Name         string               `json:"name"`
    Version      string               `json:"version"`
    Description  string               `json:"description"`
    Author       string               `json:"author"`
    License      string               `json:"license"`
    Dependencies []PluginDependency   `json:"dependencies"`
    Tools        []ToolRequirement    `json:"tools,omitempty"`
    Keywords     []string             `json:"keywords,omitempty"`
}

// RuntimeRequirement declares a non-Go runtime needed by the plugin.
type RuntimeRequirement struct {
    Type    string `json:"type"`    // java, python, node, etc.
    Version string `json:"version"` // semver constraint: ">=17", "^3.10"
    Bundled bool   `json:"bundled"` // true if runtime is included in artifact
}

// PluginDependency is an OCI reference to a required plugin.
type PluginDependency struct {
    Ref      string `json:"ref"`      // OCI reference
    Type     string `json:"type"`     // plugin type
    Optional bool   `json:"optional"` // if true, workspace works without it
}
```

### `pkg/sharing/registry` — Registry Configuration

```go
package registry

// Config holds registry settings from ~/.zhi/config.yaml
type Config struct {
    Default    string            `yaml:"default"`     // default registry host
    Registries map[string]Entry  `yaml:"registries"`  // per-host config
    Marketplace MarketplaceConfig `yaml:"marketplace"` // central marketplace settings
}

// Entry holds authentication and options for a single registry.
type Entry struct {
    Auth       AuthConfig `yaml:"auth,omitempty"`
    Mirror     string     `yaml:"mirror,omitempty"`  // local proxy URL
    Insecure   bool       `yaml:"insecure,omitempty"` // allow HTTP (dev only)
}

// MarketplaceConfig points to the discovery API.
type MarketplaceConfig struct {
    URL    string `yaml:"url"`    // default: https://marketplace.zhi.dev
    APIKey string `yaml:"apiKey,omitempty"` // for publishing and ratings
}
```

### `pkg/sharing/verify` — Signature Verification

```go
package verify

// Verifier checks artifact signatures and checksums.
type Verifier struct {
    trustedKeys []TrustedKey      // pinned public keys
    keystoreDir string            // ~/.zhi/keys/
    sigstore    bool              // enable Sigstore/Fulcio verification
}

// VerifyArtifact checks the signature and digest of a pulled artifact.
func (v *Verifier) VerifyArtifact(ctx context.Context, ref string, digest string) (*VerificationResult, error)

// VerificationResult reports the outcome.
type VerificationResult struct {
    Signed     bool   // artifact has a valid signature
    Signer     string // identity of the signer (email, OIDC subject)
    Verified   bool   // zhi project verified publisher
    Trusted    bool   // signer is in user's trust store
    DigestMatch bool  // content digest matches manifest
}
```

## Integration with Existing Architecture

### Registry Extension

The existing `Registry` (`internal/core/registry.go`) gains a new resolution path:

```
Provider lookup: name → built-in → external (disk) → shared (OCI pull)
                   ↑                    ↑                    ↑
              registry maps      ~/.zhi/plugins/     on-demand install
              (compiled in)      (discovered at       from configured
                                  startup)            registry
```

The "shared" resolution is opt-in: if a provider name is not found locally, zhi can prompt the user to install it from the marketplace (or auto-install if configured).

### Workspace Config Extension

`zhi.yaml` gains an optional `sharing` section:

```yaml
version: "1"
config:
  provider: ansible-config
  options:
    inventory: ./hosts.ini

# New: declare plugin sources for reproducible installs
sharing:
  plugins:
    - name: ansible-config
      type: config
      ref: oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
    - name: vault
      type: store
      ref: oci://ghcr.io/zhi-project/zhi-store-vault:v2.0.0
```

This allows `zhi workspace setup` to install all required plugins automatically.

### UI Controller Extension

The `Controller` interface (`pkg/zhiplugin/ui/plugin.go`) gains marketplace methods:

```go
// Extended Controller interface (backward-compatible additions)
type Controller interface {
    // ... existing methods ...

    // Marketplace operations (new)
    SearchPlugins(ctx context.Context, query string) ([]PluginInfo, error)
    InstallPlugin(ctx context.Context, ref string) error
    ListInstalledPlugins(ctx context.Context) ([]InstalledPluginInfo, error)
    CheckPluginUpdates(ctx context.Context) ([]UpdateInfo, error)
    UpdatePlugin(ctx context.Context, name string) error
}
```

This allows any UI plugin (TUI, Web, API) to provide marketplace browsing natively.

## Directory Layout on User's Machine

```
~/.zhi/
├── config.yaml              # registry auth, marketplace URL, preferences
├── plugins/                 # installed plugin binaries (existing)
│   ├── zhi-config-ansible
│   ├── zhi-store-vault
│   └── zhi-ui-web
├── runtimes/                # extracted runtime dependencies (new)
│   ├── java-21/
│   │   └── bin/java
│   └── python-3.12/
│       └── bin/python3
├── cache/                   # local blob cache (new)
│   └── oci/
│       └── blobs/
│           └── sha256/
│               ├── abc123...  (content-addressed blobs)
│               └── def456...
├── keys/                    # trusted signing keys (new)
│   ├── zhi-project.pub      # zhi project's signing key
│   └── custom/
│       └── myorg.pub
└── metadata/                # installed plugin metadata (new)
    ├── ansible-config.json  # version, ref, digest, install date
    └── vault.json
```
