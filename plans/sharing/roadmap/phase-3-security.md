# Phase 3: Security — Signing & Verification

**Goal**: All artifacts can be signed and verified. Binary integrity is enforced.

**Prerequisites**: [Phase 1 (Foundation)](phase-1-foundation.md) and [Phase 2 (Publishing)](phase-2-publishing.md) — install and publish must work before signing is layered on.

## Overview

This phase adds cryptographic trust to the sharing system. Publishers sign artifacts during `zhi plugin publish`, and consumers verify signatures during `zhi plugin install`. The existing binary audit mechanism in `internal/core/launcher.go` is extended to verify digests at launch time, catching post-install tampering.

The full threat model and security architecture are described in the [Security & Trust](../security.md) plan. This phase implements the technical signing/verification infrastructure; the organizational verified publisher program is part of [Phase 5](phase-5-community.md).

## Deliverables

### 1. Sigstore/cosign Integration

Using the `sigstore-go` library:

**Signing (during publish)**:
- `zhi plugin publish --sign` creates a cosign signature for the OCI artifact
- **Keyless signing** (recommended for CI): Uses Fulcio for ephemeral certificate issuance via OIDC (GitHub Actions, GitLab CI). The signing event is recorded in the Rekor transparency log.
- **Key-based signing**: Uses a cosign private key (`--key cosign.key` or `COSIGN_KEY` env var) for environments without OIDC
- Signatures are stored as OCI referrers in the same registry (standard cosign behavior)

**Verification (during install)**:
- Fetch cosign signature from OCI registry
- Verify against Fulcio root (keyless) or pinned public key
- Check Rekor transparency log entry
- Verify signer identity matches expected publisher
- Display verification result to user

Four verification levels as described in [Security & Trust](../security.md):
- Level 0: None (`--skip-verify`)
- Level 1: Signed (valid signature from any identity)
- Level 2: Verified publisher (identity matches marketplace registration — requires [Phase 5](phase-5-community.md))
- Level 3: Strict (`requireSignatures: true` in config)

### 2. Binary Integrity Checks

Extend the existing `auditPluginBinary()` function in `internal/core/launcher.go`:

- At install time: store the expected binary digest (from OCI manifest) in `~/.zhi/metadata/<plugin>.json`
- At launch time: compute SHA-256 of the binary and compare against the stored digest
- If mismatch: reject execution with a clear error message and remediation steps
- This catches post-install tampering (e.g., malware replacing the binary on disk)

The existing audit already computes and logs SHA-256 hashes — this phase adds the comparison step.

### 3. Trust Store (`~/.zhi/keys/`)

Local storage for trusted signing keys and identities:

```
~/.zhi/keys/
├── zhi-project.pub      # zhi project's cosign public key (bundled with zhi CLI)
└── custom/
    └── myorg.pub         # organization's signing key
```

**Policy file** (`~/.zhi/policy.yaml` or inline in `~/.zhi/config.yaml`):

```yaml
sharing:
  verification:
    requireSignatures: false    # Level 3 when true
    trustedPublishers:
      - zhi-project             # auto-accept from these identities
      - myorg
    allowedRegistries:
      - ghcr.io
      - harbor.internal:5000
    blockedPlugins:
      - untrusted/bad-plugin
```

Full policy model described in [Security & Trust](../security.md) under "Security Policies".

### 4. Verification Display

Surface verification status in all relevant CLI output:

**During install**:
```
Verifying signature...
  ✓ Signed by release@zhi.dev (Sigstore/Fulcio)
  ✓ Verified publisher: zhi-project
```

**In `zhi plugin list`**: Show signed/unsigned indicator per plugin.

**In `zhi plugin info`**: Show full signer identity, signing method, and trust level.

### 5. `zhi plugin verify` Command

Standalone verification of an OCI artifact without installing:

```bash
zhi plugin verify oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
```

Useful for auditing and compliance workflows.

## Key Files to Modify

| File | Change |
|---|---|
| `internal/core/launcher.go` | Extend `auditPluginBinary()` with digest comparison |
| `pkg/sharing/client/` | Add signature verification to pull flow |
| `internal/cli/plugin_publish.go` | Add `--sign` / `--key` flags and signing logic |
| `internal/cli/plugin_install.go` | Add verification output and `--skip-verify` flag |

## New Packages

```
pkg/sharing/verify/    # Signature verification and trust store management
```

## New Dependencies

```
github.com/sigstore/sigstore-go   # Sigstore verification (cosign, Fulcio, Rekor)
```

## Exit Criteria

- `zhi plugin publish --sign` creates a cosign signature stored as an OCI referrer
- `zhi plugin install` verifies signatures and displays signer identity
- Strict mode (`requireSignatures: true`) rejects unsigned plugins
- Binary tampering post-install is detected at launch time with a clear error
- `zhi plugin verify` checks signatures without installing
- Trust store allows auto-accepting plugins from configured publishers

## Design References

- [Security & Trust](../security.md) — Full threat model, verification levels, publisher namespaces, policy model, future sandboxing
- [Distribution Format](../distribution-format.md) — Content-addressed storage guarantees, digest pinning in lock files
- [CLI Integration](../cli-integration.md) — `zhi plugin verify` command, `--sign` / `--skip-verify` flags
