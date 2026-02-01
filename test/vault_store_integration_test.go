// Package test contains integration tests that require external services.
//
// The Vault store integration test requires a running Vault server in dev
// mode. Start one with:
//
//	vault server -dev -dev-root-token-id=test-root-token
//
// Then run:
//
//	VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=test-root-token go test -v -count=1 ./test/...
//
// Tests are skipped if VAULT_ADDR is not set.
package test

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"

	vaultstore "github.com/MrWong99/zhi/pkg/providers/store/vault"
)

func skipIfNoVault(t *testing.T) {
	t.Helper()
	if os.Getenv("VAULT_ADDR") == "" {
		t.Skip("VAULT_ADDR not set; skipping Vault integration test")
	}
}

// ensureKVv2 makes sure the KV v2 engine is enabled at the given mount.
func ensureKVv2(t *testing.T, mount string) {
	t.Helper()
	cfg := vaultapi.DefaultConfig()
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		t.Fatalf("vault client: %v", err)
	}

	// Check if the mount already exists.
	mounts, err := client.Sys().ListMounts()
	if err != nil {
		t.Fatalf("listing mounts: %v", err)
	}
	if _, ok := mounts[mount+"/"]; ok {
		return
	}

	// Enable it.
	err = client.Sys().Mount(mount, &vaultapi.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	})
	if err != nil {
		t.Fatalf("enabling KV v2 at %q: %v", mount, err)
	}
}

func newTestStore(t *testing.T, prefix string) *vaultstore.Store {
	t.Helper()
	skipIfNoVault(t)

	mount := "secret"
	ensureKVv2(t, mount)

	s, err := vaultstore.New(vaultstore.Options{
		Mount:  mount,
		Prefix: prefix,
	})
	if err != nil {
		t.Fatalf("vaultstore.New: %v", err)
	}
	return s
}

func TestVaultSaveAndLoad(t *testing.T) {
	s := newTestStore(t, "zhi-test-saveload/")
	ctx := context.Background()
	t.Cleanup(func() { _ = s.Delete(ctx, "basic") })

	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{
		Val:      "localhost",
		Metadata: map[string]any{"desc": "Database host"},
	})
	_ = tree.Set("db/port", &config.Value{Val: float64(5432)})

	if err := s.Save(ctx, "basic", tree); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, found, err := s.Load(ctx, "basic")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Fatal("Load: not found")
	}

	v, ok := loaded.Get("db/host")
	if !ok {
		t.Fatal("db/host missing from loaded tree")
	}
	if v.Val != "localhost" {
		t.Errorf("db/host = %v, want %q", v.Val, "localhost")
	}
	if v.Metadata["desc"] != "Database host" {
		t.Errorf("metadata[desc] = %v, want %q", v.Metadata["desc"], "Database host")
	}

	v, ok = loaded.Get("db/port")
	if !ok {
		t.Fatal("db/port missing from loaded tree")
	}
	if v.Val != float64(5432) {
		t.Errorf("db/port = %v, want 5432", v.Val)
	}
}

func TestVaultLoadMissing(t *testing.T) {
	s := newTestStore(t, "zhi-test-missing/")
	ctx := context.Background()

	_, found, err := s.Load(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Error("Load returned found=true for missing tree")
	}
}

