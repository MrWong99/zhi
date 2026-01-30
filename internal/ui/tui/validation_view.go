package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// ValidationView displays validation results grouped by severity.
type ValidationView struct {
	results  []config.ValidationResult
	lines    []string
	cursor   int
	offset   int
	summary  string
	width    int
	height   int
}

// NewValidationView creates an empty validation view.
func NewValidationView() ValidationView {
	return ValidationView{}
}

// NewValidationViewWith creates a validation view populated with results.
func NewValidationViewWith(results []config.ValidationResult) ValidationView {
	v := ValidationView{
		results: results,
	}
	v.buildDisplay()
	return v
}

// SetSize updates the view dimensions.
func (v *ValidationView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// UpdateValidation handles messages for the validation view.
func (v ValidationView) UpdateValidation(msg tea.Msg) (ValidationView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.lines)-1 {
				v.cursor++
				v.ensureVisible()
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
				v.ensureVisible()
			}
		}
	}
	return v, nil
}

func (v *ValidationView) ensureVisible() {
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+v.height {
		v.offset = v.cursor - v.height + 1
	}
}

func (v *ValidationView) buildDisplay() {
	var blocking, warnings, infos []config.ValidationResult
	var componentErrors []config.ValidationResult

	for _, r := range v.results {
		if strings.HasPrefix(r.Message, "component ") {
			componentErrors = append(componentErrors, r)
			continue
		}
		switch r.Severity {
		case config.Blocking:
			blocking = append(blocking, r)
		case config.Warning:
			warnings = append(warnings, r)
		case config.Info:
			infos = append(infos, r)
		}
	}

	var lines []string

	// Summary.
	v.summary = fmt.Sprintf("Blocking: %d  Warning: %d  Info: %d  Component: %d",
		len(blocking), len(warnings), len(infos), len(componentErrors))
	lines = append(lines, v.summary)
	lines = append(lines, "")

	// Component dependency errors first.
	if len(componentErrors) > 0 {
		lines = append(lines, "Component Dependency Errors:")
		for _, r := range componentErrors {
			lines = append(lines, fmt.Sprintf("  [COMPONENT] %s", r.Message))
		}
		lines = append(lines, "")
	}

	// Blocking.
	if len(blocking) > 0 {
		lines = append(lines, "Blocking Issues:")
		for _, r := range blocking {
			lines = append(lines, fmt.Sprintf("  [BLOCKING] %s", r.Message))
		}
		lines = append(lines, "")
	}

	// Warnings.
	if len(warnings) > 0 {
		lines = append(lines, "Warnings:")
		for _, r := range warnings {
			lines = append(lines, fmt.Sprintf("  [WARNING] %s", r.Message))
		}
		lines = append(lines, "")
	}

	// Info.
	if len(infos) > 0 {
		lines = append(lines, "Informational:")
		for _, r := range infos {
			lines = append(lines, fmt.Sprintf("  [INFO] %s", r.Message))
		}
		lines = append(lines, "")
	}

	if len(v.results) == 0 {
		lines = append(lines, SuccessStyle.Render("  All validations passed!"))
	}

	v.lines = lines
}

// View renders the validation view.
func (v ValidationView) View() string {
	var sb strings.Builder

	if len(v.lines) == 0 {
		sb.WriteString(DimStyle.Render("  No validation results."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	visibleLines := v.height
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := v.offset + visibleLines
	if end > len(v.lines) {
		end = len(v.lines)
	}

	for i := v.offset; i < end; i++ {
		line := v.lines[i]
		rendered := v.renderLine(line, i == v.cursor)
		sb.WriteString(rendered)
		sb.WriteString("\n")
	}

	return v.padHeight(sb.String())
}

func (v ValidationView) renderLine(line string, selected bool) string {
	var rendered string

	switch {
	case strings.Contains(line, "[BLOCKING]"):
		rendered = BlockingStyle.Render(line)
	case strings.Contains(line, "[WARNING]"):
		rendered = WarningStyle.Render(line)
	case strings.Contains(line, "[INFO]"):
		rendered = InfoStyle.Render(line)
	case strings.Contains(line, "[COMPONENT]"):
		rendered = ComponentErrorStyle.Render(line)
	case strings.HasPrefix(line, "Blocking:"):
		rendered = line // summary line
	default:
		rendered = line
	}

	if selected {
		rendered = ActiveStyle.Render("> " + line)
	}

	return rendered
}

func (v ValidationView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
