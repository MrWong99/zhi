package config

import (
	"testing"
)

func TestValidateYAML_ValidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
	}{
		{"empty string", ""},
		{"nil value", nil},
		{"simple key-value", "key: value"},
		{"list", "- item1\n- item2"},
		{"nested map", "parent:\n  child: value"},
		{"json is valid yaml", `{"key": "value"}`},
		{"multi-document", "---\nfoo: bar"},
		{"non-string type", 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := ValidateYAML(tt.input, nil)
			if len(results) != 0 {
				t.Errorf("ValidateYAML(%v) returned %d results, want 0", tt.input, len(results))
			}
		})
	}
}

func TestValidateYAML_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"duplicate key mapping", "key: value1\nkey: value2\n: invalid"},
		{"unclosed bracket", "[unclosed"},
		{"invalid mapping", "foo: bar: baz: bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := ValidateYAML(tt.input, nil)
			if len(results) == 0 {
				t.Errorf("ValidateYAML(%q) returned 0 results, want warning", tt.input)
				return
			}
			if results[0].Severity != Warning {
				t.Errorf("expected Warning severity, got %v", results[0].Severity)
			}
			if results[0].Message == "" {
				t.Error("expected non-empty error message")
			}
		})
	}
}
