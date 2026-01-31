package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
)

// ComponentView displays and manages component toggle state.
type ComponentView struct {
	controller *ui.UIController
	components []core.ComponentState
	defs       map[string]core.ComponentDef
	cursor     int
	offset     int
	message    string
	errMsg     string
	showInfo   bool
	width      int
	height     int
}

// NewComponentView creates a new component view.
func NewComponentView(controller *ui.UIController) ComponentView {
	components := controller.ListComponents()
	defs := make(map[string]core.ComponentDef)
	for _, c := range components {
		if def, ok := controller.ComponentDefinition(c.Name); ok {
			defs[c.Name] = def
		}
	}

	return ComponentView{
		controller: controller,
		components: components,
		defs:       defs,
		width:      80,
		height:     20,
	}
}

// SetSize updates the view dimensions.
func (v *ComponentView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// UpdateComponent handles messages for the component view.
func (v ComponentView) UpdateComponent(msg tea.Msg) (ComponentView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.components)-1 {
				v.cursor++
				v.showInfo = false
				v.ensureVisible()
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
				v.showInfo = false
				v.ensureVisible()
			}
		case "enter", " ":
			if len(v.components) > 0 {
				v.toggleSelected()
			}
		case "i":
			v.showInfo = !v.showInfo
		}
	}
	return v, nil
}

func (v *ComponentView) toggleSelected() {
	if v.cursor >= len(v.components) {
		return
	}

	comp := v.components[v.cursor]

	if comp.Enabled {
		// Try to disable.
		err := v.controller.DisableComponent(comp.Name)
		if err != nil {
			v.errMsg = err.Error()
			v.message = ""
		} else {
			v.message = fmt.Sprintf("Disabled '%s'", comp.Name)
			v.errMsg = ""
			v.refreshComponents()
		}
	} else {
		// Enable with dependency auto-enable.
		enabled, err := v.controller.EnableComponent(comp.Name)
		if err != nil {
			v.errMsg = err.Error()
			v.message = ""
		} else {
			if len(enabled) == 1 {
				v.message = fmt.Sprintf("Enabled '%s'", comp.Name)
			} else {
				deps := enabled[:len(enabled)-1]
				depNames := make([]string, len(deps))
				for i, d := range deps {
					depNames[i] = "'" + d + "'"
				}
				v.message = fmt.Sprintf("Enabled '%s' (also enabled: %s)", comp.Name, strings.Join(depNames, ", "))
			}
			v.errMsg = ""
			v.refreshComponents()
		}
	}
}

func (v *ComponentView) refreshComponents() {
	v.components = v.controller.ListComponents()
}

func (v *ComponentView) ensureVisible() {
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+v.height {
		v.offset = v.cursor - v.height + 1
	}
}

// View renders the component view.
func (v ComponentView) View() string {
	var sb strings.Builder

	if len(v.components) == 0 {
		sb.WriteString(DimStyle.Render("  No components defined."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	// Message/error display.
	if v.errMsg != "" {
		sb.WriteString(ErrorStyle.Render("  " + v.errMsg))
		sb.WriteString("\n")
	}
	if v.message != "" {
		sb.WriteString(SuccessStyle.Render("  " + v.message))
		sb.WriteString("\n")
	}
	if v.errMsg != "" || v.message != "" {
		sb.WriteString("\n")
	}

	visibleLines := v.height
	if v.errMsg != "" || v.message != "" {
		visibleLines -= 2
	}
	if v.showInfo {
		visibleLines -= 8 // Reserve space for info panel
	}
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := min(v.offset+visibleLines, len(v.components))

	for i := v.offset; i < end; i++ {
		comp := v.components[i]
		line := v.renderComponentLine(comp, i == v.cursor)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Info panel.
	if v.showInfo && v.cursor < len(v.components) {
		sb.WriteString("\n")
		sb.WriteString(v.renderInfoPanel(v.components[v.cursor]))
	}

	return v.padHeight(sb.String())
}

func (v ComponentView) renderComponentLine(comp core.ComponentState, selected bool) string {
	// Checkbox.
	var checkbox string
	if comp.Enabled {
		checkbox = CheckboxEnabledStyle.Render("[x]")
	} else {
		checkbox = CheckboxDisabledStyle.Render("[ ]")
	}

	// Name and description.
	def := v.defs[comp.Name]
	name := comp.Name
	desc := def.Description

	// Badges.
	var badges []string
	if comp.Mandatory {
		badges = append(badges, MandatoryBadgeStyle.Render("MANDATORY"))
	}
	if len(comp.Dependencies) > 0 {
		badges = append(badges, DimStyle.Render("deps: "+strings.Join(comp.Dependencies, ", ")))
	}
	if len(def.Paths) > 0 {
		badges = append(badges, DimStyle.Render("paths: "+strings.Join(def.Paths, ", ")))
	}

	var line string
	if comp.Enabled {
		line = fmt.Sprintf("  %s %-15s %-30s %s", checkbox, name, desc, strings.Join(badges, "  "))
	} else {
		nameRendered := DisabledComponentStyle.Render(name)
		descRendered := DisabledComponentStyle.Render(desc)
		line = fmt.Sprintf("  %s %-15s %-30s %s", checkbox, nameRendered, descRendered, strings.Join(badges, "  "))
	}

	if selected {
		plain := fmt.Sprintf("> %s %-15s %-30s %s",
			checkboxPlain(comp.Enabled), name, desc, strings.Join(badgesPlain(comp, def), "  "))
		line = ActiveStyle.Render(lipgloss.PlaceHorizontal(v.width, lipgloss.Left, plain))
	}

	return line
}

func checkboxPlain(enabled bool) string {
	if enabled {
		return "[x]"
	}
	return "[ ]"
}

func badgesPlain(comp core.ComponentState, def core.ComponentDef) []string {
	var badges []string
	if comp.Mandatory {
		badges = append(badges, "MANDATORY")
	}
	if len(comp.Dependencies) > 0 {
		badges = append(badges, "deps: "+strings.Join(comp.Dependencies, ", "))
	}
	if len(def.Paths) > 0 {
		badges = append(badges, "paths: "+strings.Join(def.Paths, ", "))
	}
	return badges
}

func (v ComponentView) renderInfoPanel(comp core.ComponentState) string {
	def := v.defs[comp.Name]
	dependents := v.controller.ComponentDependents(comp.Name)

	var sb strings.Builder
	sb.WriteString(HeaderStyle.Render(fmt.Sprintf(" Component: %s ", comp.Name)))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Description:  %s\n", def.Description))
	status := "enabled"
	if !comp.Enabled {
		status = "disabled"
	}
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", status))
	sb.WriteString(fmt.Sprintf("  Mandatory:    %v\n", comp.Mandatory))
	sb.WriteString(fmt.Sprintf("  Paths:        %s\n", strings.Join(def.Paths, ", ")))
	if len(comp.Dependencies) > 0 {
		sb.WriteString(fmt.Sprintf("  Dependencies: %s\n", strings.Join(comp.Dependencies, ", ")))
	} else {
		sb.WriteString("  Dependencies: -\n")
	}
	if len(dependents) > 0 {
		sb.WriteString(fmt.Sprintf("  Dependents:   %s\n", strings.Join(dependents, ", ")))
	} else {
		sb.WriteString("  Dependents:   -\n")
	}

	return sb.String()
}

func (v ComponentView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
