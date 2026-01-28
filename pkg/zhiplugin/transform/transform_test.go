package transform

import (
	"context"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// testPlugin is a simple transform plugin used in tests.
type testPlugin struct {
	policy ValidatePolicy
}

func (p *testPlugin) BeforeDisplay(_ context.Context, tree *config.Tree) error {
	// Uppercase all string values and add a marker path.
	for _, path := range tree.List() {
		v, ok := tree.GetPtr(path)
		if !ok {
			continue
		}
		if s, ok := v.Val.(string); ok {
			v.Val = "[display] " + s
		}
	}
	tree.Set("meta/transformed", &config.Value{Val: "before-display"})
	return nil
}

func (p *testPlugin) AfterSave(_ context.Context, tree *config.Tree) error {
	// Strip a prefix if present and remove the marker.
	for _, path := range tree.List() {
		v, ok := tree.GetPtr(path)
		if !ok {
			continue
		}
		if s, ok := v.Val.(string); ok && len(s) >= 8 && s[:8] == "[saved] " {
			v.Val = s[8:]
		}
	}
	tree.Delete("meta/temporary")
	return nil
}

func (p *testPlugin) ValidatePolicy(_ context.Context) (ValidatePolicy, error) {
	return p.policy, nil
}

// dispense starts an in-process gRPC plugin server and returns a connected
// transform.Plugin client.
func dispense(t *testing.T, impl Plugin) Plugin {
	t.Helper()
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"transform": &GRPCPlugin{Impl: impl},
	})
	raw, err := client.Dispense("transform")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(Plugin)
}

func TestValidatePolicyString(t *testing.T) {
	tests := []struct {
		p    ValidatePolicy
		want string
	}{
		{ValidateBeforeTransform, "before"},
		{ValidateAfterTransform, "after"},
		{ValidateBoth, "both"},
		{ValidatePolicy(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("ValidatePolicy(%d).String() = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

func TestBeforeDisplayModifiesTree(t *testing.T) {
	p := dispense(t, &testPlugin{policy: ValidateBeforeTransform})
	ctx := context.Background()

	tree := config.NewTree()
	tree.Set("app/name", &config.Value{Val: "zhi"})
	tree.Set("app/port", &config.Value{Val: float64(8080)})

	if err := p.BeforeDisplay(ctx, tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}

	// String value should be prefixed.
	v, ok := tree.Get("app/name")
	if !ok {
		t.Fatal("app/name missing after transform")
	}
	if v.Val != "[display] zhi" {
		t.Errorf("app/name = %v, want %q", v.Val, "[display] zhi")
	}

	// Non-string value should be unchanged.
	v, ok = tree.Get("app/port")
	if !ok {
		t.Fatal("app/port missing after transform")
	}
	if v.Val != float64(8080) {
		t.Errorf("app/port = %v, want 8080", v.Val)
	}

	// Marker path should be added.
	v, ok = tree.Get("meta/transformed")
	if !ok {
		t.Fatal("meta/transformed missing after transform")
	}
	if v.Val != "before-display" {
		t.Errorf("meta/transformed = %v, want %q", v.Val, "before-display")
	}
}

func TestAfterSaveModifiesTree(t *testing.T) {
	p := dispense(t, &testPlugin{policy: ValidateAfterTransform})
	ctx := context.Background()

	tree := config.NewTree()
	tree.Set("app/name", &config.Value{Val: "[saved] zhi"})
	tree.Set("meta/temporary", &config.Value{Val: "tmp"})

	if err := p.AfterSave(ctx, tree); err != nil {
		t.Fatalf("AfterSave: %v", err)
	}

	// Prefix should be stripped.
	v, ok := tree.Get("app/name")
	if !ok {
		t.Fatal("app/name missing after transform")
	}
	if v.Val != "zhi" {
		t.Errorf("app/name = %v, want %q", v.Val, "zhi")
	}

	// Temporary path should be removed.
	_, ok = tree.Get("meta/temporary")
	if ok {
		t.Error("meta/temporary should have been removed by AfterSave")
	}
}

func TestValidatePolicyOverGRPC(t *testing.T) {
	tests := []struct {
		name   string
		policy ValidatePolicy
	}{
		{"before", ValidateBeforeTransform},
		{"after", ValidateAfterTransform},
		{"both", ValidateBoth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := dispense(t, &testPlugin{policy: tt.policy})
			got, err := p.ValidatePolicy(context.Background())
			if err != nil {
				t.Fatalf("ValidatePolicy: %v", err)
			}
			if got != tt.policy {
				t.Errorf("ValidatePolicy() = %v, want %v", got, tt.policy)
			}
		})
	}
}

// noopPlugin does nothing, leaving the tree unchanged.
type noopPlugin struct{}

func (noopPlugin) BeforeDisplay(_ context.Context, _ *config.Tree) error { return nil }
func (noopPlugin) AfterSave(_ context.Context, _ *config.Tree) error     { return nil }
func (noopPlugin) ValidatePolicy(_ context.Context) (ValidatePolicy, error) {
	return ValidateBeforeTransform, nil
}

func TestNoopTransformLeavesTreeUnchanged(t *testing.T) {
	p := dispense(t, &noopPlugin{})
	ctx := context.Background()

	tree := config.NewTree()
	tree.Set("a/b", &config.Value{Val: "original"})

	if err := p.BeforeDisplay(ctx, tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}

	v, ok := tree.Get("a/b")
	if !ok {
		t.Fatal("a/b missing after no-op transform")
	}
	if v.Val != "original" {
		t.Errorf("Val = %v, want %q", v.Val, "original")
	}

	paths := tree.List()
	if len(paths) != 1 {
		t.Errorf("tree has %d paths, want 1", len(paths))
	}
}

func TestMetadataPreservedAcrossTransform(t *testing.T) {
	p := dispense(t, &noopPlugin{})
	ctx := context.Background()

	tree := config.NewTree()
	tree.Set("db/host", &config.Value{
		Val:      "localhost",
		Metadata: map[string]any{"description": "Database host"},
	})

	if err := p.BeforeDisplay(ctx, tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}

	v, ok := tree.Get("db/host")
	if !ok {
		t.Fatal("db/host missing after transform")
	}
	desc, exists := v.Metadata["description"]
	if !exists {
		t.Fatal("metadata missing 'description' key")
	}
	if desc != "Database host" {
		t.Errorf("description = %v, want %q", desc, "Database host")
	}
}
