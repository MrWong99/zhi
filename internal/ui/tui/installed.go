package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	internalui "github.com/MrWong99/zhi/internal/ui"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// installedPluginsMsg is sent when the installed plugins list loads.
type installedPluginsMsg struct {
	plugins []zhiui.InstalledPlugin
	err     error
}

// pluginUninstalledMsg is sent when a plugin is uninstalled.
type pluginUninstalledMsg struct {
	name string
	err  error
}

// pluginUpdatedMsg is sent when a plugin is updated.
type pluginUpdatedMsg struct {
	result *zhiui.InstallResult
	err    error
}

// InstalledView displays locally installed plugins.
type InstalledView struct {
	ctrl    *internalui.UIController
	ctx     context.Context
	plugins []zhiui.InstalledPlugin
	cursor  int
	offset  int
	loading bool
	message string
	errMsg  string
	width   int
	height  int
}

// NewInstalledView creates a new installed plugins view.
func NewInstalledView(ctx context.Context, ctrl *internalui.UIController) InstalledView {
	return InstalledView{
		ctrl:   ctrl,
		ctx:    ctx,
		width:  80,
		height: 20,
	}
}

// SetSize updates the view dimensions.
func (v *InstalledView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// Init loads the installed plugins list.
func (v InstalledView) Init() tea.Cmd {
	return v.loadPlugins()
}

// UpdateInstalled handles messages for the installed view.
func (v InstalledView) UpdateInstalled(msg tea.Msg) (InstalledView, tea.Cmd) {
	switch msg := msg.(type) {
	case installedPluginsMsg:
		v.loading = false
		if msg.err != nil {
			v.errMsg = msg.err.Error()
			v.plugins = nil
		} else {
			v.errMsg = ""
			v.plugins = msg.plugins
		}
		return v, nil

	case pluginUninstalledMsg:
		v.loading = false
		if msg.err != nil {
			v.errMsg = "Uninstall failed: " + msg.err.Error()
		} else {
			v.message = fmt.Sprintf("Uninstalled %s", msg.name)
			v.errMsg = ""
		}
		return v, v.loadPlugins()

	case pluginUpdatedMsg:
		v.loading = false
		if msg.err != nil {
			v.errMsg = "Update failed: " + msg.err.Error()
		} else {
			v.message = fmt.Sprintf("Updated %s to v%s", msg.result.Name, msg.result.Version)
			v.errMsg = ""
		}
		return v, v.loadPlugins()

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.plugins)-1 {
				v.cursor++
				v.ensureVisible()
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
				v.ensureVisible()
			}
		case "u":
			// Update selected plugin.
			if v.cursor < len(v.plugins) {
				p := v.plugins[v.cursor]
				if p.UpdateAvail != "" {
					v.loading = true
					v.message = fmt.Sprintf("Updating %s...", p.Name)
					return v, v.doUpdate(p.Name, "")
				}
			}
		case "d":
			// Delete selected plugin.
			if v.cursor < len(v.plugins) {
				p := v.plugins[v.cursor]
				if p.Source != "built-in" {
					v.loading = true
					v.message = fmt.Sprintf("Uninstalling %s...", p.Name)
					return v, v.doUninstall(p.Name, p.Type)
				}
				v.errMsg = "Cannot uninstall built-in plugins"
			}
		case "r":
			// Refresh list.
			v.loading = true
			return v, v.loadPlugins()
		}
	}
	return v, nil
}

func (v InstalledView) loadPlugins() tea.Cmd {
	ctx := v.ctx
	ctrl := v.ctrl
	return func() tea.Msg {
		plugins, err := ctrl.ListInstalledPlugins(ctx)
		return installedPluginsMsg{plugins: plugins, err: err}
	}
}

func (v InstalledView) doUpdate(name, version string) tea.Cmd {
	ctx := v.ctx
	ctrl := v.ctrl
	return func() tea.Msg {
		result, err := ctrl.UpdatePlugin(ctx, name, version)
		return pluginUpdatedMsg{result: result, err: err}
	}
}

func (v InstalledView) doUninstall(name, pluginType string) tea.Cmd {
	ctx := v.ctx
	ctrl := v.ctrl
	return func() tea.Msg {
		err := ctrl.UninstallPlugin(ctx, name, pluginType)
		return pluginUninstalledMsg{name: name, err: err}
	}
}

func (v *InstalledView) ensureVisible() {
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	visibleItems := max(v.height-6, 1)
	if v.cursor >= v.offset+visibleItems {
		v.offset = v.cursor - visibleItems + 1
	}
}

// SelectedPlugin returns the currently selected installed plugin, if any.
func (v InstalledView) SelectedPlugin() (zhiui.InstalledPlugin, bool) {
	if v.cursor >= 0 && v.cursor < len(v.plugins) {
		return v.plugins[v.cursor], true
	}
	return zhiui.InstalledPlugin{}, false
}

// View renders the installed plugins view.
func (v InstalledView) View() string {
	var sb strings.Builder

	sb.WriteString("  Installed Plugins")
	sb.WriteString("\n\n")

	// Messages.
	if v.errMsg != "" {
		sb.WriteString(ErrorStyle.Render("  " + v.errMsg))
		sb.WriteString("\n\n")
	} else if v.message != "" {
		sb.WriteString(SuccessStyle.Render("  " + v.message))
		sb.WriteString("\n\n")
	}

	if v.loading {
		sb.WriteString(DimStyle.Render("  Loading..."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	if len(v.plugins) == 0 {
		sb.WriteString(DimStyle.Render("  No plugins installed."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	// Table header.
	header := fmt.Sprintf("  %-20s %-12s %-10s %-20s %s", "NAME", "TYPE", "VERSION", "SOURCE", "UPDATE")
	sb.WriteString(DimStyle.Render(header))
	sb.WriteString("\n")
	sb.WriteString(DimStyle.Render("  " + strings.Repeat("-", min(v.width-4, 76))))
	sb.WriteString("\n")

	visibleItems := max(v.height-8, 1)
	end := min(v.offset+visibleItems, len(v.plugins))
	for i := v.offset; i < end; i++ {
		p := v.plugins[i]
		selected := i == v.cursor
		sb.WriteString(v.renderPluginLine(p, selected))
		sb.WriteString("\n")
	}

	return v.padHeight(sb.String())
}

func (v InstalledView) renderPluginLine(p zhiui.InstalledPlugin, selected bool) string {
	version := p.Version
	if version == "" {
		version = "-"
	}
	source := p.Source
	if len(source) > 20 {
		source = source[:17] + "..."
	}
	update := "-"
	if p.UpdateAvail != "" {
		update = p.UpdateAvail + " available"
	}

	line := fmt.Sprintf("  %-20s %-12s %-10s %-20s %s", p.Name, p.Type, version, source, update)

	if selected {
		plain := fmt.Sprintf("> %-20s %-12s %-10s %-20s %s", p.Name, p.Type, version, source, update)
		return ActiveStyle.Render(lipgloss.PlaceHorizontal(v.width, lipgloss.Left, plain))
	}

	return line
}

func (v InstalledView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
