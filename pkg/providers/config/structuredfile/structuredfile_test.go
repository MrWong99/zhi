package structuredfile

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func testdataDir(t *testing.T, files ...string) string {
	t.Helper()
	// Copy selected fixture files into a temp directory so tests are
	// isolated.
	dir := t.TempDir()
	for _, f := range files {
		src := filepath.Join("testdata", f)
		content, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("reading fixture %s: %v", f, err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), content, 0o644); err != nil {
			t.Fatalf("writing fixture %s: %v", f, err)
		}
	}
	return dir
}

// --- parse tests ----------------------------------------------------------

func TestParseYAML(t *testing.T) {
	entries, err := parseFile(filepath.Join("testdata", "basic.yaml"))
	if err != nil {
		t.Fatalf("parseFile: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e, ok := entries["pokedex/trainer.name"]
	if !ok {
		t.Fatal("missing pokedex/trainer.name")
	}
	if e.Val != "Ash" {
		t.Errorf("Val = %v, want %q", e.Val, "Ash")
	}
	if desc, _ := e.Metadata["description"].(string); desc != "Name of the Pokemon trainer" {
		t.Errorf("description = %q", desc)
	}
	if ro, _ := e.Metadata["ui.readonly"].(bool); !ro {
		t.Error("ui.readonly should be true")
	}

	e, ok = entries["pokedex/starter"]
	if !ok {
		t.Fatal("missing pokedex/starter")
	}
	if e.Val != "pikachu" {
		t.Errorf("Val = %v, want %q", e.Val, "pikachu")
	}
}

func TestParseJSON(t *testing.T) {
	entries, err := parseFile(filepath.Join("testdata", "basic.json"))
	if err != nil {
		t.Fatalf("parseFile: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e, ok := entries["pokedex/region"]
	if !ok {
		t.Fatal("missing pokedex/region")
	}
	if e.Val != "kanto" {
		t.Errorf("Val = %v, want %q", e.Val, "kanto")
	}

	e, ok = entries["pokedex/pokedex.goal"]
	if !ok {
		t.Fatal("missing pokedex/pokedex.goal")
	}
	// JSON numbers unmarshal as float64.
	if goal, ok := e.Val.(float64); !ok || goal != 150 {
		t.Errorf("Val = %v (%T), want float64(150)", e.Val, e.Val)
	}
}

func TestParseInvalidLeaf(t *testing.T) {
	_, err := parseFile(filepath.Join("testdata", "invalid_leaf.yaml"))
	if err == nil {
		t.Fatal("expected error for non-map leaf, got nil")
	}
}

func TestParseValidationField(t *testing.T) {
	entries, err := parseFile(filepath.Join("testdata", "with_validation.yaml"))
	if err != nil {
		t.Fatalf("parseFile: %v", err)
	}
	e, ok := entries["pokedex/trainer.name"]
	if !ok {
		t.Fatal("missing pokedex/trainer.name")
	}
	if e.ValidationSource == "" {
		t.Fatal("ValidationSource should not be empty")
	}
}

func TestParseImportsField(t *testing.T) {
	entries, err := parseFile(filepath.Join("testdata", "with_imports.yaml"))
	if err != nil {
		t.Fatalf("parseFile: %v", err)
	}
	e, ok := entries["pokedex/trainer.name"]
	if !ok {
		t.Fatal("missing pokedex/trainer.name")
	}
	if len(e.Imports) != 2 {
		t.Fatalf("got %d imports, want 2", len(e.Imports))
	}
	if e.Imports[0] != "strings" {
		t.Errorf("Imports[0] = %q, want %q", e.Imports[0], "strings")
	}
	if e.Imports[1] != "regexp" {
		t.Errorf("Imports[1] = %q, want %q", e.Imports[1], "regexp")
	}
}

func TestFlattenSliceAliasing(t *testing.T) {
	entries, err := parseFile(filepath.Join("testdata", "siblings.yaml"))
	if err != nil {
		t.Fatalf("parseFile: %v", err)
	}

	// There should be exactly 4 entries at the same nesting depth.
	want := map[string]any{
		"pokedex/region/nested/alpha":   "a",
		"pokedex/region/nested/bravo":   "b",
		"pokedex/region/nested/charlie": "c",
		"pokedex/region/nested/delta":   "d",
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(entries), len(want), entries)
	}
	for path, wantVal := range want {
		e, ok := entries[path]
		if !ok {
			t.Errorf("missing path %q", path)
			continue
		}
		if e.Val != wantVal {
			t.Errorf("entries[%q].Val = %v, want %v", path, e.Val, wantVal)
		}
	}
}

// --- overlap tests --------------------------------------------------------

func TestLoadDirOverlap(t *testing.T) {
	dir := testdataDir(t, "overlap_a.yaml", "overlap_b.yaml")
	entries, err := loadDir(dir)
	if err != nil {
		t.Fatalf("loadDir: %v", err)
	}
	e, ok := entries["app/name"]
	if !ok {
		t.Fatal("missing app/name")
	}
	// overlap_b.yaml sorts after overlap_a.yaml → "bravo" wins.
	if e.Val != "bravo" {
		t.Errorf("Val = %v, want %q (last file wins)", e.Val, "bravo")
	}
}

func TestLoadDirNotExist(t *testing.T) {
	entries, err := loadDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("loadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries for missing dir, got %d", len(entries))
	}
}

// --- plugin tests ---------------------------------------------------------

func newTestPlugin(t *testing.T, files ...string) *Plugin {
	t.Helper()
	dir := testdataDir(t, files...)
	p, err := NewFromDir(dir)
	if err != nil {
		t.Fatalf("NewFromDir: %v", err)
	}
	return p
}

func TestPluginList(t *testing.T) {
	p := newTestPlugin(t, "basic.yaml", "basic.json")
	paths, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"pokedex/pokedex.goal",
		"pokedex/region",
		"pokedex/starter",
		"pokedex/trainer.name",
	}
	if !slices.Equal(paths, want) {
		t.Errorf("List() = %v, want %v", paths, want)
	}
}

func TestPluginGet(t *testing.T) {
	p := newTestPlugin(t, "basic.yaml")
	ctx := context.Background()

	v, ok, err := p.Get(ctx, "pokedex/trainer.name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("not found")
	}
	if v.Val != "Ash" {
		t.Errorf("Val = %v, want %q", v.Val, "Ash")
	}
}

func TestPluginGetMissing(t *testing.T) {
	p := newTestPlugin(t, "basic.yaml")
	_, ok, err := p.Get(context.Background(), "does/not.exist")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("expected not found")
	}
}

