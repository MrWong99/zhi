// Package core provides the central orchestration layer for zhi, connecting
// providers (config, transform, store) into a coherent lifecycle.
package core

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// ConfigFactory constructs a config provider. The workspace is the path to the
// workspace directory. The options map comes from the workspace configuration file.
type ConfigFactory func(workspace string, options map[string]any) (config.Plugin, error)

// TransformFactory constructs a transform provider. The workspace is the path to the
// workspace directory. The options map comes from the workspace configuration file.
type TransformFactory func(workspace string, options map[string]any) (transform.Plugin, error)

// StoreFactory constructs a store provider. The workspace is the path to the
// workspace directory. The options map comes from the workspace configuration file.
type StoreFactory func(workspace string, options map[string]any) (store.Plugin, error)

// UIFactory constructs a UI provider. The workspace is the path to the
// workspace directory. The options map comes from the workspace configuration file.
type UIFactory func(workspace string, options map[string]any) (zhiui.Plugin, error)

// ProviderInfo describes a registered provider for display purposes.
type ProviderInfo struct {
	Name   string // provider name
	Source string // "built-in" or path to external binary
}

// Registry maps provider names to factory functions for each plugin type.
// It is populated at startup with compiled-in providers and used by the
// engine to resolve workspace provider references. It also supports lazy
// loading of external plugins discovered on disk.
type Registry struct {
	config    map[string]ConfigFactory
	transform map[string]TransformFactory
	store     map[string]StoreFactory
	ui        map[string]UIFactory

	// External plugin discovery.
	externalPlugins []PluginInfo

	// Cached external plugin instances (lazy-loaded).
	mu              sync.Mutex
	cachedConfig    map[string]config.Plugin
	cachedTransform map[string]transform.Plugin
	cachedStore     map[string]store.Plugin
	cachedUI        map[string]zhiui.Plugin
	cleanups        []func()
}

// NewRegistry returns an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		config:          make(map[string]ConfigFactory),
		transform:       make(map[string]TransformFactory),
		store:           make(map[string]StoreFactory),
		ui:              make(map[string]UIFactory),
		cachedConfig:    make(map[string]config.Plugin),
		cachedTransform: make(map[string]transform.Plugin),
		cachedStore:     make(map[string]store.Plugin),
		cachedUI:        make(map[string]zhiui.Plugin),
	}
}

// RegisterConfig registers a built-in config provider factory under the
// given name. It returns an error if the name is already taken.
func (r *Registry) RegisterConfig(name string, factory ConfigFactory) error {
	if _, exists := r.config[name]; exists {
		return fmt.Errorf("config provider %q already registered", name)
	}
	r.config[name] = factory
	return nil
}

// RegisterTransform registers a built-in transform provider factory under
// the given name. It returns an error if the name is already taken.
func (r *Registry) RegisterTransform(name string, factory TransformFactory) error {
	if _, exists := r.transform[name]; exists {
		return fmt.Errorf("transform provider %q already registered", name)
	}
	r.transform[name] = factory
	return nil
}

// RegisterStore registers a built-in store provider factory under the given
// name. It returns an error if the name is already taken.
func (r *Registry) RegisterStore(name string, factory StoreFactory) error {
	if _, exists := r.store[name]; exists {
		return fmt.Errorf("store provider %q already registered", name)
	}
	r.store[name] = factory
	return nil
}

// RegisterUI registers a built-in UI provider factory under the given
// name. It returns an error if the name is already taken.
func (r *Registry) RegisterUI(name string, factory UIFactory) error {
	if _, exists := r.ui[name]; exists {
		return fmt.Errorf("UI provider %q already registered", name)
	}
	r.ui[name] = factory
	return nil
}

// ConfigProvider resolves and instantiates a config provider by name.
// Built-in providers take precedence. If no built-in is found, discovered
// external plugins are checked and launched lazily.
func (r *Registry) ConfigProvider(workspace, name string, options map[string]any) (config.Plugin, error) {
	if factory, ok := r.config[name]; ok {
		return factory(workspace, options)
	}
	return r.launchExternalConfig(name)
}

