package verify

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"

	protobuf "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/sigstore/sigstore/pkg/signature"
)

// durableKeypair wraps a crypto.PrivateKey to implement sign.Keypair.
// This allows zhi to use durable keys (from disk or KMS) for signing
// instead of ephemeral keys generated per-session.
type durableKeypair struct {
	privateKey   crypto.PrivateKey
	publicKey    crypto.PublicKey
	signer       signature.Signer
	hashAlg      crypto.Hash
	protoHashAlg protobuf.HashAlgorithm
	keyAlg       protobuf.PublicKeyDetails
}

// NewDurableKeypair creates a sign.Keypair from a crypto.PrivateKey.
// Supports ECDSA, RSA, and Ed25519 keys.
func NewDurableKeypair(privateKey crypto.PrivateKey) (*durableKeypair, error) {
	var publicKey crypto.PublicKey
	var hashAlg crypto.Hash
	var protoHashAlg protobuf.HashAlgorithm
	var keyAlg protobuf.PublicKeyDetails
	var signer signature.Signer
	var err error

	switch pk := privateKey.(type) {
	case *ecdsa.PrivateKey:
		publicKey = &pk.PublicKey
		hashAlg = crypto.SHA256
		protoHashAlg = protobuf.HashAlgorithm_SHA2_256
		keyAlg = protobuf.PublicKeyDetails_PKIX_ECDSA_P256_SHA_256
		signer, err = signature.LoadECDSASignerVerifier(pk, hashAlg)
		if err != nil {
			return nil, fmt.Errorf("creating ECDSA signer: %w", err)
		}

	case *rsa.PrivateKey:
		publicKey = &pk.PublicKey
		hashAlg = crypto.SHA256
		protoHashAlg = protobuf.HashAlgorithm_SHA2_256
		keyAlg = protobuf.PublicKeyDetails_PKIX_RSA_PKCS1V15_2048_SHA256
		signer, err = signature.LoadRSAPSSSignerVerifier(pk, hashAlg, nil)
		if err != nil {
			return nil, fmt.Errorf("creating RSA signer: %w", err)
		}

	case ed25519.PrivateKey:
		publicKey = pk.Public()
		hashAlg = crypto.SHA512 // Ed25519 uses SHA-512 internally
		protoHashAlg = protobuf.HashAlgorithm_SHA2_512
		keyAlg = protobuf.PublicKeyDetails_PKIX_ED25519
		signer, err = signature.LoadED25519SignerVerifier(pk)
		if err != nil {
			return nil, fmt.Errorf("creating Ed25519 signer: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported private key type: %T (supported: ECDSA, RSA, Ed25519)", privateKey)
	}

	return &durableKeypair{
		privateKey:   privateKey,
		publicKey:    publicKey,
		signer:       signer,
		hashAlg:      hashAlg,
		protoHashAlg: protoHashAlg,
		keyAlg:       keyAlg,
	}, nil
}

// SignData implements sign.Keypair.SignData by signing the data with the private key.
// It returns both the signature and the signed data hash.
func (d *durableKeypair) SignData(ctx context.Context, data []byte) ([]byte, []byte, error) {
	// Use the sigstore signature.Signer to create the signature.
	// The signer handles the correct padding, hashing, and encoding for each key type.
	sig, err := d.signer.SignMessage(io.NopCloser(newBytesReader(data)))
	if err != nil {
		return nil, nil, fmt.Errorf("signing message: %w", err)
	}

	// Compute the hash of the data using the configured hash algorithm.
	hasher := d.hashAlg.New()
	hasher.Write(data)
	dataHash := hasher.Sum(nil)

	return sig, dataHash, nil
}

// GetPublicKey implements sign.Keypair.GetPublicKey by returning the
// crypto.PublicKey.
func (d *durableKeypair) GetPublicKey() crypto.PublicKey {
	return d.publicKey
}

// GetPublicKeyPem implements sign.Keypair.GetPublicKeyPem by encoding the
// public key as PEM.
func (d *durableKeypair) GetPublicKeyPem() (string, error) {
	// Marshal public key to PKIX format.
	pubBytes, err := x509.MarshalPKIXPublicKey(d.publicKey)
	if err != nil {
		return "", fmt.Errorf("marshaling public key: %w", err)
	}

	// Encode as PEM.
	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}
	return string(pem.EncodeToMemory(pemBlock)), nil
}

// GetHashAlgorithm implements sign.Keypair.GetHashAlgorithm by returning the
// hash algorithm used for this key type in protobuf format.
func (d *durableKeypair) GetHashAlgorithm() protobuf.HashAlgorithm {
	return d.protoHashAlg
}

// GetHint implements sign.Keypair.GetHint by returning a hint for
// public key identification. This is used by Sigstore to match the public
// key during verification. The hint must be valid UTF-8.
func (d *durableKeypair) GetHint() []byte {
	// Compute a SHA-256 hash of the public key and use it as a hint.
	// This ensures the hint is deterministic and valid UTF-8 when base64-encoded.
	fingerprint, err := ComputePublicKeyFingerprint(d.publicKey)
	if err != nil {
		return nil // Return empty hint on error
	}
	// Return the fingerprint as bytes (it's already in "sha256:hex" format).
	return []byte(fingerprint)
}

// GetKeyAlgorithm implements sign.Keypair.GetKeyAlgorithm by returning the
// public key algorithm type as a string.
func (d *durableKeypair) GetKeyAlgorithm() string {
	return d.keyAlg.String()
}

// GetSigningAlgorithm implements sign.Keypair.GetSigningAlgorithm by returning
// the signing algorithm as a PublicKeyDetails enum.
func (d *durableKeypair) GetSigningAlgorithm() protobuf.PublicKeyDetails {
	return d.keyAlg
}

// LoadPrivateKeyFromPEM loads a private key from PEM-encoded data.
// Supports PKCS#8, PKCS#1 (RSA), SEC1 (EC), and raw Ed25519 formats.
func LoadPrivateKeyFromPEM(pemData []byte) (crypto.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS#8 first (most common for modern keys).
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}

	// Try EC private key (SEC1 format).
	ecKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err == nil {
		return ecKey, nil
	}

	// Try RSA private key (PKCS#1 format).
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return rsaKey, nil
	}

	// Try Ed25519 private key (raw 64-byte seed).
	if len(block.Bytes) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(block.Bytes), nil
	}

	return nil, fmt.Errorf("failed to parse private key: not a valid PKCS#8, PKCS#1, SEC1, or Ed25519 key")
}

// bytesReader is a simple io.Reader that reads from a byte slice.
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data, pos: 0}
}

func (b *bytesReader) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

// ComputePublicKeyFingerprint computes a fingerprint of the public key
// for identification purposes. Returns "sha256:<hex>".
func ComputePublicKeyFingerprint(publicKey crypto.PublicKey) (string, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("marshaling public key: %w", err)
	}
	hash := sha256.Sum256(pubBytes)
	return fmt.Sprintf("sha256:%x", hash), nil
}
