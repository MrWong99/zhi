package ui

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// Controller provides operations that a UI plugin can invoke on the zhi
// core. For builtin plugins this is backed directly by the engine; for
// external (gRPC) plugins it is a client that calls back to the host via
// the GRPCBroker.
type Controller interface {
	// LoadTree loads or reloads the full configuration tree from the
	// config provider, applying display transforms.
	LoadTree(ctx context.Context) (*config.Tree, error)
	// SetValue stores a configuration value at the given path.
	SetValue(ctx context.Context, path string, value config.Value) error
	// Validate runs validation on the current configuration tree.
	Validate(ctx context.Context) ([]config.ValidationResult, error)
	// SaveTree persists the current tree to the store.
	SaveTree(ctx context.Context) error
	// ExportTemplates returns the workspace export template definitions.
	ExportTemplates(ctx context.Context) ([]ExportTemplate, error)
	// Export runs a single export operation.
	Export(ctx context.Context, req ExportRequest) (*ExportResult, error)
	// Apply runs the apply command for the given target. Output events are
	// delivered via the handler callback as they arrive. The method blocks
	// until the command completes and returns the final result.
	Apply(ctx context.Context, target string, handler func(ApplyEvent)) (*ApplyResult, error)
	// ListComponents returns all components with their current state.
	ListComponents(ctx context.Context) ([]ComponentInfo, error)
	// EnableComponent enables a component and its dependencies. Returns
	// the list of all components that were enabled.
	EnableComponent(ctx context.Context, name string) ([]string, error)
	// DisableComponent disables a component.
	DisableComponent(ctx context.Context, name string) error
	// WorkspaceName returns the workspace display name.
	WorkspaceName(ctx context.Context) (string, error)

	// SearchMarketplace queries the marketplace for plugins or workspaces.
	SearchMarketplace(ctx context.Context, query MarketplaceQuery) (*MarketplaceResults, error)
	// GetMarketplaceDetail returns detailed information about a marketplace artifact.
	GetMarketplaceDetail(ctx context.Context, pluginType, publisher, name string) (*MarketplaceDetail, error)
	// InstallPlugin downloads and installs a plugin from an OCI reference.
	InstallPlugin(ctx context.Context, ref string) (*InstallResult, error)
	// UninstallPlugin removes an installed plugin.
	UninstallPlugin(ctx context.Context, name string, pluginType string) error
	// ListInstalledPlugins returns all installed plugins with their metadata.
	ListInstalledPlugins(ctx context.Context) ([]InstalledPlugin, error)
	// CheckUpdates returns available updates for installed plugins.
	CheckUpdates(ctx context.Context) ([]PluginUpdate, error)
	// UpdatePlugin updates a specific plugin to the latest (or specified) version.
	UpdatePlugin(ctx context.Context, name string, version string) (*InstallResult, error)
	// RatePlugin submits a rating for a marketplace plugin.
	RatePlugin(ctx context.Context, pluginType, publisher, name string, rating Rating) error

	// StoreAuthMethods returns the authentication methods supported by the
	// configured store plugin. Returns nil if no store is configured or
	// authentication is not required.
	StoreAuthMethods(ctx context.Context) ([]StoreAuthMethod, error)
	// StoreLogin authenticates with the store using the specified method
	// and credentials. Returns the session status after the attempt.
	StoreLogin(ctx context.Context, method string, credentials map[string]string) (*StoreSession, error)
	// StoreAuthStatus returns the current authentication status.
	StoreAuthStatus(ctx context.Context) (*StoreSession, error)
	// StoreLogout clears the current authentication session.
	StoreLogout(ctx context.Context) error
}

// Plugin is the interface every zhi UI plugin must implement.
type Plugin interface {
	// Run starts the UI and blocks until the user exits or ctx is
	// cancelled. The Controller provides all operations the UI can
	// perform on the core engine.
	Run(ctx context.Context, controller Controller) error
	// Capabilities reports what this UI plugin supports.
	Capabilities(ctx context.Context) (Capabilities, error)
}

// PluginMap is the map of plugins the host expects. Plugin binaries must
// serve the same key.
var PluginMap = map[string]goplugin.Plugin{
	"ui": &GRPCPlugin{},
}

// GRPCPlugin implements hashicorp/go-plugin's GRPCPlugin interface,
// wiring the gRPC transport to Plugin. It uses the GRPCBroker to
// establish a reverse channel so the plugin can call back into the host's
// UIControllerService.
type GRPCPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	// Impl is set only on the plugin (server) side.
	Impl Plugin
}

func (p *GRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterUIServiceServer(s, &GRPCServer{Impl: p.Impl, broker: broker})
	return nil
}

func (p *GRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &GRPCClient{client: pb.NewUIServiceClient(c), broker: broker}, nil
}
