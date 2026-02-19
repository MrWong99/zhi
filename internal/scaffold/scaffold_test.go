package scaffold

import "testing"

func TestToPackageName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-plugin", "myplugin"},
		{"simple", "simple"},
		{"multi-word-name", "multiwordname"},
		{"already", "already"},
		{"a-b-c", "abc"},
	}
	for _, tt := range tests {
		got := ToPackageName(tt.input)
		if got != tt.want {
			t.Errorf("ToPackageName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetUnregistered(t *testing.T) {
	_, err := Get("nonexistent-lang-xyz")
	if err == nil {
		t.Error("Get(nonexistent) should return error")
	}
}
