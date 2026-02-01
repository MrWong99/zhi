// zhi-store-memory is an example zhi store plugin that persists configuration
// trees in memory. It demonstrates how to implement store.Plugin with the
// simplest possible storage backend: a Go map.
//
// This example does not support versioning, authentication, encryption, or
// access control.
package main

import (
	"context"
	"errors"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

type memoryStore struct {
	mu    sync.RWMutex
	trees map[string]map[string]config.Value // tree id -> path -> value
}

func newMemoryStore() *memoryStore {
	return &memoryStore{trees: make(map[string]map[string]config.Value)}
}

// --- Capabilities ---

func (m *memoryStore) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning:    store.VersioningNone,
		Encryption:    store.EncryptionNone,
		Auth:          false,
		AccessControl: false,
	}, nil
}

// --- Authentication ---

func (m *memoryStore) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	return nil, nil
}

func (m *memoryStore) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	return nil, errors.New("authentication not supported")
}

// --- Tree management ---

func (m *memoryStore) ListTrees(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.trees))
	for id := range m.trees {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *memoryStore) DeleteTree(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.trees, id)
	return nil
}

// --- Value operations ---

func (m *memoryStore) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tree, ok := m.trees[id]
	if !ok {
		return nil, nil
	}
	result := make(map[string]config.Value, len(paths))
	for _, p := range paths {
		if v, exists := tree[p]; exists {
			result[p] = v
		}
	}
	return result, nil
}

func (m *memoryStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tree, ok := m.trees[id]
	if !ok {
		tree = make(map[string]config.Value, len(values))
		m.trees[id] = tree
	}
	for p, v := range values {
		tree[p] = v
	}
	return nil
}

func (m *memoryStore) DeleteValues(_ context.Context, id string, paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tree, ok := m.trees[id]
	if !ok {
		return nil
	}
	for _, p := range paths {
		delete(tree, p)
	}
	return nil
}

// --- Tree-level versioning ---

func (m *memoryStore) ListTreeVersions(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (m *memoryStore) GetTreeVersion(_ context.Context, _ string, _ string, _ []string) (map[string]config.Value, error) {
	return nil, errors.New("versioning not supported")
}

func (m *memoryStore) RollbackTree(_ context.Context, _ string, _ string) error {
	return errors.New("versioning not supported")
}

func (m *memoryStore) DeleteTreeVersion(_ context.Context, _ string, _ string) error {
	return errors.New("versioning not supported")
}

// --- Value-level versioning ---

func (m *memoryStore) ListValueVersions(_ context.Context, _ string, _ string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (m *memoryStore) GetValueVersion(_ context.Context, _ string, _ string, _ string) (config.Value, bool, error) {
	return config.Value{}, false, errors.New("versioning not supported")
}

func (m *memoryStore) RollbackValue(_ context.Context, _ string, _ string, _ string) error {
	return errors.New("versioning not supported")
}

func (m *memoryStore) DeleteValueVersion(_ context.Context, _ string, _ string, _ string) error {
	return errors.New("versioning not supported")
}

// --- Encryption ---

func (m *memoryStore) InitEncryption(_ context.Context, _ []byte) error {
	return errors.New("encryption not supported")
}

func (m *memoryStore) RotateEncryption(_ context.Context, _, _ []byte) error {
	return errors.New("encryption not supported")
}

// --- Access control ---

func (m *memoryStore) GrantAccess(_ context.Context, _ string, _ string, _ []store.Permission) error {
	return errors.New("access control not supported")
}

func (m *memoryStore) RevokeAccess(_ context.Context, _ string, _ string, _ []string) error {
	return errors.New("access control not supported")
}

func (m *memoryStore) ListAccess(_ context.Context, _ string) (map[string][]store.Permission, error) {
	return nil, errors.New("access control not supported")
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"store": &store.GRPCPlugin{Impl: newMemoryStore()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
