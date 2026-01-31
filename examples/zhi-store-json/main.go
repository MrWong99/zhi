// zhi-store-json is an example zhi store plugin that persists configuration
// trees as JSON files on disk. Each tree ID maps to a single
// <dir>/<id>.json file, making it easy to inspect stored data.
//
// The base directory is read from the ZHI_JSON_STORE_DIR environment
// variable; if unset it defaults to a "zhi-pokedex" directory inside
// the OS temp directory.
//
// This example does not support versioning or encryption.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

type jsonStore struct {
	mu  sync.RWMutex
	dir string
}

func newJSONStore() *jsonStore {
	dir := os.Getenv("ZHI_JSON_STORE_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "zhi-pokedex")
	}
	return &jsonStore{dir: dir}
}

// filePath returns the on-disk path for the given tree ID.
func (s *jsonStore) filePath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *jsonStore) Save(_ context.Context, id string, tree config.TreeReader) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries := make(map[string]*config.Value, len(tree.List()))
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if !ok {
			continue
		}
		cp := v // copy
		entries[path] = &cp
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.filePath(id), data, 0o644)
}

func (s *jsonStore) Load(_ context.Context, id string) (*config.Tree, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var entries map[string]*config.Value
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, false, err
	}

	tree := config.NewTree()
	for path, v := range entries {
		if err := tree.Set(path, v); err != nil {
			return nil, false, err
		}
	}
	return tree, true, nil
}

func (s *jsonStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.filePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *jsonStore) ListTrees(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matches, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		name := filepath.Base(m)
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	return ids, nil
}

func (s *jsonStore) SupportsVersioning(_ context.Context) (bool, error) {
	return false, nil
}

func (s *jsonStore) ListVersions(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (s *jsonStore) LoadVersion(_ context.Context, _ string, _ string) (*config.Tree, bool, error) {
	return nil, false, errors.New("versioning not supported")
}

func (s *jsonStore) DeleteVersion(_ context.Context, _ string, _ string) error {
	return errors.New("versioning not supported")
}

func (s *jsonStore) EncryptionStatus(_ context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionNone, nil
}

func (s *jsonStore) InitEncryption(_ context.Context, _ []byte) error {
	return errors.New("encryption not supported")
}

func (s *jsonStore) RotateEncryption(_ context.Context, _, _ []byte) error {
	return errors.New("encryption not supported")
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"store": &store.GRPCPlugin{Impl: newJSONStore()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
