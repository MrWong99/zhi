package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// TreeView displays the configuration tree as a navigable list.
type TreeView struct {
	controller      *ui.UIController
	paths           []string
	filteredPaths   []string
	cursor          int
	offset          int
	filter          string
	filtering       bool
	componentStates map[string]bool
	componentNames  map[string]string // path -> component name
	width           int
	height          int
}

// NewTreeView creates a new tree view.
func NewTreeView(controller *ui.UIController, tree *config.Tree) TreeView {
	paths := tree.List()
	sort.Strings(paths)

	componentStates := make(map[string]bool)
	componentNames := make(map[string]string)

	for _, comp := range controller.ListComponents() {
		componentStates[comp.Name] = comp.Enabled
	}

	for _, path := range paths {
		if name, ok := controller.PathBelongsToComponent(path); ok {
			componentNames[path] = name
		}
	}

	return TreeView{
		controller:      controller,
		paths:           paths,
		filteredPaths:   paths,
		componentStates: componentStates,
		componentNames:  componentNames,
		width:           80,
		height:          20,
	}
}

// SetSize updates the view dimensions.
func (v *TreeView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SelectedPath returns the currently highlighted path.
func (v *TreeView) SelectedPath() string {
	if len(v.filteredPaths) == 0 {
		return ""
	}
	if v.cursor >= len(v.filteredPaths) {
		v.cursor = len(v.filteredPaths) - 1
	}
	return v.filteredPaths[v.cursor]
}

// ClearFilter removes the active filter.
func (v *TreeView) ClearFilter() {
	v.filter = ""
	v.filtering = false
	v.filteredPaths = v.paths
	v.cursor = 0
	v.offset = 0
}

// UpdateTree handles messages for the tree view.
func (v TreeView) UpdateTree(msg tea.Msg) (TreeView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.filtering {
			switch msg.String() {
			case "enter":
				v.filtering = false
				return v, nil
			case "esc":
				v.ClearFilter()
				return v, nil
			case "backspace":
				if len(v.filter) > 0 {
					v.filter = v.filter[:len(v.filter)-1]
					v.applyFilter()
				}
				return v, nil
			default:
				if len(msg.String()) == 1 {
					v.filter += msg.String()
					v.applyFilter()
				}
				return v, nil
			}
		}

		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.filteredPaths)-1 {
				v.cursor++
				v.ensureVisible()
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
				v.ensureVisible()
			}
		case "/":
			v.filtering = true
			v.filter = ""
		}
	}
	return v, nil
}

func (v *TreeView) applyFilter() {
	if v.filter == "" {
		v.filteredPaths = v.paths
	} else {
		var filtered []string
		lowerFilter := strings.ToLower(v.filter)
		for _, p := range v.paths {
			if strings.Contains(strings.ToLower(p), lowerFilter) {
				filtered = append(filtered, p)
			}
		}
		v.filteredPaths = filtered
	}
	v.cursor = 0
	v.offset = 0
}

func (v *TreeView) ensureVisible() {
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+v.height {
		v.offset = v.cursor - v.height + 1
	}
}

// View renders the tree view.
func (v TreeView) View() string {
	var sb strings.Builder

	if v.filtering {
		sb.WriteString(InfoStyle.Render(fmt.Sprintf("Filter: %s_", v.filter)))
		sb.WriteString("\n")
	}

	if len(v.filteredPaths) == 0 {
		sb.WriteString(DimStyle.Render("  No configuration paths found."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	visibleLines := v.height
	if v.filtering {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := min(v.offset+visibleLines, len(v.filteredPaths))

	for i := v.offset; i < end; i++ {
		path := v.filteredPaths[i]
		line := v.renderPathLine(path, i == v.cursor)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return v.padHeight(sb.String())
}

func (v TreeView) renderPathLine(path string, selected bool) string {
	compName := v.componentNames[path]
	isDisabled := compName != "" && !v.componentStates[compName]

	// Build value string.
	valStr := ""
	if val, ok := v.controller.GetValue(path); ok {
		valStr = fmt.Sprintf("%v", val.Val)
		if len(valStr) > 40 {
			valStr = valStr[:37] + "..."
		}
	}

	// Build the line.
	var line string
	if isDisabled {
		line = DisabledComponentStyle.Render(fmt.Sprintf("  %-30s  %s", path, valStr))
		if compName != "" {
			line += "  " + DisabledComponentStyle.Render("["+compName+"]")
		}
	} else {
		pathRendered := PathStyle.Render(fmt.Sprintf("%-30s", path))
		valRendered := ValueStyle.Render(valStr)
		line = fmt.Sprintf("  %s  %s", pathRendered, valRendered)
		if compName != "" {
			line += "  " + ComponentBadgeStyle.Render("["+compName+"]")
		}
	}

	if selected {
		// Re-render the entire line with active styling.
		plainPath := fmt.Sprintf("%-30s", path)
		plainVal := valStr
		plain := fmt.Sprintf("> %s  %s", plainPath, plainVal)
		if compName != "" {
			plain += "  [" + compName + "]"
		}
		line = ActiveStyle.Render(lipgloss.PlaceHorizontal(v.width, lipgloss.Left, plain))
	}

	return line
}

func (v TreeView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
