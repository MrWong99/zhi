package core

import (
	"fmt"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// ComponentDef is a component definition from the workspace configuration.
type ComponentDef struct {
	Name         string   `yaml:"name" json:"name"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	Paths        []string `yaml:"paths" json:"paths"`
	Mandatory    bool     `yaml:"mandatory,omitempty" json:"mandatory,omitempty"`
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
}

// ComponentState is the runtime state of a component.
type ComponentState struct {
	Name         string
	Enabled      bool
	Mandatory    bool
	Dependencies []string
}

// ComponentManager manages component definitions and their enabled/disabled state.
type ComponentManager struct {
	definitions []ComponentDef
	state       map[string]bool
	defsByName  map[string]*ComponentDef
}

// NewComponentManager validates the given definitions and returns a new
// manager. Mandatory components start enabled. Validation includes unique
// name checks, cycle detection, and overlapping path prefix checks.
func NewComponentManager(defs []ComponentDef) (*ComponentManager, error) {
	cm := &ComponentManager{
		definitions: defs,
		state:       make(map[string]bool, len(defs)),
		defsByName:  make(map[string]*ComponentDef, len(defs)),
	}

	// Check unique names.
	for i := range defs {
		d := &cm.definitions[i]
		if _, exists := cm.defsByName[d.Name]; exists {
			return nil, fmt.Errorf("duplicate component name: %q", d.Name)
		}
		cm.defsByName[d.Name] = d
	}

	// Check that all dependency references are valid.
	for _, d := range defs {
		for _, dep := range d.Dependencies {
			if _, ok := cm.defsByName[dep]; !ok {
				return nil, fmt.Errorf("component %q depends on unknown component %q", d.Name, dep)
			}
		}
	}

	// Check for cycles using topological sort (Kahn's algorithm).
	if err := cm.detectCycles(); err != nil {
		return nil, err
	}

	// Check for overlapping path prefixes.
	if err := cm.checkOverlappingPaths(); err != nil {
		return nil, err
	}

	// Mandatory components start enabled; others start disabled.
	for _, d := range defs {
		cm.state[d.Name] = d.Mandatory
	}

	return cm, nil
}

// detectCycles checks for circular dependencies using Kahn's algorithm.
func (cm *ComponentManager) detectCycles() error {
	// Build in-degree counts and adjacency: dep -> dependents.
	inDegree := make(map[string]int, len(cm.definitions))
	dependents := make(map[string][]string, len(cm.definitions))
	for _, d := range cm.definitions {
		inDegree[d.Name] += 0 // ensure entry exists
		for _, dep := range d.Dependencies {
			dependents[dep] = append(dependents[dep], d.Name)
			inDegree[d.Name]++
		}
	}

	// Seed the queue with nodes having no dependencies.
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, dependent := range dependents[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if visited != len(cm.definitions) {
		return fmt.Errorf("circular dependency detected among components")
	}
	return nil
}

// checkOverlappingPaths rejects definitions where two components claim
// overlapping path prefixes.
func (cm *ComponentManager) checkOverlappingPaths() error {
	type ownership struct {
		prefix    string
		component string
	}
	var all []ownership
	for _, d := range cm.definitions {
		for _, p := range d.Paths {
			all = append(all, ownership{prefix: p, component: d.Name})
		}
	}
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			a, b := all[i], all[j]
			if a.component == b.component {
				continue
			}
			if prefixOverlaps(a.prefix, b.prefix) {
				return fmt.Errorf("overlapping path prefixes: component %q prefix %q overlaps with component %q prefix %q",
					a.component, a.prefix, b.component, b.prefix)
			}
		}
	}
	return nil
}

// prefixOverlaps returns true if either prefix contains the other.
func prefixOverlaps(a, b string) bool {
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// Enable enables a component and transitively enables its dependencies.
func (cm *ComponentManager) Enable(name string) error {
	if _, ok := cm.defsByName[name]; !ok {
		return fmt.Errorf("unknown component: %q", name)
	}
	cm.enableTransitive(name)
	return nil
}

func (cm *ComponentManager) enableTransitive(name string) {
	if cm.state[name] {
		return // already enabled
	}
	cm.state[name] = true
	def := cm.defsByName[name]
	for _, dep := range def.Dependencies {
		cm.enableTransitive(dep)
	}
}

// Disable disables a component. It returns an error if the component is
// mandatory or if other enabled components depend on it.
func (cm *ComponentManager) Disable(name string) error {
	def, ok := cm.defsByName[name]
	if !ok {
		return fmt.Errorf("unknown component: %q", name)
	}
	if def.Mandatory {
		return fmt.Errorf("cannot disable %q: component is mandatory", name)
	}
	// Check if any enabled component depends on this one.
	for _, d := range cm.definitions {
		if !cm.state[d.Name] {
			continue
		}
		for _, dep := range d.Dependencies {
			if dep == name {
				return fmt.Errorf("cannot disable %q: required by enabled component %q", name, d.Name)
			}
		}
	}
	cm.state[name] = false
	return nil
}

// IsEnabled reports whether the named component is currently enabled.
func (cm *ComponentManager) IsEnabled(name string) bool {
	return cm.state[name]
}

// ListComponents returns all components with their current state.
func (cm *ComponentManager) ListComponents() []ComponentState {
	out := make([]ComponentState, len(cm.definitions))
	for i, d := range cm.definitions {
		deps := make([]string, len(d.Dependencies))
		copy(deps, d.Dependencies)
		out[i] = ComponentState{
			Name:         d.Name,
			Enabled:      cm.state[d.Name],
			Mandatory:    d.Mandatory,
			Dependencies: deps,
		}
	}
	return out
}

// EnabledPaths returns all path prefixes belonging to enabled components.
func (cm *ComponentManager) EnabledPaths() []string {
	var paths []string
	for _, d := range cm.definitions {
		if cm.state[d.Name] {
			paths = append(paths, d.Paths...)
		}
	}
	return paths
}

// PathBelongsToComponent returns the component name that owns the given
// config path, if any.
func (cm *ComponentManager) PathBelongsToComponent(path string) (string, bool) {
	for _, d := range cm.definitions {
		for _, prefix := range d.Paths {
			if strings.HasPrefix(path, prefix) {
				return d.Name, true
			}
		}
	}
	return "", false
}

// FilterTree returns a new tree containing only paths that belong to
// enabled components or paths not claimed by any component.
func (cm *ComponentManager) FilterTree(tree *config.Tree) *config.Tree {
	filtered := config.NewTree()
	for _, path := range tree.List() {
		comp, owned := cm.PathBelongsToComponent(path)
		if !owned || cm.state[comp] {
			v, ok := tree.Get(path)
			if ok {
				// Ignore error; paths from tree.List() are already valid.
				_ = filtered.Set(path, &v)
			}
		}
	}
	return filtered
}

// ValidateDependencies checks that all dependency constraints are
// currently satisfied. It returns an error for each violation.
func (cm *ComponentManager) ValidateDependencies() []error {
	var errs []error
	for _, d := range cm.definitions {
		if !cm.state[d.Name] {
			continue
		}
		for _, dep := range d.Dependencies {
			if !cm.state[dep] {
				errs = append(errs, fmt.Errorf("component %q is enabled but its dependency %q is disabled", d.Name, dep))
			}
		}
	}
	return errs
}

// LoadState restores component enabled/disabled state from a previously
// saved map. Unknown names are silently ignored. Mandatory components
// are always forced enabled.
func (cm *ComponentManager) LoadState(state map[string]bool) {
	for name, enabled := range state {
		if def, ok := cm.defsByName[name]; ok {
			if def.Mandatory {
				cm.state[name] = true
			} else {
				cm.state[name] = enabled
			}
		}
	}
}

// SaveState exports the current component enabled/disabled state for
// persistence.
func (cm *ComponentManager) SaveState() map[string]bool {
	out := make(map[string]bool, len(cm.state))
	for k, v := range cm.state {
		out[k] = v
	}
	return out
}