func TestPluginSetAndGet(t *testing.T) {
	p := newTestPlugin(t, "basic.yaml")
	ctx := context.Background()

	err := p.Set(ctx, "pokedex/trainer.name", config.Value{Val: "Misty"})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok, err := p.Get(ctx, "pokedex/trainer.name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("not found after Set")
	}
	if v.Val != "Misty" {
		t.Errorf("Val = %v, want %q", v.Val, "Misty")
	}
}

func TestPluginSetNewPath(t *testing.T) {
	p := newTestPlugin(t, "basic.yaml")
	ctx := context.Background()

	err := p.Set(ctx, "pokedex/new.key", config.Value{Val: "hello"})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok, err := p.Get(ctx, "pokedex/new.key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("not found after Set on new path")
	}
	if v.Val != "hello" {
		t.Errorf("Val = %v, want %q", v.Val, "hello")
	}

	// New path should appear in List.
	paths, _ := p.List(ctx)
	if !slices.Contains(paths, "pokedex/new.key") {
		t.Errorf("List() missing new path: %v", paths)
	}
}

func TestPluginValidateNoValidation(t *testing.T) {
	p := newTestPlugin(t, "basic.yaml")
	tree := config.NewTree()
	_ = tree.Set("pokedex/trainer.name", &config.Value{Val: "Ash"})

	results, err := p.Validate(context.Background(), "pokedex/trainer.name", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (no validation defined)", len(results))
	}
}

