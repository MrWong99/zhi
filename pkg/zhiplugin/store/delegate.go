package store

import (
	"context"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// Compile-time assertion that DelegatingPlugin implements Plugin.
var _ Plugin = (*DelegatingPlugin)(nil)

// DelegatingPlugin forwards all store.Plugin calls to Base.
// Embed this struct in your own type and override only the methods you
// need. Use [NewDelegatingPlugin] to create one safely.
type DelegatingPlugin struct {
	Base Plugin
}

// NewDelegatingPlugin creates a DelegatingPlugin that delegates to base.
// It panics if base is nil.
func NewDelegatingPlugin(base Plugin) DelegatingPlugin {
	if base == nil {
		panic("store.NewDelegatingPlugin: base must not be nil")
	}
	return DelegatingPlugin{Base: base}
}

// --- Capabilities ---

func (d *DelegatingPlugin) Capabilities(ctx context.Context) (*Capabilities, error) {
	return d.Base.Capabilities(ctx)
}

// --- Authentication ---

func (d *DelegatingPlugin) AuthMethods(ctx context.Context) ([]AuthMethod, error) {
	return d.Base.AuthMethods(ctx)
}

func (d *DelegatingPlugin) Login(ctx context.Context, method string, credentials map[string]string) (*Credential, error) {
	return d.Base.Login(ctx, method, credentials)
}

func (d *DelegatingPlugin) LoginInteractive(ctx context.Context, method string, params map[string]string) (*InteractiveChallenge, error) {
	return d.Base.LoginInteractive(ctx, method, params)
}

func (d *DelegatingPlugin) LoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*Credential, error) {
	return d.Base.LoginInteractiveCallback(ctx, challengeID, callbackParams)
}

// --- Tree management ---

func (d *DelegatingPlugin) ListTrees(ctx context.Context) ([]string, error) {
	return d.Base.ListTrees(ctx)
}

func (d *DelegatingPlugin) DeleteTree(ctx context.Context, id string) error {
	return d.Base.DeleteTree(ctx, id)
}

// --- Value operations ---

func (d *DelegatingPlugin) GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error) {
	return d.Base.GetValues(ctx, id, paths)
}

func (d *DelegatingPlugin) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *PutOptions) error {
	return d.Base.PutValues(ctx, id, values, opts)
}

func (d *DelegatingPlugin) DeleteValues(ctx context.Context, id string, paths []string) error {
	return d.Base.DeleteValues(ctx, id, paths)
}

// --- Tree-level versioning ---

func (d *DelegatingPlugin) ListTreeVersions(ctx context.Context, id string) ([]string, error) {
	return d.Base.ListTreeVersions(ctx, id)
}

func (d *DelegatingPlugin) GetTreeVersion(ctx context.Context, id string, version string, paths []string) (map[string]config.Value, error) {
	return d.Base.GetTreeVersion(ctx, id, version, paths)
}

func (d *DelegatingPlugin) RollbackTree(ctx context.Context, id string, version string) error {
	return d.Base.RollbackTree(ctx, id, version)
}

func (d *DelegatingPlugin) DeleteTreeVersion(ctx context.Context, id string, version string) error {
	return d.Base.DeleteTreeVersion(ctx, id, version)
}

// --- Value-level versioning ---

func (d *DelegatingPlugin) ListValueVersions(ctx context.Context, id string, path string) ([]string, error) {
	return d.Base.ListValueVersions(ctx, id, path)
}

func (d *DelegatingPlugin) GetValueVersion(ctx context.Context, id string, path string, version string) (config.Value, bool, error) {
	return d.Base.GetValueVersion(ctx, id, path, version)
}

func (d *DelegatingPlugin) RollbackValue(ctx context.Context, id string, path string, version string) error {
	return d.Base.RollbackValue(ctx, id, path, version)
}

func (d *DelegatingPlugin) DeleteValueVersion(ctx context.Context, id string, path string, version string) error {
	return d.Base.DeleteValueVersion(ctx, id, path, version)
}

// --- Encryption ---

func (d *DelegatingPlugin) InitEncryption(ctx context.Context, passphrase []byte) error {
	return d.Base.InitEncryption(ctx, passphrase)
}

func (d *DelegatingPlugin) RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error {
	return d.Base.RotateEncryption(ctx, oldPassphrase, newPassphrase)
}

// --- Access control ---

func (d *DelegatingPlugin) GrantAccess(ctx context.Context, id string, user string, permissions []Permission) error {
	return d.Base.GrantAccess(ctx, id, user, permissions)
}

func (d *DelegatingPlugin) RevokeAccess(ctx context.Context, id string, user string, paths []string) error {
	return d.Base.RevokeAccess(ctx, id, user, paths)
}

func (d *DelegatingPlugin) ListAccess(ctx context.Context, id string) (map[string][]Permission, error) {
	return d.Base.ListAccess(ctx, id)
}
