// Package ui defines the UI abstraction layer for zhi. It provides the
// UIDriver interface, the UIController API surface, and a driver registry
// that decouples the core engine from any specific user interface.
package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// UIDriver is the interface that all UI frontends must implement.
type UIDriver interface {
	// Run starts the UI and blocks until the user exits or ctx is cancelled.
	// The UIController provides all operations the UI can perform.
	Run(ctx context.Context, controller *UIController) error
}

// UIController is the engine-facing API that all UIs consume. It wraps the
// core engine and provides a stable surface for UI operations.
type UIController struct {
	engine         *core.Engine
	tree           *config.Tree
	marketplace    *marketplace.Client
	marketplaceErr error
}

// NewUIController creates a new UIController wrapping the given engine.
func NewUIController(engine *core.Engine) *UIController {
	return &UIController{
		engine: engine,
	}
}

// SetMarketplace configures the marketplace client for browsing and
// managing plugins. If not called, marketplace operations return
// ErrMarketplaceNotConfigured.
func (c *UIController) SetMarketplace(mc *marketplace.Client) {
	c.marketplace = mc
	c.marketplaceErr = nil
}

// SetMarketplaceError records a marketplace configuration error so that
// marketplace operations surface the underlying cause rather than the
// generic ErrMarketplaceNotConfigured.
func (c *UIController) SetMarketplaceError(err error) {
	c.marketplaceErr = err
}

// LoadTree loads or reloads the full configuration tree from the config
// provider, applying display transforms.
func (c *UIController) LoadTree(ctx context.Context) (*config.Tree, error) {
	tree, err := c.engine.LoadTree(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading tree: %w", err)
	}
	if err := c.engine.TransformForDisplay(ctx, tree); err != nil {
		return nil, fmt.Errorf("transforming tree: %w", err)
	}
	c.tree = tree
	return tree, nil
}

// FilteredTree returns the tree filtered to only include paths belonging
// to enabled components (plus unmanaged paths).
func (c *UIController) FilteredTree(ctx context.Context) (*config.Tree, error) {
	tree, err := c.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	return c.engine.Components().FilterTree(tree), nil
}

// Tree returns the currently cached tree without reloading.
func (c *UIController) Tree() *config.Tree {
	return c.tree
}

// GetValue retrieves a single value from the cached tree.
func (c *UIController) GetValue(path string) (config.Value, bool) {
	if c.tree == nil {
		return config.Value{}, false
	}
	return c.tree.Get(path)
}

// SetValue stores a value at the given path via the config provider and
// updates the cached tree.
func (c *UIController) SetValue(ctx context.Context, path string, value config.Value) error {
	if err := c.engine.SetValue(ctx, path, value); err != nil {
		return err
	}
	// Update the cached tree.
	if c.tree != nil {
		if err := c.tree.Set(path, &value); err != nil {
			return err
		}
	}
	return nil
}

// Validate runs config provider validation for all paths and component
// dependency validation. Returns all validation results.
func (c *UIController) Validate(ctx context.Context) ([]config.ValidationResult, error) {
	if c.tree == nil {
		if _, err := c.LoadTree(ctx); err != nil {
			return nil, err
		}
	}
	return c.engine.Validate(ctx, c.tree)
}

// SaveTree persists the current tree and component state to the store.
func (c *UIController) SaveTree(ctx context.Context) error {
	if c.tree == nil {
		return fmt.Errorf("no tree loaded")
	}
	// Apply save transforms.
	if err := c.engine.TransformForSave(ctx, c.tree); err != nil {
		return fmt.Errorf("transforming tree for save: %w", err)
	}
	return c.engine.SaveTree(ctx, c.tree)
}

// ListComponents returns all components with their current state.
func (c *UIController) ListComponents() []core.ComponentState {
	return c.engine.Components().ListComponents()
}

// EnableComponent enables a component and its dependencies. Returns the
// list of all components that were enabled (including auto-enabled deps).
func (c *UIController) EnableComponent(name string) ([]string, error) {
	return c.engine.Components().EnableWithReport(name)
}

// DisableComponent disables a component. Returns an error if the component
// is mandatory or has enabled dependents.
func (c *UIController) DisableComponent(name string) error {
	return c.engine.Components().Disable(name)
}

// IsComponentEnabled checks whether a component is currently enabled.
func (c *UIController) IsComponentEnabled(name string) bool {
	return c.engine.Components().IsEnabled(name)
}

// PathBelongsToComponent returns the component name that owns the given
// config path, if any.
func (c *UIController) PathBelongsToComponent(path string) (string, bool) {
	return c.engine.Components().PathBelongsToComponent(path)
}

