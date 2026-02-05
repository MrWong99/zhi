# Security & Trust

Plugin distribution introduces a significant attack surface — users download and execute arbitrary binaries. The security model must make it safe by default while remaining practical for the community.

## Threat Model

| Threat | Impact | Mitigation |
|---|---|---|
| **Malicious plugin binary** | Arbitrary code execution on user's machine | Signature verification, verified publishers, sandboxing |
| **Supply chain attack** (compromised publisher account) | Malicious update pushed to trusted plugin | Digest pinning in lock files, cosign keyless verification with OIDC identity |
| **Typosquatting** | User installs `ansibl-config` instead of `ansible-config` | Namespace ownership, name similarity warnings |
| **Man-in-the-middle** | Modified binary during download | OCI content-addressed digests, TLS enforcement |
| **Dependency confusion** | Internal plugin name clashes with public marketplace | Namespace scoping, built-in providers always take precedence |
| **Malicious runtime dependency** | Trojan JVM or Python runtime bundled with plugin | Runtime layer checksums, verified publisher for runtime bundles |
| **Stale/vulnerable plugin** | Known CVE in installed plugin | Update notifications, vulnerability advisory system |

## Signature Verification

### Sigstore Integration (Primary)

zhi uses [Sigstore](https://sigstore.dev/) for artifact signing and verification, specifically:

- **cosign** for signing OCI artifacts
- **Fulcio** for keyless certificate issuance (identity-based signing via OIDC)
- **Rekor** transparency log for auditable signing events
- **sigstore-go** library for verification in the zhi client

#### How It Works

**Publishing (signing):**
```bash
# Publisher signs during 'zhi plugin publish'
# Option 1: Keyless (GitHub Actions OIDC — recommended for CI)
cosign sign --yes ghcr.io/myorg/zhi-config-myplugin:v1.0.0
# Fulcio issues ephemeral certificate, Rekor logs the event

# Option 2: Key-based (for local signing)
cosign sign --key cosign.key ghcr.io/myorg/zhi-config-myplugin:v1.0.0
```

**Installing (verification):**
```bash
# zhi verifies during 'zhi plugin install'
# 1. Fetch cosign signature from OCI registry (stored as referrer)
# 2. Verify against Fulcio root or pinned public key
# 3. Check Rekor transparency log entry
# 4. Verify signer identity matches expected publisher
```

#### Verification Levels

```
Level 0: None (--skip-verify)
  → No signature check. Binary checksum still verified against OCI digest.
  → Warning displayed to user.

Level 1: Signed (default)
  → Artifact has a valid cosign signature from any identity.
  → Signer identity displayed to user.

Level 2: Verified publisher
  → Signer identity matches the registered publisher in the marketplace.
  → Displayed as ✓ in search results and info.

Level 3: Strict (requireSignatures: true in config)
  → Only signed artifacts from trusted publishers can be installed.
  → Trusted publishers configured in ~/.zhi/config.yaml.
```

### Existing Binary Auditing

zhi already audits plugin binaries at launch time (`internal/core/launcher.go:22-51`):
- Computes SHA-256 hash of the binary
- Logs the hash at INFO level
- Warns about world-writable permissions

This auditing is extended for shared plugins:
- Compare binary hash against the OCI manifest digest
- Reject execution if the hash doesn't match (binary was tampered with post-install)
- Store the expected hash in `~/.zhi/metadata/<plugin>.json`

## Publisher Verification

### Verified Publisher Program

The zhi project maintains a verified publisher program for the central marketplace:

1. **Identity verification**: Publisher proves control of a GitHub/GitLab organization or domain.
2. **Code review**: Initial plugin submission undergoes basic code review by zhi maintainers.
3. **Ongoing monitoring**: Automated checks for known vulnerabilities, license compliance, and binary reproducibility.

Verified publishers get:
- A ✓ badge on the marketplace
- Priority in search results
- Ability to publish to the `zhi-project` namespace (for officially endorsed plugins)

### Namespace Ownership

Plugin names are scoped by publisher namespace:

```
zhi-project/ansible-config     # Official zhi project plugin
myorg/custom-config            # Organization plugin
alice/experiment               # Individual developer plugin
```

The marketplace enforces namespace ownership:
- `zhi-project/*` is reserved for the zhi project team
- Organization namespaces require GitHub/GitLab org ownership proof
- Individual namespaces use GitHub/GitLab username

### Typosquatting Prevention

When installing a plugin, the client checks for name similarity with popular plugins:

```
⚠ "ansibl-config" is similar to the verified plugin "ansible-config" by zhi-project.
  Did you mean "ansible-config"? [Y/n]
```

Implementation: Levenshtein distance check against top-100 plugins by download count.

## Sandboxing Considerations

Plugins run as separate processes (via hashicorp/go-plugin) communicating over gRPC. This provides process isolation but not security sandboxing — a malicious plugin has full access to the user's filesystem, network, and other resources.

### Future Sandboxing Options

These are not required for the initial release but should be considered for the roadmap:

1. **Filesystem sandboxing**: Use `landlock` (Linux 5.13+) or `pledge`/`unveil` (OpenBSD) to restrict plugin filesystem access to declared paths.

2. **Network sandboxing**: Plugins that don't need network access (most config and transform plugins) could be restricted from making network calls.

3. **Permission declarations**: Plugins declare required permissions in their manifest:
   ```yaml
   permissions:
     filesystem:
       - read: ~/.ansible/
       - write: /tmp/zhi-*
     network:
       - connect: vault.internal:8200
   ```
   The user reviews and approves permissions at install time (similar to Android app permissions).

4. **WASM plugins**: For maximum isolation, support WebAssembly-based plugins using Wazero (pure Go WASM runtime). WASM plugins run in a memory-safe sandbox with explicit capability grants. This is a long-term direction — the gRPC-based plugin model remains primary.

## Content-Addressed Storage

All OCI artifacts are content-addressed by SHA-256 digest. This provides:

- **Integrity**: Downloaded bytes are verified against the expected digest
- **Immutability**: A given digest always refers to the exact same content
- **Reproducibility**: Lock files pin digests, not tags (tags can be overwritten)

```yaml
# zhi-plugins.lock
plugins:
  - name: ansible-config
    ref: oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
    digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    # Even if :v1.2.0 tag is overwritten, this digest ensures the same binary
```

## Security Policies

### Organization Policy File

Organizations can enforce security policies via a `~/.zhi/policy.yaml`:

```yaml
# Security policy (can be distributed via config management)
sharing:
  # Only allow plugins from these publishers
  allowedPublishers:
    - zhi-project
    - myorg

  # Block specific plugins
  blockedPlugins:
    - untrusted-org/sketchy-plugin

  # Require all plugins to be signed
  requireSignatures: true

  # Only allow specific registries
  allowedRegistries:
    - ghcr.io
    - harbor.internal:5000

  # Disable auto-update checks
  allowAutoUpdate: false
```

### Vulnerability Advisories

The marketplace supports publishing security advisories for plugins:

```
GET /api/v1/advisories

[
  {
    "id": "ZHI-2026-001",
    "plugin": "zhi-project/old-store",
    "affectedVersions": "< 1.5.0",
    "severity": "high",
    "description": "Path traversal in file-based store allows reading arbitrary files",
    "fixedVersion": "1.5.0",
    "publishedAt": "2026-02-01T00:00:00Z"
  }
]
```

The client checks advisories during `zhi plugin update --check` and shows warnings for affected installed plugins.

## Key Rotation

If a publisher's signing key is compromised:

1. Publisher generates a new key pair
2. Publisher re-signs all current versions with the new key
3. Marketplace is notified of key rotation
4. Clients that pin the old key are warned to update their trust store

For Sigstore keyless signing, key compromise is less of a concern since certificates are ephemeral — the OIDC identity (e.g., GitHub Actions workflow) is what matters.
