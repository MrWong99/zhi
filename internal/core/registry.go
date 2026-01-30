// Package core provides the central orchestration layer for zhi, connecting
// providers (config, transform, store) into a coherent lifecycle.
package core

import (
	"fmt"
	"sort"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// ConfigFactory constructs a config provider. The options map comes from
// the workspace configuration file.
type ConfigFactory func(options map[string]any) (config.Plugin, error)

// TransformFactory constructs a transform provider.
type TransformFactory func(options map[string]any) (transform.Plugin, error)

// StoreFactory constructs a store provider.
type StoreFactory func(options map[string]any) (store.Plugin, error)

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

	// External plugin discovery.
	externalPlugins []PluginInfo

	// Cached external plugin instances (lazy-loaded).
	mu              sync.Mutex
	cachedConfig    map[string]config.Plugin
	cachedTransform map[string]transform.Plugin
	cachedStore     map[string]store.Plugin
	cleanups        []func()
}

// NewRegistry returns an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		config:          make(map[string]ConfigFactory),
		transform:       make(map[string]TransformFactory),
		store:           make(map[string]StoreFactory),
		cachedConfig:    make(map[string]config.Plugin),
		cachedTransform: make(map[string]transform.Plugin),
		cachedStore:     make(map[string]store.Plugin),
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

// ConfigProvider resolves and instantiates a config provider by name.
// Built-in providers take precedence. If no built-in is found, discovered
// external plugins are checked and launched lazily.
func (r *Registry) ConfigProvider(name string, options map[string]any) (config.Plugin, error) {
	if factory, ok := r.config[name]; ok {
		return factory(options)
	}
	return r.launchExternalConfig(name)
}

// TransformProvider resolves and instantiates a transform provider by name.
func (r *Registry) TransformProvider(name string, options map[string]any) (transform.Plugin, error) {
	if factory, ok := r.transform[name]; ok {
		return factory(options)
	}
	return r.launchExternalTransform(name)
}

// StoreProvider resolves and instantiates a store provider by name.
func (r *Registry) StoreProvider(name string, options map[string]any) (store.Plugin, error) {
	if factory, ok := r.store[name]; ok {
		return factory(options)
	}
	return r.launchExternalStore(name)
}

// ListConfig returns the sorted names of all registered config providers
// (built-in and external).
func (r *Registry) ListConfig() []string {
	names := sortedKeys(r.config)
	for _, p := range r.externalPlugins {
		if p.Type == PluginTypeConfig && !contains(names, p.Name) {
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
}

// ListTransform returns the sorted names of all registered transform
// providers (built-in and external).
func (r *Registry) ListTransform() []string {
	names := sortedKeys(r.transform)
	for _, p := range r.externalPlugins {
		if p.Type == PluginTypeTransform && !contains(names, p.Name) {
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
}

// ListStore returns the sorted names of all registered store providers
// (built-in and external).
func (r *Registry) ListStore() []string {
	names := sortedKeys(r.store)
	for _, p := range r.externalPlugins {
		if p.Type == PluginTypeStore && !contains(names, p.Name) {
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
}

// ListConfigProviders returns detailed provider info including source.
func (r *Registry) ListConfigProviders() []ProviderInfo {
	var infos []ProviderInfo
	for _, name := range sortedKeys(r.config) {
		infos = append(infos, ProviderInfo{Name: name, Source: "built-in"})
	}
	for _, p := range r.externalPlugins {
		if p.Type == PluginTypeConfig && !r.isBuiltinConfig(p.Name) {
			infos = append(infos, ProviderInfo{Name: p.Name, Source: p.Path})
		}
	}
	return infos
}

// ListTransformProviders returns detailed provider info including source.
func (r *Registry) ListTransformProviders() []ProviderInfo {
	var infos []ProviderInfo
	for _, name := range sortedKeys(r.transform) {
		infos = append(infos, ProviderInfo{Name: name, Source: "built-in"})
	}
	for _, p := range r.externalPlugins {
		if p.Type == PluginTypeTransform && !r.isBuiltinTransform(p.Name) {
			infos = append(infos, ProviderInfo{Name: p.Name, Source: p.Path})
		}
	}
	return infos
}

// ListStoreProviders returns detailed provider info including source.
func (r *Registry) ListStoreProviders() []ProviderInfo {
	var infos []ProviderInfo
	for _, name := range sortedKeys(r.store) {
		infos = append(infos, ProviderInfo{Name: name, Source: "built-in"})
	}
	for _, p := range r.externalPlugins {
		if p.Type == PluginTypeStore && !r.isBuiltinStore(p.Name) {
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

func (r *Registry) isBuiltinConfig(name string) bool {
	_, ok := r.config[name]
	return ok
}

func (r *Registry) isBuiltinTransform(name string) bool {
	_, ok := r.transform[name]
	return ok
}

func (r *Registry) isBuiltinStore(name string) bool {
	_, ok := r.store[name]
	return ok
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
