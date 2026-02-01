package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// newTestStore returns a jsonStore that writes to a temporary directory
// cleaned up automatically when the test finishes.
func newTestStore(t *testing.T) *jsonStore {
	t.Helper()
	dir := t.TempDir()
	return &jsonStore{dir: dir}
}

// dispense starts an in-process gRPC plugin server and returns a connected
// store.Plugin client. No subprocess is spawned. The returned dir is the
// temporary directory used by the store so tests can inspect the files.
func dispense(t *testing.T) (store.Plugin, string) {
	t.Helper()
	s := newTestStore(t)
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"store": &store.GRPCPlugin{Impl: s},
	})
	raw, err := client.Dispense("store")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(store.Plugin), s.dir
}

func TestPutAndGetValues(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	values := map[string]config.Value{
		"db/host": {Val: "localhost"},
		"db/port": {Val: float64(5432)},
	}

	if err := p.PutValues(ctx, "prod", values, nil); err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := p.GetValues(ctx, "prod", []string{"db/host", "db/port"})
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
	p, _ := dispense(t)
	got, err := p.GetValues(context.Background(), "nope", []string{"a/b"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestGetValuesFilters(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	values := map[string]config.Value{
		"a/b": {Val: "1"},
		"a/c": {Val: "2"},
		"a/d": {Val: "3"},
	}
	_ = p.PutValues(ctx, "t", values, nil)

	got, err := p.GetValues(ctx, "t", []string{"a/b", "a/d"})
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
	p, _ := dispense(t)
	ctx := context.Background()

	_ = p.PutValues(ctx, "temp", map[string]config.Value{
		"a/b": {Val: "x"},
	}, nil)

	if err := p.DeleteTree(ctx, "temp"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	got, err := p.GetValues(ctx, "temp", []string{"a/b"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Error("values still found after DeleteTree")
	}
}

func TestDeleteValues(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	_ = p.PutValues(ctx, "t", map[string]config.Value{
		"a/b": {Val: "1"},
		"a/c": {Val: "2"},
	}, nil)

	if err := p.DeleteValues(ctx, "t", []string{"a/b"}); err != nil {
		t.Fatalf("DeleteValues: %v", err)
	}

	got, err := p.GetValues(ctx, "t", []string{"a/b", "a/c"})
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
	p, _ := dispense(t)
	ctx := context.Background()

	_ = p.PutValues(ctx, "one", map[string]config.Value{"a/b": {Val: "x"}}, nil)
	_ = p.PutValues(ctx, "two", map[string]config.Value{"a/b": {Val: "x"}}, nil)

	ids, err := p.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"one", "two"}) {
		t.Errorf("ListTrees() = %v, want [one, two]", ids)
	}
}

func TestPutValuesMerges(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "old"},
	}, nil)

	_ = p.PutValues(ctx, "app", map[string]config.Value{
		"app/name": {Val: "new"},
	}, nil)

	got, err := p.GetValues(ctx, "app", []string{"app/name"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if got["app/name"].Val != "new" {
		t.Errorf("Val = %v, want %q", got["app/name"].Val, "new")
	}
}

func TestJSONFileCreatedOnDisk(t *testing.T) {
	p, dir := dispense(t)
	ctx := context.Background()

	_ = p.PutValues(ctx, "disk-check", map[string]config.Value{
		"key": {Val: "value"},
	}, nil)

	path := filepath.Join(dir, "disk-check.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if len(data) == 0 {
		t.Error("JSON file is empty")
	}
}

func TestMetadataPreserved(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	_ = p.PutValues(ctx, "meta", map[string]config.Value{
		"key": {
			Val:      "value",
			Metadata: map[string]any{"description": "test key"},
		},
	}, nil)

	got, err := p.GetValues(ctx, "meta", []string{"key"})
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
	p, _ := dispense(t)
	ctx := context.Background()

	caps, err := p.Capabilities(ctx)
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
	p, _ := dispense(t)
	ctx := context.Background()

	_, err := p.ListTreeVersions(ctx, "any")
	if err == nil {
		t.Error("ListTreeVersions should return error")
	}

	_, err = p.GetTreeVersion(ctx, "any", "v1", []string{"a/b"})
	if err == nil {
		t.Error("GetTreeVersion should return error")
	}

	if err := p.RollbackTree(ctx, "any", "v1"); err == nil {
		t.Error("RollbackTree should return error")
	}

	if err := p.DeleteTreeVersion(ctx, "any", "v1"); err == nil {
		t.Error("DeleteTreeVersion should return error")
	}

	_, err = p.ListValueVersions(ctx, "any", "a/b")
	if err == nil {
		t.Error("ListValueVersions should return error")
	}

	_, _, err = p.GetValueVersion(ctx, "any", "a/b", "v1")
	if err == nil {
		t.Error("GetValueVersion should return error")
	}

	if err := p.RollbackValue(ctx, "any", "a/b", "v1"); err == nil {
		t.Error("RollbackValue should return error")
	}

	if err := p.DeleteValueVersion(ctx, "any", "a/b", "v1"); err == nil {
		t.Error("DeleteValueVersion should return error")
	}
}

func TestEncryptionNotSupported(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	if err := p.InitEncryption(ctx, []byte("pass")); err == nil {
		t.Error("InitEncryption should return error")
	}

	if err := p.RotateEncryption(ctx, []byte("old"), []byte("new")); err == nil {
		t.Error("RotateEncryption should return error")
	}
}

func TestAccessControlNotSupported(t *testing.T) {
	p, _ := dispense(t)
	ctx := context.Background()

	if err := p.GrantAccess(ctx, "t", "alice", []store.Permission{{Path: "a/b", Actions: []store.Action{store.ActionRead}}}); err == nil {
		t.Error("GrantAccess should return error")
	}

	if err := p.RevokeAccess(ctx, "t", "alice", []string{"a/b"}); err == nil {
		t.Error("RevokeAccess should return error")
	}

	_, err := p.ListAccess(ctx, "t")
	if err == nil {
		t.Error("ListAccess should return error")
	}
}
