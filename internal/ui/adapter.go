package ui

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// BuiltinAdapter wraps a legacy UIDriver to implement the zhiui.Plugin
// interface. This allows existing builtin UI drivers (like the TUI) to be
// used through the new plugin system without code changes.
//
// When Run is called, BuiltinAdapter type-asserts the Controller to a
// *ControllerAdapter (which wraps UIController) and passes the inner
// UIController to the legacy driver.
type BuiltinAdapter struct {
	Driver UIDriver
}

// Run starts the legacy UI driver. The controller must be a
// *ControllerAdapter so the driver can access the full UIController.
func (a *BuiltinAdapter) Run(ctx context.Context, controller zhiui.Controller) error {
	ca, ok := controller.(*ControllerAdapter)
	if !ok {
		return fmt.Errorf("builtin UI driver requires a ControllerAdapter (got %T)", controller)
	}
	return a.Driver.Run(ctx, ca.Inner)
}

// Capabilities reports that builtin TUI drivers require TTY access.
func (a *BuiltinAdapter) Capabilities(_ context.Context) (zhiui.Capabilities, error) {
	return zhiui.Capabilities{RequiresTTY: true}, nil
}

// ControllerAdapter wraps a *UIController to implement zhiui.Controller.
// It bridges the internal UIController (which references core types
// directly) to the public plugin Controller interface.
type ControllerAdapter struct {
	Inner *UIController
}

func (c *ControllerAdapter) LoadTree(ctx context.Context) (*config.Tree, error) {
	return c.Inner.LoadTree(ctx)
}

func (c *ControllerAdapter) SetValue(ctx context.Context, path string, value config.Value) error {
	return c.Inner.SetValue(ctx, path, value)
}

func (c *ControllerAdapter) Validate(ctx context.Context) ([]config.ValidationResult, error) {
	return c.Inner.Validate(ctx)
}

func (c *ControllerAdapter) SaveTree(ctx context.Context) error {
	return c.Inner.SaveTree(ctx)
}

func (c *ControllerAdapter) ExportTemplates(_ context.Context) ([]zhiui.ExportTemplate, error) {
	templates := c.Inner.ExportTemplates()
	result := make([]zhiui.ExportTemplate, 0, len(templates))
	for _, t := range templates {
		result = append(result, zhiui.ExportTemplate{
			Name:     t.Name,
			Template: t.Template,
			Format:   t.Format,
			Output:   t.Output,
			Prefix:   t.Prefix,
		})
	}
	return result, nil
}

