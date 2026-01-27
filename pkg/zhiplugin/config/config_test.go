package config

import (
	"fmt"
	"slices"
	"testing"
)

func TestValidatePath(t *testing.T) {
	valid := []string{
		"a",
		"abc",
		"a1",
		"my.value",
		"my-value",
		"my_value",
		"a/b",
		"something/abc/my.first.value",
		"db/host",
		"app/tls/cert.pem",
		"v2",
		"a/b/c/d/e",
	}
	for _, p := range valid {
		t.Run("valid/"+p, func(t *testing.T) {
			if err := ValidatePath(p); err != nil {
				t.Errorf("ValidatePath(%q) = %v, want nil", p, err)
			}
		})
	}

	invalid := []string{
		"",
		"/",
		"/a",
		"a/",
		"a//b",
		"A",
		"ABC",
		"a/B",
		"1abc",
		"abc-",
		"abc.",
		"abc_",
		"-abc",
		".abc",
		"_abc",
		"a b",
		"a/b c",
		"a/$",
	}
	for _, p := range invalid {
		t.Run("invalid/"+p, func(t *testing.T) {
			if err := ValidatePath(p); err == nil {
				t.Errorf("ValidatePath(%q) = nil, want error", p)
			}
		})
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{Info, "info"},
		{Warning, "warning"},
		{Blocking, "blocking"},
		{Severity(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestValueValidate(t *testing.T) {
	t.Run("no validators", func(t *testing.T) {
		v := Value{Val: "hello"}
		results := v.Validate(NewTree())
		if len(results) != 0 {
			t.Fatalf("got %d results, want 0", len(results))
		}
	})

	t.Run("single validator", func(t *testing.T) {
		nonEmpty := func(value any, _ TreeReader) []ValidationResult {
			if value == "" {
				return []ValidationResult{{Severity: Blocking, Message: "must not be empty"}}
			}
			return nil
		}
		v := Value{Val: "", Validators: []ValidateFunc{nonEmpty}}
		results := v.Validate(NewTree())
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Severity != Blocking {
			t.Errorf("severity = %v, want Blocking", results[0].Severity)
		}
	})

	t.Run("multiple validators aggregate", func(t *testing.T) {
		warn := func(any, TreeReader) []ValidationResult {
			return []ValidationResult{{Severity: Warning, Message: "w"}}
		}
		info := func(any, TreeReader) []ValidationResult {
			return []ValidationResult{{Severity: Info, Message: "i"}}
		}
		v := Value{Val: 42, Validators: []ValidateFunc{warn, info}}
		results := v.Validate(NewTree())
		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
		if results[0].Severity != Warning || results[1].Severity != Info {
			t.Errorf("got severities %v, %v; want Warning, Info", results[0].Severity, results[1].Severity)
		}
	})
}

func TestTreeSetAndGet(t *testing.T) {
	tree := NewTree()

	t.Run("set and get", func(t *testing.T) {
		err := tree.Set("db/host", &Value{Val: "localhost"})
		if err != nil {
			t.Fatalf("Set returned error: %v", err)
		}
		v, ok := tree.Get("db/host")
		if !ok {
			t.Fatal("Get returned ok=false, want true")
		}
		if v.Val != "localhost" {
			t.Errorf("Val = %v, want %q", v.Val, "localhost")
		}
	})

	t.Run("get missing path", func(t *testing.T) {
		_, ok := tree.Get("does/not/exist")
		if ok {
			t.Error("Get returned ok=true for missing path")
		}
	})

	t.Run("set replaces", func(t *testing.T) {
		_ = tree.Set("db/host", &Value{Val: "remotehost"})
		v, _ := tree.Get("db/host")
		if v.Val != "remotehost" {
			t.Errorf("Val = %v, want %q", v.Val, "remotehost")
		}
	})

	t.Run("set rejects invalid path", func(t *testing.T) {
		err := tree.Set("", &Value{Val: "x"})
		if err == nil {
			t.Error("Set with empty path returned nil error")
		}
		err = tree.Set("Bad/path", &Value{Val: "x"})
		if err == nil {
			t.Error("Set with uppercase segment returned nil error")
		}
	})
}

func TestTreeList(t *testing.T) {
	tree := NewTree()
	_ = tree.Set("a/b", &Value{Val: 1})
	_ = tree.Set("c/d", &Value{Val: 2})
	_ = tree.Set("e", &Value{Val: 3})

	paths := tree.List()
	slices.Sort(paths)
	want := []string{"a/b", "c/d", "e"}
	if !slices.Equal(paths, want) {
		t.Errorf("List() = %v, want %v", paths, want)
	}
}

func TestTreeValidate(t *testing.T) {
	t.Run("no validators returns empty map", func(t *testing.T) {
		tree := NewTree()
		_ = tree.Set("a", &Value{Val: "ok"})
		results := tree.Validate()
		if len(results) != 0 {
			t.Errorf("got %d paths with results, want 0", len(results))
		}
	})

	t.Run("collects results by path", func(t *testing.T) {
		alwaysWarn := func(any, TreeReader) []ValidationResult {
			return []ValidationResult{{Severity: Warning, Message: "check"}}
		}
		tree := NewTree()
		_ = tree.Set("a", &Value{Val: 1, Validators: []ValidateFunc{alwaysWarn}})
		_ = tree.Set("b", &Value{Val: 2}) // no validators

		results := tree.Validate()
		if len(results) != 1 {
			t.Fatalf("got results for %d paths, want 1", len(results))
		}
		if _, ok := results["a"]; !ok {
			t.Error("expected results for path \"a\"")
		}
	})
}

func TestCrossValueValidation(t *testing.T) {
	tree := NewTree()
	_ = tree.Set("db/port", &Value{Val: 443})

	checkPort := func(value any, tree TreeReader) []ValidationResult {
		port, ok := tree.Get("db/port")
		if !ok {
			return []ValidationResult{{Severity: Blocking, Message: "db/port not configured"}}
		}
		p, _ := port.Val.(int)
		host, _ := value.(string)
		if host == "localhost" && p < 1024 {
			return []ValidationResult{{
				Severity: Warning,
				Message:  fmt.Sprintf("localhost with privileged port %d", p),
			}}
		}
		return nil
	}

	_ = tree.Set("db/host", &Value{Val: "localhost", Validators: []ValidateFunc{checkPort}})

	results := tree.Validate()
	hostResults, ok := results["db/host"]
	if !ok {
		t.Fatal("expected validation results for db/host")
	}
	if len(hostResults) != 1 {
		t.Fatalf("got %d results, want 1", len(hostResults))
	}
	if hostResults[0].Severity != Warning {
		t.Errorf("severity = %v, want Warning", hostResults[0].Severity)
	}
}

func TestGetReturnsCopy(t *testing.T) {
	tree := NewTree()
	_ = tree.Set("key", &Value{
		Val:      "original",
		Metadata: map[string]any{"hint": "display"},
	})

	got, _ := tree.Get("key")
	got.Val = "mutated"

	original, _ := tree.Get("key")
	if original.Val != "original" {
		t.Errorf("tree was mutated through Get copy: Val = %v, want %q", original.Val, "original")
	}
}

func TestValidationResultMetadata(t *testing.T) {
	withMeta := func(any, TreeReader) []ValidationResult {
		return []ValidationResult{{
			Severity: Info,
			Message:  "documented",
			Metadata: map[string]any{"doc": "https://example.com"},
		}}
	}
	v := Value{Val: "x", Validators: []ValidateFunc{withMeta}}
	results := v.Validate(NewTree())
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Metadata["doc"] != "https://example.com" {
		t.Errorf("metadata[doc] = %v, want URL", results[0].Metadata["doc"])
	}
}
