// Package verify implements signature verification and trust policy
// enforcement for zhi's plugin sharing system. It provides a Verifier
// that checks artifact signatures and enforces security policies
// configured in ~/.zhi/policy.yaml.
//
// This package supports four verification levels:
//
//   - Level 0 (None): No signature check; binary checksum still verified
//     against OCI digest. Requires --skip-verify flag.
//   - Level 1 (Signed): Artifact has a valid signature from any identity.
//     This is the default level.
//   - Level 2 (VerifiedPublisher): Signer identity matches a registered
//     publisher in the marketplace (requires Phase 5).
//   - Level 3 (Strict): Only artifacts signed by trusted publishers are
//     allowed. Enabled via requireSignatures in policy.
package verify

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

// Level represents the signature verification level applied during install.
type Level int

const (
	// LevelNone skips signature verification entirely (--skip-verify).
	LevelNone Level = iota
	// LevelSigned requires a valid signature from any identity (default).
	LevelSigned
	// LevelVerifiedPublisher requires the signer to match a registered
	// publisher in the marketplace (Phase 5).
	LevelVerifiedPublisher
	// LevelStrict only allows artifacts from trusted publishers configured
	// in the local policy.
	LevelStrict
)

// String returns a human-readable label for the verification level.
func (l Level) String() string {
	switch l {
	case LevelNone:
		return "none"
	case LevelSigned:
		return "signed"
	case LevelVerifiedPublisher:
		return "verified-publisher"
	case LevelStrict:
		return "strict"
	default:
		return fmt.Sprintf("Level(%d)", int(l))
	}
}

// SigningMethod describes how an artifact was signed.
type SigningMethod string

const (
	// SigningMethodKeyless indicates Sigstore keyless signing via Fulcio/OIDC.
	SigningMethodKeyless SigningMethod = "keyless"
	// SigningMethodKey indicates signing with a cosign private key.
	SigningMethodKey SigningMethod = "key"
	// SigningMethodNone indicates the artifact is unsigned.
	SigningMethodNone SigningMethod = "none"
)

// Result holds the outcome of a signature verification check.
type Result struct {
	// Signed indicates whether the artifact has a valid signature.
	Signed bool
	// SigningIdentity is the signer's identity (e.g. email, OIDC subject).
	SigningIdentity string
	// SigningMethod describes how the artifact was signed.
	SigningMethod SigningMethod
	// TrustedPublisher indicates the signer matches a trusted publisher
	// from the local policy.
	TrustedPublisher bool
	// Level is the effective verification level that was applied.
	Level Level
	// Error holds any verification error (nil if verification passed).
	Error error
}

// OK returns true when the verification passed without error.
func (r *Result) OK() bool {
	return r.Error == nil
}

// Summary returns a short human-readable summary of the result.
func (r *Result) Summary() string {
	if r.Error != nil {
		return fmt.Sprintf("verification failed: %v", r.Error)
	}
	if !r.Signed {
		return "unsigned"
	}
	s := "signed"
	if r.SigningIdentity != "" {
		s += " by " + r.SigningIdentity
	}
	if r.TrustedPublisher {
		s += " (trusted publisher)"
	}
	return s
}

// StatusIndicator returns a short status string for use in plugin list output.
func (r *Result) StatusIndicator() string {
	if !r.Signed {
		return "unsigned"
	}
	if r.TrustedPublisher {
		return "verified"
	}
	return "signed"
}

// Verifier checks artifact signatures and enforces security policy.
type Verifier struct {
	policy *Policy
}

// NewVerifier creates a verifier with the given policy. If policy is nil,
// a default permissive policy is used.
func NewVerifier(policy *Policy) *Verifier {
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Verifier{policy: policy}
}

// VerifyArtifact checks the signature and policy for an OCI artifact
// identified by its reference. When skipVerify is true, verification is
// skipped but a warning-level result is returned.
//
// In Phase 3 the actual Sigstore/cosign verification is stubbed because
// the sigstore-go dependency is not yet integrated. The verification
// infrastructure (policy enforcement, result types, CLI integration) is
// fully wired so that adding real signature checks is a matter of
// implementing the cosign verification call.
func (v *Verifier) VerifyArtifact(ref string, skipVerify bool) *Result {
	// Check if the plugin is blocked by policy.
	if v.policy.IsBlocked(ref) {
		return &Result{
			Level: v.policy.EffectiveLevel(),
			Error: fmt.Errorf("plugin %q is blocked by policy", ref),
		}
	}

	// Check if the registry is allowed.
	if !v.policy.IsRegistryAllowed(ref) {
		return &Result{
			Level: v.policy.EffectiveLevel(),
			Error: fmt.Errorf("registry for %q is not in the allowed list", ref),
		}
	}

	if skipVerify {
		return &Result{
			Signed:        false,
			SigningMethod: SigningMethodNone,
			Level:         LevelNone,
		}
	}

	// In strict mode, unsigned artifacts are rejected.
	// Since we cannot yet verify real cosign signatures (sigstore-go not
	// integrated), we treat all artifacts as unsigned for now.
	// When sigstore-go is added, this is where the cosign.Verify call goes.
	if v.policy.RequireSignatures {
		return &Result{
			Signed:        false,
			SigningMethod: SigningMethodNone,
			Level:         LevelStrict,
			Error:         fmt.Errorf("policy requires signatures but artifact %q is unsigned (sigstore verification not yet integrated)", ref),
		}
	}

	// Default: artifact is treated as unsigned but accepted.
	return &Result{
		Signed:        false,
		SigningMethod: SigningMethodNone,
		Level:         LevelSigned,
	}
}

// VerifyBinaryDigest computes the SHA-256 digest of a binary file and
// compares it against the expected digest stored in metadata.
func VerifyBinaryDigest(binaryPath, expectedDigest string) error {
	if expectedDigest == "" {
		return nil // No expected digest to compare against.
	}

	actual, err := ComputeBinaryDigest(binaryPath)
	if err != nil {
		return fmt.Errorf("computing binary digest: %w", err)
	}

	if actual != expectedDigest {
		return fmt.Errorf(
			"binary integrity check failed for %s: expected digest %s, got %s\n"+
				"The plugin binary may have been tampered with after installation.\n"+
				"Reinstall the plugin with: zhi plugin install <reference> --force",
			binaryPath, expectedDigest, actual,
		)
	}
	return nil
}

// ComputeBinaryDigest computes the SHA-256 digest of a file and returns
// it in the "sha256:<hex>" format used by OCI digests.
func ComputeBinaryDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// extractRegistryHost extracts the registry host from an OCI reference.
func extractRegistryHost(ref string) string {
	ref = strings.TrimPrefix(ref, "oci://")
	// The host is everything before the first '/'.
	if idx := strings.Index(ref, "/"); idx > 0 {
		return ref[:idx]
	}
	return ref
}
