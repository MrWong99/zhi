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

// marketplaceResultsMsg is sent when a marketplace search completes.
type marketplaceResultsMsg struct {
	results *zhiui.MarketplaceResults
	err     error
}

// marketplaceInstallMsg is sent when a plugin install completes.
type marketplaceInstallMsg struct {
	result *zhiui.InstallResult
	err    error
}

// MarketplaceView displays the marketplace search and browse interface.
type MarketplaceView struct {
	ctrl       *internalui.UIController
	ctx        context.Context
	results    []zhiui.MarketplaceEntry
	total      int
	cursor     int
	offset     int
	query      string
	typeFilter string
	sortBy     string
	searching  bool
	loading    bool
	message    string
	errMsg     string
	width      int
	height     int
}

// NewMarketplaceView creates a new marketplace view.
func NewMarketplaceView(ctx context.Context, ctrl *internalui.UIController) MarketplaceView {
	return MarketplaceView{
		ctrl:   ctrl,
		ctx:    ctx,
		sortBy: "relevance",
		width:  80,
		height: 20,
	}
}

// SetSize updates the view dimensions.
func (v *MarketplaceView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// Init triggers the initial search.
func (v MarketplaceView) Init() tea.Cmd {
	return v.doSearch()
}

// UpdateMarketplace handles messages for the marketplace view.
func (v MarketplaceView) UpdateMarketplace(msg tea.Msg) (MarketplaceView, tea.Cmd) {
	switch msg := msg.(type) {
	case marketplaceResultsMsg:
		v.loading = false
		if msg.err != nil {
			v.errMsg = msg.err.Error()
			v.results = nil
			v.total = 0
		} else {
			v.errMsg = ""
			v.results = msg.results.Results
			v.total = msg.results.Total
			v.cursor = 0
			v.offset = 0
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
		if v.searching {
			return v.updateSearchInput(msg)
		}

		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.results)-1 {
				v.cursor++
				v.ensureVisible()
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
				v.ensureVisible()
			}
		case "/":
			v.searching = true
			v.query = ""
			return v, nil
		case "enter":
			if len(v.results) > 0 && v.cursor < len(v.results) {
				entry := v.results[v.cursor]
				v.loading = true
				v.message = fmt.Sprintf("Installing %s...", entry.Name)
				return v, v.doInstall(entry)
			}
		case "t":
			// Cycle type filter.
			types := []string{"", "config", "transform", "store", "ui", "workspace"}
			idx := 0
			for i, t := range types {
				if t == v.typeFilter {
					idx = i
					break
				}
			}
			v.typeFilter = types[(idx+1)%len(types)]
			v.loading = true
			return v, v.doSearch()
		case "o":
			// Cycle sort order.
			sorts := []string{"relevance", "downloads", "rating", "updated"}
			idx := 0
			for i, s := range sorts {
				if s == v.sortBy {
					idx = i
					break
				}
			}
			v.sortBy = sorts[(idx+1)%len(sorts)]
			v.loading = true
			return v, v.doSearch()
		}
	}
	return v, nil
}

func (v MarketplaceView) updateSearchInput(msg tea.KeyMsg) (MarketplaceView, tea.Cmd) {
	switch msg.String() {
	case "enter":
		v.searching = false
		v.loading = true
		return v, v.doSearch()
	case "esc":
		v.searching = false
		return v, nil
	case "backspace":
		if len(v.query) > 0 {
			v.query = v.query[:len(v.query)-1]
		}
	default:
		if len(msg.String()) == 1 {
			v.query += msg.String()
		}
	}
	return v, nil
}

func (v MarketplaceView) doSearch() tea.Cmd {
	query := zhiui.MarketplaceQuery{
		Query:   v.query,
		Type:    v.typeFilter,
		Sort:    v.sortBy,
		Page:    1,
		PerPage: 50,
	}
	ctx := v.ctx
	ctrl := v.ctrl
	return func() tea.Msg {
		results, err := ctrl.SearchMarketplace(ctx, query)
		return marketplaceResultsMsg{results: results, err: err}
	}
}

