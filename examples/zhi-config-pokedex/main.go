// zhi-config-pokedex is an example zhi configuration plugin that manages a small
// Pokedex. It demonstrates how to implement config.Plugin with typed values,
// metadata, and cross-value validation.
//
// Managed paths:
//
//	pokedex/trainer.name   string   name of the Pokemon trainer
//	pokedex/starter        string   chosen starter Pokemon (bulbasaur, charmander, squirtle)
//	pokedex/region         string   home region (kanto, johto, hoenn, sinnoh, unova, kalos, alola, galar, paldea)
//	pokedex/pokedex.goal   int      number of Pokemon the trainer aims to catch
package main

import (
	"context"
	"fmt"
	"slices"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// Allowed values for constrained fields.
var (
	validStarters = []string{"bulbasaur", "charmander", "squirtle"}
	validRegions  = []string{"kanto", "johto", "hoenn", "sinnoh", "unova", "kalos", "alola", "galar", "paldea"}

	// regionStarters maps each region to its canonical starter set. Only
	// Kanto starters are wired up here to keep the example short; the
	// validator uses this to emit a cross-value warning.
	regionStarters = map[string][]string{
		"kanto": {"bulbasaur", "charmander", "squirtle"},
	}
)

// paths is the fixed set of configuration paths this plugin owns.
var paths = []string{
	"pokedex/trainer.name",
	"pokedex/starter",
	"pokedex/region",
	"pokedex/pokedex.goal",
}

// defaults returns the initial configuration tree for a fresh Pokedex.
func defaults() map[string]*config.Value {
	return map[string]*config.Value{
		"pokedex/trainer.name": {
			Val:      "Ash",
			Metadata: map[string]any{"description": "Name of the Pokemon trainer"},
		},
		"pokedex/starter": {
			Val:      "pikachu",
			Metadata: map[string]any{"description": "Chosen starter Pokemon"},
		},
		"pokedex/region": {
			Val:      "kanto",
			Metadata: map[string]any{"description": "Home region of the trainer"},
		},
		"pokedex/pokedex.goal": {
			Val:      150,
			Metadata: map[string]any{"description": "Number of Pokemon to catch"},
		},
	}
}

// pokedexPlugin implements config.Plugin.
type pokedexPlugin struct {
	mu     sync.RWMutex
	values map[string]*config.Value
}

func newPokedexPlugin() *pokedexPlugin {
	return &pokedexPlugin{values: defaults()}
}

func (p *pokedexPlugin) List(_ context.Context) ([]string, error) {
	return paths, nil
}

func (p *pokedexPlugin) Get(_ context.Context, path string) (config.Value, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.values[path]
	if !ok {
		return config.Value{}, false, nil
	}
	return *v, true, nil
}

func (p *pokedexPlugin) Set(_ context.Context, path string, v config.Value) error {
	if err := config.ValidatePath(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.values[path] = &v
	return nil
}

func (p *pokedexPlugin) Validate(_ context.Context, path string, tree config.TreeReader) ([]config.ValidationResult, error) {
	v, ok := tree.Get(path)
	if !ok {
		return nil, nil
	}

	switch path {
	case "pokedex/trainer.name":
		return validateTrainerName(v)
	case "pokedex/starter":
		return validateStarter(v, tree)
	case "pokedex/region":
		return validateRegion(v)
	case "pokedex/pokedex.goal":
		return validateGoal(v)
	}
	return nil, nil
}

// --- validators -----------------------------------------------------------

func validateTrainerName(v config.Value) ([]config.ValidationResult, error) {
	name, ok := v.Val.(string)
	if !ok {
		return []config.ValidationResult{{
			Severity: config.Blocking,
			Message:  "trainer name must be a string",
		}}, nil
	}
	if name == "" {
		return []config.ValidationResult{{
			Severity: config.Blocking,
			Message:  "every trainer needs a name!",
		}}, nil
	}
	return nil, nil
}

func validateStarter(v config.Value, tree config.TreeReader) ([]config.ValidationResult, error) {
	starter, ok := v.Val.(string)
	if !ok {
		return []config.ValidationResult{{
			Severity: config.Blocking,
			Message:  "starter must be a string",
		}}, nil
	}

	var results []config.ValidationResult

	// Single-value check: must be one of the classic Kanto starters.
	if !slices.Contains(validStarters, starter) {
		results = append(results, config.ValidationResult{
			Severity: config.Warning,
			Message:  fmt.Sprintf("%q is not a classic Kanto starter -- Professor Oak might raise an eyebrow", starter),
			Metadata: map[string]any{"allowed": validStarters},
		})
	}

	// Cross-value check: warn if the starter doesn't match the region.
	if region, found := tree.Get("pokedex/region"); found {
		r, _ := region.Val.(string)
		if starters, known := regionStarters[r]; known && !slices.Contains(starters, starter) {
			results = append(results, config.ValidationResult{
				Severity: config.Info,
				Message:  fmt.Sprintf("%q is not a typical starter in the %s region", starter, r),
			})
		}
	}

	return results, nil
}

func validateRegion(v config.Value) ([]config.ValidationResult, error) {
	region, ok := v.Val.(string)
	if !ok {
		return []config.ValidationResult{{
			Severity: config.Blocking,
			Message:  "region must be a string",
		}}, nil
	}
	if !slices.Contains(validRegions, region) {
		return []config.ValidationResult{{
			Severity: config.Blocking,
			Message:  fmt.Sprintf("unknown region %q -- are you sure that's in the Pokemon world?", region),
			Metadata: map[string]any{"allowed": validRegions},
		}}, nil
	}
	return nil, nil
}

func validateGoal(v config.Value) ([]config.ValidationResult, error) {
	// JSON numbers unmarshal as float64 when going through any.
	goal, ok := v.Val.(float64)
	if !ok {
		return []config.ValidationResult{{
			Severity: config.Blocking,
			Message:  "pokedex goal must be a number",
		}}, nil
	}
	if goal < 1 {
		return []config.ValidationResult{{
			Severity: config.Blocking,
			Message:  "you need to catch at least 1 Pokemon!",
		}}, nil
	}
	if goal > 1025 {
		return []config.ValidationResult{{
			Severity: config.Warning,
			Message:  "that's more Pokemon than currently exist -- ambitious!",
		}}, nil
	}
	return nil, nil
}

// --- main -----------------------------------------------------------------

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"config": &config.GRPCPlugin{Impl: newPokedexPlugin()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
