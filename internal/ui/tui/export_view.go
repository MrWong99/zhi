package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
)

// ExportView displays export templates and allows execution/preview.
type ExportView struct {
	controller        *ui.UIController
	templates         []core.ExportTemplate
	builtinFormats    []string
	items             []exportItem
	cursor            int
	offset            int
	preview           string
	showPreview       bool
	exported          bool
	exportMsg         string
	enabledComponents []string
	width             int
	height            int
}

type exportItem struct {
	name       string
	isTemplate bool
	template   *core.ExportTemplate
	format     string
}

// NewExportView creates a new export view.
func NewExportView(controller *ui.UIController) ExportView {
	templates := controller.ExportTemplates()
	builtinFormats := []string{"json", "yaml", "toml", "dotenv"}

	var items []exportItem
	for i := range templates {
		name := templates[i].Name
		if name == "" {
			if templates[i].Template != "" {
				name = filepath.Base(templates[i].Template)
			} else {
				name = templates[i].Format
			}
		}
		items = append(items, exportItem{
			name:       name,
			isTemplate: true,
			template:   &templates[i],
		})
	}
	for _, f := range builtinFormats {
		// Don't duplicate formats already in templates.
		found := false
		for _, t := range templates {
			if t.Format == f && t.Template == "" {
				found = true
				break
			}
		}
		if !found {
			items = append(items, exportItem{
				name:   f + " (built-in)",
				format: f,
			})
		}
	}

	var enabledComps []string
	for _, c := range controller.ListComponents() {
		if c.Enabled {
			enabledComps = append(enabledComps, c.Name)
		}
	}

	return ExportView{
		controller:        controller,
		templates:         templates,
		builtinFormats:    builtinFormats,
		items:             items,
		enabledComponents: enabledComps,
		width:             80,
		height:            20,
	}
}

// SetSize updates the view dimensions.
func (v *ExportView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// UpdateExport handles messages for the export view.
func (v ExportView) UpdateExport(msg tea.Msg, ctx context.Context) (ExportView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.items)-1 {
				v.cursor++
				v.showPreview = false
				v.ensureVisible()
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
				v.showPreview = false
				v.ensureVisible()
			}
		case "enter":
			if len(v.items) > 0 {
				v.executeExport(ctx)
			}
		case "p":
			if len(v.items) > 0 {
				v.togglePreview(ctx)
			}
		}
	}
	return v, nil
}

func (v *ExportView) executeExport(ctx context.Context) {
	item := v.items[v.cursor]
	cfg := v.buildRunConfig(item)

	result, err := v.controller.Export(ctx, cfg)
	if err != nil {
		v.exportMsg = "Export error: " + err.Error()
		return
	}

	if result.OutputPath != "" && result.OutputPath != "-" {
		v.exportMsg = fmt.Sprintf("Exported: %s -> %s", result.Name, result.OutputPath)
	} else {
		v.exportMsg = fmt.Sprintf("Exported: %s (to stdout)", result.Name)
		v.preview = result.Content
		v.showPreview = true
	}
	v.exported = true
}

func (v *ExportView) togglePreview(ctx context.Context) {
	if v.showPreview {
		v.showPreview = false
		return
	}

	item := v.items[v.cursor]
	cfg := v.buildRunConfig(item)

	content, err := v.controller.ExportPreview(ctx, cfg)
	if err != nil {
		v.exportMsg = "Preview error: " + err.Error()
		return
	}

	v.preview = content
	v.showPreview = true
}

func (v *ExportView) buildRunConfig(item exportItem) core.ExportRunConfig {
	cfg := core.ExportRunConfig{
		DryRun: false,
	}

	if item.isTemplate && item.template != nil {
		t := item.template
		if t.Template != "" {
			cfg.TemplatePath = t.Template
		}
		if t.Format != "" {
			cfg.Format = t.Format
		}
		if t.Output != "" {
			cfg.OutputPath = t.Output
		}
		cfg.Prefix = t.Prefix
	} else {
		cfg.Format = item.format
		cfg.OutputPath = "-"
	}

	return cfg
}

func (v *ExportView) ensureVisible() {
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	listHeight := v.height / 2
	if v.cursor >= v.offset+listHeight {
		v.offset = v.cursor - listHeight + 1
	}
}

// View renders the export view.
func (v ExportView) View() string {
	var sb strings.Builder

	// Enabled components header.
	if len(v.enabledComponents) > 0 {
		sb.WriteString(DimStyle.Render(fmt.Sprintf("  Enabled components: %s", strings.Join(v.enabledComponents, ", "))))
		sb.WriteString("\n\n")
	}

	if v.exportMsg != "" {
		if strings.HasPrefix(v.exportMsg, "Export error:") || strings.HasPrefix(v.exportMsg, "Preview error:") {
			sb.WriteString(ErrorStyle.Render("  " + v.exportMsg))
		} else {
			sb.WriteString(SuccessStyle.Render("  " + v.exportMsg))
		}
		sb.WriteString("\n\n")
	}

	if len(v.items) == 0 {
		sb.WriteString(DimStyle.Render("  No export templates or formats available."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	// List panel.
	sb.WriteString("  Export Templates & Formats:\n")
	listHeight := max(v.height/2, 3)

	end := min(v.offset+listHeight, len(v.items))

	for i := v.offset; i < end; i++ {
		item := v.items[i]
		prefix := "  "
		if i == v.cursor {
			prefix = "> "
		}

		line := prefix + item.name
		if item.isTemplate && item.template != nil {
			if item.template.Output != "" {
				line += DimStyle.Render(fmt.Sprintf("  -> %s", item.template.Output))
			}
		}

		if i == v.cursor {
			sb.WriteString(ActiveStyle.Render(line))
		} else {
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	// Preview panel.
	if v.showPreview && v.preview != "" {
		sb.WriteString("\n")
		sb.WriteString(HeaderStyle.Render(" Preview "))
		sb.WriteString("\n")
		previewLines := strings.Split(v.preview, "\n")
		maxPreviewLines := max(v.height-listHeight-6, 3)
		for i, line := range previewLines {
			if i >= maxPreviewLines {
				sb.WriteString(DimStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(previewLines)-maxPreviewLines)))
				sb.WriteString("\n")
				break
			}
			sb.WriteString("  " + line + "\n")
		}
	}

	return v.padHeight(sb.String())
}

func (v ExportView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