func (v MarketplaceView) doInstall(entry zhiui.MarketplaceEntry) tea.Cmd {
	ref := fmt.Sprintf("%s/%s", entry.Publisher, entry.Name)
	ctx := v.ctx
	ctrl := v.ctrl
	return func() tea.Msg {
		result, err := ctrl.InstallPlugin(ctx, ref)
		return marketplaceInstallMsg{result: result, err: err}
	}
}

func (v *MarketplaceView) ensureVisible() {
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	// Each entry takes 3 lines.
	visibleItems := max((v.height-6)/3, 1)
	if v.cursor >= v.offset+visibleItems {
		v.offset = v.cursor - visibleItems + 1
	}
}

// SelectedEntry returns the currently selected marketplace entry, if any.
func (v MarketplaceView) SelectedEntry() (zhiui.MarketplaceEntry, bool) {
	if v.cursor >= 0 && v.cursor < len(v.results) {
		return v.results[v.cursor], true
	}
	return zhiui.MarketplaceEntry{}, false
}

// View renders the marketplace view.
func (v MarketplaceView) View() string {
	var sb strings.Builder

	// Search bar.
	searchLabel := "  Search: "
	if v.searching {
		sb.WriteString(searchLabel + v.query + "_")
	} else if v.query != "" {
		sb.WriteString(searchLabel + v.query)
	} else {
		sb.WriteString(searchLabel + DimStyle.Render("press / to search"))
	}
	sb.WriteString("\n")

	// Filters.
	typeLabel := "All"
	if v.typeFilter != "" {
		typeLabel = v.typeFilter
	}
	sb.WriteString(fmt.Sprintf("  Filter: [%s]  Sort: [%s]  (t:type o:sort)", typeLabel, v.sortBy))
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

	if len(v.results) == 0 {
		sb.WriteString(DimStyle.Render("  No results found."))
		sb.WriteString("\n")
		return v.padHeight(sb.String())
	}

	// Results count.
	sb.WriteString(DimStyle.Render(fmt.Sprintf("  %d results", v.total)))
	sb.WriteString("\n")

	// Result list.
	visibleItems := max((v.height-8)/3, 1)
	end := min(v.offset+visibleItems, len(v.results))
	for i := v.offset; i < end; i++ {
		entry := v.results[i]
		selected := i == v.cursor
		sb.WriteString(v.renderEntry(entry, selected))
	}

	return v.padHeight(sb.String())
}

func (v MarketplaceView) renderEntry(entry zhiui.MarketplaceEntry, selected bool) string {
	var sb strings.Builder

	// Rating and name line.
	rating := fmt.Sprintf("%.1f", entry.Rating)
	verified := ""
	if entry.Verified {
		verified = " [verified]"
	}
	typeStr := entry.Type

	nameStr := fmt.Sprintf("  %.4s  %-30s %10s%s", rating, entry.Name, typeStr, verified)
	descStr := fmt.Sprintf("        %s", entry.Description)
	metaStr := fmt.Sprintf("        by %s  v%s  %d downloads",
		entry.Publisher, entry.LatestVersion, entry.Downloads)

	if entry.Installed {
		installed := fmt.Sprintf("  [installed v%s]", entry.InstalledVer)
		if entry.UpdateAvail {
			installed += " [update available]"
		}
		metaStr += installed
	}

	if selected {
		plain := fmt.Sprintf("> %.4s  %-30s %10s%s", rating, entry.Name, typeStr, verified)
		nameStr = ActiveStyle.Render(lipgloss.PlaceHorizontal(v.width, lipgloss.Left, plain))
		descStr = ActiveStyle.Render(lipgloss.PlaceHorizontal(v.width, lipgloss.Left,
			fmt.Sprintf("        %s", entry.Description)))
		metaStr = ActiveStyle.Render(lipgloss.PlaceHorizontal(v.width, lipgloss.Left,
			fmt.Sprintf("        by %s  v%s  %d downloads", entry.Publisher, entry.LatestVersion, entry.Downloads)))
	}

	sb.WriteString(nameStr + "\n")
	sb.WriteString(descStr + "\n")
	sb.WriteString(metaStr + "\n")

	return sb.String()
}

func (v MarketplaceView) padHeight(content string) string {
	lines := strings.Count(content, "\n")
	for lines < v.height {
		content += "\n"
		lines++
	}
	return content
}
