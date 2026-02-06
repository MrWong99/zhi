package semver

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"1.2.0", false},
		{"0.1.0", false},
		{"v1.2.0", false},
		{"1.0.0-beta.1", false},
		{"", true},
		{"notaversion", true},
	}
	for _, tt := range tests {
		v, err := Parse(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Parse(%q) expected error, got %v", tt.input, v)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tt.input, err)
		}
	}
}

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"1.2.0", false},
		{">=1.2.0", false},
		{"~1.2.0", false},
		{"^1.2.0", false},
		{">=1.0, <2.0", false},
		{">=1.0,<2.0", false},
	}
	for _, tt := range tests {
		_, err := ParseConstraint(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseConstraint(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseConstraint(%q) unexpected error: %v", tt.input, err)
		}
	}
}

func TestMatches(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
		want       bool
	}{
		{"1.2.0", "1.2.0", true},
		{"1.2.1", "1.2.0", false},
		{"1.2.0", ">=1.2.0", true},
		{"1.3.0", ">=1.2.0", true},
		{"1.1.0", ">=1.2.0", false},
		{"1.2.1", "~1.2.0", true},
		{"1.2.99", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"1.3.0", "^1.2.0", true},
		{"1.99.0", "^1.2.0", true},
		{"2.0.0", "^1.2.0", false},
		{"1.5.0", ">=1.0,<2.0", true},
		{"2.0.0", ">=1.0,<2.0", false},
	}
	for _, tt := range tests {
		got, err := Matches(tt.version, tt.constraint)
		if err != nil {
			t.Errorf("Matches(%q, %q) error: %v", tt.version, tt.constraint, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", tt.version, tt.constraint, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want UpdateScope
	}{
		{"1.2.0", "1.2.0", ScopeNone},
		{"1.2.0", "1.2.1", ScopePatch},
		{"1.2.0", "1.3.0", ScopeMinor},
		{"1.2.0", "2.0.0", ScopeMajor},
		{"1.2.3", "1.2.4", ScopePatch},
		{"0.9.0", "1.0.0", ScopeMajor},
	}
	for _, tt := range tests {
		got, err := CompareVersions(tt.from, tt.to)
		if err != nil {
			t.Errorf("CompareVersions(%q, %q) error: %v", tt.from, tt.to, err)
			continue
		}
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestIsAutoUpdateAllowed(t *testing.T) {
	tests := []struct {
		from  string
		to    string
		scope string
		want  bool
	}{
		{"1.2.0", "1.2.1", "patch", true},
		{"1.2.0", "1.3.0", "patch", false},
		{"1.2.0", "2.0.0", "patch", false},
		{"1.2.0", "1.2.1", "minor", true},
		{"1.2.0", "1.3.0", "minor", true},
		{"1.2.0", "2.0.0", "minor", false},
		{"1.2.0", "2.0.0", "all", true},
		{"1.2.0", "1.3.0", "all", true},
	}
	for _, tt := range tests {
		got, err := IsAutoUpdateAllowed(tt.from, tt.to, tt.scope)
		if err != nil {
			t.Errorf("IsAutoUpdateAllowed(%q, %q, %q) error: %v", tt.from, tt.to, tt.scope, err)
			continue
		}
		if got != tt.want {
			t.Errorf("IsAutoUpdateAllowed(%q, %q, %q) = %v, want %v", tt.from, tt.to, tt.scope, got, tt.want)
		}
	}
}

func TestLatestMatching(t *testing.T) {
	candidates := []string{"1.0.0", "1.1.0", "1.2.0", "1.2.1", "2.0.0"}

	tests := []struct {
		constraint string
		want       string
	}{
		{"^1.0.0", "1.2.1"},
		{"~1.2.0", "1.2.1"},
		{">=2.0.0", "2.0.0"},
		{">=3.0.0", ""},
	}
	for _, tt := range tests {
		got, err := LatestMatching(candidates, tt.constraint)
		if err != nil {
			t.Errorf("LatestMatching(%q) error: %v", tt.constraint, err)
			continue
		}
		if got != tt.want {
			t.Errorf("LatestMatching(%q) = %q, want %q", tt.constraint, got, tt.want)
		}
	}
}

func TestLatest(t *testing.T) {
	tests := []struct {
		candidates []string
		want       string
	}{
		{[]string{"1.0.0", "2.0.0", "1.5.0"}, "2.0.0"},
		{[]string{"0.1.0"}, "0.1.0"},
		{[]string{}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		got := Latest(tt.candidates)
		if got != tt.want {
			t.Errorf("Latest(%v) = %q, want %q", tt.candidates, got, tt.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current   string
		candidate string
		want      bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.0.0", false},
		{"2.0.0", "1.0.0", false},
	}
	for _, tt := range tests {
		got, err := IsNewer(tt.current, tt.candidate)
		if err != nil {
			t.Errorf("IsNewer(%q, %q) error: %v", tt.current, tt.candidate, err)
			continue
		}
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.candidate, got, tt.want)
		}
	}
}
