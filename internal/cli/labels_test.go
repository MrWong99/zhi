package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

func resetLabelsFlags() {
	labelsJSON = false
	labelsNamespace = ""
}

func TestLabelsListCmd(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantStrs  []string
		wantNoStr []string
	}{
		{
			name:     "list all",
			args:     []string{"labels", "list"},
			wantStrs: []string{"ui.readonly", "store.writeonly", "transform.hidden", "core.description"},
		},
		{
			name:      "list ui namespace",
			args:      []string{"labels", "list", "--namespace", "ui"},
			wantStrs:  []string{"ui.readonly", "ui.password", "ui.hidden"},
			wantNoStr: []string{"store.writeonly"},
		},
		{
			name:      "list store namespace",
			args:      []string{"labels", "list", "--namespace", "store"},
			wantStrs:  []string{"store.writeonly", "store.encrypt"},
			wantNoStr: []string{"ui.readonly"},
		},
		{
			name:     "unknown namespace",
			args:     []string{"labels", "list", "--namespace", "unknown"},
			wantStrs: []string{"Unknown namespace", "Available namespaces"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLabelsFlags()
			output, err := executeCommand(rootCmd, tt.args...)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			for _, want := range tt.wantStrs {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got:\n%s", want, output)
				}
			}
			for _, noWant := range tt.wantNoStr {
				if strings.Contains(output, noWant) {
					t.Errorf("output should not contain %q, got:\n%s", noWant, output)
				}
			}
		})
	}
}

func TestLabelsListCmd_JSON(t *testing.T) {
	resetLabelsFlags()
	output, err := executeCommand(rootCmd, "labels", "list", "--namespace", "ui", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var entries []labelJSONEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, output)
	}

	if len(entries) == 0 {
		t.Error("expected non-empty label list")
	}

	// Check all entries are in ui namespace
	for _, e := range entries {
		if e.Namespace != "ui" {
			t.Errorf("expected namespace ui, got %s", e.Namespace)
		}
	}

	// Check ui.readonly is present
	found := false
	for _, e := range entries {
		if e.Name == "ui.readonly" {
			found = true
			if e.ValueType != "bool" {
				t.Errorf("ui.readonly should have type bool, got %s", e.ValueType)
			}
			break
		}
	}
	if !found {
		t.Error("ui.readonly should be in the list")
	}
}

func TestLabelsInfoCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantStrs []string
	}{
		{
			name:     "ui.readonly",
			args:     []string{"labels", "info", "ui.readonly"},
			wantStrs: []string{"ui.readonly", "bool", "Prevents users"},
		},
		{
			name:     "ui.pattern",
			args:     []string{"labels", "info", "ui.pattern"},
			wantStrs: []string{"ui.pattern", "string", "Regular expression", "Examples:"},
		},
		{
			name:     "unknown label",
			args:     []string{"labels", "info", "unknown.label"},
			wantStrs: []string{"Label not found"},
		},
		{
			name:     "partial match suggestions",
			args:     []string{"labels", "info", "ui.read"},
			wantStrs: []string{"Label not found", "Did you mean"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLabelsFlags()
			output, err := executeCommand(rootCmd, tt.args...)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			for _, want := range tt.wantStrs {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got:\n%s", want, output)
				}
			}
		})
	}
}

func TestLabelsInfoCmd_JSON(t *testing.T) {
	resetLabelsFlags()
	output, err := executeCommand(rootCmd, "labels", "info", "ui.readonly", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, output)
	}

	if result["name"] != "ui.readonly" {
		t.Errorf("expected name ui.readonly, got %v", result["name"])
	}
	if result["namespace"] != "ui" {
		t.Errorf("expected namespace ui, got %v", result["namespace"])
	}
	if result["value_type"] != "bool" {
		t.Errorf("expected value_type bool, got %v", result["value_type"])
	}
}

func TestLabelsInfoCmd_WithConstraints(t *testing.T) {
	resetLabelsFlags()
	output, err := executeCommand(rootCmd, "labels", "info", "core.type")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// core.type has an enum constraint
	if !strings.Contains(output, "Constraints:") {
		t.Errorf("output should contain Constraints section, got:\n%s", output)
	}
	if !strings.Contains(output, "Allowed:") {
		t.Errorf("output should contain Allowed values, got:\n%s", output)
	}
}

func TestFindSimilarLabels(t *testing.T) {
	registry := labels.DefaultRegistry

	tests := []struct {
		name         string
		search       string
		wantAny      []string // at least one of these should be suggested
		wantPrefix   string   // if set, all suggestions should start with this
		wantMinCount int      // minimum number of suggestions expected
	}{
		{
			name:         "partial match readonly",
			search:       "ui.read",
			wantAny:      []string{"ui.readonly"},
			wantMinCount: 1,
		},
		{
			name:         "namespace only returns ui labels",
			search:       "ui",
			wantPrefix:   "ui.",
			wantMinCount: 5, // Should return 5 ui.* labels
		},
		{
			name:         "partial password",
			search:       "ui.pass",
			wantAny:      []string{"ui.password"},
			wantMinCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := findSimilarLabels(registry, tt.search)

			if len(suggestions) < tt.wantMinCount {
				t.Errorf("expected at least %d suggestions, got %d: %v", tt.wantMinCount, len(suggestions), suggestions)
			}

			if tt.wantPrefix != "" {
				for _, s := range suggestions {
					if !strings.HasPrefix(s, tt.wantPrefix) {
						t.Errorf("expected suggestion %q to start with %q", s, tt.wantPrefix)
					}
				}
			}

			if len(tt.wantAny) > 0 {
				foundAny := false
				for _, want := range tt.wantAny {
					for _, s := range suggestions {
						if s == want {
							foundAny = true
							break
						}
					}
				}
				if !foundAny {
					t.Errorf("expected at least one of %v in suggestions, got %v", tt.wantAny, suggestions)
				}
			}
		})
	}
}
