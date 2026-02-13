package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	internalui "github.com/MrWong99/zhi/internal/ui"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// pluginDetailMsg is sent when marketplace detail loads.
type pluginDetailMsg struct {
	detail *zhiui.MarketplaceDetail
	err    error
}

// PluginDetailView shows full information for a single marketplace plugin.
type PluginDetailView struct {
	ctrl       *internalui.UIController
	ctx        context.Context
	detail     *zhiui.MarketplaceDetail
	pluginType string
	publisher  string
	name       string
	loading    bool
	message    string
	errMsg     string
	scrollY    int
	width      int
	height     int
}

// NewPluginDetailView creates a new plugin detail view.
func NewPluginDetailView(ctx context.Context, ctrl *internalui.UIController, pluginType, publisher, name string) PluginDetailView {
	return PluginDetailView{
		ctrl:       ctrl,
		ctx:        ctx,
		pluginType: pluginType,
		publisher:  publisher,
		name:       name,
		loading:    true,
		width:      80,
		height:     20,
	}
}

// SetSize updates the view dimensions.
func (v *PluginDetailView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// Init loads the plugin detail.
func (v PluginDetailView) Init() tea.Cmd {
	ctx := v.ctx
	ctrl := v.ctrl
	pluginType := v.pluginType
	publisher := v.publisher
	name := v.name
	return func() tea.Msg {
		detail, err := ctrl.GetMarketplaceDetail(ctx, pluginType, publisher, name)
		return pluginDetailMsg{detail: detail, err: err}
	}
}

// UpdateDetail handles messages for the plugin detail view.
func (v PluginDetailView) UpdateDetail(msg tea.Msg) (PluginDetailView, tea.Cmd) {
	switch msg := msg.(type) {
	case pluginDetailMsg:
		v.loading = false
		if msg.err != nil {
			v.errMsg = msg.err.Error()
		} else {
			v.detail = msg.detail
			v.errMsg = ""
		}
		return v, nil

	case marketplaceInstallMsg:
		v.loading = false
		if msg.err != nil {
			v.errMsg = "Install failed: " + msg.err.Error()
		} else {
			v.message = fmt.Sprintf("Installed %s v%s", msg.result.Name, msg.result.Version)
			v.errMsg = ""
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			v.scrollY++
		case "k", "up":
			if v.scrollY > 0 {
				v.scrollY--
			}
		case "enter":
			if v.detail != nil {
				ref := fmt.Sprintf("%s/%s", v.publisher, v.name)
				v.loading = true
				v.message = "Installing..."
				ctx := v.ctx
				ctrl := v.ctrl
				return v, func() tea.Msg {
					result, err := ctrl.InstallPlugin(ctx, ref)
					return marketplaceInstallMsg{result: result, err: err}
				}
			}
		case "r":
			if v.detail != nil {
				// Rate with score 5 (simplified; real UI would prompt).
				ctx := v.ctx
				ctrl := v.ctrl
				pluginType := v.pluginType
				publisher := v.publisher
				name := v.name
				return v, func() tea.Msg {
					err := ctrl.RatePlugin(ctx, pluginType, publisher, name, zhiui.Rating{Score: 5})
					if err != nil {
						return errMsg{err: err}
					}
					return statusMsg("Rating submitted")
				}
			}
		}
	}
	return v, nil
}

// View renders the plugin detail view.
func (v PluginDetailView) View() string {
	var sb strings.Builder

	if v.loading {
		sb.WriteString(DimStyle.Render("  Loading plugin details..."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	if v.errMsg != "" {
		sb.WriteString(ErrorStyle.Render("  " + v.errMsg))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	if v.detail == nil {
		sb.WriteString(DimStyle.Render("  No detail available."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	d := v.detail

	// Build all content lines.
	var lines []string

	// Header.
	lines = append(lines, fmt.Sprintf("  %s", d.Name))
	verified := ""
	if d.Verified {
		verified = " (verified)"
	}
	lines = append(lines, fmt.Sprintf("  by %s%s  |  %s", d.Publisher, verified, d.Type))
	lines = append(lines, "")

	// Rating line.
	stars := strings.Repeat("*", int(d.Rating))
	lines = append(lines, fmt.Sprintf("  %s %.1f (%d ratings)    %d downloads    %s",
		stars, d.Rating, d.RatingCount, d.Downloads, d.License))
	lines = append(lines, "")

	// Description.
	desc := d.LongDescription
	if desc == "" {
		desc = d.Description
	}
	for line := range strings.SplitSeq(desc, "\n") {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "")

	// Metadata.
	lines = append(lines, fmt.Sprintf("  Version:    %s", d.LatestVersion))
	if len(d.Platforms) > 0 {
		lines = append(lines, fmt.Sprintf("  Platforms:  %s", strings.Join(d.Platforms, ", ")))
	}
	if d.Homepage != "" {
		lines = append(lines, fmt.Sprintf("  Homepage:   %s", d.Homepage))
	}
	if d.Repository != "" {
		lines = append(lines, fmt.Sprintf("  Repository: %s", d.Repository))
	}
	lines = append(lines, "")

	// Install status.
	if d.Installed {
		status := fmt.Sprintf("  Status:     Installed (v%s)", d.InstalledVer)
		if d.UpdateAvail {
			status += " -- update available"
		}
		lines = append(lines, status)
	} else {
		lines = append(lines, "  Status:     Not installed")
	}
	lines = append(lines, "")

	// Versions.
	if len(d.Versions) > 0 {
		lines = append(lines, "  Versions:")
		for _, ver := range d.Versions {
			lines = append(lines, fmt.Sprintf("    %s  %s", ver.Version, ver.CreatedAt.Format("2006-01-02")))
		}
		lines = append(lines, "")
	}

	// Keywords.
	if len(d.Keywords) > 0 {
		lines = append(lines, fmt.Sprintf("  Keywords: %s", strings.Join(d.Keywords, ", ")))
	}

	// Messages.
	if v.message != "" {
		lines = append(lines, "")
		lines = append(lines, "  "+v.message)
	}

	// Apply scroll and render.
	start := min(v.scrollY, max(len(lines)-v.height, 0))
	end := min(start+v.height, len(lines))
	for i := start; i < end; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}

	return v.padHeight(sb.String())
}

func (v PluginDetailView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
