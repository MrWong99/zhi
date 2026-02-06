package cli

import (
	"testing"
)

func TestParsePluginRef(t *testing.T) {
	tests := []struct {
		input     string
		publisher string
		name      string
		wantErr   bool
	}{
		{"zhi-project/ansible-config", "zhi-project", "ansible-config", false},
		{"org/plugin", "org", "plugin", false},
		{"invalid", "", "", true},
		{"/name", "", "", true},
		{"pub/", "", "", true},
		{"", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pub, name, err := parsePluginRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePluginRef(%q): %v", tt.input, err)
			}
			if pub != tt.publisher {
				t.Errorf("publisher = %q, want %q", pub, tt.publisher)
			}
			if name != tt.name {
				t.Errorf("name = %q, want %q", name, tt.name)
			}
		})
	}
}
