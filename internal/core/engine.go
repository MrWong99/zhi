package core

import (
	"context"
	"fmt"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// Engine is the central orchestrator. It holds references to resolved
// providers and exposes high-level operations on the configuration tree.
type Engine struct {
	registry         *Registry
	workspace        *WorkspaceConfig
	configPlugin     config.Plugin
	transformPlugins []transform.Plugin
	storePlugin      store.Plugin // nil if no store is configured
	components       *ComponentManager
}

// NewEngine resolves all providers from the workspace config using the
// registry, initializes the ComponentManager, and returns a ready engine.
func NewEngine(registry *Registry, workspace *WorkspaceConfig) (*Engine, error) {
	e := &Engine{
		registry:  registry,
		workspace: workspace,
	}

	// Resolve config provider.
	cp, err := registry.ConfigProvider(workspace.Config.Provider, workspace.Config.Options)
	if err != nil {
		return nil, fmt.Errorf("resolving config provider: %w", err)
	}
	e.configPlugin = cp

	// Resolve transform providers.
	for _, t := range workspace.Transform {
		tp, err := registry.TransformProvider(t.Provider, t.Options)
		if err != nil {
			return nil, fmt.Errorf("resolving transform provider %q: %w", t.Provider, err)
		}
		e.transformPlugins = append(e.transformPlugins, tp)
	}

	// Resolve store provider (optional).
	if workspace.Store.Provider != "" {
		sp, err := registry.StoreProvider(workspace.Store.Provider, workspace.Store.Options)
		if err != nil {
			return nil, fmt.Errorf("resolving store provider: %w", err)
		}
		e.storePlugin = sp
	}

	// Initialize component manager.
	cm, err := NewComponentManager(workspace.Components)
	if err != nil {
		return nil, fmt.Errorf("initializing components: %w", err)
	}
	e.components = cm

	return e, nil
}

// LoadTree assembles a configuration tree by calling List() and then Get()
// for each path on the config provider.
func (e *Engine) LoadTree(ctx context.Context) (*config.Tree, error) {
	paths, err := e.configPlugin.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing config paths: %w", err)
	}

	tree := config.NewTree()
	for _, path := range paths {
		v, found, err := e.configPlugin.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("getting config value at %q: %w", path, err)
		}
		if !found {
			continue
		}
		if err := tree.Set(path, &v); err != nil {
			return nil, fmt.Errorf("setting tree value at %q: %w", path, err)
		}
	}
	return tree, nil
}

// TransformForDisplay applies all transform plugins' BeforeDisplay
// operations to the tree in order.
func (e *Engine) TransformForDisplay(ctx context.Context, tree *config.Tree) error {
	for _, tp := range e.transformPlugins {
		if err := tp.BeforeDisplay(ctx, tree); err != nil {
			return fmt.Errorf("transform BeforeDisplay: %w", err)
		}
	}
	return nil
}

// TransformForSave applies all transform plugins' AfterSave operations
// to the tree in order.
func (e *Engine) TransformForSave(ctx context.Context, tree *config.Tree) error {
	for _, tp := range e.transformPlugins {
		if err := tp.AfterSave(ctx, tree); err != nil {
			return fmt.Errorf("transform AfterSave: %w", err)
		}
	}
	return nil
}

// Validate runs config provider validation for each path in the tree and
// also validates component dependency constraints. The returned results
// include both config validation findings and component dependency errors
// (reported as Blocking severity under the special path
// "_components/<name>").
func (e *Engine) Validate(ctx context.Context, tree *config.Tree) ([]config.ValidationResult, error) {
	var results []config.ValidationResult

	for _, path := range tree.List() {
		vr, err := e.configPlugin.Validate(ctx, path, tree)
		if err != nil {
			return nil, fmt.Errorf("validating %q: %w", path, err)
		}
		results = append(results, vr...)
	}

	// Validate component dependency constraints.
	for _, depErr := range e.components.ValidateDependencies() {
		results = append(results, config.ValidationResult{
			Severity: config.Blocking,
			Message:  depErr.Error(),
		})
	}

	return results, nil
}

// SetValue stores a value at the given path via the config provider.
func (e *Engine) SetValue(ctx context.Context, path string, value config.Value) error {
	return e.configPlugin.Set(ctx, path, value)
}

// SaveTree persists a tree to the store provider. It also persists the
// current component state. Returns an error if no store is configured.
func (e *Engine) SaveTree(ctx context.Context, id string, tree *config.Tree) error {
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.Save(ctx, id, tree)
}

// LoadStoredTree loads a tree from the store provider. Returns an error if
// no store is configured.
func (e *Engine) LoadStoredTree(ctx context.Context, id string) (*config.Tree, bool, error) {
	if e.storePlugin == nil {
		return nil, false, fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.Load(ctx, id)
}

// ListTrees returns all tree IDs from the store provider. Returns an error
// if no store is configured.
func (e *Engine) ListTrees(ctx context.Context) ([]string, error) {
	if e.storePlugin == nil {
		return nil, fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.ListTrees(ctx)
}

// Components returns the component manager for UI and CLI access.
func (e *Engine) Components() *ComponentManager {
	return e.components
}

// FilteredTree loads the configuration tree and filters it to only include
// paths belonging to enabled components (plus unmanaged paths).
func (e *Engine) FilteredTree(ctx context.Context) (*config.Tree, error) {
	tree, err := e.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	return e.components.FilterTree(tree), nil
}

// StorePlugin returns the resolved store plugin, or nil if no store is
// configured.
func (e *Engine) StorePlugin() store.Plugin {
	return e.storePlugin
}

// WorkspaceDir returns the directory containing the workspace config file.
func (e *Engine) WorkspaceDir() string {
	if e.workspace == nil {
		return ""
	}
	return e.workspace.Dir
}

// ValidatePath runs config provider validation for a single path in the tree.
func (e *Engine) ValidatePath(ctx context.Context, path string, tree *config.Tree) ([]config.ValidationResult, error) {
	return e.configPlugin.Validate(ctx, path, tree)
}

// Workspace returns the workspace configuration.
func (e *Engine) Workspace() *WorkspaceConfig {
	return e.workspace
}

// SetTestWorkspaceDir sets the workspace directory for testing purposes.
// This should only be used in tests.
func (e *Engine) SetTestWorkspaceDir(dir string) {
	if e.workspace == nil {
		e.workspace = &WorkspaceConfig{}
	}
	e.workspace.Dir = dir
}
