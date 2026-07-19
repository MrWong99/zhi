package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/hashicorp/go-hclog"
)

// MirroredPlugin writes to all stores but reads from the primary (first).
// Mirror write errors are logged but do not fail the operation unless the
// primary write fails. The logger is optional; if nil, hclog.Default() is
// used.
//
// Operations that change current tree values are treated as writes and
// forwarded to mirrors: PutValues, DeleteValues, DeleteTree, and the
// rollback operations (RollbackTree, RollbackValue). Because mirror version
// namespaces may differ from the primary's, a mirror rollback may fail; such
// failures are logged and the mirror should be resynced out of band. Version
// history operations that do not affect current values (DeleteTreeVersion,
// DeleteValueVersion) and all reads delegate to the primary only.
func MirroredPlugin(logger hclog.Logger, primary Plugin, mirrors ...Plugin) Plugin {
	if logger == nil {
		logger = hclog.Default()
	}
	return &mirroredPlugin{
		primary: primary,
		mirrors: mirrors,
		log:     logger,
	}
}

type mirroredPlugin struct {
	primary Plugin
	mirrors []Plugin
	log     hclog.Logger
}

// --- Capabilities ---

// Capabilities reports the primary's capabilities verbatim. Every
// capability-gated operation (auth, encryption, versioning, access control)
// delegates to the primary only, so the primary's capabilities accurately
// describe what the composed plugin can do. Mirror capabilities are only
// inspected to warn when a mirror cannot mirror what the primary supports.
func (m *mirroredPlugin) Capabilities(ctx context.Context) (*Capabilities, error) {
	caps, err := m.primary.Capabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("mirrored primary capabilities: %w", err)
	}

	for i, mirror := range m.mirrors {
		mc, err := mirror.Capabilities(ctx)
		if err != nil {
			m.log.Warn("mirror capabilities failed", "mirror", i, "error", err)
			continue
		}
		if mc.Versioning < caps.Versioning || mc.Encryption < caps.Encryption ||
			(caps.Auth && !mc.Auth) || (caps.AccessControl && !mc.AccessControl) {
			m.log.Warn("mirror has weaker capabilities than primary; mirrored writes may lose fidelity",
				"mirror", i,
				"primary_versioning", caps.Versioning, "mirror_versioning", mc.Versioning,
				"primary_encryption", caps.Encryption, "mirror_encryption", mc.Encryption,
			)
		}
	}

	return caps, nil
}

// --- Authentication (delegates to primary only) ---

func (m *mirroredPlugin) AuthMethods(ctx context.Context) ([]AuthMethod, error) {
	return m.primary.AuthMethods(ctx)
}

func (m *mirroredPlugin) Login(ctx context.Context, method string, credentials map[string]string) (*Credential, error) {
	return m.primary.Login(ctx, method, credentials)
}

func (m *mirroredPlugin) LoginInteractive(ctx context.Context, method string, params map[string]string) (*InteractiveChallenge, error) {
	return m.primary.LoginInteractive(ctx, method, params)
}

func (m *mirroredPlugin) LoginInteractiveCallback(ctx context.Context, challengeID string, callbackParams map[string]string) (*Credential, error) {
	return m.primary.LoginInteractiveCallback(ctx, challengeID, callbackParams)
}

// --- Tree management ---

func (m *mirroredPlugin) ListTrees(ctx context.Context) ([]string, error) {
	return m.primary.ListTrees(ctx)
}

func (m *mirroredPlugin) DeleteTree(ctx context.Context, id string) error {
	if err := m.primary.DeleteTree(ctx, id); err != nil {
		return fmt.Errorf("mirrored primary delete tree: %w", err)
	}
	m.mirrorAction(ctx, "DeleteTree", func(mirror Plugin) error {
		return mirror.DeleteTree(ctx, id)
	})
	return nil
}

// --- Value operations ---

func (m *mirroredPlugin) GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error) {
	return m.primary.GetValues(ctx, id, paths)
}

func (m *mirroredPlugin) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *PutOptions) error {
	if err := m.primary.PutValues(ctx, id, values, opts); err != nil {
		return fmt.Errorf("mirrored primary put values: %w", err)
	}
	m.mirrorAction(ctx, "PutValues", func(mirror Plugin) error {
		return mirror.PutValues(ctx, id, values, nil) // No CAS for mirrors.
	})
	return nil
}

