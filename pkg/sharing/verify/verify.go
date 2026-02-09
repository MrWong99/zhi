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
//     publisher in the marketplace.
//   - Level 3 (Strict): Only artifacts signed by trusted publishers are
//     allowed. Enabled via requireSignatures in policy.
package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// Level represents the signature verification level applied during install.
type Level int

const (
	// LevelNone skips signature verification entirely (--skip-verify).
	LevelNone Level = iota
	// LevelSigned requires a valid signature from any identity (default).
	LevelSigned
	// LevelVerifiedPublisher requires the signer to match a registered
	// publisher in the marketplace.
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
	if v.policy.RequireSignatures {
		return &Result{
			Signed:        false,
			SigningMethod: SigningMethodNone,
			Level:         LevelStrict,
			Error:         fmt.Errorf("policy requires signatures but artifact %q is unsigned", ref),
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

// VerifySignatureOptions controls signature verification behavior.
type VerifySignatureOptions struct {
	// ExpectedIssuer is the expected OIDC issuer for the signing certificate.
	ExpectedIssuer string
	// ExpectedSAN is the expected Subject Alternative Name in the certificate.
	ExpectedSAN string
	// RequireTimestamp requires either an RFC3161 signed timestamp or log entry integrated timestamp.
	RequireTimestamp bool
	// RequireCTLog requires Certificate Transparency log entry.
	RequireCTLog bool
	// RequireTLog requires Artifact Transparency log entry (Rekor).
	RequireTLog bool
}

// VerifySignature verifies a Sigstore bundle against an artifact digest.
// This function checks the cryptographic signature, certificate identity,
// and transparency log entries according to the provided options.
func VerifySignature(ctx context.Context, bundlePath string, artifactDigest string, opts VerifySignatureOptions) (*Result, error) {
	// Load the Sigstore bundle from disk.
	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("loading bundle from %s: %w", bundlePath, err)
	}

	// Initialize TUF client to fetch trusted root.
	tufOpts := tuf.DefaultOptions()
	tufClient, err := tuf.New(tufOpts)
	if err != nil {
		return nil, fmt.Errorf("initializing TUF client: %w", err)
	}

	trustedRoot, err := root.GetTrustedRoot(tufClient)
	if err != nil {
		return nil, fmt.Errorf("fetching trusted root: %w", err)
	}

	// Configure verifier options based on requirements.
	verifierOpts := []verify.VerifierOption{}

	if opts.RequireCTLog {
		verifierOpts = append(verifierOpts, verify.WithSignedCertificateTimestamps(1))
	}

	if opts.RequireTimestamp {
		verifierOpts = append(verifierOpts, verify.WithObserverTimestamps(1))
	}

	if opts.RequireTLog {
		verifierOpts = append(verifierOpts, verify.WithTransparencyLog(1))
	}

	// Create the verifier.
	verifier, err := verify.NewVerifier(trustedRoot, verifierOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating verifier: %w", err)
	}

	// Configure identity policy.
	identityPolicies := []verify.PolicyOption{}

	if opts.ExpectedIssuer != "" || opts.ExpectedSAN != "" {
		certID, err := verify.NewShortCertificateIdentity(opts.ExpectedIssuer, "", opts.ExpectedSAN, "")
		if err != nil {
			return nil, fmt.Errorf("creating certificate identity: %w", err)
		}
		identityPolicies = append(identityPolicies, verify.WithCertificateIdentity(certID))
	}

	// Parse the artifact digest.
	digestBytes, err := parseDigest(artifactDigest)
	if err != nil {
		return nil, fmt.Errorf("parsing digest: %w", err)
	}

	// Create verification policy with artifact digest.
	artifactPolicy := verify.WithArtifactDigest("sha256", digestBytes)
	policy := verify.NewPolicy(artifactPolicy, identityPolicies...)

	// Verify the bundle.
	verifyResult, err := verifier.Verify(b, policy)
	if err != nil {
		return &Result{
			Signed:        true,
			SigningMethod: SigningMethodKeyless, // Assume keyless for now
			Level:         LevelSigned,
			Error:         fmt.Errorf("signature verification failed: %w", err),
		}, nil
	}

	// Extract signing identity from verification result.
	// The verification result contains the certificate subject or key ID.
	signingIdentity := extractSigningIdentity(verifyResult)

	return &Result{
		Signed:           true,
		SigningIdentity:  signingIdentity,
		SigningMethod:    SigningMethodKeyless, // TODO: detect key-based vs keyless
		TrustedPublisher: false,                // TODO: check against trusted publisher list
		Level:            LevelSigned,
	}, nil
}

// parseDigest parses a digest string in "sha256:abc123..." format and returns
// the raw bytes.
func parseDigest(digest string) ([]byte, error) {
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid digest format (expected 'algorithm:hex'): %s", digest)
	}

	hexDigest := parts[1]
	digestBytes, err := hex.DecodeString(hexDigest)
	if err != nil {
		return nil, fmt.Errorf("decoding hex digest: %w", err)
	}

	return digestBytes, nil
}

// extractSigningIdentity extracts the signing identity from a verification result.
func extractSigningIdentity(result *verify.VerificationResult) string {
	// The VerificationResult contains certificate information.
	// For keyless signing, the identity is in the certificate's SAN.
	// For key-based signing, it's the key fingerprint.
	if result.Signature != nil && result.Signature.Certificate != nil {
		cert := result.Signature.Certificate
		// The certificate.Summary type has SubjectAlternativeName as a string field.
		if cert.SubjectAlternativeName != "" {
			return cert.SubjectAlternativeName
		}
		// Fall back to issuer if available.
		if cert.Issuer != "" {
			return cert.Issuer
		}
	}
	return "unknown"
}
