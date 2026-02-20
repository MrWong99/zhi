package webui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// treeNode represents a node in the hierarchical tree view.
type treeNode struct {
	Name             string
	Path             string
	PathID           string
	IsLeaf           bool
	Children         []*treeNode
	DisplayValue     string
	ValueType        string
	Component        string
	ComponentEnabled bool

	// Label-aware fields for leaf nodes.
	DisplayName        string // ui.displayName: shown instead of Name
	IsPassword         bool   // ui.password: mask the displayed value
	IsReadonly         bool   // ui.readonly or config.immutable
	IsRequired         bool   // config.required
	IsDeprecated       bool   // core.deprecated is set
	Description        string // core.description: tooltip / subtitle
	Section            string // ui.section: collapsible grouping
	Group              string // ui.group: visual group badge
	Order              int    // ui.order: sort key
	ShowSectionHeader  bool   // true if this node starts a new section
	SectionHeaderLabel string // the section name to display as header
}

// buildNestedTree converts flat config paths into a hierarchical tree of
// treeNodes suitable for template rendering. fullTree is used for evaluating
// ui.showIf conditions and may differ from displayTree when a text filter
// is active.
func buildNestedTree(displayTree config.TreeReader, fullTree config.TreeReader, components []ui.ComponentInfo) []*treeNode {
	compMap := buildComponentMap(components)

	paths := displayTree.List()
	sort.Strings(paths)

	root := &treeNode{}
	for _, path := range paths {
		value, ok := displayTree.Get(path)
		if !ok {
			continue
		}

		// ui.hidden: skip entirely.
		if labels.IsHidden(value.Metadata) {
			continue
		}

		// ui.showIf: evaluate against the full tree.
		if condPath, condVal := labels.ParseShowIf(value.Metadata); condPath != "" {
			depVal, depOk := fullTree.Get(condPath)
			if !depOk || fmt.Sprintf("%v", depVal.Val) != condVal {
				continue
			}
		}

		segments := strings.Split(path, "/")
		insertTreeNode(root, segments, path, value, compMap)
	}

	// Sort children at every level and inject section headers.
	sortTreeChildren(root)

	return root.Children
}

// filterTree filters tree paths to those matching the filter string
// (prefix or substring match on the full path or display name).
func filterTree(tree config.TreeReader, filter string) config.TreeReader {
	if filter == "" {
		return tree
	}
	filtered := config.NewTree()
	lower := strings.ToLower(filter)
	for _, path := range tree.List() {
		value, ok := tree.Get(path)
		if !ok {
			continue
		}
		// Match against path or display name.
		dn := strings.ToLower(labels.GetDisplayName(value.Metadata))
		if strings.Contains(strings.ToLower(path), lower) || (dn != "" && strings.Contains(dn, lower)) {
			_ = filtered.Set(path, &value)
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
		meta := value.Metadata
		displayVal := formatValue(value.Val)
		if labels.IsPassword(meta) {
			displayVal = "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"
		}

		leaf := &treeNode{
			Name:         name,
			Path:         fullPath,
			PathID:       pathToID(fullPath),
			IsLeaf:       true,
			DisplayValue: displayVal,
			ValueType:    valueType(value.Val),
			DisplayName:  labels.GetDisplayName(meta),
			IsPassword:   labels.IsPassword(meta),
			IsReadonly:   labels.IsReadonly(meta) || labels.IsImmutable(meta),
			IsRequired:   labels.IsRequired(meta),
			IsDeprecated: labels.IsDeprecatedValue(meta),
			Description:  labels.GetDescription(meta),
			Section:      labels.GetSection(meta),
			Group:        labels.GetGroup(meta),
			Order:        labels.GetOrder(meta),
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

// sortTreeChildren recursively sorts children at every level by ui.order,
// then ui.section, then name. It also injects ShowSectionHeader flags
// when the section changes between consecutive leaf children.
func sortTreeChildren(node *treeNode) {
	if len(node.Children) == 0 {
		return
	}

	// Sort children: by Order, then Section, then Name.
	sort.SliceStable(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		return a.Name < b.Name
	})

	// Inject section headers for leaf children when section changes.
	lastSection := ""
	for _, child := range node.Children {
		if child.IsLeaf && child.Section != "" && child.Section != lastSection {
			child.ShowSectionHeader = true
			child.SectionHeaderLabel = child.Section
		}
		if child.IsLeaf {
			lastSection = child.Section
		} else {
			// Non-leaf (directory) nodes reset the section tracking.
			lastSection = ""
		}

		// Recurse.
		sortTreeChildren(child)
	}
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
