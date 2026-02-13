package cli

import (
	"testing"
)

func TestParsePluginRef(t *testing.T) {
	tests := []struct {
		input      string
		pluginType string
		publisher  string
		name       string
		wantErr    bool
	}{
		{"config/zhi-project/ansible-config", "config", "zhi-project", "ansible-config", false},
		{"store/org/plugin", "store", "org", "plugin", false},
		{"zhi-project/ansible-config", "", "zhi-project", "ansible-config", false},
		{"org/plugin", "", "org", "plugin", false},
		{"invalid", "", "", "", true},
		{"/name", "", "", "", true},
		{"pub/", "", "", "", true},
		{"", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pluginType, pub, name, err := parsePluginRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePluginRef(%q): %v", tt.input, err)
			}
			if pluginType != tt.pluginType {
				t.Errorf("pluginType = %q, want %q", pluginType, tt.pluginType)
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
