package config

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
)

// Plugin is the interface every zhi configuration plugin must implement.
// The host communicates with it over gRPC.
type Plugin interface {
	// List returns every configuration path managed by this plugin.
	List(ctx context.Context) ([]string, error)
	// Get retrieves a single configuration value by path.
	Get(ctx context.Context, path string) (Value, bool, error)
	// Set stores a configuration value at the given path.
	Set(ctx context.Context, path string, v Value) error
	// Validate checks the value at path. The tree provides read-only access
	// to the full merged configuration so cross-value validation is possible.
	Validate(ctx context.Context, path string, tree TreeReader) ([]ValidationResult, error)
}

// PluginMap is the map of plugins the host expects. Plugin binaries must
// serve the same key.
var PluginMap = map[string]goplugin.Plugin{
	"config": &GRPCPlugin{},
}

// GRPCPlugin implements hashicorp/go-plugin's GRPCPlugin interface,
// wiring the gRPC transport to Plugin.
type GRPCPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	// Impl is set only on the plugin (server) side.
	Impl Plugin
}

func (p *GRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterConfigServiceServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *GRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &GRPCClient{client: pb.NewConfigServiceClient(c)}, nil
}
