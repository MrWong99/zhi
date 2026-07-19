package airgap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MrWong99/zhi/cmd/zhi-mirror/storage"
)

func TestImportNoInput(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	_, err = Import(store, ImportOptions{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestImportBundle(t *testing.T) {
	// Create a test bundle.
	bundleDir := t.TempDir()
	blobsDir := filepath.Join(bundleDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a test blob.
	blobData := []byte(`{"schemaVersion":2}`)
	dgst := digestSHA256(blobData)
	hexStr := dgst[7:] // strip "sha256:"
	if err := os.WriteFile(filepath.Join(blobsDir, hexStr), blobData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write bundle manifest.
	manifest := BundleManifest{
		Version:   "1",
		CreatedAt: "2026-02-05T10:00:00Z",
		Artifacts: []BundleEntry{
			{Ref: "zhi-project/plugin:v1.0.0", Digest: dgst},
		},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "bundle.json"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create the tarball.
	tarPath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := createTarball(tarPath, bundleDir); err != nil {
		t.Fatalf("createTarball: %v", err)
	}

	// Import into a new store.
	storeDir := t.TempDir()
	store, err := storage.NewOCILayout(storeDir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	result, err := Import(store, ImportOptions{Input: tarPath})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(result.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(result.Artifacts))
	}
	if result.Artifacts[0].Ref != "zhi-project/plugin:v1.0.0" {
		t.Errorf("artifact ref = %q", result.Artifacts[0].Ref)
	}

	// Verify the blob was imported.
	if !store.HasBlob(dgst) {
		t.Error("expected blob to be imported")
	}

	// Verify the tag was created.
	gotDgst, ok := store.GetTag("zhi-project/plugin", "v1.0.0")
	if !ok {
		t.Fatal("expected tag to be created")
	}
	if gotDgst != dgst {
		t.Errorf("tag digest = %q, want %q", gotDgst, dgst)
	}
}

// TestImportRejectsCorruptBlob is an F5 regression: a blob whose filename
// (its declared digest) does not match its bytes must fail the import loudly
// instead of being stored under the recomputed digest.
func TestImportRejectsCorruptBlob(t *testing.T) {
	bundleDir := t.TempDir()
	blobsDir := filepath.Join(bundleDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Filename claims one digest; contents hash to a different one.
	realDgst := digestSHA256([]byte(`{"schemaVersion":2}`))
	corruptName := realDgst[7:] // strip "sha256:"
	if err := os.WriteFile(filepath.Join(blobsDir, corruptName), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := BundleManifest{
		Version:   "1",
		Artifacts: []BundleEntry{{Ref: "zhi-project/plugin:v1.0.0", Digest: realDgst}},
	}
	writeBundleMeta(t, bundleDir, manifest)

	tarPath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := createTarball(tarPath, bundleDir); err != nil {
		t.Fatalf("createTarball: %v", err)
	}

	store, err := storage.NewOCILayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}
	if _, err := Import(store, ImportOptions{Input: tarPath}); err == nil {
		t.Fatal("expected import to reject corrupt blob")
	}
}

// TestImportRejectsMissingReferencedDigest is an F5 regression: an artifact
// whose digest is absent from the bundle blobs must fail the import rather than
// creating a dangling tag.
func TestImportRejectsMissingReferencedDigest(t *testing.T) {
	bundleDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundleDir, "blobs", "sha256"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := BundleManifest{
		Version: "1",
		Artifacts: []BundleEntry{
			{Ref: "zhi-project/plugin:v1.0.0", Digest: "sha256:" + repeatHex(64)},
		},
	}
	writeBundleMeta(t, bundleDir, manifest)

	tarPath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := createTarball(tarPath, bundleDir); err != nil {
		t.Fatalf("createTarball: %v", err)
	}

	store, err := storage.NewOCILayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}
	if _, err := Import(store, ImportOptions{Input: tarPath}); err == nil {
		t.Fatal("expected import to reject a bundle referencing a missing digest")
	}
}

func writeBundleMeta(t *testing.T, bundleDir string, manifest BundleManifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "bundle.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repeatHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
