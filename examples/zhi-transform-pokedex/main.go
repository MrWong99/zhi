// zhi-transform-pokedex is an example zhi transform plugin that evolves starter
// Pokemon before their values are displayed in the UI.
//
// It demonstrates how to implement transform.Plugin by reading and mutating
// the shared configuration tree that the zhi-config-pokedex plugin manages.
//
// Transformations applied by BeforeDisplay:
//
//   - pokedex/starter: the starter Pokemon is replaced with its final
//     evolution (e.g. charmander → charizard).
//   - pokedex/pokedex.goal: the catch goal is increased by the number of
//     evolution stages the starter went through, because evolving a Pokemon
//     should count towards the goal.
//   - pokedex/starter.original: a new path is added that preserves the
//     original (pre-evolution) starter name so the UI can display both.
//
// AfterSave maps evolved names back to their base form so that the stored
// value always uses the unevolved name (which the config plugin validates).
package main

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// evolution describes a single evolution chain for a starter Pokemon.
type evolution struct {
	stages []string // e.g. ["charmander", "charmeleon", "charizard"]
}

// evolutions maps base-form starters to their full evolution chain.
var evolutions = map[string]evolution{
	"bulbasaur":  {stages: []string{"bulbasaur", "ivysaur", "venusaur"}},
	"charmander": {stages: []string{"charmander", "charmeleon", "charizard"}},
	"squirtle":   {stages: []string{"squirtle", "wartortle", "blastoise"}},
	"pikachu":    {stages: []string{"pikachu", "raichu"}},
}

// evolvedToBase maps every evolved form back to its base-form starter.
var evolvedToBase = func() map[string]string {
	m := make(map[string]string)
	for base, evo := range evolutions {
		for _, stage := range evo.stages {
			m[stage] = base
		}
	}
	return m
}()

type evolvePlugin struct{}

func (p *evolvePlugin) BeforeDisplay(_ context.Context, tree *config.Tree) error {
	v, ok := tree.GetPtr("pokedex/starter")
	if !ok {
		return nil
	}
	starter, ok := v.Val.(string)
	if !ok {
		return nil
	}

	evo, known := evolutions[starter]
	if !known {
		return nil
	}

	final := evo.stages[len(evo.stages)-1]
	stagesGained := len(evo.stages) - 1

	// Preserve the original starter name in a new path.
	_ = tree.Set("pokedex/starter.original", &config.Value{
		Val:      starter,
		Metadata: map[string]any{"description": "Original starter before evolution"},
	})

	// Replace the starter with its final evolution.
	v.Val = final

	// Increase the catch goal by the number of evolution stages.
	if stagesGained > 0 {
		if goalVal, ok := tree.GetPtr("pokedex/pokedex.goal"); ok {
			if goal, ok := goalVal.Val.(float64); ok {
				goalVal.Val = goal + float64(stagesGained)
			}
		}
	}

	return nil
}

func (p *evolvePlugin) AfterSave(_ context.Context, tree *config.Tree) error {
	v, ok := tree.GetPtr("pokedex/starter")
	if !ok {
		return nil
	}
	starter, ok := v.Val.(string)
	if !ok {
		return nil
	}

	// Map evolved names back to the base form for storage.
	if base, known := evolvedToBase[starter]; known {
		v.Val = base
	}

	// Remove the display-only original path.
	tree.Delete("pokedex/starter.original")

	return nil
}

func (p *evolvePlugin) ValidatePolicy(_ context.Context) (transform.ValidatePolicy, error) {
	// Validate before transforming so the base-form starter name is
	// checked against the config plugin's allowed list first.
	return transform.ValidateBeforeTransform, nil
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"transform": &transform.GRPCPlugin{Impl: &evolvePlugin{}},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
