package launch

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// useMetadataDir temporarily points metadataDirFunc at dir for the duration
// of the test, restoring the original afterwards.
func useMetadataDir(t *testing.T, dir string) {
	t.Helper()
	orig := metadataDirFunc
	metadataDirFunc = func() string { return dir }
	t.Cleanup(func() { metadataDirFunc = orig })
}

// writeStorePluginBinary writes a fake plugin binary named "zhi-store-<name>"
// into a fresh temp dir and returns its path and content.
func writeStorePluginBinary(t *testing.T, name string) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	content := []byte("fake plugin binary for " + name)
	path := filepath.Join(dir, "zhi-store-"+name)
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	return path, content
}

func saveBinaryDigest(t *testing.T, metaDir, name, digest string) {
	t.Helper()
	if err := metadata.NewStore(metaDir).Save(&metadata.InstalledPlugin{
		Name:         name,
		Type:         "store",
		BinaryDigest: digest,
	}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
}

func TestAuditBinaryMatchingDigest(t *testing.T) {
	metaDir := t.TempDir()
	useMetadataDir(t, metaDir)

	path, _ := writeStorePluginBinary(t, "match")
	digest, err := verify.ComputeBinaryDigest(path)
	if err != nil {
		t.Fatalf("compute digest: %v", err)
	}
	saveBinaryDigest(t, metaDir, "match", digest)

	if err := AuditBinary(hclog.NewNullLogger(), path); err != nil {
		t.Fatalf("AuditBinary with matching digest returned error: %v", err)
	}
}

func TestAuditBinaryMismatchedDigest(t *testing.T) {
	metaDir := t.TempDir()
	useMetadataDir(t, metaDir)

	path, _ := writeStorePluginBinary(t, "tampered")
	// Store a digest for different content, then the on-disk binary differs.
	saveBinaryDigest(t, metaDir, "tampered", "sha256:"+
		"0000000000000000000000000000000000000000000000000000000000000000")

	if err := AuditBinary(hclog.NewNullLogger(), path); err == nil {
		t.Fatal("AuditBinary with mismatched digest should return an error")
	}
}

func TestAuditBinaryNoMetadata(t *testing.T) {
	metaDir := t.TempDir()
	useMetadataDir(t, metaDir)

	path, _ := writeStorePluginBinary(t, "nometa")
	if err := AuditBinary(hclog.NewNullLogger(), path); err != nil {
		t.Fatalf("AuditBinary without metadata should skip verification, got: %v", err)
	}
}

func TestAuditBinaryEmptyStoredDigest(t *testing.T) {
	metaDir := t.TempDir()
	useMetadataDir(t, metaDir)

	path, _ := writeStorePluginBinary(t, "emptydigest")
	saveBinaryDigest(t, metaDir, "emptydigest", "") // legacy metadata, no digest
	if err := AuditBinary(hclog.NewNullLogger(), path); err != nil {
		t.Fatalf("AuditBinary with empty stored digest should skip, got: %v", err)
	}
}

func TestStoredBinaryDigest(t *testing.T) {
	metaDir := t.TempDir()
	useMetadataDir(t, metaDir)

	path, content := writeStorePluginBinary(t, "digestbytes")
	digest, err := verify.ComputeBinaryDigest(path)
	if err != nil {
		t.Fatalf("compute digest: %v", err)
	}
	saveBinaryDigest(t, metaDir, "digestbytes", digest)

	got, err := storedBinaryDigest(path)
	if err != nil {
		t.Fatalf("storedBinaryDigest: %v", err)
	}
	want := sha256.Sum256(content)
	if !bytes.Equal(got, want[:]) {
		t.Fatalf("storedBinaryDigest = %x, want %x", got, want[:])
	}
}

func TestStoredBinaryDigestNoMetadata(t *testing.T) {
	metaDir := t.TempDir()
	useMetadataDir(t, metaDir)

	path, _ := writeStorePluginBinary(t, "nodigest")
	got, err := storedBinaryDigest(path)
	if err != nil {
		t.Fatalf("storedBinaryDigest: %v", err)
	}
	if got != nil {
		t.Fatalf("storedBinaryDigest without metadata = %x, want nil", got)
	}
}

func TestLaunchClientHardFailOnTamper(t *testing.T) {
	metaDir := t.TempDir()
	useMetadataDir(t, metaDir)

	path, _ := writeStorePluginBinary(t, "hardfail")
	saveBinaryDigest(t, metaDir, "hardfail", "sha256:"+
		"1111111111111111111111111111111111111111111111111111111111111111")

	// Default mode is AuditHardFail: launch must be refused.
	o := applyOptions(nil)
	if _, err := launchClient(path, store.PluginMap, o); err == nil {
		t.Fatal("launchClient should fail hard on digest mismatch")
	}

	// AuditWarnOnly: launch proceeds (client constructed, not started).
	o = applyOptions([]Option{WithAuditMode(AuditWarnOnly)})
	client, err := launchClient(path, store.PluginMap, o)
	if err != nil {
		t.Fatalf("launchClient with AuditWarnOnly should proceed, got: %v", err)
	}
	if client == nil {
		t.Fatal("launchClient returned nil client in warn-only mode")
	}
	client.Kill()
}
