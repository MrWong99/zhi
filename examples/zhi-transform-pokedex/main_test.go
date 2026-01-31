package main

import (
	"context"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// dispense starts an in-process gRPC plugin server and returns a connected
// transform.Plugin client. No subprocess is spawned.
func dispense(t *testing.T) transform.Plugin {
	t.Helper()
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"transform": &transform.GRPCPlugin{Impl: &evolvePlugin{}},
	})
	raw, err := client.Dispense("transform")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(transform.Plugin)
}

// pokedexTree builds a config tree resembling the zhi-config-pokedex defaults.
func pokedexTree(starter string, goal float64) *config.Tree {
	tree := config.NewTree()
	_ = tree.Set("pokedex/trainer.name", &config.Value{Val: "Ash"})
	_ = tree.Set("pokedex/starter", &config.Value{Val: starter})
	_ = tree.Set("pokedex/region", &config.Value{Val: "kanto"})
	_ = tree.Set("pokedex/pokedex.goal", &config.Value{Val: goal})
	return tree
}

func TestBeforeDisplayEvolvesStarter(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	tests := []struct {
		starter     string
		wantEvolved string
		wantGoal    float64
	}{
		{"charmander", "charizard", 152}, // 150 + 2 stages
		{"bulbasaur", "venusaur", 152},   // 150 + 2 stages
		{"squirtle", "blastoise", 152},   // 150 + 2 stages
		{"pikachu", "raichu", 151},       // 150 + 1 stage
	}

	for _, tt := range tests {
		t.Run(tt.starter, func(t *testing.T) {
			tree := pokedexTree(tt.starter, 150)

			if err := p.BeforeDisplay(ctx, tree); err != nil {
				t.Fatalf("BeforeDisplay: %v", err)
			}

			// Starter should be evolved.
			v, ok := tree.Get("pokedex/starter")
			if !ok {
				t.Fatal("pokedex/starter missing after transform")
			}
			if v.Val != tt.wantEvolved {
				t.Errorf("starter = %v, want %q", v.Val, tt.wantEvolved)
			}

			// Goal should be increased.
			g, ok := tree.Get("pokedex/pokedex.goal")
			if !ok {
				t.Fatal("pokedex/pokedex.goal missing after transform")
			}
			if g.Val != tt.wantGoal {
				t.Errorf("goal = %v, want %v", g.Val, tt.wantGoal)
			}

			// Original starter should be preserved.
			orig, ok := tree.Get("pokedex/starter.original")
			if !ok {
				t.Fatal("pokedex/starter.original missing after transform")
			}
			if orig.Val != tt.starter {
				t.Errorf("starter.original = %v, want %q", orig.Val, tt.starter)
			}
		})
	}
}

func TestBeforeDisplayUnknownStarterNoOp(t *testing.T) {
	p := dispense(t)
	tree := pokedexTree("eevee", 150)

	if err := p.BeforeDisplay(context.Background(), tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}

	v, ok := tree.Get("pokedex/starter")
	if !ok {
		t.Fatal("pokedex/starter missing")
	}
	if v.Val != "eevee" {
		t.Errorf("starter = %v, want %q (should be unchanged)", v.Val, "eevee")
	}

	_, ok = tree.Get("pokedex/starter.original")
	if ok {
		t.Error("pokedex/starter.original should not be added for unknown starters")
	}
}

func TestBeforeDisplayNoStarterPath(t *testing.T) {
	p := dispense(t)
	tree := config.NewTree()
	_ = tree.Set("pokedex/region", &config.Value{Val: "kanto"})

	if err := p.BeforeDisplay(context.Background(), tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}

	// Tree should be unchanged.
	paths := tree.List()
	if len(paths) != 1 {
		t.Errorf("tree has %d paths, want 1", len(paths))
	}
}

func TestAfterSaveDevolvesStarter(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	tests := []struct {
		evolved  string
		wantBase string
	}{
		{"charizard", "charmander"},
		{"venusaur", "bulbasaur"},
		{"blastoise", "squirtle"},
		{"raichu", "pikachu"},
		// Base forms map to themselves.
		{"charmander", "charmander"},
		{"pikachu", "pikachu"},
	}

	for _, tt := range tests {
		t.Run(tt.evolved, func(t *testing.T) {
			tree := pokedexTree(tt.evolved, 150)
			// Add the display-only path that BeforeDisplay would have created.
			_ = tree.Set("pokedex/starter.original", &config.Value{Val: "original"})

			if err := p.AfterSave(ctx, tree); err != nil {
				t.Fatalf("AfterSave: %v", err)
			}

			v, ok := tree.Get("pokedex/starter")
			if !ok {
				t.Fatal("pokedex/starter missing after AfterSave")
			}
			if v.Val != tt.wantBase {
				t.Errorf("starter = %v, want %q", v.Val, tt.wantBase)
			}

			// Display-only path should be removed.
			_, ok = tree.Get("pokedex/starter.original")
			if ok {
				t.Error("pokedex/starter.original should be removed by AfterSave")
			}
		})
	}
}

func TestAfterSaveUnknownStarterNoOp(t *testing.T) {
	p := dispense(t)
	tree := pokedexTree("eevee", 150)

	if err := p.AfterSave(context.Background(), tree); err != nil {
		t.Fatalf("AfterSave: %v", err)
	}

	v, _ := tree.Get("pokedex/starter")
	if v.Val != "eevee" {
		t.Errorf("starter = %v, want %q (should be unchanged)", v.Val, "eevee")
	}
}

func TestValidatePolicyIsBeforeTransform(t *testing.T) {
	p := dispense(t)
	policy, err := p.ValidatePolicy(context.Background())
	if err != nil {
		t.Fatalf("ValidatePolicy: %v", err)
	}
	if policy != transform.ValidateBeforeTransform {
		t.Errorf("ValidatePolicy() = %v, want ValidateBeforeTransform", policy)
	}
}

func TestRoundTripBeforeDisplayThenAfterSave(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	tree := pokedexTree("charmander", 150)

	// Simulate display: evolve.
	if err := p.BeforeDisplay(ctx, tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}
	v, _ := tree.Get("pokedex/starter")
	if v.Val != "charizard" {
		t.Fatalf("after BeforeDisplay: starter = %v, want charizard", v.Val)
	}

	// Simulate save: devolve back.
	if err := p.AfterSave(ctx, tree); err != nil {
		t.Fatalf("AfterSave: %v", err)
	}
	v, _ = tree.Get("pokedex/starter")
	if v.Val != "charmander" {
		t.Errorf("after AfterSave: starter = %v, want charmander", v.Val)
	}

	// Display-only path should be gone.
	_, ok := tree.Get("pokedex/starter.original")
	if ok {
		t.Error("pokedex/starter.original should be removed after AfterSave")
	}
}

func TestMetadataOnOriginalStarter(t *testing.T) {
	p := dispense(t)
	tree := pokedexTree("squirtle", 150)

	if err := p.BeforeDisplay(context.Background(), tree); err != nil {
		t.Fatalf("BeforeDisplay: %v", err)
	}

	orig, ok := tree.Get("pokedex/starter.original")
	if !ok {
		t.Fatal("pokedex/starter.original missing")
	}
	desc, exists := orig.Metadata["description"]
	if !exists {
		t.Fatal("metadata missing 'description' key")
	}
	if desc != "Original starter before evolution" {
		t.Errorf("description = %v, want %q", desc, "Original starter before evolution")
	}
}