// TransformProvider resolves and instantiates a transform provider by name.
func (r *Registry) TransformProvider(workspace, name string, options map[string]any) (transform.Plugin, error) {
	if factory, ok := r.transform[name]; ok {
		return factory(workspace, options)
	}
	return r.launchExternalTransform(name)
}

// StoreProvider resolves and instantiates a store provider by name.
func (r *Registry) StoreProvider(workspace, name string, options map[string]any) (store.Plugin, error) {
	if factory, ok := r.store[name]; ok {
		return factory(workspace, options)
	}
	return r.launchExternalStore(name)
}

// UIProvider resolves and instantiates a UI provider by name.
// Built-in providers take precedence. If no built-in is found, discovered
// external plugins are checked and launched lazily.
func (r *Registry) UIProvider(workspace, name string, options map[string]any) (zhiui.Plugin, error) {
	if factory, ok := r.ui[name]; ok {
		return factory(workspace, options)
	}
	return r.launchExternalUI(name)
}

// ListConfig returns the sorted names of all registered config providers
// (built-in and external).
func (r *Registry) ListConfig() []string {
	return listNames(r, r.config, PluginTypeConfig)
}

// ListTransform returns the sorted names of all registered transform
// providers (built-in and external).
func (r *Registry) ListTransform() []string {
	return listNames(r, r.transform, PluginTypeTransform)
}

// ListStore returns the sorted names of all registered store providers
// (built-in and external).
func (r *Registry) ListStore() []string {
	return listNames(r, r.store, PluginTypeStore)
}

// ListUI returns the sorted names of all registered UI providers
// (built-in and external).
func (r *Registry) ListUI() []string {
	return listNames(r, r.ui, PluginTypeUI)
}

// ListConfigProviders returns detailed provider info including source.
func (r *Registry) ListConfigProviders() []ProviderInfo {
	return listProviderInfos(r, r.config, PluginTypeConfig)
}

// ListTransformProviders returns detailed provider info including source.
func (r *Registry) ListTransformProviders() []ProviderInfo {
	return listProviderInfos(r, r.transform, PluginTypeTransform)
}

// ListStoreProviders returns detailed provider info including source.
func (r *Registry) ListStoreProviders() []ProviderInfo {
	return listProviderInfos(r, r.store, PluginTypeStore)
}

// ListUIProviders returns detailed provider info including source.
func (r *Registry) ListUIProviders() []ProviderInfo {
	return listProviderInfos(r, r.ui, PluginTypeUI)
}

// listNames returns the sorted names of all built-in and external providers
// of the given type.
func listNames[V any](r *Registry, builtins map[string]V, pt PluginType) []string {
	names := sortedKeys(builtins)
	for _, p := range r.externalPlugins {
		if p.Type == pt && !slices.Contains(names, p.Name) {
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
}

// listProviderInfos returns detailed info for all built-in and external
// providers of the given type.
func listProviderInfos[V any](r *Registry, builtins map[string]V, pt PluginType) []ProviderInfo {
	var infos []ProviderInfo
	for _, name := range sortedKeys(builtins) {
		infos = append(infos, ProviderInfo{Name: name, Source: "built-in"})
	}
	for _, p := range r.externalPlugins {
		if p.Type == pt && !isBuiltin(builtins, p.Name) {
			infos = append(infos, ProviderInfo{Name: p.Name, Source: p.Path})
		}
	}
	return infos
}

// RefreshExternal re-scans plugin directories and updates the list of
// discovered external plugins.
func (r *Registry) RefreshExternal(cfg DiscoveryConfig) error {
	plugins, err := Discover(cfg)
	if err != nil {
		return fmt.Errorf("discovering external plugins: %w", err)
	}
	r.mu.Lock()
	r.externalPlugins = plugins
	r.mu.Unlock()
	return nil
}

// SetExternalPlugins sets the discovered external plugins directly.
// This is useful when discovery has already been performed.
func (r *Registry) SetExternalPlugins(plugins []PluginInfo) {
	r.mu.Lock()
	r.externalPlugins = plugins
	r.mu.Unlock()
}

// Close kills all launched external plugin processes and clears caches.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cleanup := range r.cleanups {
		cleanup()
	}
	r.cleanups = nil
	r.cachedConfig = make(map[string]config.Plugin)
	r.cachedTransform = make(map[string]transform.Plugin)
	r.cachedStore = make(map[string]store.Plugin)
	r.cachedUI = make(map[string]zhiui.Plugin)
}

