# Distribution Format: OCI Artifacts vs Alternatives

## Decision

**Use OCI (Open Container Initiative) artifacts as the distribution format**, not raw Docker container images.

Docker images were the original inspiration, but OCI artifacts are the refined version of the same idea — they use the same registries, the same wire protocol, and the same tooling ecosystem, but without the unnecessary container runtime semantics (entrypoints, CMD, layers meant for filesystem overlays). This distinction matters because zhi plugins are standalone binaries, not running containers.

## Why Not Raw Docker Container Images?

The initial idea of using Docker images has merit — the ecosystem is established, registries are ubiquitous, and images can bundle dependencies. However, there are significant drawbacks to using *container images specifically*:

| Concern | Issue |
|---|---|
| **Runtime overhead** | Extracting a plugin binary from a Docker image requires either a container runtime or reimplementing image layer extraction. Users should not need Docker installed just to manage zhi plugins. |
| **Semantic mismatch** | Docker images model a filesystem with layers, an entrypoint, environment variables, and a runtime config. A zhi plugin is a single binary (or a binary + runtime). The container abstraction adds complexity without value. |
| **Size bloat** | A Go plugin binary is 10-30 MB. A Docker image with a base layer (even `scratch`) adds overhead in metadata, layer tarballs, and manifest structures. For a Java plugin with a JVM, the image would work, but an OCI artifact with a runtime layer achieves the same result without the container semantics. |
| **Extraction complexity** | To get a binary out of a Docker image without Docker, you need to: pull the manifest, identify the correct layer, download and decompress each layer, apply the filesystem overlay, and locate the binary. OCI artifacts skip all of this — each layer is a directly addressable blob. |

## What Are OCI Artifacts?

The OCI Distribution Specification (v1.1, finalized 2024) extends container registries to store **arbitrary content** with custom media types. An OCI artifact is:

- A **manifest** (JSON) pointing to content-addressed blobs
- One or more **layers** (blobs) containing the actual content
- A **config** blob with artifact metadata
- Custom **media types** to distinguish artifact types from container images

OCI artifacts use the exact same registry API as Docker images. They work with Docker Hub, GHCR, AWS ECR, Azure ACR, Google Artifact Registry, Harbor, and any OCI-compliant registry.

### Precedent

| Project | Use of OCI Artifacts |
|---|---|
| **Helm** | Charts are stored as OCI artifacts since v3.8. `helm push` / `helm install oci://...` |
| **WebAssembly** | CNCF defined a Wasm OCI artifact layout with `application/wasm` media type |
| **Sigstore/cosign** | Signatures and attestations are stored as OCI artifacts via the referrers API |
| **Docker Model Runner** | LLM models are packaged as OCI artifacts |
| **SBOMs** | Software Bills of Materials are attached to images as OCI artifacts |

## Proposed Media Types for zhi

```
# Plugin artifacts
application/vnd.zhi.plugin.config.v1+json      # Plugin metadata (config blob)
application/vnd.zhi.plugin.binary.v1            # Plugin binary (layer)
application/vnd.zhi.plugin.runtime.v1+tar.gz    # Optional runtime dependencies (layer)
application/vnd.zhi.plugin.readme.v1+markdown    # Optional README (layer)
application/vnd.zhi.plugin.license.v1           # Optional license text (layer)

# Workspace artifacts
application/vnd.zhi.workspace.config.v1+json    # Workspace metadata (config blob)
application/vnd.zhi.workspace.bundle.v1+tar.gz  # Workspace files: zhi.yaml, templates, etc.
application/vnd.zhi.workspace.readme.v1+markdown # Optional README
```

## Plugin Artifact Structure

### Single-Platform Plugin

```
OCI Manifest
├── config: application/vnd.zhi.plugin.config.v1+json
│   {
│     "name": "ansible-config",
│     "type": "config",
│     "version": "1.2.0",
│     "zhiProtocolVersion": "1",
│     "description": "Configuration provider backed by Ansible inventory",
│     "author": "zhi-community",
│     "license": "Apache-2.0",
│     "runtime": null,
│     "homepage": "https://github.com/zhi-community/zhi-config-ansible",
│     "keywords": ["ansible", "inventory", "config"]
│   }
└── layers:
    ├── [0] application/vnd.zhi.plugin.binary.v1
    │   (the zhi-config-ansible executable, ~15 MB)
    ├── [1] application/vnd.zhi.plugin.readme.v1+markdown (optional)
    │   (README.md content)
    └── [2] application/vnd.zhi.plugin.license.v1 (optional)
        (LICENSE file content)
```

### Multi-Platform Plugin (typical case)

```
OCI Image Index (fat manifest)
├── linux/amd64 → Manifest (structure above)
├── linux/arm64 → Manifest
├── darwin/amd64 → Manifest
├── darwin/arm64 → Manifest
└── windows/amd64 → Manifest
```

The OCI Image Index natively models multi-platform artifacts. The client selects the correct platform at pull time — exactly like `docker pull` selects the right architecture.

### Plugin with Runtime Dependencies

For a Java-based plugin that needs a JVM:

