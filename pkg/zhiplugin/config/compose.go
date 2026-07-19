package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// MountedPlugin associates a config plugin with a path prefix.
type MountedPlugin struct {
	// Impl is the underlying config plugin.
	Impl Plugin
	// Prefix is the path prefix for this plugin (e.g., "app-a/").
	// Must end with "/" to represent a namespace.
	Prefix string
}

// MergedPlugin combines multiple config plugins under distinct path
// prefixes. Each child's paths are prefixed with its mount point.
// Returns an error if any prefixes overlap or if a prefix is empty.
func MergedPlugin(children ...MountedPlugin) (Plugin, error) {
	if len(children) == 0 {
		return nil, fmt.Errorf("MergedPlugin: at least one child is required")
	}

	// Validate prefixes don't overlap.
	for i, a := range children {
		if a.Prefix == "" {
			return nil, fmt.Errorf("MergedPlugin: child %d has an empty prefix", i)
		}
		if !strings.HasSuffix(a.Prefix, "/") {
			return nil, fmt.Errorf("MergedPlugin: child %d prefix %q must end with '/'", i, a.Prefix)
		}
		if a.Impl == nil {
			return nil, fmt.Errorf("MergedPlugin: child %d (prefix %q) has nil Impl", i, a.Prefix)
		}
		for j, b := range children {
			if i == j {
				continue
			}
			if strings.HasPrefix(a.Prefix, b.Prefix) || strings.HasPrefix(b.Prefix, a.Prefix) {
				return nil, fmt.Errorf("MergedPlugin: overlapping prefixes %q and %q", a.Prefix, b.Prefix)
			}
		}
	}

	return &mergedPlugin{children: children}, nil
}

type mergedPlugin struct {
	children []MountedPlugin
}

// List returns the union of all children's paths, each prefixed with the
// child's mount point. Children are queried in parallel.
func (m *mergedPlugin) List(ctx context.Context) ([]string, error) {
	type result struct {
		paths []string
		err   error
	}

	results := make([]result, len(m.children))
	var wg sync.WaitGroup
	wg.Add(len(m.children))

	for i, child := range m.children {
		go func(idx int, c MountedPlugin) {
			defer wg.Done()
			paths, err := c.Impl.List(ctx)
			results[idx] = result{paths: paths, err: err}
		}(i, child)
	}

	wg.Wait()

	var all []string
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("MergedPlugin: listing child %q: %w", m.children[i].Prefix, r.err)
		}
		for _, p := range r.paths {
			all = append(all, m.children[i].Prefix+p)
		}
	}
	return all, nil
}

// Get routes by prefix to the correct child.
func (m *mergedPlugin) Get(ctx context.Context, path string) (Value, bool, error) {
	for _, child := range m.children {
		if strings.HasPrefix(path, child.Prefix) {
			return child.Impl.Get(ctx, strings.TrimPrefix(path, child.Prefix))
		}
	}
	return Value{}, false, nil
}

// Set routes by prefix to the correct child.
func (m *mergedPlugin) Set(ctx context.Context, path string, v Value) error {
	for _, child := range m.children {
		if strings.HasPrefix(path, child.Prefix) {
			return child.Impl.Set(ctx, strings.TrimPrefix(path, child.Prefix), v)
		}
	}
	return fmt.Errorf("MergedPlugin: no child manages path %q", path)
}

// Validate routes by prefix to the correct child. The child receives
// a filtered TreeReader scoped to its own prefix.
func (m *mergedPlugin) Validate(ctx context.Context, path string, tree TreeReader) ([]ValidationResult, error) {
	for _, child := range m.children {
		if strings.HasPrefix(path, child.Prefix) {
			subTree := &prefixedTreeReader{tree: tree, prefix: child.Prefix}
			return child.Impl.Validate(ctx, strings.TrimPrefix(path, child.Prefix), subTree)
		}
	}
	return nil, nil
}

// prefixedTreeReader adapts a TreeReader by stripping a prefix from paths.
type prefixedTreeReader struct {
	tree   TreeReader
	prefix string
}

func (p *prefixedTreeReader) Get(path string) (Value, bool) {
	return p.tree.Get(p.prefix + path)
}

func (p *prefixedTreeReader) List() []string {
	var result []string
	for _, path := range p.tree.List() {
		if strings.HasPrefix(path, p.prefix) {
			result = append(result, strings.TrimPrefix(path, p.prefix))
		}
	}
	return result
}
