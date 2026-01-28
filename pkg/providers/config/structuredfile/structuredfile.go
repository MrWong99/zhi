// Package structuredfile implements a zhi configuration plugin that loads
// values from JSON and YAML files in a predefined directory.
//
// Files are read from ./structuredfile relative to the working directory.
// The nested structure inside each file maps to slash-delimited configuration
// paths. A node is a leaf (configuration entry) when it contains a "val" key.
//
// YAML files may additionally carry a "validation" field with a Go function
// body that is compiled at load time using the Yaegi interpreter.
package structuredfile

import (
	"context"
	"sort"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// DefaultDir is the directory from which structured config files are loaded.
const DefaultDir = "./structuredfile"

// storedValue holds a config.Value together with an optional compiled
// validation function.
type storedValue struct {
	value    config.Value
	validate validateFunc // nil when no validation code was provided
}

// Plugin implements config.Plugin by reading configuration from structured
// JSON and YAML files.
type Plugin struct {
	mu     sync.RWMutex
	values map[string]*storedValue
	paths  []string // sorted for deterministic List output
}

// New creates a Plugin and loads configuration from DefaultDir.
func New() (*Plugin, error) {
	return NewFromDir(DefaultDir)
}

// NewFromDir creates a Plugin and loads configuration from the given
// directory. If dir does not exist the plugin starts with no values.
func NewFromDir(dir string) (*Plugin, error) {
	p := &Plugin{values: make(map[string]*storedValue)}
	if err := p.load(dir); err != nil {
		return nil, err
	}
	return p, nil
}

// load reads all config files from dir and populates the plugin's values.
func (p *Plugin) load(dir string) error {
	entries, err := loadDir(dir)
	if err != nil {
		return err
	}

	for path, e := range entries {
		sv := &storedValue{
			value: config.Value{
				Val:      e.Val,
				Metadata: e.Metadata,
			},
		}

		if e.ValidationSource != "" {
			fn, err := compileValidation(path, e.ValidationSource, e.Imports)
			if err != nil {
				return err
			}
			sv.validate = fn
		}

		p.values[path] = sv
	}

	p.paths = make([]string, 0, len(p.values))
	for path := range p.values {
		p.paths = append(p.paths, path)
	}
	sort.Strings(p.paths)

	return nil
}

// List returns every configuration path managed by this plugin.
func (p *Plugin) List(_ context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, len(p.paths))
	copy(out, p.paths)
	return out, nil
}

// Get retrieves a single configuration value by path.
func (p *Plugin) Get(_ context.Context, path string) (config.Value, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	sv, ok := p.values[path]
	if !ok {
		return config.Value{}, false, nil
	}
	return sv.value, true, nil
}

// Set stores a configuration value at the given path. The change is
// in-memory only; it is not written back to disk.
func (p *Plugin) Set(_ context.Context, path string, v config.Value) error {
	if err := config.ValidatePath(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if sv, ok := p.values[path]; ok {
		sv.value = v
	} else {
		p.values[path] = &storedValue{value: v}
		p.paths = append(p.paths, path)
		sort.Strings(p.paths)
	}
	return nil
}

// Validate checks the value at path using the compiled validation function
// loaded from the file. If no validation code was provided for the path the
// value is considered valid.
func (p *Plugin) Validate(_ context.Context, path string, tree config.TreeReader) ([]config.ValidationResult, error) {
	p.mu.RLock()
	sv, ok := p.values[path]
	p.mu.RUnlock()

	if !ok {
		return nil, nil
	}
	if sv.validate == nil {
		return nil, nil
	}

	v, found := tree.Get(path)
	if !found {
		return nil, nil
	}

	return sv.validate(v, tree)
}
