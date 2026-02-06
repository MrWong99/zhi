package storage

import (
	"strings"
	"testing"
)

func TestNewOCILayout(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.Root() != dir {
		t.Errorf("Root() = %q, want %q", store.Root(), dir)
	}
}

func TestPutAndGetBlob(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	data := []byte("hello world")
	dgst, err := store.PutBlob(data)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}

	if !strings.HasPrefix(dgst, "sha256:") {
		t.Errorf("digest should start with sha256:, got %q", dgst)
	}

	// Verify blob exists.
	if !store.HasBlob(dgst) {
		t.Error("HasBlob should return true after PutBlob")
	}

	// Get the blob.
	got, ok, err := store.GetBlob(dgst)
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	if !ok {
		t.Fatal("GetBlob should return true")
	}
	if string(got) != "hello world" {
		t.Errorf("GetBlob data = %q, want %q", string(got), "hello world")
	}
}

func TestPutBlobIdempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	data := []byte("test data")
	dgst1, err := store.PutBlob(data)
	if err != nil {
		t.Fatalf("first PutBlob: %v", err)
	}

	dgst2, err := store.PutBlob(data)
	if err != nil {
		t.Fatalf("second PutBlob: %v", err)
	}

	if dgst1 != dgst2 {
		t.Errorf("digests differ: %q vs %q", dgst1, dgst2)
	}
}

func TestGetBlobNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	_, ok, err := store.GetBlob("sha256:doesnotexist")
	if err != nil {
		t.Fatalf("GetBlob error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for missing blob")
	}
}

func TestDeleteBlob(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	data := []byte("delete me")
	dgst, err := store.PutBlob(data)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}

	if err := store.DeleteBlob(dgst); err != nil {
		t.Fatalf("DeleteBlob: %v", err)
	}

	if store.HasBlob(dgst) {
		t.Error("HasBlob should return false after delete")
	}
}

func TestPutAndGetTag(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	repo := "zhi-project/zhi-config-ansible"
	tag := "v1.2.0"
	dgst := "sha256:abc123"

	if err := store.PutTag(repo, tag, dgst); err != nil {
		t.Fatalf("PutTag: %v", err)
	}

	got, ok := store.GetTag(repo, tag)
	if !ok {
		t.Fatal("GetTag should return true")
	}
	if got != dgst {
		t.Errorf("GetTag = %q, want %q", got, dgst)
	}
}

func TestGetTagNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	_, ok := store.GetTag("nonexistent/repo", "v1.0.0")
	if ok {
		t.Error("expected ok=false for missing tag")
	}
}

func TestListTags(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	repo := "myorg/plugin"
	if err := store.PutTag(repo, "v1.0.0", "sha256:aaa"); err != nil {
		t.Fatal(err)
	}
	if err := store.PutTag(repo, "v2.0.0", "sha256:bbb"); err != nil {
		t.Fatal(err)
	}

	tags, err := store.ListTags(repo)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestListTagsEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	tags, err := store.ListTags("nonexistent/repo")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}
}

func TestPutBlobFromReader(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	data := "reader data"
	dgst, size, err := store.PutBlobFromReader(strings.NewReader(data))
	if err != nil {
		t.Fatalf("PutBlobFromReader: %v", err)
	}
	if size != int64(len(data)) {
		t.Errorf("size = %d, want %d", size, len(data))
	}
	if !strings.HasPrefix(dgst, "sha256:") {
		t.Errorf("digest should start with sha256:, got %q", dgst)
	}
}

func TestGetTagEntry(t *testing.T) {
	dir := t.TempDir()
	store, err := NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}

	if err := store.PutTag("repo/name", "v1", "sha256:test"); err != nil {
		t.Fatal(err)
	}

	entry, ok := store.GetTagEntry("repo/name", "v1")
	if !ok {
		t.Fatal("expected tag entry")
	}
	if entry.Digest != "sha256:test" {
		t.Errorf("digest = %q", entry.Digest)
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestComputeSHA256(t *testing.T) {
	dgst := ComputeSHA256([]byte("test"))
	if !strings.HasPrefix(dgst, "sha256:") {
		t.Errorf("expected sha256: prefix, got %q", dgst)
	}
	// Known SHA-256 of "test".
	want := "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if dgst != want {
		t.Errorf("ComputeSHA256(\"test\") = %q, want %q", dgst, want)
	}
}
