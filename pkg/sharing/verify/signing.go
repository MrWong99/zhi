package verify

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore/pkg/signature"
	"google.golang.org/protobuf/encoding/protojson"
)

// SignOptions controls how an artifact is signed.
type SignOptions struct {
	// KeyPath is the path to a cosign private key file. If empty, keyless
	// signing via Fulcio/OIDC is used.
	KeyPath string
	// IDToken is an OIDC token for keyless signing. Required when KeyPath
	// is empty. Set via SIGSTORE_ID_TOKEN env var or --identity-token flag.
	IDToken string
	// UseRekor enables logging the signature to Rekor transparency log.
	UseRekor bool
	// UseTimestamp enables signed timestamps from TSA.
	UseTimestamp bool
}

// SignResult holds the outcome of a signing operation.
type SignResult struct {
	// BundleJSON is the Sigstore bundle as JSON bytes, containing the
	// signature and verification material.
	BundleJSON []byte
	// SigningIdentity is the identity that signed the artifact.
	SigningIdentity string
	// SigningMethod describes how the artifact was signed.
	SigningMethod SigningMethod
}

// Signer signs OCI artifacts using Sigstore/cosign.
type Signer struct {
	// tufClient fetches trusted root and signing config from TUF.
	tufClient *tuf.Client
}

// NewSigner creates a new signer. It initializes a TUF client to fetch
// trusted root and signing configuration from Sigstore's public-good instance.
func NewSigner(ctx context.Context) (*Signer, error) {
	// Use Sigstore's production public-good instance by default.
	opts := tuf.DefaultOptions()
	tufClient, err := tuf.New(opts)
	if err != nil {
		return nil, fmt.Errorf("initializing TUF client: %w", err)
	}

	return &Signer{
		tufClient: tufClient,
	}, nil
}

