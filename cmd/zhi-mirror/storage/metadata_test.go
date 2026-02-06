package storage

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMetadataStorePutAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMetadataStore(dir)
	if err != nil {
		t.Fatalf("NewMetadataStore: %v", err)
	}

	data := json.RawMessage(`{"total":5,"results":[]}`)
	if err := store.Put("search:q=ansible", data, "https://marketplace.example.com/api/v1/search?q=ansible"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get("search:q=ansible", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected cached response, got nil")
	}
	if string(got.Data) != `{"total":5,"results":[]}` {
		t.Errorf("data = %s", string(got.Data))
	}
	if got.SourceURL != "https://marketplace.example.com/api/v1/search?q=ansible" {
		t.Errorf("sourceURL = %q", got.SourceURL)
	}
}

func TestMetadataStoreExpiry(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMetadataStore(dir)
	if err != nil {
		t.Fatalf("NewMetadataStore: %v", err)
	}

	data := json.RawMessage(`{"test":true}`)
	if err := store.Put("key", data, ""); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Should be found with a long TTL.
	got, err := store.Get("key", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected cached response")
	}

	// Should not be found with zero TTL (immediate expiry).
	got, err = store.Get("key", 0)
	if err != nil {
		t.Fatalf("Get with 0 TTL: %v", err)
	}
	// maxAge=0 means no expiry check (unlimited TTL), so it should still return the entry.
	if got == nil {
		t.Fatal("expected cached response with 0 TTL (unlimited)")
	}
}

func TestMetadataStoreNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMetadataStore(dir)
	if err != nil {
		t.Fatalf("NewMetadataStore: %v", err)
	}

	got, err := store.Get("nonexistent", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing key")
	}
}

func TestMetadataStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMetadataStore(dir)
	if err != nil {
		t.Fatalf("NewMetadataStore: %v", err)
	}

	data := json.RawMessage(`{"delete":true}`)
	if err := store.Put("to-delete", data, ""); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.Get("to-delete", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestMetadataStoreDeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMetadataStore(dir)
	if err != nil {
		t.Fatalf("NewMetadataStore: %v", err)
	}

	// Should not error on deleting non-existent key.
	if err := store.Delete("nonexistent"); err != nil {
		t.Errorf("Delete nonexistent: %v", err)
	}
}
