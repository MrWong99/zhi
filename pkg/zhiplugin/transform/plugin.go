package transform

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/transform/proto"
)

// Plugin is the interface every zhi transform plugin must implement.
// The host communicates with it over gRPC.
type Plugin interface {
	// BeforeDisplay transforms the configuration tree before values are
	// displayed in the UI. The tree is mutable: use GetPtr to modify
	// existing values in place, Set to add new entries, and List to
	// enumerate all paths.
	BeforeDisplay(ctx context.Context, tree *config.Tree) error
	// AfterSave transforms the configuration tree after the UI stores
	// updates. The same mutability rules as BeforeDisplay apply.
	AfterSave(ctx context.Context, tree *config.Tree) error
	// ValidatePolicy reports when config-plugin validations should run
	// relative to this plugin's transformations.
	ValidatePolicy(ctx context.Context) (ValidatePolicy, error)
}

// PluginMap is the map of plugins the host expects. Plugin binaries must
// serve the same key.
var PluginMap = map[string]goplugin.Plugin{
	"transform": &GRPCPlugin{},
}

// GRPCPlugin implements hashicorp/go-plugin's GRPCPlugin interface,
// wiring the gRPC transport to Plugin.
type GRPCPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	// Impl is set only on the plugin (server) side.
	Impl Plugin
}

func (p *GRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterTransformServiceServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *GRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &GRPCClient{client: pb.NewTransformServiceClient(c)}, nil
}