func TestPluginValidateWithCode(t *testing.T) {
	p := newTestPlugin(t, "with_validation.yaml")
	ctx := context.Background()

	t.Run("valid name", func(t *testing.T) {
		tree := config.NewTree()
		_ = tree.Set("pokedex/trainer.name", &config.Value{Val: "Ash"})

		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0: %v", len(results), results)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		tree := config.NewTree()
		_ = tree.Set("pokedex/trainer.name", &config.Value{Val: ""})

		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Severity != config.Blocking {
			t.Errorf("Severity = %v, want Blocking", results[0].Severity)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		tree := config.NewTree()
		_ = tree.Set("pokedex/trainer.name", &config.Value{Val: 42})

		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Severity != config.Blocking {
			t.Errorf("Severity = %v, want Blocking", results[0].Severity)
		}
	})
}

func TestPluginValidateWithImports(t *testing.T) {
	p := newTestPlugin(t, "with_imports.yaml")
	ctx := context.Background()

	t.Run("valid name", func(t *testing.T) {
		tree := config.NewTree()
		_ = tree.Set("pokedex/trainer.name", &config.Value{Val: "Ash"})

		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0: %v", len(results), results)
		}
	})

	t.Run("wrong prefix", func(t *testing.T) {
		tree := config.NewTree()
		_ = tree.Set("pokedex/trainer.name", &config.Value{Val: "Misty"})

		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Severity != config.Warning {
			t.Errorf("Severity = %v, want Warning", results[0].Severity)
		}
	})

	t.Run("invalid characters", func(t *testing.T) {
		tree := config.NewTree()
		_ = tree.Set("pokedex/trainer.name", &config.Value{Val: "Ash123"})

		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Severity != config.Blocking {
			t.Errorf("Severity = %v, want Blocking", results[0].Severity)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		tree := config.NewTree()
		_ = tree.Set("pokedex/trainer.name", &config.Value{Val: 42})

		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Severity != config.Blocking {
			t.Errorf("Severity = %v, want Blocking", results[0].Severity)
		}
	})
}

func TestPluginValidateMissingPath(t *testing.T) {
	p := newTestPlugin(t, "with_validation.yaml")
	tree := config.NewTree()

	results, err := p.Validate(context.Background(), "pokedex/trainer.name", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results for missing path, want 0", len(results))
	}
}

// --- gRPC integration test ------------------------------------------------

func dispense(t *testing.T, files ...string) config.Plugin {
	t.Helper()
	p := newTestPlugin(t, files...)
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"config": &config.GRPCPlugin{Impl: p},
	})
	raw, err := client.Dispense("config")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(config.Plugin)
}

func TestGRPCList(t *testing.T) {
	p := dispense(t, "basic.yaml", "basic.json")
	paths, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"pokedex/pokedex.goal",
		"pokedex/region",
		"pokedex/starter",
		"pokedex/trainer.name",
	}
	if !slices.Equal(paths, want) {
		t.Errorf("List() = %v, want %v", paths, want)
	}
}

func TestGRPCGetDefaults(t *testing.T) {
	p := dispense(t, "basic.yaml", "basic.json")
	ctx := context.Background()

	tests := []struct {
		path    string
		wantVal any
	}{
		{"pokedex/trainer.name", "Ash"},
		{"pokedex/starter", "pikachu"},
		{"pokedex/region", "kanto"},
		// integers become float64 through JSON round-trip
		{"pokedex/pokedex.goal", float64(150)},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			v, ok, err := p.Get(ctx, tt.path)
			if err != nil {
				t.Fatalf("Get(%q): %v", tt.path, err)
			}
			if !ok {
				t.Fatalf("Get(%q): not found", tt.path)
			}
			if v.Val != tt.wantVal {
				t.Errorf("Val = %v (%T), want %v (%T)", v.Val, v.Val, tt.wantVal, tt.wantVal)
			}
		})
	}
}

func TestGRPCSetAndGet(t *testing.T) {
	p := dispense(t, "basic.yaml")
	ctx := context.Background()

	if err := p.Set(ctx, "pokedex/trainer.name", config.Value{Val: "Brock"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok, err := p.Get(ctx, "pokedex/trainer.name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("not found after Set")
	}
	if v.Val != "Brock" {
		t.Errorf("Val = %v, want %q", v.Val, "Brock")
	}
}

func TestGRPCValidate(t *testing.T) {
	p := dispense(t, "with_validation.yaml")
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("pokedex/trainer.name", &config.Value{Val: ""})

	results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Severity != config.Blocking {
		t.Errorf("Severity = %v, want Blocking", results[0].Severity)
	}
}

func TestGRPCGetMetadata(t *testing.T) {
	p := dispense(t, "basic.yaml")
	v, ok, err := p.Get(context.Background(), "pokedex/trainer.name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("not found")
	}
	desc, exists := v.Metadata["description"]
	if !exists {
		t.Fatal("metadata missing 'description' key")
	}
	if desc != "Name of the Pokemon trainer" {
		t.Errorf("description = %q", desc)
	}
}
