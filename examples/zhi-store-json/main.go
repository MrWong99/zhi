// zhi-store-json is an example zhi store plugin that persists configuration
// trees as JSON files on disk. Each tree ID maps to a single
// <dir>/<id>.json file, making it easy to inspect stored data.
//
// The base directory is read from the ZHI_JSON_STORE_DIR environment
// variable; if unset it defaults to a "zhi-pokedex" directory inside
// the OS temp directory.
//
// This example does not support versioning, authentication, encryption, or
// access control.
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

// readFile reads and unmarshals the JSON file for the given tree ID.
// Returns nil (not an error) if the file does not exist.
func (s *jsonStore) readFile(id string) (map[string]*config.Value, error) {
	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var entries map[string]*config.Value
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// writeFile marshals and writes the entries to the JSON file for the
// given tree ID.
func (s *jsonStore) writeFile(id string, entries map[string]*config.Value) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.filePath(id), data, 0o644)
}

// --- Capabilities ---

func (s *jsonStore) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning:    store.VersioningNone,
		Encryption:    store.EncryptionNone,
		Auth:          false,
		AccessControl: false,
	}, nil
}

// --- Authentication ---

func (s *jsonStore) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	return nil, nil
}

func (s *jsonStore) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	return nil, errors.New("authentication not supported")
}

// --- Tree management ---

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

func (s *jsonStore) DeleteTree(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.filePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// --- Value operations ---

func (s *jsonStore) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := s.readFile(id)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, nil
	}

	result := make(map[string]config.Value, len(paths))
	for _, p := range paths {
		if v, ok := entries[p]; ok && v != nil {
			result[p] = *v
		}
	}
	return result, nil
}

func (s *jsonStore) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.readFile(id)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = make(map[string]*config.Value, len(values))
	}

	for p, v := range values {
		cp := v // copy
		entries[p] = &cp
	}

	return s.writeFile(id, entries)
}

func (s *jsonStore) DeleteValues(_ context.Context, id string, paths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.readFile(id)
	if err != nil {
		return err
	}
	if entries == nil {
		return nil
	}

	for _, p := range paths {
		delete(entries, p)
	}

	return s.writeFile(id, entries)
}

// --- Tree-level versioning ---

func (s *jsonStore) ListTreeVersions(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (s *jsonStore) GetTreeVersion(_ context.Context, _ string, _ string, _ []string) (map[string]config.Value, error) {
	return nil, errors.New("versioning not supported")
}

func (s *jsonStore) RollbackTree(_ context.Context, _ string, _ string) error {
	return errors.New("versioning not supported")
}

func (s *jsonStore) DeleteTreeVersion(_ context.Context, _ string, _ string) error {
	return errors.New("versioning not supported")
}

// --- Value-level versioning ---

func (s *jsonStore) ListValueVersions(_ context.Context, _ string, _ string) ([]string, error) {
	return nil, errors.New("versioning not supported")
}

func (s *jsonStore) GetValueVersion(_ context.Context, _ string, _ string, _ string) (config.Value, bool, error) {
	return config.Value{}, false, errors.New("versioning not supported")
}

func (s *jsonStore) RollbackValue(_ context.Context, _ string, _ string, _ string) error {
	return errors.New("versioning not supported")
}

func (s *jsonStore) DeleteValueVersion(_ context.Context, _ string, _ string, _ string) error {
	return errors.New("versioning not supported")
}

// --- Encryption ---

func (s *jsonStore) InitEncryption(_ context.Context, _ []byte) error {
	return errors.New("encryption not supported")
}

func (s *jsonStore) RotateEncryption(_ context.Context, _, _ []byte) error {
	return errors.New("encryption not supported")
}

// --- Access control ---

func (s *jsonStore) GrantAccess(_ context.Context, _ string, _ string, _ []store.Permission) error {
	return errors.New("access control not supported")
}

func (s *jsonStore) RevokeAccess(_ context.Context, _ string, _ string, _ []string) error {
	return errors.New("access control not supported")
}

func (s *jsonStore) ListAccess(_ context.Context, _ string) (map[string][]store.Permission, error) {
	return nil, errors.New("access control not supported")
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