// SignArtifact signs an OCI artifact identified by its digest. The artifact
// must already be pushed to the registry before signing.
//
// For keyless signing (opts.KeyPath == ""), this function uses Sigstore's
// Fulcio CA and requires an OIDC identity token in opts.IDToken or the
// SIGSTORE_ID_TOKEN environment variable.
//
// For key-based signing (opts.KeyPath != ""), a PEM-encoded private key
// (ECDSA, RSA, or Ed25519) is loaded from disk and used to sign.
//
// The returned SignResult.BundleJSON contains a Sigstore bundle that should
// be stored alongside the artifact (e.g., as an OCI image attachment or
// separate file).
func (s *Signer) SignArtifact(ctx context.Context, artifactDigest string, opts SignOptions) (*SignResult, error) {
	if artifactDigest == "" {
		return nil, fmt.Errorf("artifactDigest is required")
	}

	// Load trusted root and signing config from TUF.
	trustedRoot, err := root.GetTrustedRoot(s.tufClient)
	if err != nil {
		return nil, fmt.Errorf("fetching trusted root: %w", err)
	}

	signingConfig, err := root.GetSigningConfig(s.tufClient)
	if err != nil {
		return nil, fmt.Errorf("fetching signing config: %w", err)
	}

	// Create the content to sign (the artifact digest).
	content := &sign.PlainData{
		Data: []byte(artifactDigest),
	}

	// Declare keypair as the interface type so we can assign either
	// ephemeral or durable keypairs.
	var keypair sign.Keypair

	bundleOpts := sign.BundleOptions{
		TrustedRoot: trustedRoot,
	}

	var signingIdentity string
	var signingMethod SigningMethod

	if opts.KeyPath != "" {
		// Key-based signing: use a durable private key.
		signingMethod = SigningMethodKey

		// Load the private key from disk.
		keyData, err := os.ReadFile(opts.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading private key from %s: %w", opts.KeyPath, err)
		}

		// Parse the private key using our helper function.
		privateKey, err := LoadPrivateKeyFromPEM(keyData)
		if err != nil {
			return nil, fmt.Errorf("parsing private key from %s: %w", opts.KeyPath, err)
		}

		// Create a durable keypair that wraps the loaded key.
		durableKP, err := NewDurableKeypair(privateKey)
		if err != nil {
			return nil, fmt.Errorf("creating durable keypair: %w", err)
		}
		keypair = durableKP

		// Compute a fingerprint of the public key for identification.
		fingerprint, err := ComputePublicKeyFingerprint(durableKP.publicKey)
		if err != nil {
			return nil, fmt.Errorf("computing public key fingerprint: %w", err)
		}
		signingIdentity = fingerprint

		// For key-based signing, we need to add the public key to the
		// trusted material so verification will work without Fulcio.
		publicKeyPem, err := keypair.GetPublicKeyPem()
		if err != nil {
			return nil, fmt.Errorf("getting public key PEM: %w", err)
		}

		block, _ := pem.Decode([]byte(publicKeyPem))
		if block == nil {
			return nil, fmt.Errorf("failed to decode public key PEM")
		}

		pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing public key: %w", err)
		}

		verifier, err := signature.LoadVerifier(pubKey, crypto.SHA256)
		if err != nil {
			return nil, fmt.Errorf("creating verifier: %w", err)
		}

		key := root.NewExpiringKey(verifier, time.Time{}, time.Time{})
		keyTrustedMaterial := root.NewTrustedPublicKeyMaterial(func(_ string) (root.TimeConstrainedVerifier, error) {
			return key, nil
		})

		// Combine the trusted root with the public key material.
		bundleOpts.TrustedRoot = &combinedTrustedMaterial{
			TrustedMaterial:    trustedRoot,
			keyTrustedMaterial: keyTrustedMaterial,
		}
	} else {
		// Keyless signing: use Fulcio certificate authority.
		signingMethod = SigningMethodKeyless

		// Generate an ephemeral keypair for keyless signing.
		ephemeralKP, err := sign.NewEphemeralKeypair(nil)
		if err != nil {
			return nil, fmt.Errorf("generating ephemeral keypair: %w", err)
		}
		keypair = ephemeralKP

		// Require an ID token for keyless signing.
		idToken := opts.IDToken
		if idToken == "" {
			idToken = os.Getenv("SIGSTORE_ID_TOKEN")
		}
		if idToken == "" {
			return nil, fmt.Errorf("OIDC ID token required for keyless signing: set SIGSTORE_ID_TOKEN environment variable or pass --identity-token flag")
		}

		// Select Fulcio service from signing config.
		fulcioService, err := root.SelectService(signingConfig.FulcioCertificateAuthorityURLs(), sign.FulcioAPIVersions, time.Now())
		if err != nil {
			return nil, fmt.Errorf("selecting Fulcio service: %w", err)
		}

		fulcioOpts := &sign.FulcioOptions{
			BaseURL: fulcioService.URL,
			Timeout: 30 * time.Second,
			Retries: 1,
		}

		bundleOpts.CertificateProvider = sign.NewFulcio(fulcioOpts)
		bundleOpts.CertificateProviderOptions = &sign.CertificateProviderOptions{
			IDToken: idToken,
		}

		// Extract identity from ID token (this will be set by Fulcio in the cert).
		signingIdentity = "keyless-identity" // Placeholder; real identity comes from the cert SAN.
	}

	// Add Rekor transparency log if requested.
	if opts.UseRekor {
		rekorServices, err := root.SelectServices(signingConfig.RekorLogURLs(),
			signingConfig.RekorLogURLsConfig(), sign.RekorAPIVersions, time.Now())
		if err != nil {
			return nil, fmt.Errorf("selecting Rekor services: %w", err)
		}

		for _, rekorService := range rekorServices {
			rekorOpts := &sign.RekorOptions{
				BaseURL: rekorService.URL,
				Timeout: 90 * time.Second,
				Retries: 1,
				Version: rekorService.MajorAPIVersion,
			}
			bundleOpts.TransparencyLogs = append(bundleOpts.TransparencyLogs, sign.NewRekor(rekorOpts))
		}
	}

	// Add TSA timestamps if requested.
	if opts.UseTimestamp {
		tsaServices, err := root.SelectServices(signingConfig.TimestampAuthorityURLs(),
			signingConfig.TimestampAuthorityURLsConfig(), sign.TimestampAuthorityAPIVersions, time.Now())
		if err != nil {
			return nil, fmt.Errorf("selecting TSA services: %w", err)
		}

		for _, tsaService := range tsaServices {
			tsaOpts := &sign.TimestampAuthorityOptions{
				URL:     tsaService.URL,
				Timeout: 30 * time.Second,
				Retries: 1,
			}
			bundleOpts.TimestampAuthorities = append(bundleOpts.TimestampAuthorities, sign.NewTimestampAuthority(tsaOpts))
		}
	}

	// Create the signed bundle.
	sigBundle, err := sign.Bundle(content, keypair, bundleOpts)
	if err != nil {
		return nil, fmt.Errorf("creating signed bundle: %w", err)
	}

	// Marshal the bundle to JSON.
	bundleJSON, err := protojson.Marshal(sigBundle)
	if err != nil {
		return nil, fmt.Errorf("marshalling bundle to JSON: %w", err)
	}

	return &SignResult{
		BundleJSON:      bundleJSON,
		SigningIdentity: signingIdentity,
		SigningMethod:   signingMethod,
	}, nil
}

// SaveBundleToFile writes a Sigstore bundle to a file.
func SaveBundleToFile(bundleJSON []byte, path string) error {
	if err := os.WriteFile(path, bundleJSON, 0o644); err != nil {
		return fmt.Errorf("writing bundle to %s: %w", path, err)
	}
	return nil
}

// LoadBundleFromFile loads a Sigstore bundle from a JSON file.
func LoadBundleFromFile(path string) (*bundle.Bundle, error) {
	return bundle.LoadJSONFromPath(path)
}

// combinedTrustedMaterial combines trusted root with public key material for
// key-based signing. This allows verification to work with both Fulcio
// certificates (from keyless signing) and raw public keys (from key-based signing).
type combinedTrustedMaterial struct {
	root.TrustedMaterial
	keyTrustedMaterial root.TrustedMaterial
}

func (c *combinedTrustedMaterial) PublicKeyVerifier(hint string) (root.TimeConstrainedVerifier, error) {
	return c.keyTrustedMaterial.PublicKeyVerifier(hint)
}
