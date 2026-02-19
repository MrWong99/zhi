// Package mcpbridge maps the zhi UI Controller interface to MCP (Model
// Context Protocol) tools, resources, and prompts. It provides a shared
// bridge library used by both the builtin mcp-stdio plugin and the
// external mcp-sse plugin.
package mcpbridge

import (
	"context"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// SafeController wraps a ui.Controller with a sync.Mutex to serialize all
// calls. This is necessary because multiple MCP sessions (SSE) may share
// one Controller, and UIController's cached tree is not synchronized.
type SafeController struct {
	mu   sync.Mutex
	ctrl ui.Controller
}

// NewSafeController wraps ctrl so that every method call is serialized.
func NewSafeController(ctrl ui.Controller) *SafeController {
	return &SafeController{ctrl: ctrl}
}

func (s *SafeController) LoadTree(ctx context.Context) (*config.Tree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.LoadTree(ctx)
}

func (s *SafeController) SetValue(ctx context.Context, path string, value config.Value) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.SetValue(ctx, path, value)
}

func (s *SafeController) Validate(ctx context.Context) ([]config.ValidationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.Validate(ctx)
}

func (s *SafeController) SaveTree(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.SaveTree(ctx)
}

func (s *SafeController) ExportTemplates(ctx context.Context) ([]ui.ExportTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.ExportTemplates(ctx)
}

func (s *SafeController) Export(ctx context.Context, req ui.ExportRequest) (*ui.ExportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.Export(ctx, req)
}

func (s *SafeController) Apply(ctx context.Context, target string, handler func(ui.ApplyEvent)) (*ui.ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.Apply(ctx, target, handler)
}

func (s *SafeController) ListComponents(ctx context.Context) ([]ui.ComponentInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.ListComponents(ctx)
}

func (s *SafeController) EnableComponent(ctx context.Context, name string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.EnableComponent(ctx, name)
}

func (s *SafeController) DisableComponent(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.DisableComponent(ctx, name)
}

func (s *SafeController) WorkspaceName(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.WorkspaceName(ctx)
}

func (s *SafeController) SearchMarketplace(ctx context.Context, query ui.MarketplaceQuery) (*ui.MarketplaceResults, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.SearchMarketplace(ctx, query)
}

func (s *SafeController) GetMarketplaceDetail(ctx context.Context, pluginType, publisher, name string) (*ui.MarketplaceDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.GetMarketplaceDetail(ctx, pluginType, publisher, name)
}

func (s *SafeController) InstallPlugin(ctx context.Context, ref string) (*ui.InstallResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.InstallPlugin(ctx, ref)
}

func (s *SafeController) UninstallPlugin(ctx context.Context, name string, pluginType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.UninstallPlugin(ctx, name, pluginType)
}

func (s *SafeController) ListInstalledPlugins(ctx context.Context) ([]ui.InstalledPlugin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.ListInstalledPlugins(ctx)
}

func (s *SafeController) CheckUpdates(ctx context.Context) ([]ui.PluginUpdate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.CheckUpdates(ctx)
}

func (s *SafeController) UpdatePlugin(ctx context.Context, name string, version string) (*ui.InstallResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.UpdatePlugin(ctx, name, version)
}

func (s *SafeController) RatePlugin(ctx context.Context, pluginType, publisher, name string, rating ui.Rating) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.RatePlugin(ctx, pluginType, publisher, name, rating)
}

func (s *SafeController) StoreAuthMethods(ctx context.Context) ([]ui.StoreAuthMethod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.StoreAuthMethods(ctx)
}

func (s *SafeController) StoreLogin(ctx context.Context, method string, credentials map[string]string) (*ui.StoreSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.StoreLogin(ctx, method, credentials)
}

func (s *SafeController) StoreLoginInteractive(ctx context.Context, method string, params map[string]string) (*ui.StoreInteractiveChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.StoreLoginInteractive(ctx, method, params)
}

func (s *SafeController) StoreLoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*ui.StoreSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.StoreLoginInteractiveCallback(ctx, challengeID, callbackParams)
}

func (s *SafeController) StoreAuthStatus(ctx context.Context) (*ui.StoreSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.StoreAuthStatus(ctx)
}

func (s *SafeController) StoreLogout(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctrl.StoreLogout(ctx)
}
