package lockfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	data := []byte(`
version: 1
generatedAt: "2026-02-05T10:00:00Z"
zhiVersion: "0.8.0"
plugins:
  - name: ansible-config
    type: config
    ref: oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
    digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    platforms:
      linux/amd64: sha256:abc123
      darwin/arm64: sha256:def456
    signed: true
    signingIdentity: release@zhi.dev
  - name: vault
    type: store
    ref: oci://ghcr.io/zhi-project/zhi-store-vault:v2.0.0
    digest: sha256:789xyz
    signed: false
`)
	lf, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}
	if lf.ZhiVersion != "0.8.0" {
		t.Errorf("ZhiVersion = %q", lf.ZhiVersion)
	}
	if len(lf.Plugins) != 2 {
		t.Fatalf("Plugins = %d, want 2", len(lf.Plugins))
	}
	if lf.Plugins[0].Name != "ansible-config" {
		t.Errorf("Plugins[0].Name = %q", lf.Plugins[0].Name)
	}
	if lf.Plugins[0].Digest != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Plugins[0].Digest = %q", lf.Plugins[0].Digest)
	}
	if len(lf.Plugins[0].Platforms) != 2 {
		t.Errorf("Plugins[0].Platforms = %d, want 2", len(lf.Plugins[0].Platforms))
	}
	if !lf.Plugins[0].Signed {
		t.Error("Plugins[0].Signed = false, want true")
	}
	if lf.Plugins[0].SigningIdentity != "release@zhi.dev" {
		t.Errorf("Plugins[0].SigningIdentity = %q", lf.Plugins[0].SigningIdentity)
	}
}

func TestParseMissingVersion(t *testing.T) {
	data := []byte(`
plugins:
  - name: test
    ref: oci://example.com/test:v1.0
    digest: sha256:abc
`)
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for missing version")
	}
}

func TestParseUnsupportedVersion(t *testing.T) {
	data := []byte(`
version: 99
plugins:
  - name: test
    ref: oci://example.com/test:v1.0
    digest: sha256:abc
`)
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestParseMissingPluginName(t *testing.T) {
	data := []byte(`
version: 1
plugins:
  - ref: oci://example.com/test:v1.0
    digest: sha256:abc
`)
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for missing plugin name")
	}
}

func TestParseMissingPluginRef(t *testing.T) {
	data := []byte(`
version: 1
plugins:
  - name: test
    digest: sha256:abc
`)
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for missing plugin ref")
	}
}

func TestParseMissingPluginDigest(t *testing.T) {
	data := []byte(`
version: 1
plugins:
  - name: test
    ref: oci://example.com/test:v1.0
`)
	_, err := Parse(data)
	if err == nil {
		t.Error("expected error for missing plugin digest")
	}
}

func TestFindPlugin(t *testing.T) {
	lf := &LockFile{
		Version: 1,
		Plugins: []LockedPlugin{
			{Name: "alpha", Ref: "oci://example.com/alpha:v1.0", Digest: "sha256:aaa"},
			{Name: "beta", Ref: "oci://example.com/beta:v2.0", Digest: "sha256:bbb"},
		},
	}

	found := lf.FindPlugin("alpha")
	if found == nil {
		t.Fatal("FindPlugin(alpha) returned nil")
	}
	if found.Digest != "sha256:aaa" {
		t.Errorf("Digest = %q", found.Digest)
	}

	found = lf.FindPlugin("gamma")
	if found != nil {
		t.Error("FindPlugin(gamma) should return nil")
	}
}

func TestVerifyDigest(t *testing.T) {
	lp := &LockedPlugin{
		Name:   "test",
		Digest: "sha256:abc123",
	}

	if err := lp.VerifyDigest("sha256:abc123"); err != nil {
		t.Errorf("VerifyDigest matching: %v", err)
	}

	if err := lp.VerifyDigest("sha256:wrong"); err == nil {
		t.Error("VerifyDigest mismatched: expected error")
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	original := &LockFile{
		Version:     1,
		GeneratedAt: time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC),
		ZhiVersion:  "0.8.0",
		Plugins: []LockedPlugin{
			{
				Name:   "test-plugin",
				Type:   "config",
				Ref:    "oci://example.com/test:v1.0",
				Digest: "sha256:abc123",
				Platforms: map[string]string{
					"linux/amd64": "sha256:plat1",
				},
				Signed: true,
			},
		},
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if parsed.Version != original.Version {
		t.Errorf("Version = %d, want %d", parsed.Version, original.Version)
	}
	if len(parsed.Plugins) != 1 {
		t.Fatalf("Plugins = %d, want 1", len(parsed.Plugins))
	}
	if parsed.Plugins[0].Name != "test-plugin" {
		t.Errorf("Name = %q", parsed.Plugins[0].Name)
	}
	if parsed.Plugins[0].Digest != "sha256:abc123" {
		t.Errorf("Digest = %q", parsed.Plugins[0].Digest)
	}
}

func TestSaveAndLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zhi-plugins.lock")

	lf := &LockFile{
		Version:     1,
		GeneratedAt: time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC),
		ZhiVersion:  "0.8.0",
		Plugins: []LockedPlugin{
			{
				Name:   "test",
				Type:   "config",
				Ref:    "oci://example.com/test:v1.0",
				Digest: "sha256:abc",
			},
		},
	}

	if err := lf.SaveFile(path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	// Verify file has the header comment.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("file is empty")
	}

	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("Version = %d", loaded.Version)
	}
	if len(loaded.Plugins) != 1 {
		t.Errorf("Plugins = %d", len(loaded.Plugins))
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/zhi-plugins.lock")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := Parse([]byte("not: valid: yaml: {{"))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
