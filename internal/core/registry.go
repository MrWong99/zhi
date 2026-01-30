// Package core provides the central orchestration layer for zhi, connecting
// providers (config, transform, store) into a coherent lifecycle.
package core

import (
	"fmt"
	"sort"

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

// Registry maps provider names to factory functions for each plugin type.
// It is populated at startup with compiled-in providers and used by the
// engine to resolve workspace provider references.
type Registry struct {
	config    map[string]ConfigFactory
	transform map[string]TransformFactory
	store     map[string]StoreFactory
}

// NewRegistry returns an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		config:    make(map[string]ConfigFactory),
		transform: make(map[string]TransformFactory),
		store:     make(map[string]StoreFactory),
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
func (r *Registry) ConfigProvider(name string, options map[string]any) (config.Plugin, error) {
	factory, ok := r.config[name]
	if !ok {
		return nil, fmt.Errorf("unknown config provider: %q", name)
	}
	return factory(options)
}

// TransformProvider resolves and instantiates a transform provider by name.
func (r *Registry) TransformProvider(name string, options map[string]any) (transform.Plugin, error) {
	factory, ok := r.transform[name]
	if !ok {
		return nil, fmt.Errorf("unknown transform provider: %q", name)
	}
	return factory(options)
}

// StoreProvider resolves and instantiates a store provider by name.
func (r *Registry) StoreProvider(name string, options map[string]any) (store.Plugin, error) {
	factory, ok := r.store[name]
	if !ok {
		return nil, fmt.Errorf("unknown store provider: %q", name)
	}
	return factory(options)
}

// ListConfig returns the sorted names of all registered config providers.
func (r *Registry) ListConfig() []string {
	return sortedKeys(r.config)
}

// ListTransform returns the sorted names of all registered transform providers.
func (r *Registry) ListTransform() []string {
	return sortedKeys(r.transform)
}

// ListStore returns the sorted names of all registered store providers.
func (r *Registry) ListStore() []string {
	return sortedKeys(r.store)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
