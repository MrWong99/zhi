package webui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// treeNode represents a node in the hierarchical tree view.
type treeNode struct {
	Name             string
	Path             string
	IsLeaf           bool
	Children         []*treeNode
	DisplayValue     string
	ValueType        string
	Component        string
	ComponentEnabled bool
}

// buildNestedTree converts flat config paths into a hierarchical tree of
// treeNodes suitable for template rendering.
func buildNestedTree(tree config.TreeReader, components []ui.ComponentInfo) []*treeNode {
	compMap := buildComponentMap(components)

	paths := tree.List()
	sort.Strings(paths)

	root := &treeNode{}
	for _, path := range paths {
		value, ok := tree.Get(path)
		if !ok {
			continue
		}
		segments := strings.Split(path, "/")
		insertTreeNode(root, segments, path, value, compMap)
	}

	return root.Children
}

// filterTree filters tree paths to those matching the filter string
// (prefix or substring match on the full path).
func filterTree(tree config.TreeReader, filter string) config.TreeReader {
	if filter == "" {
		return tree
	}
	filtered := config.NewTree()
	lower := strings.ToLower(filter)
	for _, path := range tree.List() {
		if strings.Contains(strings.ToLower(path), lower) {
			value, ok := tree.Get(path)
			if ok {
				_ = filtered.Set(path, &value)
			}
		}
	}
	return filtered
}

func buildComponentMap(components []ui.ComponentInfo) map[string]ui.ComponentInfo {
	compMap := make(map[string]ui.ComponentInfo)
	for _, c := range components {
		for _, p := range c.Paths {
			compMap[p] = c
		}
	}
	return compMap
}

func insertTreeNode(parent *treeNode, segments []string, fullPath string, value config.Value, compMap map[string]ui.ComponentInfo) {
	if len(segments) == 0 {
		return
	}

	name := segments[0]

	// Check if this segment already exists in children.
	var existing *treeNode
	for _, child := range parent.Children {
		if child.Name == name {
			existing = child
			break
		}
	}

	if len(segments) == 1 {
		// Leaf node.
		leaf := &treeNode{
			Name:         name,
			Path:         fullPath,
			IsLeaf:       true,
			DisplayValue: formatValue(value.Val),
			ValueType:    valueType(value.Val),
		}
		if comp, ok := findComponent(fullPath, compMap); ok {
			leaf.Component = comp.Name
			leaf.ComponentEnabled = comp.Enabled
		}
		parent.Children = append(parent.Children, leaf)
		return
	}

	if existing == nil {
		existing = &treeNode{
			Name: name,
		}
		// Check component ownership for intermediate nodes by checking
		// if any component path is a prefix.
		if comp, ok := findComponent(strings.Join(segments[:1], "/"), compMap); ok {
			existing.Component = comp.Name
			existing.ComponentEnabled = comp.Enabled
		}
		parent.Children = append(parent.Children, existing)
	}

	insertTreeNode(existing, segments[1:], fullPath, value, compMap)
}

func findComponent(path string, compMap map[string]ui.ComponentInfo) (ui.ComponentInfo, bool) {
	if comp, ok := compMap[path]; ok {
		return comp, true
	}
	for compPath, comp := range compMap {
		if strings.HasPrefix(path, compPath+"/") || strings.HasPrefix(compPath, path+"/") {
			return comp, true
		}
	}
	return ui.ComponentInfo{}, false
}

func formatValue(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%v", v)
}

func valueType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64, float32, int, int64, int32:
		return "number"
	case bool:
		return "bool"
	default:
		return "other"
	}
}
