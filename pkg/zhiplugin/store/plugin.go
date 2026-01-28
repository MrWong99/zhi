package store

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/store/proto"
)

// Plugin is the interface every zhi store plugin must implement.
// The host communicates with it over gRPC.
type Plugin interface {
	// Save persists a configuration tree under the given ID.
	// If versioning is supported, this creates a new version.
	Save(ctx context.Context, id string, tree config.TreeReader) error
	// Load retrieves the latest version of the configuration tree with
	// the given ID. The bool reports whether the tree was found.
	Load(ctx context.Context, id string) (*config.Tree, bool, error)
	// Delete removes the configuration tree and all its versions.
	Delete(ctx context.Context, id string) error
	// ListTrees returns all stored tree IDs.
	ListTrees(ctx context.Context) ([]string, error)

	// SupportsVersioning reports whether this store keeps older versions
	// of trees.
	SupportsVersioning(ctx context.Context) (bool, error)
	// ListVersions returns version identifiers for the given tree ID,
	// ordered newest first. Returns an error if versioning is not
	// supported.
	ListVersions(ctx context.Context, id string) ([]string, error)
	// LoadVersion retrieves a specific version of the configuration tree.
	// Returns an error if versioning is not supported.
	LoadVersion(ctx context.Context, id string, version string) (*config.Tree, bool, error)
	// DeleteVersion permanently removes a single version of a
	// configuration tree. Returns an error if versioning is not supported.
	DeleteVersion(ctx context.Context, id string, version string) error

	// EncryptionStatus reports the current encryption state of the store.
	EncryptionStatus(ctx context.Context) (EncryptionStatus, error)
	// InitEncryption initializes encryption for the store with the given
	// passphrase. The exact semantics depend on the implementation.
	InitEncryption(ctx context.Context, passphrase []byte) error
	// RotateEncryption re-encrypts all stored data with a new passphrase.
	RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error
}

// PluginMap is the map of plugins the host expects. Plugin binaries must
// serve the same key.
var PluginMap = map[string]goplugin.Plugin{
	"store": &GRPCPlugin{},
}

// GRPCPlugin implements hashicorp/go-plugin's GRPCPlugin interface,
// wiring the gRPC transport to Plugin.
type GRPCPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	// Impl is set only on the plugin (server) side.
	Impl Plugin
}

func (p *GRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterStoreServiceServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *GRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &GRPCClient{client: pb.NewStoreServiceClient(c)}, nil
}
