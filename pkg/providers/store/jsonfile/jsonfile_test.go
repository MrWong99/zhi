package jsonfile

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestNew_EmptyDir(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Error("expected error for empty dir")
	}
}

func TestPutAndGetValues(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	values := map[string]config.Value{
		"db/host": {Val: "localhost"},
		"db/port": {Val: float64(5432)},
	}

	if err := s.PutValues(ctx, "prod", values, nil); err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := s.GetValues(ctx, "prod", []string{"db/host", "db/port"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}

	if got["db/host"].Val != "localhost" {
		t.Errorf("db/host = %v, want %q", got["db/host"].Val, "localhost")
	}
	if got["db/port"].Val != float64(5432) {
		t.Errorf("db/port = %v, want %v", got["db/port"].Val, float64(5432))
	}
}

func TestGetValuesMissing(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetValues(context.Background(), "nope", []string{"a/b"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestGetValuesFilters(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	values := map[string]config.Value{
		"a/b": {Val: "1"},
		"a/c": {Val: "2"},
		"a/d": {Val: "3"},
	}
	_ = s.PutValues(ctx, "t", values, nil)

	got, err := s.GetValues(ctx, "t", []string{"a/b", "a/d"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %d", len(got))
	}
	if got["a/b"].Val != "1" {
		t.Errorf("a/b = %v, want %q", got["a/b"].Val, "1")
	}
	if got["a/d"].Val != "3" {
		t.Errorf("a/d = %v, want %q", got["a/d"].Val, "3")
	}
}

func TestDeleteTree(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "temp", map[string]config.Value{
		"a/b": {Val: "x"},
	}, nil)

	if err := s.DeleteTree(ctx, "temp"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	got, err := s.GetValues(ctx, "temp", []string{"a/b"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Error("values still found after DeleteTree")
	}
}

func TestDeleteValues(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "t", map[string]config.Value{
		"a/b": {Val: "1"},
		"a/c": {Val: "2"},
	}, nil)

	if err := s.DeleteValues(ctx, "t", []string{"a/b"}); err != nil {
		t.Fatalf("DeleteValues: %v", err)
	}

	got, err := s.GetValues(ctx, "t", []string{"a/b", "a/c"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if _, exists := got["a/b"]; exists {
		t.Error("a/b still found after DeleteValues")
	}
	if got["a/c"].Val != "2" {
		t.Errorf("a/c = %v, want %q", got["a/c"].Val, "2")
	}
}

func TestListTrees(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "one", map[string]config.Value{"a/b": {Val: "x"}}, nil)
	_ = s.PutValues(ctx, "two", map[string]config.Value{"a/b": {Val: "x"}}, nil)

	ids, err := s.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"one", "two"}) {
		t.Errorf("ListTrees() = %v, want [one, two]", ids)
	}
}

func TestPutValuesMerges(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "old"},
	}, nil)

	_ = s.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "new"},
	}, nil)

	got, err := s.GetValues(ctx, "app", []string{"app/name"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if got["app/name"].Val != "new" {
		t.Errorf("Val = %v, want %q", got["app/name"].Val, "new")
	}
}

func TestJSONFileCreatedOnDisk(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "disk-check", map[string]config.Value{
		"key": {Val: "value"},
	}, nil)

	path := filepath.Join(s.dir, "disk-check.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if len(data) == 0 {
		t.Error("JSON file is empty")
	}
}

func TestMetadataPreserved(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "meta", map[string]config.Value{
		"key": {
			Val:      "value",
			Metadata: map[string]any{"description": "test key"},
		},
	}, nil)

	got, err := s.GetValues(ctx, "meta", []string{"key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	v, ok := got["key"]
	if !ok {
		t.Fatal("key missing")
	}
	if v.Metadata["description"] != "test key" {
		t.Errorf("description = %v, want %q", v.Metadata["description"], "test key")
	}
}

func TestCapabilities(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	caps, err := s.Capabilities(ctx)
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.Versioning != store.VersioningNone {
		t.Errorf("Versioning = %v, want VersioningNone", caps.Versioning)
	}
	if caps.Encryption != store.EncryptionNone {
		t.Errorf("Encryption = %v, want EncryptionNone", caps.Encryption)
	}
	if caps.Auth {
		t.Error("Auth should be false")
	}
	if caps.AccessControl {
		t.Error("AccessControl should be false")
	}
}

func TestVersioningNotSupported(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.ListTreeVersions(ctx, "any"); !store.IsNotSupported(err) {
		t.Errorf("ListTreeVersions: expected ErrNotSupported, got %v", err)
	}
	if _, err := s.GetTreeVersion(ctx, "any", "v1", []string{"a/b"}); !store.IsNotSupported(err) {
		t.Errorf("GetTreeVersion: expected ErrNotSupported, got %v", err)
	}
	if err := s.RollbackTree(ctx, "any", "v1"); !store.IsNotSupported(err) {
		t.Errorf("RollbackTree: expected ErrNotSupported, got %v", err)
	}
	if err := s.DeleteTreeVersion(ctx, "any", "v1"); !store.IsNotSupported(err) {
		t.Errorf("DeleteTreeVersion: expected ErrNotSupported, got %v", err)
	}
	if _, err := s.ListValueVersions(ctx, "any", "a/b"); !store.IsNotSupported(err) {
		t.Errorf("ListValueVersions: expected ErrNotSupported, got %v", err)
	}
	if _, _, err := s.GetValueVersion(ctx, "any", "a/b", "v1"); !store.IsNotSupported(err) {
		t.Errorf("GetValueVersion: expected ErrNotSupported, got %v", err)
	}
	if err := s.RollbackValue(ctx, "any", "a/b", "v1"); !store.IsNotSupported(err) {
		t.Errorf("RollbackValue: expected ErrNotSupported, got %v", err)
	}
	if err := s.DeleteValueVersion(ctx, "any", "a/b", "v1"); !store.IsNotSupported(err) {
		t.Errorf("DeleteValueVersion: expected ErrNotSupported, got %v", err)
	}
}
