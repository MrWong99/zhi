package webui

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// handleMarketplacePage renders the marketplace search page.
// GET /marketplace
func (s *Server) handleMarketplacePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsName, _ := s.ctrl.WorkspaceName(ctx)

	query := buildMarketplaceQuery(r)

	results, err := s.ctrl.SearchMarketplace(ctx, query)
	if err != nil {
		// Marketplace unavailable: show the page with an error message.
		data := pageData{
			WorkspaceName:      wsName,
			ActiveNav:          "marketplace",
			Nonce:              nonceFromCtx(ctx),
			CSRFToken:          csrfFromCtx(ctx),
			MarketplaceError:   "Marketplace is currently unavailable: " + err.Error(),
			MarketplaceQuery:   query.Query,
			MarketplaceType:    query.Type,
			MarketplaceSort:    query.Sort,
			MarketplaceVerOnly: query.Verified,
			Breadcrumbs: []breadcrumb{
				{Label: "Marketplace", Href: "/marketplace"},
			},
		}
		if isHTMX(r) {
			if renderErr := s.engine.renderFragment(w, "marketplace_grid", data); renderErr != nil {
				s.renderError(w, r, http.StatusInternalServerError, renderErr.Error())
			}
			return
		}
		if renderErr := s.engine.renderPage(w, "marketplace", data); renderErr != nil {
			s.renderError(w, r, http.StatusInternalServerError, renderErr.Error())
		}
		return
	}

	data := pageData{
		WorkspaceName:        wsName,
		ActiveNav:            "marketplace",
		Nonce:                nonceFromCtx(ctx),
		CSRFToken:            csrfFromCtx(ctx),
		MarketplaceResults:   toMarketplaceCardData(results.Results),
		MarketplaceTotal:     results.Total,
		MarketplaceQuery:     query.Query,
		MarketplaceType:      query.Type,
		MarketplaceSort:      query.Sort,
		MarketplaceVerOnly:   query.Verified,
		MarketplacePage:      query.Page,
		MarketplacePageCount: pageCount(results.Total, query.PerPage),
		Breadcrumbs: []breadcrumb{
			{Label: "Marketplace", Href: "/marketplace"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "marketplace_grid", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := s.engine.renderPage(w, "marketplace", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handlePluginDetail renders the plugin detail page.
// GET /marketplace/{publisher}/{name}
func (s *Server) handlePluginDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	publisher := r.PathValue("publisher")
	name := r.PathValue("name")
	if publisher == "" || name == "" {
		s.renderError(w, r, http.StatusBadRequest, "publisher and name are required")
		return
	}

	wsName, _ := s.ctrl.WorkspaceName(ctx)

	detail, err := s.ctrl.GetMarketplaceDetail(ctx, publisher, name)
	if err != nil {
		s.renderError(w, r, http.StatusNotFound, "Plugin not found: "+err.Error())
		return
	}

	data := pageData{
		WorkspaceName: wsName,
		ActiveNav:     "marketplace",
		Nonce:         nonceFromCtx(ctx),
		CSRFToken:     csrfFromCtx(ctx),
		PluginDetail:  toPluginDetailData(detail),
		Breadcrumbs: []breadcrumb{
			{Label: "Marketplace", Href: "/marketplace"},
			{Label: publisher + "/" + name, Href: "/marketplace/" + publisher + "/" + name},
		},
	}

	if err := s.engine.renderPage(w, "plugin_detail", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleRatePlugin submits a rating for a plugin.
// POST /marketplace/{publisher}/{name}/rate
func (s *Server) handleRatePlugin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	publisher := r.PathValue("publisher")
	name := r.PathValue("name")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	scoreStr := r.FormValue("score")
	score, err := strconv.Atoi(scoreStr)
	if err != nil || score < 1 || score > 5 {
		http.Error(w, "score must be between 1 and 5", http.StatusBadRequest)
		return
	}

	rating := ui.Rating{
		Score:   score,
		Comment: r.FormValue("comment"),
	}

	if err := s.ctrl.RatePlugin(ctx, publisher, name, rating); err != nil {
		w.Header().Set("HX-Trigger", triggerNotification("error", "Failed to submit rating: "+err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", triggerNotification("success", "Rating submitted"))
	// Re-render the detail page to show updated ratings.
	detail, err := s.ctrl.GetMarketplaceDetail(ctx, publisher, name)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	data := toPluginDetailData(detail)
	if renderErr := s.engine.renderFragment(w, "rating_section", data); renderErr != nil {
		w.WriteHeader(http.StatusOK)
	}
}

// buildMarketplaceQuery constructs a MarketplaceQuery from request parameters.
func buildMarketplaceQuery(r *http.Request) ui.MarketplaceQuery {
	q := ui.MarketplaceQuery{
		Query:   r.URL.Query().Get("q"),
		Type:    r.URL.Query().Get("type"),
		Sort:    r.URL.Query().Get("sort"),
		PerPage: 20,
	}
	if q.Sort == "" {
		q.Sort = "relevance"
	}
	if r.URL.Query().Get("verified") == "true" {
		q.Verified = true
	}
	if page, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && page > 0 {
		q.Page = page
	} else {
		q.Page = 1
	}
	return q
}

// pageCount calculates the total number of pages.
func pageCount(total, perPage int) int {
	if perPage <= 0 {
		return 1
	}
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	if pages < 1 {
		return 1
	}
	return pages
}

// marketplaceCardData holds data for a single marketplace result card.
type marketplaceCardData struct {
	Name          string
	Publisher     string
	Type          string
	Description   string
	LatestVersion string
	Rating        float64
	RatingStars   string
	RatingCount   int
	Downloads     string
	Verified      bool
	Installed     bool
	InstalledVer  string
	UpdateAvail   bool
	Platforms     []string
}

// pluginDetailData holds data for the plugin detail page.
type pluginDetailData struct {
	marketplaceCardData
	LongDescription string
	License         string
	Homepage        string
	Repository      string
	Versions        []ui.VersionEntry
	Ratings         []ui.RatingEntry
	Dependencies    []ui.DependencyEntry
	Keywords        []string
}

// toMarketplaceCardData converts marketplace entries to card view data.
func toMarketplaceCardData(entries []ui.MarketplaceEntry) []marketplaceCardData {
	result := make([]marketplaceCardData, len(entries))
	for i, e := range entries {
		result[i] = marketplaceCardData{
			Name:          e.Name,
			Publisher:     e.Publisher,
			Type:          e.Type,
			Description:   e.Description,
			LatestVersion: e.LatestVersion,
			Rating:        e.Rating,
			RatingStars:   ratingStars(e.Rating),
			RatingCount:   e.RatingCount,
			Downloads:     formatDownloads(e.Downloads),
			Verified:      e.Verified,
			Installed:     e.Installed,
			InstalledVer:  e.InstalledVer,
			UpdateAvail:   e.UpdateAvail,
			Platforms:     e.Platforms,
		}
	}
	return result
}

// toPluginDetailData converts a marketplace detail to the template data.
func toPluginDetailData(d *ui.MarketplaceDetail) *pluginDetailData {
	return &pluginDetailData{
		marketplaceCardData: marketplaceCardData{
			Name:          d.Name,
			Publisher:     d.Publisher,
			Type:          d.Type,
			Description:   d.Description,
			LatestVersion: d.LatestVersion,
			Rating:        d.Rating,
			RatingStars:   ratingStars(d.Rating),
			RatingCount:   d.RatingCount,
			Downloads:     formatDownloads(d.Downloads),
			Verified:      d.Verified,
			Installed:     d.Installed,
			InstalledVer:  d.InstalledVer,
			UpdateAvail:   d.UpdateAvail,
			Platforms:     d.Platforms,
		},
		LongDescription: d.LongDescription,
		License:         d.License,
		Homepage:        d.Homepage,
		Repository:      d.Repository,
		Versions:        d.Versions,
		Ratings:         d.Ratings,
		Dependencies:    d.Dependencies,
		Keywords:        d.Keywords,
	}
}

// ratingStars returns a string of filled and empty star characters.
func ratingStars(rating float64) string {
	full := int(rating)
	empty := 5 - full
	s := ""
	for range full {
		s += "\u2605" // ★
	}
	for range empty {
		s += "\u2606" // ☆
	}
	return s
}

// formatDownloads returns a human-friendly download count.
func formatDownloads(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return strconv.Itoa(n)
	}
}
