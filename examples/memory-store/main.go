// Memory-store is an example zhi store plugin that persists configuration
// trees in memory. It demonstrates how to implement store.Plugin with the
// simplest possible storage backend: a Go map.
//
// This example does not support versioning or encryption.
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
	trees map[string]*config.Tree
}

func newMemoryStore() *memoryStore {
	return &memoryStore{trees: make(map[string]*config.Tree)}
}

func (m *memoryStore) Save(_ context.Context, id string, tree config.TreeReader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := config.NewTree()
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if !ok {
			continue
		}
		_ = cp.Set(path, &v)
	}
	m.trees[id] = cp
	return nil
}

func (m *memoryStore) Load(_ context.Context, id string) (*config.Tree, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.trees[id]
	if !ok {
		return nil, false, nil
	}
	return t, true, nil
}

func (m *memoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.trees, id)
	return nil
}

func (m *memoryStore) ListTrees(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.trees))
	for id := range m.trees {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *memoryStore) SupportsVersioning(_ context.Context) (bool, error) {
	return false, nil
}

func (m *memoryStore) ListVersions(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (m *memoryStore) LoadVersion(_ context.Context, _ string, _ string) (*config.Tree, bool, error) {
	return nil, false, errors.New("versioning not supported")
}

func (m *memoryStore) DeleteVersion(_ context.Context, _ string, _ string) error {
	return errors.New("versioning not supported")
}

func (m *memoryStore) EncryptionStatus(_ context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionNone, nil
}

func (m *memoryStore) InitEncryption(_ context.Context, _ []byte) error {
	return errors.New("encryption not supported")
}

func (m *memoryStore) RotateEncryption(_ context.Context, _, _ []byte) error {
	return errors.New("encryption not supported")
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
