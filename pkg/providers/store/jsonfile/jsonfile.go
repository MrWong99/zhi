// Package jsonfile implements a zhi store plugin that persists configuration
// trees as JSON files on disk. Each tree ID maps to a single
// <dir>/<id>.json file, making it easy to inspect stored data.
//
// This provider does not support versioning, authentication, encryption, or
// access control.
package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// Config holds the configuration for the JSON file store.
type Config struct {
	// Dir is the directory where JSON files are stored. Required.
	Dir string
}

// Store persists configuration trees as JSON files on disk.
type Store struct {
	mu  sync.RWMutex
	dir string
}

// New creates a new JSON file store with the given configuration.
func New(cfg Config) (*Store, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("jsonfile: directory is required")
	}
	return &Store{dir: cfg.Dir}, nil
}

// filePath returns the on-disk path for the given tree ID.
func (s *Store) filePath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// readFile reads and unmarshals the JSON file for the given tree ID.
// Returns nil (not an error) if the file does not exist.
func (s *Store) readFile(id string) (map[string]*config.Value, error) {
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
func (s *Store) writeFile(id string, entries map[string]*config.Value) error {
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

func (s *Store) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning:    store.VersioningNone,
		Encryption:    store.EncryptionNone,
		Auth:          false,
		AccessControl: false,
	}, nil
}

// --- Authentication ---

func (s *Store) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	return nil, nil
}

func (s *Store) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	return nil, &store.ErrNotSupported{Feature: "authentication"}
}

func (s *Store) LoginInteractive(_ context.Context, _ string, _ map[string]string) (*store.InteractiveChallenge, error) {
	return nil, &store.ErrNotSupported{Feature: "interactive login"}
}

func (s *Store) LoginInteractiveCallback(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	return nil, &store.ErrNotSupported{Feature: "interactive login"}
}

// --- Tree management ---

func (s *Store) ListTrees(_ context.Context) ([]string, error) {
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

func (s *Store) DeleteTree(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.filePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// --- Value operations ---

func (s *Store) GetValues(_ context.Context, id string, paths []string) (map[string]config.Value, error) {
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

func (s *Store) PutValues(_ context.Context, id string, values map[string]config.Value, _ *store.PutOptions) error {
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

func (s *Store) DeleteValues(_ context.Context, id string, paths []string) error {
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

func (s *Store) ListTreeVersions(_ context.Context, _ string) ([]string, error) {
	return nil, &store.ErrNotSupported{Feature: "versioning"}
}

func (s *Store) GetTreeVersion(_ context.Context, _ string, _ string, _ []string) (map[string]config.Value, error) {
	return nil, &store.ErrNotSupported{Feature: "versioning"}
}

func (s *Store) RollbackTree(_ context.Context, _ string, _ string) error {
	return &store.ErrNotSupported{Feature: "versioning"}
}

func (s *Store) DeleteTreeVersion(_ context.Context, _ string, _ string) error {
	return &store.ErrNotSupported{Feature: "versioning"}
}

// --- Value-level versioning ---

func (s *Store) ListValueVersions(_ context.Context, _ string, _ string) ([]string, error) {
	return nil, &store.ErrNotSupported{Feature: "versioning"}
}

func (s *Store) GetValueVersion(_ context.Context, _ string, _ string, _ string) (config.Value, bool, error) {
	return config.Value{}, false, &store.ErrNotSupported{Feature: "versioning"}
}

func (s *Store) RollbackValue(_ context.Context, _ string, _ string, _ string) error {
	return &store.ErrNotSupported{Feature: "versioning"}
}

func (s *Store) DeleteValueVersion(_ context.Context, _ string, _ string, _ string) error {
	return &store.ErrNotSupported{Feature: "versioning"}
}

// --- Encryption ---

func (s *Store) InitEncryption(_ context.Context, _ []byte) error {
	return &store.ErrNotSupported{Feature: "encryption"}
}

func (s *Store) RotateEncryption(_ context.Context, _, _ []byte) error {
	return &store.ErrNotSupported{Feature: "encryption"}
}

// --- Access control ---

func (s *Store) GrantAccess(_ context.Context, _ string, _ string, _ []store.Permission) error {
	return &store.ErrNotSupported{Feature: "access control"}
}

func (s *Store) RevokeAccess(_ context.Context, _ string, _ string, _ []string) error {
	return &store.ErrNotSupported{Feature: "access control"}
}

func (s *Store) ListAccess(_ context.Context, _ string) (map[string][]store.Permission, error) {
	return nil, &store.ErrNotSupported{Feature: "access control"}
}