```
OCI Manifest
├── config: application/vnd.zhi.plugin.config.v1+json
│   {
│     "name": "vault-store",
│     "type": "store",
│     "version": "2.0.0",
│     "runtime": {
│       "type": "java",
│       "version": ">=17",
│       "bundled": true
│     }
│   }
└── layers:
    ├── [0] application/vnd.zhi.plugin.binary.v1
    │   (wrapper script + JAR, ~5 MB)
    └── [1] application/vnd.zhi.plugin.runtime.v1+tar.gz
        (JRE 21 minimal, ~45 MB, extracted to ~/.zhi/runtimes/java-21/)
```

The `runtime.bundled: true` flag tells zhi to extract the runtime layer. If `bundled: false`, zhi checks the system for the required runtime and provides installation instructions if missing.

## Workspace Artifact Structure

```
OCI Manifest
├── config: application/vnd.zhi.workspace.config.v1+json
│   {
│     "name": "kubernetes-cluster",
│     "version": "1.0.0",
│     "description": "Configure and provision a Kubernetes cluster",
│     "author": "zhi-project",
│     "license": "Apache-2.0",
│     "dependencies": [
│       { "ref": "oci://ghcr.io/zhi-project/zhi-config-structured:v1.0.0" },
│       { "ref": "oci://ghcr.io/zhi-project/zhi-transform-k8s:v1.0.0" },
│       { "ref": "oci://ghcr.io/zhi-project/zhi-store-vault:v2.0.0" }
│     ],
│     "tools": [
│       { "name": "kubectl", "version": ">=1.28" },
│       { "name": "helm", "version": ">=3.12" }
│     ]
│   }
└── layers:
    ├── [0] application/vnd.zhi.workspace.bundle.v1+tar.gz
    │   Contains:
    │   ├── zhi.yaml
    │   ├── templates/
    │   │   ├── kubeconfig.tmpl
    │   │   └── helm-values.yaml.tmpl
    │   └── apply/
    │       └── setup.sh
    └── [1] application/vnd.zhi.workspace.readme.v1+markdown
        (README with setup instructions)
```

## Go Library: oras-go v2

The recommended library for OCI artifact operations is **oras-go v2** (`oras.land/oras-go/v2`), the CNCF project purpose-built for non-container OCI artifacts.

### Why oras-go

- Purpose-built for artifact workflows (not container-focused like go-containerregistry)
- Used by Helm for OCI chart distribution — proven at scale
- Clean API: `oras.Copy()`, `oras.PackManifest()`, `content.File` for local files
- Supports authentication (Basic, Bearer, OAuth), content-addressed storage
- Native support for OCI Layout (local directory format) for offline use
- Active CNCF maintenance, Go 1.24+ compatible

### Key Operations

```go
import "oras.land/oras-go/v2"

// Push a plugin
oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, mediaType, opts)
oras.Copy(ctx, store, tag, remoteRepo, tag, copyOpts)

// Pull a plugin
oras.Copy(ctx, remoteRepo, tag, localStore, tag, copyOpts)

// Resolve platform
// Use OCI Image Index platform matching (built into oras-go)
```

## Alternatives Considered

### Custom HTTP Registry (Terraform-style)

A REST API with version listing and download URL endpoints. Simple to implement (~300 lines for the client) but requires standing up custom server infrastructure. OCI registries already exist in most organizations.

**Verdict**: Adopted as a supplementary metadata/discovery layer (see [Marketplace Server](marketplace-server.md)), but not as the primary artifact storage.

### Git-Based Distribution (Homebrew Taps)

A Git repository containing metadata files with pointers to release asset binaries. Low implementation complexity but poor for large artifacts, no native multi-platform support, and difficult in air-gapped environments.

**Verdict**: Rejected as primary mechanism. The plugin index repository (see [Marketplace Server](marketplace-server.md)) borrows the "tap" concept for community contributions.

### Simple tar.gz + HTTP

Direct download of archives from any HTTP server. Minimal implementation but no version resolution, no multi-platform selection, and poor discoverability.

**Verdict**: Rejected. Becomes a custom HTTP registry the moment you add version listing and checksums.

### OS Package Managers (APT, Homebrew, Chocolatey)

Native package manager distribution. Excellent for dependency management but impractical for a dynamic plugin ecosystem — the packaging overhead per plugin per platform per package manager is prohibitive for third-party publishers.

**Verdict**: Recommended for distributing the zhi CLI itself. Not suitable for plugin distribution.

## Addressing Format

```bash
# Full OCI reference
oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0

# Digest-pinned (immutable)
oci://ghcr.io/zhi-project/zhi-config-ansible@sha256:abc123...

# Short form using configured default registry
ansible-config@1.2.0

# Local OCI layout (offline)
oci-layout:///path/to/bundle/
```

## Lock File

Workspaces that depend on shared plugins should include a `zhi-plugins.lock` file that pins exact OCI digests for reproducibility:

```yaml
# zhi-plugins.lock (auto-generated, committed to version control)
version: 1
plugins:
  - name: ansible-config
    type: config
    ref: oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
    digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    platform: linux/amd64
  - name: vault
    type: store
    ref: oci://ghcr.io/zhi-project/zhi-store-vault:v2.0.0
    digest: sha256:abc123...
    platform: linux/amd64
```
