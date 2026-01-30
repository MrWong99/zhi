// Package ui defines the UI abstraction layer that decouples the core engine
// from any specific user interface. It provides the UIDriver interface,
// UIController API surface, and a driver registry.
package ui

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// UIDriver is the interface contract between the engine and any UI frontend.
// Implementations include TUI (Bubbletea), Web UI, AI Chat, headless API, etc.
type UIDriver interface {
	// Run starts the UI and blocks until the user exits or ctx is cancelled.
	// The UIController provides all operations the UI can perform.
	Run(ctx context.Context, controller *UIController) error
}

// UIController is the engine-facing API that all UIs consume. It wraps the
// core Engine to provide a stable surface for UI implementations without
// exposing engine internals.
type UIController struct {
	engine *core.Engine
}

// NewUIController creates a new UIController wrapping the given engine.
func NewUIController(engine *core.Engine) *UIController {
	return &UIController{
		engine: engine,
	}
}

// --- Tree operations ---

// LoadTree loads or reloads the full configuration tree from the config provider.
func (c *UIController) LoadTree(ctx context.Context) (*config.Tree, error) {
	return c.engine.LoadTree(ctx)
}

// FilteredTree returns the tree filtered to only include paths belonging to
// enabled components (plus unmanaged paths).
func (c *UIController) FilteredTree(ctx context.Context) (*config.Tree, error) {
	return c.engine.FilteredTree(ctx)
}

// GetValue retrieves a single value from the configuration tree.
func (c *UIController) GetValue(ctx context.Context, path string) (*config.Value, bool, error) {
	tree, err := c.engine.LoadTree(ctx)
	if err != nil {
		return nil, false, err
	}
	v, ok := tree.Get(path)
	if !ok {
		return nil, false, nil
	}
	return &v, true, nil
}

// SetValue stores a single value at the given path.
func (c *UIController) SetValue(ctx context.Context, path string, value config.Value) error {
	return c.engine.SetValue(ctx, path, value)
}

// Validate runs config provider validation for all paths and component
// dependency constraints.
func (c *UIController) Validate(ctx context.Context) ([]config.ValidationResult, error) {
	tree, err := c.engine.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	return c.engine.Validate(ctx, tree)
}

// SaveTree persists the tree and component state to the store provider.
func (c *UIController) SaveTree(ctx context.Context, id string) error {
	tree, err := c.engine.LoadTree(ctx)
	if err != nil {
		return err
	}
	return c.engine.SaveTree(ctx, id, tree)
}

// --- Component operations ---

// ListComponents returns all components with their current status.
func (c *UIController) ListComponents() []core.ComponentState {
	return c.engine.Components().ListComponents()
}

// EnableComponent enables a component and auto-enables its dependencies.
func (c *UIController) EnableComponent(name string) error {
	return c.engine.Components().Enable(name)
}

// DisableComponent disables a component with guard rails (rejects mandatory
// components and components required by other enabled components).
func (c *UIController) DisableComponent(name string) error {
	return c.engine.Components().Disable(name)
}

// IsComponentEnabled checks whether the named component is currently enabled.
func (c *UIController) IsComponentEnabled(name string) bool {
	return c.engine.Components().IsEnabled(name)
}

// --- Export operations ---

// Export renders a single export using the given configuration.
func (c *UIController) Export(ctx context.Context, cfg core.ExportRunConfig) (*core.ExportResult, error) {
	td, err := core.PrepareTreeData(ctx, c.engine, cfg.AllComponents, cfg.Prefix)
	if err != nil {
		return nil, err
	}
	return core.Export(ctx, td, cfg)
}

// ExportAll exports all workspace templates.
func (c *UIController) ExportAll(ctx context.Context) ([]*core.ExportResult, error) {
	ws := c.engine.Workspace()
	if ws == nil || len(ws.Export.Templates) == 0 {
		return nil, fmt.Errorf("no export templates defined in workspace config")
	}

	td, err := core.PrepareTreeData(ctx, c.engine, false, "")
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

// ExportPreview renders an export preview without writing to disk.
func (c *UIController) ExportPreview(ctx context.Context, cfg core.ExportRunConfig) (string, error) {
	cfg.DryRun = true
	td, err := core.PrepareTreeData(ctx, c.engine, cfg.AllComponents, cfg.Prefix)
	if err != nil {
		return "", err
	}
	result, err := core.Export(ctx, td, cfg)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// --- Apply operations ---

// Apply runs the configured apply command and streams output to the channel.
func (c *UIController) Apply(ctx context.Context, output chan<- core.ApplyOutput) (*core.ApplyResult, error) {
	runCfg, err := c.engine.BuildApplyRunConfig("")
	if err != nil {
		return nil, err
	}
	return core.Apply(ctx, runCfg, output)
}

// ApplyConfig returns the configured apply settings from the workspace.
func (c *UIController) ApplyConfig() *core.ApplyConfig {
	ws := c.engine.Workspace()
	if ws == nil {
		return nil
	}
	ac := ws.Apply
	return &ac
}

// --- Metadata ---

// WorkspaceName returns the workspace directory name for display.
func (c *UIController) WorkspaceName() string {
	dir := c.engine.WorkspaceDir()
	if dir == "" {
		return ""
	}
	return filepath.Base(dir)
}

// ProviderInfo returns provider names for status display.
func (c *UIController) ProviderInfo() map[string]string {
	ws := c.engine.Workspace()
	if ws == nil {
		return nil
	}
	info := map[string]string{
		"config": ws.Config.Provider,
	}
	if ws.Store.Provider != "" {
		info["store"] = ws.Store.Provider
	}
	for i, t := range ws.Transform {
		info[fmt.Sprintf("transform[%d]", i)] = t.Provider
	}
	return info
}
