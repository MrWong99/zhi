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

func TestSecretPath(t *testing.T) {
	s := &Store{prefix: "zhi/"}
	if got := s.secretPath("production"); got != "zhi/production" {
		t.Errorf("secretPath = %q, want %q", got, "zhi/production")
	}
}

func TestDataToTree(t *testing.T) {
	data := map[string]any{
		"db/host": `{"value":"localhost","metadata":{"desc":"host"}}`,
		"db/port": `{"value":5432}`,
	}
	tree, err := dataToTree(data)
	if err != nil {
		t.Fatalf("dataToTree: %v", err)
	}

	v, ok := tree.Get("db/host")
	if !ok {
		t.Fatal("db/host missing")
	}
	if v.Val != "localhost" {
		t.Errorf("db/host = %v, want %q", v.Val, "localhost")
	}
	if v.Metadata["desc"] != "host" {
		t.Errorf("metadata[desc] = %v, want %q", v.Metadata["desc"], "host")
	}

	v, ok = tree.Get("db/port")
	if !ok {
		t.Fatal("db/port missing")
	}
	// JSON unmarshals numbers as float64.
	if v.Val != float64(5432) {
		t.Errorf("db/port = %v, want 5432", v.Val)
	}
}

func TestDataToTreeInvalidJSON(t *testing.T) {
	data := map[string]any{
		"bad": `{not json`,
	}
	_, err := dataToTree(data)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDataToTreeBadType(t *testing.T) {
	data := map[string]any{
		"bad": 42, // not a string
	}
	_, err := dataToTree(data)
	if err == nil {
		t.Error("expected error for non-string value")
	}
}

func TestIsNotFound(t *testing.T) {
	if isNotFound(nil) {
		t.Error("nil should not be not-found")
	}
}
