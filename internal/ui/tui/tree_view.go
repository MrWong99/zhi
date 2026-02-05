package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

// TreeView displays the configuration tree as a navigable list.
type TreeView struct {
	controller      *ui.UIController
	paths           []string // all paths after label-aware sorting
	filteredPaths   []string // visible paths after filtering (text filter + label visibility)
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

	// Sort paths using label-aware ordering: first by ui.order, then
	// by ui.group, then alphabetically within each group.
	sort.Slice(paths, func(i, j int) bool {
		vi, _ := tree.Get(paths[i])
		vj, _ := tree.Get(paths[j])

		orderI := labels.GetOrder(vi.Metadata)
		orderJ := labels.GetOrder(vj.Metadata)
		if orderI != orderJ {
			return orderI < orderJ
		}

		groupI := labels.GetGroup(vi.Metadata)
		groupJ := labels.GetGroup(vj.Metadata)
		if groupI != groupJ {
			return groupI < groupJ
		}

		sectionI := labels.GetSection(vi.Metadata)
		sectionJ := labels.GetSection(vj.Metadata)
		if sectionI != sectionJ {
			return sectionI < sectionJ
		}

		return paths[i] < paths[j]
	})

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

	tv := TreeView{
		controller:      controller,
		paths:           paths,
		componentStates: componentStates,
		componentNames:  componentNames,
		width:           80,
		height:          20,
	}
	tv.filteredPaths = tv.visiblePaths()
	return tv
}

// visiblePaths returns paths after applying label-based visibility rules
// (ui.hidden, ui.showIf) and the current text filter.
func (v *TreeView) visiblePaths() []string {
	var visible []string
	for _, p := range v.paths {
		val, ok := v.controller.GetValue(p)
		if !ok {
			continue
		}

		// ui.hidden: completely hide.
		if labels.IsHidden(val.Metadata) {
			continue
		}

		// ui.showIf: conditional visibility.
		if condPath, condVal := labels.ParseShowIf(val.Metadata); condPath != "" {
			depVal, depOk := v.controller.GetValue(condPath)
			if !depOk || fmt.Sprintf("%v", depVal.Val) != condVal {
				continue
			}
		}

		// Text filter.
		if v.filter != "" {
			lf := strings.ToLower(v.filter)
			lp := strings.ToLower(p)
			// Also check display name.
			dn := strings.ToLower(labels.GetDisplayName(val.Metadata))
			if !strings.Contains(lp, lf) && !strings.Contains(dn, lf) {
				continue
			}
		}

		visible = append(visible, p)
	}
	return visible
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
	v.filteredPaths = v.visiblePaths()
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
	v.filteredPaths = v.visiblePaths()
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

	lastSection := ""
	for i := v.offset; i < end; i++ {
		path := v.filteredPaths[i]

		// Render section headers when section changes.
		if val, ok := v.controller.GetValue(path); ok {
			section := labels.GetSection(val.Metadata)
			if section != "" && section != lastSection {
				sb.WriteString("  " + SectionHeaderStyle.Render(section))
				sb.WriteString("\n")
				lastSection = section
			} else if section == "" && lastSection != "" {
				lastSection = ""
			}
		}

		line := v.renderPathLine(path, i == v.cursor)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return v.padHeight(sb.String())
}

func (v TreeView) renderPathLine(path string, selected bool) string {
	compName := v.componentNames[path]
	isDisabled := compName != "" && !v.componentStates[compName]

	val, hasVal := v.controller.GetValue(path)

	// Determine display name: ui.displayName or the raw path.
	displayPath := path
	if hasVal {
		if dn := labels.GetDisplayName(val.Metadata); dn != "" {
			displayPath = dn
		}
	}

	// Build value string.
	valStr := ""
	if hasVal {
		if labels.IsPassword(val.Metadata) {
			valStr = "••••••••"
		} else {
			valStr = fmt.Sprintf("%v", val.Val)
			if len(valStr) > 40 {
				valStr = valStr[:37] + "..."
			}
		}
	}

	// Build badges.
	var badgeParts []string
	if hasVal {
		if labels.IsReadonly(val.Metadata) {
			badgeParts = append(badgeParts, ReadonlyBadgeStyle.Render("[ro]"))
		}
		if labels.IsRequired(val.Metadata) {
			badgeParts = append(badgeParts, RequiredBadgeStyle.Render("[req]"))
		}
		if labels.ShouldConfirm(val.Metadata) {
			badgeParts = append(badgeParts, ConfirmStyle.Render("[confirm]"))
		}
		if labels.IsDeprecatedValue(val.Metadata) {
			badgeParts = append(badgeParts, DimStyle.Render("[deprecated]"))
		}
		if e := labels.GetEnum(val.Metadata); len(e) > 0 {
			badgeParts = append(badgeParts, EnumBadgeStyle.Render("[enum]"))
		}
		if g := labels.GetGroup(val.Metadata); g != "" {
			badgeParts = append(badgeParts, GroupBadgeStyle.Render("["+g+"]"))
		}
	}

	if compName != "" {
		badgeParts = append(badgeParts, ComponentBadgeStyle.Render("["+compName+"]"))
	}

	badges := ""
	if len(badgeParts) > 0 {
		badges = "  " + strings.Join(badgeParts, " ")
	}

	// Build the line.
	var line string
	if isDisabled {
		line = DisabledComponentStyle.Render(fmt.Sprintf("  %-30s  %s", displayPath, valStr))
		if badges != "" {
			line += DisabledComponentStyle.Render(badges)
		}
	} else {
		isDeprecated := hasVal && labels.IsDeprecatedValue(val.Metadata)
		var pathRendered string
		if isDeprecated {
			pathRendered = DeprecatedStyle.Render(fmt.Sprintf("%-30s", displayPath))
		} else {
			pathRendered = PathStyle.Render(fmt.Sprintf("%-30s", displayPath))
		}

		var valRendered string
		if hasVal && labels.IsPassword(val.Metadata) {
			valRendered = PasswordStyle.Render(valStr)
		} else {
			valRendered = ValueStyle.Render(valStr)
		}

		line = fmt.Sprintf("  %s  %s", pathRendered, valRendered)
		line += badges
	}

	if selected {
		// Re-render the entire line with active styling.
		plainPath := fmt.Sprintf("%-30s", displayPath)
		plainVal := valStr
		plain := fmt.Sprintf("> %s  %s", plainPath, plainVal)
		if badges != "" {
			// Strip ANSI from badges for the active render.
			plain += badges
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