// launchExternalConfig finds and launches an external config plugin.
func (r *Registry) launchExternalConfig(name string) (config.Plugin, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache first.
	if p, ok := r.cachedConfig[name]; ok {
		return p, nil
	}

	// Find the plugin in discovered list.
	info, ok := r.findExternal(name, PluginTypeConfig)
	if !ok {
		return nil, fmt.Errorf("unknown config provider: %q", name)
	}

	p, cleanup, err := LaunchConfig(info.Path)
	if err != nil {
		return nil, fmt.Errorf("launching external config plugin %q: %w", name, err)
	}

	r.cachedConfig[name] = p
	r.cleanups = append(r.cleanups, cleanup)
	return p, nil
}

// launchExternalTransform finds and launches an external transform plugin.
func (r *Registry) launchExternalTransform(name string) (transform.Plugin, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, ok := r.cachedTransform[name]; ok {
		return p, nil
	}

	info, ok := r.findExternal(name, PluginTypeTransform)
	if !ok {
		return nil, fmt.Errorf("unknown transform provider: %q", name)
	}

	p, cleanup, err := LaunchTransform(info.Path)
	if err != nil {
		return nil, fmt.Errorf("launching external transform plugin %q: %w", name, err)
	}

	r.cachedTransform[name] = p
	r.cleanups = append(r.cleanups, cleanup)
	return p, nil
}

// launchExternalStore finds and launches an external store plugin.
func (r *Registry) launchExternalStore(name string) (store.Plugin, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, ok := r.cachedStore[name]; ok {
		return p, nil
	}

	info, ok := r.findExternal(name, PluginTypeStore)
	if !ok {
		return nil, fmt.Errorf("unknown store provider: %q", name)
	}

	p, cleanup, err := LaunchStore(info.Path)
	if err != nil {
		return nil, fmt.Errorf("launching external store plugin %q: %w", name, err)
	}

	r.cachedStore[name] = p
	r.cleanups = append(r.cleanups, cleanup)
	return p, nil
}

// launchExternalUI finds and launches an external UI plugin.
func (r *Registry) launchExternalUI(name string) (zhiui.Plugin, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, ok := r.cachedUI[name]; ok {
		return p, nil
	}

	info, ok := r.findExternal(name, PluginTypeUI)
	if !ok {
		return nil, fmt.Errorf("unknown UI provider: %q", name)
	}

	p, cleanup, err := LaunchUI(info.Path)
	if err != nil {
		return nil, fmt.Errorf("launching external UI plugin %q: %w", name, err)
	}

	r.cachedUI[name] = p
	r.cleanups = append(r.cleanups, cleanup)
	return p, nil
}

// findExternal searches the discovered plugins for a match. Must be called
// with r.mu held.
func (r *Registry) findExternal(name string, pt PluginType) (PluginInfo, bool) {
	for _, p := range r.externalPlugins {
		if p.Name == name && p.Type == pt {
			return p, true
		}
	}
	return PluginInfo{}, false
}

// isBuiltin reports whether the given name is registered as a built-in
// provider in the provided factory map.
func isBuiltin[V any](m map[string]V, name string) bool {
	_, ok := m[name]
	return ok
}

func sortedKeys[V any](m map[string]V) []string {
	keys := slices.Sorted(maps.Keys(m))
	return keys
}
