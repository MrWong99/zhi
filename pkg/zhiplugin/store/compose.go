package store

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// MirroredPlugin writes to all stores but reads from the primary (first).
// Mirror write errors are logged but do not fail the operation unless the
// primary write fails. The logger is optional; if nil, slog.Default() is
// used.
func MirroredPlugin(logger *slog.Logger, primary Plugin, mirrors ...Plugin) Plugin {
	if logger == nil {
		logger = slog.Default()
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
	log     *slog.Logger
}

// --- Capabilities ---

// Capabilities returns the intersection of all stores' capabilities.
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
		// Intersect: use the most restrictive.
		if mc.Versioning < caps.Versioning {
			caps.Versioning = mc.Versioning
		}
		if mc.Encryption < caps.Encryption {
			caps.Encryption = mc.Encryption
		}
		if !mc.Auth {
			caps.Auth = false
		}
		if !mc.AccessControl {
			caps.AccessControl = false
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

func (m *mirroredPlugin) RollbackTree(ctx context.Context, id string, version string) error {
	return m.primary.RollbackTree(ctx, id, version)
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

func (m *mirroredPlugin) RollbackValue(ctx context.Context, id string, path string, version string) error {
	return m.primary.RollbackValue(ctx, id, path, version)
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
