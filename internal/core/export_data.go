package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// IterateData is the template dot value when an export uses the iterate feature.
// The template receives one IterateData per direct child of the iterate prefix.
type IterateData struct {
	// Key is the name of the direct child (e.g. "webservers").
	Key string
	// Value is a TreeData scoped to the child's subtree.
	Value *TreeData
}

// TreeData wraps a filtered TreeReader and ComponentManager to provide
// template-friendly access methods for use inside Go templates.
type TreeData struct {
	tree       config.TreeReader
	components *ComponentManager
}

// NewTreeData creates a TreeData wrapper. The tree should already be filtered
// through the ComponentManager (unless --all-components is in effect).
func NewTreeData(tree config.TreeReader, cm *ComponentManager) *TreeData {
	return &TreeData{tree: tree, components: cm}
}

// Get returns the value at path as a string. If the path does not exist,
// it returns an empty string (no error).
func (td *TreeData) Get(path string) string {
	v, ok := td.tree.Get(path)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", v.Val)
}

// GetOr returns the value at path as a string, falling back to defaultVal
// if the path does not exist.
func (td *TreeData) GetOr(path, defaultVal string) string {
	v, ok := td.tree.Get(path)
	if !ok {
		return defaultVal
	}
	return fmt.Sprintf("%v", v.Val)
}

// Has returns true if the path exists in the (filtered) tree.
func (td *TreeData) Has(path string) bool {
	_, ok := td.tree.Get(path)
	return ok
}

// All returns all key-value pairs in the (filtered) tree as a map
// from path to value. Keys are sorted for deterministic output.
func (td *TreeData) All() map[string]any {
	out := make(map[string]any)
	for _, path := range td.tree.List() {
		v, ok := td.tree.Get(path)
		if ok {
			out[path] = v.Val
		}
	}
	return out
}

// Prefix returns all key-value pairs under the given prefix as a flat map.
// The prefix is stripped from the keys. For example, with prefix "database",
// "database/host" becomes "host".
func (td *TreeData) Prefix(prefix string) map[string]any {
	out := make(map[string]any)
	// Ensure prefix ends with "/" for matching.
	p := prefix
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	for _, path := range td.tree.List() {
		if strings.HasPrefix(path, p) || path == prefix {
			v, ok := td.tree.Get(path)
			if ok {
				key := strings.TrimPrefix(path, p)
				if key == "" {
					key = path
				}
				out[key] = v.Val
			}
		}
	}
	return out
}

// Nested returns all key-value pairs under the given prefix as a nested map,
// splitting on "/". For example, "database/connection/host" with prefix
// "database" becomes {"connection": {"host": value}}. An empty prefix
// nests all paths.
func (td *TreeData) Nested(prefix string) map[string]any {
	out := make(map[string]any)
	for _, path := range td.tree.List() {
		if prefix != "" {
			p := prefix
			if !strings.HasSuffix(p, "/") {
				p += "/"
			}
			if !strings.HasPrefix(path, p) && path != prefix {
				continue
			}
			v, ok := td.tree.Get(path)
			if !ok {
				continue
			}
			key := strings.TrimPrefix(path, p)
			if key == "" {
				continue
			}
			parts := strings.Split(key, "/")
			setNested(out, parts, v.Val)
		} else {
			v, ok := td.tree.Get(path)
			if !ok {
				continue
			}
			parts := strings.Split(path, "/")
			setNested(out, parts, v.Val)
		}
	}
	return out
}

// setNested places val at the nested path within m.
func setNested(m map[string]any, parts []string, val any) {
	for i, part := range parts {
		if i == len(parts)-1 {
			m[part] = val
			return
		}
		child, ok := m[part]
		if !ok {
			child = make(map[string]any)
			m[part] = child
		}
		childMap, ok := child.(map[string]any)
		if !ok {
			// Conflict: overwrite with a nested map.
			childMap = make(map[string]any)
			m[part] = childMap
		}
		m = childMap
	}
}

// Meta returns a metadata value for the given path and key. Returns an
// empty string if the path or metadata key does not exist.
func (td *TreeData) Meta(path, key string) string {
	v, ok := td.tree.Get(path)
	if !ok {
		return ""
	}
	if v.Metadata == nil {
		return ""
	}
	mv, ok := v.Metadata[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", mv)
}

// ComponentEnabled returns true if the named component is enabled.
func (td *TreeData) ComponentEnabled(name string) bool {
	if td.components == nil {
		return false
	}
	return td.components.IsEnabled(name)
}

// EnabledComponents returns a sorted list of enabled component names.
func (td *TreeData) EnabledComponents() []string {
	if td.components == nil {
		return nil
	}
	var names []string
	for _, c := range td.components.ListComponents() {
		if c.Enabled {
			names = append(names, c.Name)
		}
	}
	sort.Strings(names)
	return names
}

// DisabledComponents returns a sorted list of disabled component names.
func (td *TreeData) DisabledComponents() []string {
	if td.components == nil {
		return nil
	}
	var names []string
	for _, c := range td.components.ListComponents() {
		if !c.Enabled {
			names = append(names, c.Name)
		}
	}
	sort.Strings(names)
	return names
}

// ComponentPaths returns the path prefixes belonging to the named component.
func (td *TreeData) ComponentPaths(name string) []string {
	if td.components == nil {
		return nil
	}
	def, ok := td.components.Definition(name)
	if !ok {
		return nil
	}
	paths := make([]string, len(def.Paths))
	copy(paths, def.Paths)
	return paths
}