func TestVaultDelete(t *testing.T) {
	s := newTestStore(t, "zhi-test-delete/")
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("a/b", &config.Value{Val: "x"})
	_ = s.Save(ctx, "temp", tree)

	if err := s.Delete(ctx, "temp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, found, _ := s.Load(ctx, "temp")
	if found {
		t.Error("tree still found after Delete")
	}
}

func TestVaultListTrees(t *testing.T) {
	s := newTestStore(t, "zhi-test-list/")
	ctx := context.Background()
	t.Cleanup(func() {
		_ = s.Delete(ctx, "alpha")
		_ = s.Delete(ctx, "beta")
	})

	tree := config.NewTree()
	_ = tree.Set("a/b", &config.Value{Val: "x"})

	_ = s.Save(ctx, "alpha", tree)
	_ = s.Save(ctx, "beta", tree)

	ids, err := s.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	slices.Sort(ids)
	want := []string{"alpha", "beta"}
	if !slices.Equal(ids, want) {
		t.Errorf("ListTrees() = %v, want %v", ids, want)
	}
}

func TestVaultSupportsVersioning(t *testing.T) {
	s := newTestStore(t, "zhi-test-ver/")
	supported, err := s.SupportsVersioning(context.Background())
	if err != nil {
		t.Fatalf("SupportsVersioning: %v", err)
	}
	if !supported {
		t.Error("SupportsVersioning() = false, want true")
	}
}

func TestVaultVersioning(t *testing.T) {
	s := newTestStore(t, "zhi-test-versioning/")
	ctx := context.Background()
	t.Cleanup(func() { _ = s.Delete(ctx, "myapp") })

	// Save two versions.
	tree1 := config.NewTree()
	_ = tree1.Set("app/version", &config.Value{Val: "1.0"})
	if err := s.Save(ctx, "myapp", tree1); err != nil {
		t.Fatalf("Save v1: %v", err)
	}

	tree2 := config.NewTree()
	_ = tree2.Set("app/version", &config.Value{Val: "2.0"})
	if err := s.Save(ctx, "myapp", tree2); err != nil {
		t.Fatalf("Save v2: %v", err)
	}

	// Latest should be v2.
	latest, found, err := s.Load(ctx, "myapp")
	if err != nil || !found {
		t.Fatalf("Load latest: err=%v, found=%v", err, found)
	}
	v, _ := latest.Get("app/version")
	if v.Val != "2.0" {
		t.Errorf("latest = %v, want %q", v.Val, "2.0")
	}

	// List versions.
	versions, err := s.ListVersions(ctx, "myapp")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	// Newest first: version 2 before version 1.
	v2, _ := strconv.Atoi(versions[0])
	v1, _ := strconv.Atoi(versions[1])
	if v2 <= v1 {
		t.Errorf("versions not newest first: %v", versions)
	}

	// Load older version.
	old, found, err := s.LoadVersion(ctx, "myapp", versions[1])
	if err != nil || !found {
		t.Fatalf("LoadVersion v1: err=%v, found=%v", err, found)
	}
	v, _ = old.Get("app/version")
	if v.Val != "1.0" {
		t.Errorf("v1 = %v, want %q", v.Val, "1.0")
	}
}

func TestVaultDeleteVersion(t *testing.T) {
	s := newTestStore(t, "zhi-test-delver/")
	ctx := context.Background()
	t.Cleanup(func() { _ = s.Delete(ctx, "myapp") })

	// Save three versions.
	for i := 1; i <= 3; i++ {
		tree := config.NewTree()
		_ = tree.Set("app/version", &config.Value{Val: strconv.Itoa(i) + ".0"})
		_ = s.Save(ctx, "myapp", tree)
	}

	// List versions, delete the middle one.
	versions, err := s.ListVersions(ctx, "myapp")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("got %d versions, want 3", len(versions))
	}

	// Delete the middle version (second in the newest-first list).
	midVersion := versions[1]
	if err := s.DeleteVersion(ctx, "myapp", midVersion); err != nil {
		t.Fatalf("DeleteVersion: %v", err)
	}

	// Should have 2 versions remaining.
	remaining, err := s.ListVersions(ctx, "myapp")
	if err != nil {
		t.Fatalf("ListVersions after delete: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("got %d versions after delete, want 2", len(remaining))
	}

	// Latest should still be the last saved version.
	latest, found, _ := s.Load(ctx, "myapp")
	if !found {
		t.Fatal("tree not found after DeleteVersion")
	}
	v, _ := latest.Get("app/version")
	if v.Val != "3.0" {
		t.Errorf("latest = %v, want %q", v.Val, "3.0")
	}
}

func TestVaultEncryptionStatus(t *testing.T) {
	s := newTestStore(t, "zhi-test-enc/")

	status, err := s.EncryptionStatus(context.Background())
	if err != nil {
		t.Fatalf("EncryptionStatus: %v", err)
	}
	if status != store.EncryptionActive {
		t.Errorf("EncryptionStatus() = %v, want EncryptionActive", status)
	}
}

func TestVaultInitEncryptionReturnsError(t *testing.T) {
	s := newTestStore(t, "zhi-test-enc2/")

	err := s.InitEncryption(context.Background(), []byte("passphrase"))
	if err == nil {
		t.Error("InitEncryption should return error (Vault manages encryption)")
	}
}

func TestVaultRotateEncryptionReturnsError(t *testing.T) {
	s := newTestStore(t, "zhi-test-enc3/")

	err := s.RotateEncryption(context.Background(), []byte("old"), []byte("new"))
	if err == nil {
		t.Error("RotateEncryption should return error (Vault manages encryption)")
	}
}

func TestVaultMetadataPreserved(t *testing.T) {
	s := newTestStore(t, "zhi-test-meta/")
	ctx := context.Background()
	t.Cleanup(func() { _ = s.Delete(ctx, "meta") })

	tree := config.NewTree()
	_ = tree.Set("db/host", &config.Value{
		Val:      "localhost",
		Metadata: map[string]any{"description": "Database host", "required": true},
	})

	_ = s.Save(ctx, "meta", tree)

	loaded, found, _ := s.Load(ctx, "meta")
	if !found {
		t.Fatal("not found")
	}

	v, ok := loaded.Get("db/host")
	if !ok {
		t.Fatal("db/host missing")
	}
	if v.Metadata["description"] != "Database host" {
		t.Errorf("description = %v, want %q", v.Metadata["description"], "Database host")
	}
	if v.Metadata["required"] != true {
		t.Errorf("required = %v, want true", v.Metadata["required"])
	}
}

func TestVaultValidatorsNotStored(t *testing.T) {
	s := newTestStore(t, "zhi-test-validators/")
	ctx := context.Background()
	t.Cleanup(func() { _ = s.Delete(ctx, "validators") })

	tree := config.NewTree()
	_ = tree.Set("key", &config.Value{
		Val: "value",
		Validators: []config.ValidateFunc{
			func(any, config.TreeReader) []config.ValidationResult {
				return []config.ValidationResult{{Severity: config.Blocking, Message: "fail"}}
			},
		},
	})

	_ = s.Save(ctx, "validators", tree)

	loaded, found, _ := s.Load(ctx, "validators")
	if !found {
		t.Fatal("not found")
	}

	v, ok := loaded.Get("key")
	if !ok {
		t.Fatal("key missing")
	}
	if len(v.Validators) != 0 {
		t.Errorf("got %d validators, want 0", len(v.Validators))
	}
}
