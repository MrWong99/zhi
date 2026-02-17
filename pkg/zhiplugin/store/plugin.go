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
//
// Plugins may support different subsets of the interface. Methods that
// do not apply to a plugin's capabilities should return a descriptive
// error (e.g. "versioning not supported"). The Capabilities method
// lets the host discover which features are available.
type Plugin interface {
	// --- Capabilities ---

	// Capabilities reports the store's supported features including
	// versioning mode, encryption state, and auth/access control.
	Capabilities(ctx context.Context) (*Capabilities, error)

	// --- Authentication ---

	// AuthMethods returns the authentication methods supported by this
	// store. Returns nil or empty if authentication is not required.
	AuthMethods(ctx context.Context) ([]AuthMethod, error)
	// Login authenticates with the store using the given method and
	// credentials. The plugin stores the resulting session internally;
	// the returned Credential is informational for the host/UI.
	Login(ctx context.Context, method string, credentials map[string]string) (*Credential, error)
	// LoginInteractive starts an interactive authentication flow (e.g.
	// OIDC). The store returns an InteractiveChallenge containing a URL
	// to open in the user's browser and metadata needed to complete the
	// flow. The params map carries method-specific parameters such as
	// "role" and "redirect_uri".
	LoginInteractive(ctx context.Context, method string, params map[string]string) (*InteractiveChallenge, error)
	// LoginInteractiveCallback completes an interactive auth flow with
	// the callback parameters received from the identity provider
	// (e.g. "code", "state"). The challengeID must match the one
	// returned by LoginInteractive.
	LoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*Credential, error)

	// --- Tree management ---

	// ListTrees returns all tree IDs the current identity has access to.
	ListTrees(ctx context.Context) ([]string, error)
	// DeleteTree removes an entire tree and all its versions/values.
	DeleteTree(ctx context.Context, id string) error

	// --- Value operations (structure-aware) ---

	// GetValues retrieves specific values from a tree. The paths slice
	// specifies exactly which values to read, enabling the plugin to
	// fetch them directly without recursive traversal. Paths that do
	// not exist are omitted from the result.
	GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error)
	// PutValues writes specific values to a tree. Only the specified
	// paths are created or updated; other existing paths are not
	// affected. The opts parameter may be nil when no CAS is needed.
	PutValues(ctx context.Context, id string, values map[string]config.Value, opts *PutOptions) error
	// DeleteValues removes specific values from a tree.
	DeleteValues(ctx context.Context, id string, paths []string) error

	// --- Tree-level versioning (VersioningTree mode) ---

	// ListTreeVersions returns version identifiers for the tree,
	// ordered newest first. Returns an error if versioning mode is not
	// VersioningTree.
	ListTreeVersions(ctx context.Context, id string) ([]string, error)
	// GetTreeVersion retrieves specific values from a specific tree
	// version. Returns an error if versioning mode is not VersioningTree.
	GetTreeVersion(ctx context.Context, id string, version string, paths []string) (map[string]config.Value, error)
	// RollbackTree restores the tree to a previous version, creating a
	// new version whose contents match the specified version. Returns an
	// error if versioning mode is not VersioningTree.
	RollbackTree(ctx context.Context, id string, version string) error
	// DeleteTreeVersion permanently removes a single tree version.
	// Returns an error if versioning mode is not VersioningTree.
	DeleteTreeVersion(ctx context.Context, id string, version string) error

	// --- Value-level versioning (VersioningValue mode) ---

	// ListValueVersions returns version identifiers for a specific
	// value, ordered newest first. Returns an error if versioning mode
	// is not VersioningValue.
	ListValueVersions(ctx context.Context, id string, path string) ([]string, error)
	// GetValueVersion retrieves a specific version of a single value.
	// Returns an error if versioning mode is not VersioningValue.
	GetValueVersion(ctx context.Context, id string, path string, version string) (config.Value, bool, error)
	// RollbackValue restores a value to a previous version, creating
	// a new version with the old content. Returns an error if
	// versioning mode is not VersioningValue.
	RollbackValue(ctx context.Context, id string, path string, version string) error
	// DeleteValueVersion permanently removes a single value version.
	// Returns an error if versioning mode is not VersioningValue.
	DeleteValueVersion(ctx context.Context, id string, path string, version string) error

	// --- Encryption ---

	// InitEncryption initializes encryption for the store with the
	// given passphrase. The exact semantics depend on the implementation.
	InitEncryption(ctx context.Context, passphrase []byte) error
	// RotateEncryption re-encrypts all stored data with a new passphrase.
	RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error

	// --- Access control ---

	// GrantAccess grants a user access to specific paths within a tree.
	// Returns an error if the store does not support access control.
	GrantAccess(ctx context.Context, id string, user string, permissions []Permission) error
	// RevokeAccess removes a user's access to specific paths within a
	// tree. Returns an error if the store does not support access control.
	RevokeAccess(ctx context.Context, id string, user string, paths []string) error
	// ListAccess returns the current access permissions for a tree,
	// keyed by user. Returns an error if the store does not support
	// access control.
	ListAccess(ctx context.Context, id string) (map[string][]Permission, error)
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