func (c *ControllerAdapter) Export(ctx context.Context, req zhiui.ExportRequest) (*zhiui.ExportResult, error) {
	cfg := core.ExportRunConfig{
		TemplatePath: req.TemplatePath,
		Format:       req.Format,
		OutputPath:   req.OutputPath,
		Prefix:       req.Prefix,
		DryRun:       req.DryRun,
	}

	// Resolve relative template path against workspace dir.
	if cfg.TemplatePath != "" && !filepath.IsAbs(cfg.TemplatePath) {
		cfg.TemplatePath = filepath.Join(c.Inner.engine.WorkspaceDir(), cfg.TemplatePath)
	}
	if cfg.OutputPath != "" && !filepath.IsAbs(cfg.OutputPath) {
		cfg.OutputPath = filepath.Join(c.Inner.engine.WorkspaceDir(), cfg.OutputPath)
	}

	result, err := c.Inner.Export(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &zhiui.ExportResult{
		Name:       result.Name,
		Content:    result.Content,
		OutputPath: result.OutputPath,
	}, nil
}

func (c *ControllerAdapter) Apply(ctx context.Context, target string, handler func(zhiui.ApplyEvent)) (*zhiui.ApplyResult, error) {
	output := make(chan core.ApplyOutput, 64)

	// Run apply in a goroutine, streaming output to the handler.
	type applyResult struct {
		result *core.ApplyResult
		err    error
	}
	done := make(chan applyResult, 1)
	go func() {
		r, err := c.Inner.Apply(ctx, target, output)
		done <- applyResult{result: r, err: err}
	}()

	// Stream events to the handler.
	for ev := range output {
		if handler != nil {
			handler(zhiui.ApplyEvent{
				Line:   ev.Line,
				Stream: ev.Stream,
			})
		}
	}

	// Wait for the result.
	res := <-done
	result := &zhiui.ApplyResult{}
	if res.result != nil {
		result.ExitCode = res.result.ExitCode
		if res.result.Err != nil {
			result.Error = res.result.Err.Error()
		}
	}
	if res.err != nil {
		result.Error = res.err.Error()
	}
	return result, nil
}

func (c *ControllerAdapter) ListComponents(_ context.Context) ([]zhiui.ComponentInfo, error) {
	states := c.Inner.ListComponents()
	result := make([]zhiui.ComponentInfo, 0, len(states))
	for _, s := range states {
		info := zhiui.ComponentInfo{
			Name:         s.Name,
			Enabled:      s.Enabled,
			Mandatory:    s.Mandatory,
			Dependencies: s.Dependencies,
		}
		// Get additional details from the component definition.
		if def, ok := c.Inner.ComponentDefinition(s.Name); ok {
			info.Description = def.Description
			info.Paths = def.Paths
		}
		result = append(result, info)
	}
	return result, nil
}

func (c *ControllerAdapter) EnableComponent(_ context.Context, name string) ([]string, error) {
	return c.Inner.EnableComponent(name)
}

func (c *ControllerAdapter) DisableComponent(_ context.Context, name string) error {
	return c.Inner.DisableComponent(name)
}

func (c *ControllerAdapter) WorkspaceName(_ context.Context) (string, error) {
	return c.Inner.WorkspaceName(), nil
}

func (c *ControllerAdapter) SearchMarketplace(ctx context.Context, query zhiui.MarketplaceQuery) (*zhiui.MarketplaceResults, error) {
	return c.Inner.SearchMarketplace(ctx, query)
}

func (c *ControllerAdapter) GetMarketplaceDetail(ctx context.Context, pluginType, publisher, name string) (*zhiui.MarketplaceDetail, error) {
	return c.Inner.GetMarketplaceDetail(ctx, pluginType, publisher, name)
}

func (c *ControllerAdapter) InstallPlugin(ctx context.Context, ref string) (*zhiui.InstallResult, error) {
	return c.Inner.InstallPlugin(ctx, ref)
}

func (c *ControllerAdapter) UninstallPlugin(ctx context.Context, name string, pluginType string) error {
	return c.Inner.UninstallPlugin(ctx, name, pluginType)
}

func (c *ControllerAdapter) ListInstalledPlugins(ctx context.Context) ([]zhiui.InstalledPlugin, error) {
	return c.Inner.ListInstalledPlugins(ctx)
}

func (c *ControllerAdapter) CheckUpdates(ctx context.Context) ([]zhiui.PluginUpdate, error) {
	return c.Inner.CheckUpdates(ctx)
}

func (c *ControllerAdapter) UpdatePlugin(ctx context.Context, name string, version string) (*zhiui.InstallResult, error) {
	return c.Inner.UpdatePlugin(ctx, name, version)
}

func (c *ControllerAdapter) RatePlugin(ctx context.Context, pluginType, publisher, name string, rating zhiui.Rating) error {
	return c.Inner.RatePlugin(ctx, pluginType, publisher, name, rating)
}

func (c *ControllerAdapter) StoreAuthMethods(ctx context.Context) ([]zhiui.StoreAuthMethod, error) {
	return c.Inner.StoreAuthMethods(ctx)
}

func (c *ControllerAdapter) StoreLogin(ctx context.Context, method string, credentials map[string]string) (*zhiui.StoreSession, error) {
	return c.Inner.StoreLogin(ctx, method, credentials)
}

func (c *ControllerAdapter) StoreLoginInteractive(ctx context.Context, method string, params map[string]string) (*zhiui.StoreInteractiveChallenge, error) {
	return c.Inner.StoreLoginInteractive(ctx, method, params)
}

func (c *ControllerAdapter) StoreLoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*zhiui.StoreSession, error) {
	return c.Inner.StoreLoginInteractiveCallback(ctx, challengeID, callbackParams)
}

func (c *ControllerAdapter) StoreAuthStatus(ctx context.Context) (*zhiui.StoreSession, error) {
	return c.Inner.StoreAuthStatus(ctx)
}

func (c *ControllerAdapter) StoreLogout(ctx context.Context) error {
	return c.Inner.StoreLogout(ctx)
}