// ComponentDefinition returns the definition of a component by name.
func (c *UIController) ComponentDefinition(name string) (core.ComponentDef, bool) {
	return c.engine.Components().Definition(name)
}

// ComponentDependents returns the components that depend on the named component.
func (c *UIController) ComponentDependents(name string) []string {
	return c.engine.Components().Dependents(name)
}

// Export runs a single export operation.
func (c *UIController) Export(ctx context.Context, cfg core.ExportRunConfig) (*core.ExportResult, error) {
	td, err := c.prepareTreeData(ctx, cfg.AllComponents, cfg.Prefix)
	if err != nil {
		return nil, err
	}
	return core.Export(ctx, td, cfg)
}

// ExportPreview renders an export template without writing to disk.
func (c *UIController) ExportPreview(ctx context.Context, cfg core.ExportRunConfig) (string, error) {
	cfg.DryRun = true
	cfg.OutputPath = ""
	result, err := c.Export(ctx, cfg)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// ExportAll runs all exports defined in the workspace configuration.
func (c *UIController) ExportAll(ctx context.Context) ([]*core.ExportResult, error) {
	ws := c.engine.Workspace()
	if ws == nil || len(ws.Export.Templates) == 0 {
		return nil, fmt.Errorf("no export templates defined")
	}

	td, err := c.prepareTreeData(ctx, false, "")
	if err != nil {
		return nil, err
	}

	var configs []core.ExportRunConfig
	for _, tmpl := range ws.Export.Templates {
		cfg := core.ExportRunConfig{
			Prefix: tmpl.Prefix,
		}
		if tmpl.Template != "" {
			p := tmpl.Template
			if !filepath.IsAbs(p) {
				p = filepath.Join(ws.Dir, p)
			}
			cfg.TemplatePath = p
		}
		if tmpl.Format != "" {
			cfg.Format = tmpl.Format
		}
		if tmpl.Output != "" {
			p := tmpl.Output
			if !filepath.IsAbs(p) {
				p = filepath.Join(ws.Dir, p)
			}
			cfg.OutputPath = p
		}
		configs = append(configs, cfg)
	}

	return core.ExportAll(ctx, td, configs)
}

// ExportTemplates returns the export templates defined in the workspace.
func (c *UIController) ExportTemplates() []core.ExportTemplate {
	ws := c.engine.Workspace()
	if ws == nil {
		return nil
	}
	return ws.Export.Templates
}

// Apply runs the apply command and streams output to the provided channel.
func (c *UIController) Apply(ctx context.Context, target string, output chan<- core.ApplyOutput) (*core.ApplyResult, error) {
	runCfg, err := c.engine.BuildApplyRunConfig(target)
	if err != nil {
		return nil, err
	}
	return core.Apply(ctx, runCfg, output)
}

// ApplyConfig returns the workspace apply configuration.
func (c *UIController) ApplyConfig() *core.ApplyConfig {
	ws := c.engine.Workspace()
	if ws == nil {
		return nil
	}
	return &ws.Apply
}

// WorkspaceName returns the workspace directory name for display.
func (c *UIController) WorkspaceName() string {
	dir := c.engine.WorkspaceDir()
	if dir == "" {
		return "unknown"
	}
	return filepath.Base(dir)
}

// SaveComponentState persists the current component state to the workspace.
func (c *UIController) SaveComponentState() map[string]bool {
	return c.engine.Components().SaveState()
}

func (c *UIController) prepareTreeData(ctx context.Context, allComponents bool, prefix string) (*core.TreeData, error) {
	return core.PrepareTreeData(ctx, c.engine, allComponents, prefix)
}

// ErrMarketplaceNotConfigured is returned when marketplace operations are
// attempted but no marketplace backend has been configured.
var ErrMarketplaceNotConfigured = errors.New("marketplace not configured")

// marketplaceError returns the appropriate error when the marketplace client
// is not available: the original configuration error if one was recorded,
// or ErrMarketplaceNotConfigured as a fallback.
func (c *UIController) marketplaceError() error {
	if c.marketplaceErr != nil {
		return c.marketplaceErr
	}
	return ErrMarketplaceNotConfigured
}

// SearchMarketplace queries the marketplace for plugins or workspaces.
// Returns ErrMarketplaceNotConfigured if no marketplace client is set.
func (c *UIController) SearchMarketplace(ctx context.Context, query zhiui.MarketplaceQuery) (*zhiui.MarketplaceResults, error) {
	if c.marketplace == nil {
		return nil, c.marketplaceError()
	}

	resp, err := c.marketplace.Search(ctx, query.Query, marketplace.SearchOptions{
		Type:     query.Type,
		Sort:     query.Sort,
		Verified: query.Verified,
		Limit:    query.PerPage,
		Page:     query.Page,
	})
	if err != nil {
		return nil, fmt.Errorf("searching marketplace: %w", err)
	}

	results := make([]zhiui.MarketplaceEntry, 0, len(resp.Results))
	for _, r := range resp.Results {
		publisher := r.Author
		name := r.Name
		if publisher == "" {
			publisher, name = splitPublisher(r.Name)
		}
		results = append(results, zhiui.MarketplaceEntry{
			Name:          name,
			Publisher:     publisher,
			Type:          r.Type,
			Description:   r.Description,
			LatestVersion: r.LatestVersion,
			Rating:        r.Rating,
			RatingCount:   r.RatingCount,
			Downloads:     int(r.Downloads),
			Verified:      r.Verified,
			Platforms:     r.Platforms,
		})
	}

	return &zhiui.MarketplaceResults{
		Total:   resp.Total,
		Results: results,
	}, nil
}

// GetMarketplaceDetail returns detailed information about a marketplace artifact.
func (c *UIController) GetMarketplaceDetail(ctx context.Context, pluginType, publisher, name string) (*zhiui.MarketplaceDetail, error) {
	if c.marketplace == nil {
		return nil, c.marketplaceError()
	}

	detail, err := c.marketplace.GetPlugin(ctx, pluginType, publisher, name)
	if err != nil {
		return nil, fmt.Errorf("getting marketplace detail: %w", err)
	}

	entry := zhiui.MarketplaceEntry{
		Name:        detail.Name,
		Publisher:   detail.Author,
		Type:        detail.Type,
		Description: detail.Description,
		Rating:      detail.Rating.Average,
		RatingCount: detail.Rating.Count,
		Downloads:   int(detail.Statistics.TotalDownloads),
		Verified:    detail.Verified,
	}
	if len(detail.Versions) > 0 {
		entry.LatestVersion = detail.Versions[0].Version
		entry.Platforms = detail.Versions[0].Platforms
	}

	versions := make([]zhiui.VersionEntry, 0, len(detail.Versions))
	for _, v := range detail.Versions {
		versions = append(versions, zhiui.VersionEntry{
			Version:   v.Version,
			Digest:    v.Digest,
			Platforms: v.Platforms,
		})
	}

	return &zhiui.MarketplaceDetail{
		MarketplaceEntry: entry,
		LongDescription:  detail.LongDescription,
		License:          detail.License,
		Homepage:         detail.Homepage,
		Repository:       detail.Repository,
		Versions:         versions,
	}, nil
}

// InstallPlugin downloads and installs a plugin from an OCI reference.
func (c *UIController) InstallPlugin(_ context.Context, _ string) (*zhiui.InstallResult, error) {
	return nil, c.marketplaceError()
}

// UninstallPlugin removes an installed plugin.
func (c *UIController) UninstallPlugin(_ context.Context, _, _ string) error {
	return c.marketplaceError()
}

// ListInstalledPlugins returns all installed plugins with their metadata.
// For now, returns built-in plugins only.
func (c *UIController) ListInstalledPlugins(_ context.Context) ([]zhiui.InstalledPlugin, error) {
	return nil, nil
}

// CheckUpdates returns available updates for installed plugins.
func (c *UIController) CheckUpdates(_ context.Context) ([]zhiui.PluginUpdate, error) {
	return nil, nil
}

// UpdatePlugin updates a specific plugin to the latest (or specified) version.
func (c *UIController) UpdatePlugin(_ context.Context, _, _ string) (*zhiui.InstallResult, error) {
	return nil, c.marketplaceError()
}

// RatePlugin submits a rating for a marketplace plugin.
func (c *UIController) RatePlugin(ctx context.Context, pluginType, publisher, name string, rating zhiui.Rating) error {
	if c.marketplace == nil {
		return c.marketplaceError()
	}

	return c.marketplace.SubmitRating(ctx, pluginType, publisher, name, marketplace.SubmitRatingRequest{
		Score:   rating.Score,
		Comment: rating.Comment,
	})
}

// splitPublisher splits "publisher/name" into its parts. If there is no
// slash the entire string is returned as the name with an empty publisher.
func splitPublisher(s string) (publisher, name string) {
	if i := strings.Index(s, "/"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}
