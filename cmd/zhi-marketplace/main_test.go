package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAPIKeys(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"single", "key1=pub1", map[string]string{"key1": "pub1"}},
		{"multiple", "key1=pub1,key2=pub2", map[string]string{"key1": "pub1", "key2": "pub2"}},
		{"spaces", " key1=pub1 , key2=pub2 ", map[string]string{"key1": "pub1", "key2": "pub2"}},
		{"no equals", "invalid", map[string]string{}},
		{"mixed", "key1=pub1,invalid,key2=pub2", map[string]string{"key1": "pub1", "key2": "pub2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAPIKeys(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d keys, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestReadAPIKeysFile(t *testing.T) {
	t.Run("reads and deletes file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "keys.txt")
		content := "key1=pub1\nkey2=pub2\n"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		keys, err := readAPIKeysFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) != 2 {
			t.Fatalf("got %d keys, want 2", len(keys))
		}
		if keys["key1"] != "pub1" {
			t.Errorf("key1: got %q, want %q", keys["key1"], "pub1")
		}
		if keys["key2"] != "pub2" {
			t.Errorf("key2: got %q, want %q", keys["key2"], "pub2")
		}

		// File must be deleted.
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("file was not deleted after reading")
		}
	})

	t.Run("skips comments and blank lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "keys.txt")
		content := "# This is a comment\n\nkey1=pub1\n  # another comment\n\nkey2=pub2\n"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		keys, err := readAPIKeysFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) != 2 {
			t.Fatalf("got %d keys, want 2", len(keys))
		}
	})

	t.Run("skips malformed lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "keys.txt")
		content := "key1=pub1\ninvalid-line\nkey2=pub2\n"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		keys, err := readAPIKeysFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) != 2 {
			t.Fatalf("got %d keys, want 2", len(keys))
		}
	})

	t.Run("error on missing file", func(t *testing.T) {
		_, err := readAPIKeysFile("/nonexistent/path/keys.txt")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}
