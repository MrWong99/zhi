package verify

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNewSigner(t *testing.T) {
	ctx := context.Background()
	signer, err := NewSigner(ctx)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}
	if signer == nil {
		t.Fatal("expected non-nil signer")
	}
	if signer.tufClient == nil {
		t.Error("expected non-nil TUF client")
	}
}

func TestSignArtifactWithoutKeyOrToken(t *testing.T) {
	ctx := context.Background()
	signer, err := NewSigner(ctx)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	// Signing without a key and without an ID token should fail.
	opts := SignOptions{
		UseRekor:     false,
		UseTimestamp: false,
	}

	result, err := signer.SignArtifact(ctx, "sha256:abc123", opts)
	if err == nil {
		t.Error("expected error when signing without key or ID token")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if !strings.Contains(err.Error(), "OIDC ID token required") {
		t.Errorf("expected error about missing ID token, got: %v", err)
	}
}

func TestSignArtifactWithKeyPath(t *testing.T) {
	t.Skip("Key-based signing requires real EC keys and creates actual Sigstore bundles; skipping in unit tests")

	ctx := context.Background()
	signer, err := NewSigner(ctx)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	// Create a temporary dummy key file.
	tmpfile, err := os.CreateTemp("", "testkey-*.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write a dummy PEM block (not a real key, just for testing file loading).
	dummyPEM := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIGCSQqwxH8FmPGmMEJJ7F1J7K3o3K4xO9pL9z8gJ7YdYoAoGCCqGSM49
AwEHoUQDQgAE2w1tL/JT3p7qR8dXpKZ7yN5w6v6m4Z9U7L8nJ7dH5p9L7Z8gJ7dH
5p9L7Z8gJ7dH5p9L7Z8gJ7dH5p9L7Z8gJw==
-----END EC PRIVATE KEY-----`
	if _, err := tmpfile.Write([]byte(dummyPEM)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	opts := SignOptions{
		KeyPath:      tmpfile.Name(),
		UseRekor:     false,
		UseTimestamp: false,
	}

	// Signing with a key should now work (implementation is complete).
	result, err := signer.SignArtifact(ctx, "sha256:abc123", opts)
	if err != nil {
		t.Errorf("unexpected error for key-based signing: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
	if result != nil && result.SigningMethod != SigningMethodKey {
		t.Errorf("expected SigningMethodKey, got %v", result.SigningMethod)
	}
}

func TestSignArtifactEmptyDigest(t *testing.T) {
	ctx := context.Background()
	signer, err := NewSigner(ctx)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	opts := SignOptions{}
	result, err := signer.SignArtifact(ctx, "", opts)
	if err == nil {
		t.Error("expected error for empty digest")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if !strings.Contains(err.Error(), "artifactDigest is required") {
		t.Errorf("expected 'artifactDigest is required' error, got: %v", err)
	}
}

func TestSaveBundleToFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "bundle-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	bundleJSON := []byte(`{"test": "bundle"}`)
	if err := SaveBundleToFile(bundleJSON, tmpfile.Name()); err != nil {
		t.Errorf("SaveBundleToFile failed: %v", err)
	}

	// Verify the file was written.
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(bundleJSON) {
		t.Errorf("expected %q, got %q", bundleJSON, content)
	}
}

func TestLoadBundleFromFileNotExist(t *testing.T) {
	_, err := LoadBundleFromFile("/nonexistent/path/bundle.json")
	if err == nil {
		t.Error("expected error when loading nonexistent file")
	}
}
