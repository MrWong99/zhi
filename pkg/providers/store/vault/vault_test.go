package vault

import (
	"testing"
)

func TestOptionsDefaults(t *testing.T) {
	// Verify that New applies sensible defaults.
	s, err := New(Options{
		Address: "http://127.0.0.1:8200",
		Token:   "test-token",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.mount != "secret" {
		t.Errorf("mount = %q, want %q", s.mount, "secret")
	}
	if s.prefix != "zhi/" {
		t.Errorf("prefix = %q, want %q", s.prefix, "zhi/")
	}
}

func TestOptionsPrefixTrailingSlash(t *testing.T) {
	s, err := New(Options{
		Address: "http://127.0.0.1:8200",
		Token:   "test-token",
		Prefix:  "myapp",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.prefix != "myapp/" {
		t.Errorf("prefix = %q, want %q", s.prefix, "myapp/")
	}
}

func TestValuePath(t *testing.T) {
	s := &Store{prefix: "zhi/"}
	got := s.valuePath("production", "db/host")
	want := "zhi/production/db/host"
	if got != want {
		t.Errorf("valuePath = %q, want %q", got, want)
	}
}

func TestTreePrefixPath(t *testing.T) {
	s := &Store{mount: "secret", prefix: "zhi/"}
	got := s.treePrefixPath("production")
	want := "secret/metadata/zhi/production/"
	if got != want {
		t.Errorf("treePrefixPath = %q, want %q", got, want)
	}
}

func TestIsNotFound(t *testing.T) {
	if isNotFound(nil) {
		t.Error("nil should not be not-found")
	}
}
