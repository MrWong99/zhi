package main

import (
	"context"
	"slices"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// dispense starts an in-process gRPC plugin server and returns a connected
// config.Plugin client. No subprocess is spawned.
func dispense(t *testing.T) config.Plugin {
	t.Helper()
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"config": &config.GRPCPlugin{Impl: newPokedexPlugin()},
	})
	raw, err := client.Dispense("config")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(config.Plugin)
}

func TestList(t *testing.T) {
	p := dispense(t)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"pokedex/trainer.name",
		"pokedex/starter",
		"pokedex/region",
		"pokedex/pokedex.goal",
	}
	if !slices.Equal(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func TestGetDefaults(t *testing.T) {
	p := dispense(t)
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

func TestGetMissing(t *testing.T) {
	p := dispense(t)
	_, ok, err := p.Get(context.Background(), "pokedex/does.not.exist")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("Get returned ok=true for a missing path")
	}
}

func TestGetMetadata(t *testing.T) {
	p := dispense(t)
	v, ok, err := p.Get(context.Background(), "pokedex/trainer.name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	desc, exists := v.Metadata["description"]
	if !exists {
		t.Fatal("metadata missing 'description' key")
	}
	if desc != "Name of the Pokemon trainer" {
		t.Errorf("description = %q, want %q", desc, "Name of the Pokemon trainer")
	}
}

func TestSetAndGet(t *testing.T) {
	p := dispense(t)
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
		t.Fatal("Get: not found after Set")
	}
	if v.Val != "Misty" {
		t.Errorf("Val = %v, want %q", v.Val, "Misty")
	}
}

func TestValidateDefaults(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	// Build a tree snapshot from the plugin's own values.
	tree := config.NewTree()
	paths, _ := p.List(ctx)
	for _, path := range paths {
		v, ok, _ := p.Get(ctx, path)
		if ok {
			_ = tree.Set(path, &config.Value{Val: v.Val, Metadata: v.Metadata})
		}
	}

	t.Run("trainer.name is valid", func(t *testing.T) {
		results, err := p.Validate(ctx, "pokedex/trainer.name", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0: %v", len(results), results)
		}
	})

	t.Run("starter pikachu triggers warnings", func(t *testing.T) {
		results, err := p.Validate(ctx, "pokedex/starter", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		// pikachu is not a classic starter → Warning
		// pikachu is not a Kanto starter  → Info
		if len(results) != 2 {
			t.Fatalf("got %d results, want 2: %v", len(results), results)
		}
		if results[0].Severity != config.Warning {
			t.Errorf("results[0].Severity = %v, want Warning", results[0].Severity)
		}
		if results[1].Severity != config.Info {
			t.Errorf("results[1].Severity = %v, want Info", results[1].Severity)
		}
	})

	t.Run("region kanto is valid", func(t *testing.T) {
		results, err := p.Validate(ctx, "pokedex/region", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0: %v", len(results), results)
		}
	})

	t.Run("goal 150 is valid", func(t *testing.T) {
		results, err := p.Validate(ctx, "pokedex/pokedex.goal", tree)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0: %v", len(results), results)
		}
	})
}

func TestValidateEmptyTrainerName(t *testing.T) {
	p := dispense(t)
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

func TestValidateUnknownRegion(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("pokedex/region", &config.Value{Val: "unown-dimension"})

	results, err := p.Validate(ctx, "pokedex/region", tree)
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

func TestValidateClassicStarter(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("pokedex/starter", &config.Value{Val: "charmander"})
	_ = tree.Set("pokedex/region", &config.Value{Val: "kanto"})

	results, err := p.Validate(ctx, "pokedex/starter", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 for a valid Kanto starter: %v", len(results), results)
	}
}

func TestValidateCrossRegionStarter(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	// charmander in johto: it's a valid starter name, but not typical for
	// johto. Since regionStarters only has kanto mapped, no cross-value
	// info result should fire.
	tree := config.NewTree()
	_ = tree.Set("pokedex/starter", &config.Value{Val: "charmander"})
	_ = tree.Set("pokedex/region", &config.Value{Val: "johto"})

	results, err := p.Validate(ctx, "pokedex/starter", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (johto has no mapped starters): %v", len(results), results)
	}
}

func TestValidateGoalBounds(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		goal     float64
		severity config.Severity
		wantN    int
	}{
		{"zero", 0, config.Blocking, 1},
		{"negative", -5, config.Blocking, 1},
		{"one", 1, 0, 0},
		{"normal", 150, 0, 0},
		{"over_1025", 2000, config.Warning, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			_ = tree.Set("pokedex/pokedex.goal", &config.Value{Val: tt.goal})

			results, err := p.Validate(ctx, "pokedex/pokedex.goal", tree)
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if len(results) != tt.wantN {
				t.Fatalf("got %d results, want %d: %v", len(results), tt.wantN, results)
			}
			if tt.wantN > 0 && results[0].Severity != tt.severity {
				t.Errorf("Severity = %v, want %v", results[0].Severity, tt.severity)
			}
		})
	}
}

func TestValidateMissingPath(t *testing.T) {
	p := dispense(t)
	tree := config.NewTree()

	results, err := p.Validate(context.Background(), "pokedex/trainer.name", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results for missing path, want 0", len(results))
	}
}