func (m *mirroredPlugin) DeleteValues(ctx context.Context, id string, paths []string) error {
	if err := m.primary.DeleteValues(ctx, id, paths); err != nil {
		return fmt.Errorf("mirrored primary delete values: %w", err)
	}
	m.mirrorAction(ctx, "DeleteValues", func(mirror Plugin) error {
		return mirror.DeleteValues(ctx, id, paths)
	})
	return nil
}

// --- Tree-level versioning (delegates to primary only) ---

func (m *mirroredPlugin) ListTreeVersions(ctx context.Context, id string) ([]string, error) {
	return m.primary.ListTreeVersions(ctx, id)
}

func (m *mirroredPlugin) GetTreeVersion(ctx context.Context, id string, version string, paths []string) (map[string]config.Value, error) {
	return m.primary.GetTreeVersion(ctx, id, version, paths)
}

// RollbackTree changes the tree's current values, so it is a write and must
// be forwarded to mirrors to keep them in sync. Mirror version namespaces may
// differ from the primary's, so mirror rollback failures are logged (like
// other mirror writes) rather than failing the operation; a persistently
// failing mirror will diverge and should be resynced out of band.
func (m *mirroredPlugin) RollbackTree(ctx context.Context, id string, version string) error {
	if err := m.primary.RollbackTree(ctx, id, version); err != nil {
		return fmt.Errorf("mirrored primary rollback tree: %w", err)
	}
	m.mirrorAction(ctx, "RollbackTree", func(mirror Plugin) error {
		return mirror.RollbackTree(ctx, id, version)
	})
	return nil
}

func (m *mirroredPlugin) DeleteTreeVersion(ctx context.Context, id string, version string) error {
	return m.primary.DeleteTreeVersion(ctx, id, version)
}

// --- Value-level versioning (delegates to primary only) ---

func (m *mirroredPlugin) ListValueVersions(ctx context.Context, id string, path string) ([]string, error) {
	return m.primary.ListValueVersions(ctx, id, path)
}

func (m *mirroredPlugin) GetValueVersion(ctx context.Context, id string, path string, version string) (config.Value, bool, error) {
	return m.primary.GetValueVersion(ctx, id, path, version)
}

// RollbackValue changes the value's current state, so it is a write and is
// forwarded to mirrors (best-effort, logged) to keep them in sync. See
// RollbackTree for the divergence caveat.
func (m *mirroredPlugin) RollbackValue(ctx context.Context, id string, path string, version string) error {
	if err := m.primary.RollbackValue(ctx, id, path, version); err != nil {
		return fmt.Errorf("mirrored primary rollback value: %w", err)
	}
	m.mirrorAction(ctx, "RollbackValue", func(mirror Plugin) error {
		return mirror.RollbackValue(ctx, id, path, version)
	})
	return nil
}

func (m *mirroredPlugin) DeleteValueVersion(ctx context.Context, id string, path string, version string) error {
	return m.primary.DeleteValueVersion(ctx, id, path, version)
}

// --- Encryption (delegates to primary only) ---

func (m *mirroredPlugin) InitEncryption(ctx context.Context, passphrase []byte) error {
	return m.primary.InitEncryption(ctx, passphrase)
}

func (m *mirroredPlugin) RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error {
	return m.primary.RotateEncryption(ctx, oldPassphrase, newPassphrase)
}

// --- Access control (delegates to primary only) ---

func (m *mirroredPlugin) GrantAccess(ctx context.Context, id string, user string, permissions []Permission) error {
	return m.primary.GrantAccess(ctx, id, user, permissions)
}

func (m *mirroredPlugin) RevokeAccess(ctx context.Context, id string, user string, paths []string) error {
	return m.primary.RevokeAccess(ctx, id, user, paths)
}

func (m *mirroredPlugin) ListAccess(ctx context.Context, id string) (map[string][]Permission, error) {
	return m.primary.ListAccess(ctx, id)
}

// mirrorAction dispatches an action to all mirrors in parallel.
// Errors are logged but do not fail the operation.
func (m *mirroredPlugin) mirrorAction(_ context.Context, operation string, fn func(Plugin) error) {
	m.log.Debug("mirroring operation to replicas", "operation", operation, "mirror_count", len(m.mirrors))
	var wg sync.WaitGroup
	wg.Add(len(m.mirrors))

	for i, mirror := range m.mirrors {
		go func(idx int, mirror Plugin) {
			defer wg.Done()
			if err := fn(mirror); err != nil {
				m.log.Warn("mirror write failed",
					"operation", operation,
					"mirror", idx,
					"error", err,
				)
			}
		}(i, mirror)
	}

	wg.Wait()
}
