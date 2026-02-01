// Package vault implements a zhi store plugin backed by HashiCorp Vault's
// KV v2 secret engine. Each tree entry is stored as its own Vault secret
// at <mount>/data/<prefix>/<tree-id>/<config-path>. This enables
// fine-grained Vault policies per configuration value.
//
// Because each value is a separate secret with independent version
// histories, tree-level versioning is not supported (SupportsVersioning
// returns false). Vault encrypts data at rest, so EncryptionStatus always
// reports EncryptionActive.
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// Store implements store.Plugin using HashiCorp Vault KV v2.
type Store struct {
	client *vaultapi.Client
	mount  string // KV v2 mount path (e.g. "secret")
	prefix string // path prefix inside the mount (e.g. "zhi/")
}

// Options configures the Vault store.
type Options struct {
	// Address is the Vault server address. If empty, falls back to
	// VAULT_ADDR.
	Address string

	// Token is the Vault authentication token. If empty, falls back to
	// VAULT_TOKEN and then ~/.vault-token.
	Token string

	// Mount is the KV v2 mount path. Defaults to "secret".
	Mount string

	// Prefix is the path prefix under the mount. Tree IDs are appended
	// to this prefix. Defaults to "zhi/". A trailing slash is added if
	// missing.
	Prefix string
}

// New creates a Vault store with the given options.
func New(opts Options) (*Store, error) {
	cfg := vaultapi.DefaultConfig()
	if opts.Address != "" {
		cfg.Address = opts.Address
	}

	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault: creating client: %w", err)
	}

	if opts.Token != "" {
		client.SetToken(opts.Token)
	}
	// If no token was set explicitly, the Vault client reads VAULT_TOKEN
	// and ~/.vault-token automatically.

	mount := opts.Mount
	if mount == "" {
		mount = "secret"
	}

	prefix := opts.Prefix
	if prefix == "" {
		prefix = "zhi/"
	}
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	return &Store{
		client: client,
		mount:  mount,
		prefix: prefix,
	}, nil
}

// valuePath returns the Vault secret path for a single tree entry.
// For example: "zhi/production/db/host".
func (s *Store) valuePath(id, configPath string) string {
	return s.prefix + id + "/" + configPath
}

// treePrefixPath returns the metadata listing path for a tree.
func (s *Store) treePrefixPath(id string) string {
	return s.mount + "/metadata/" + s.prefix + id + "/"
}

// entryValue is the JSON structure persisted per tree entry inside a
// Vault secret's data map.
type entryValue struct {
	Val      any            `json:"value"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Save persists each tree entry as a separate Vault KV v2 secret at
// <prefix>/<id>/<config-path>. Entries that existed in a previous save
// but are no longer present in the tree are deleted.
func (s *Store) Save(ctx context.Context, id string, tree config.TreeReader) error {
	paths := tree.List()
	currentPaths := make(map[string]bool, len(paths))

	// Write each entry as its own secret.
	for _, path := range paths {
		currentPaths[path] = true
		v, ok := tree.Get(path)
		if !ok {
			continue
		}
		encoded, err := json.Marshal(entryValue{Val: v.Val, Metadata: v.Metadata})
		if err != nil {
			return fmt.Errorf("vault: encoding %q: %w", path, err)
		}
		data := map[string]any{path: string(encoded)}
		_, err = s.client.KVv2(s.mount).Put(ctx, s.valuePath(id, path), data)
		if err != nil {
			return fmt.Errorf("vault: saving %q in tree %q: %w", path, id, err)
		}
	}

	// Remove entries that are no longer in the tree.
	existing, err := s.listRecursive(ctx, s.treePrefixPath(id))
	if err != nil {
		return fmt.Errorf("vault: listing existing entries for cleanup: %w", err)
	}
	for _, path := range existing {
		if !currentPaths[path] {
			_ = s.client.KVv2(s.mount).DeleteMetadata(ctx, s.valuePath(id, path))
		}
	}

	return nil
}

// Load retrieves all entries for a tree by recursively listing secrets
// under <prefix>/<id>/ and reading each one.
func (s *Store) Load(ctx context.Context, id string) (*config.Tree, bool, error) {
	paths, err := s.listRecursive(ctx, s.treePrefixPath(id))
	if err != nil {
		return nil, false, fmt.Errorf("vault: listing tree %q: %w", id, err)
	}
	if len(paths) == 0 {
		return nil, false, nil
	}

	tree := config.NewTree()
	for _, path := range paths {
		secret, err := s.client.KVv2(s.mount).Get(ctx, s.valuePath(id, path))
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, false, fmt.Errorf("vault: loading %q from tree %q: %w", path, id, err)
		}
		if secret == nil || secret.Data == nil {
			continue
		}

		raw, ok := secret.Data[path]
		if !ok {
			continue
		}
		str, ok := raw.(string)
		if !ok {
			return nil, false, fmt.Errorf("vault: unexpected type %T for %q", raw, path)
		}
		var ev entryValue
		if err := json.Unmarshal([]byte(str), &ev); err != nil {
			return nil, false, fmt.Errorf("vault: decoding %q: %w", path, err)
		}
		if err := tree.Set(path, &config.Value{Val: ev.Val, Metadata: ev.Metadata}); err != nil {
			return nil, false, fmt.Errorf("vault: setting %q: %w", path, err)
		}
	}

	return tree, true, nil
}

// Delete removes all secrets belonging to the tree.
func (s *Store) Delete(ctx context.Context, id string) error {
	paths, err := s.listRecursive(ctx, s.treePrefixPath(id))
	if err != nil {
		// If the tree prefix doesn't exist, nothing to delete.
		return nil
	}

	for _, path := range paths {
		err := s.client.KVv2(s.mount).DeleteMetadata(ctx, s.valuePath(id, path))
		if err != nil && !isNotFound(err) {
			return fmt.Errorf("vault: deleting %q from tree %q: %w", path, id, err)
		}
	}
	return nil
}

// ListTrees returns all stored tree IDs under the configured prefix.
// Tree IDs appear as directory entries (trailing /) at the prefix level.
func (s *Store) ListTrees(ctx context.Context) ([]string, error) {
	secret, err := s.client.Logical().ListWithContext(ctx,
		s.mount+"/metadata/"+s.prefix)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("vault: listing trees: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	keysRaw, ok := secret.Data["keys"]
	if !ok {
		return nil, nil
	}
	keysSlice, ok := keysRaw.([]any)
	if !ok {
		return nil, nil
	}

	var ids []string
	for _, k := range keysSlice {
		if str, ok := k.(string); ok {
			// Tree IDs are directories (ending with /).
			if strings.HasSuffix(str, "/") {
				ids = append(ids, strings.TrimSuffix(str, "/"))
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// SupportsVersioning returns false. Each tree entry is stored as a
// separate Vault secret with its own independent version history, so
// coherent tree-level versioning is not possible.
func (s *Store) SupportsVersioning(_ context.Context) (bool, error) {
	return false, nil
}

// ListVersions is not supported because tree entries have independent
// version histories.
func (s *Store) ListVersions(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("vault: tree-level versioning not supported (each value is a separate secret)")
}

// LoadVersion is not supported because tree entries have independent
// version histories.
func (s *Store) LoadVersion(_ context.Context, _ string, _ string) (*config.Tree, bool, error) {
	return nil, false, errors.New("vault: tree-level versioning not supported (each value is a separate secret)")
}

// DeleteVersion is not supported because tree entries have independent
// version histories.
func (s *Store) DeleteVersion(_ context.Context, _ string, _ string) error {
	return errors.New("vault: tree-level versioning not supported (each value is a separate secret)")
}

// EncryptionStatus reports EncryptionActive because Vault encrypts all
// data at rest.
func (s *Store) EncryptionStatus(_ context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionActive, nil
}

// InitEncryption is a no-op for Vault since encryption is always active.
func (s *Store) InitEncryption(_ context.Context, _ []byte) error {
	return errors.New("vault: encryption is always active; no initialization needed")
}

// RotateEncryption is a no-op for Vault. Key rotation should be performed
// using Vault's own operator tools (vault operator rotate).
func (s *Store) RotateEncryption(_ context.Context, _, _ []byte) error {
	return errors.New("vault: key rotation is managed by Vault itself (use 'vault operator rotate')")
}

// listRecursive returns all leaf secret paths under the given Vault
// metadata listing path. Returned paths are relative to root (e.g.
// "pokedex/trainer.name", not the full Vault path).
func (s *Store) listRecursive(ctx context.Context, root string) ([]string, error) {
	secret, err := s.client.Logical().ListWithContext(ctx, root)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("vault: listing %q: %w", root, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	keysRaw, ok := secret.Data["keys"].([]any)
	if !ok {
		return nil, nil
	}

	var paths []string
	for _, k := range keysRaw {
		key, ok := k.(string)
		if !ok {
			continue
		}
		if strings.HasSuffix(key, "/") {
			// Directory — recurse deeper.
			sub, err := s.listRecursive(ctx, root+key)
			if err != nil {
				return nil, err
			}
			for _, s := range sub {
				paths = append(paths, key+s)
			}
		} else {
			paths = append(paths, key)
		}
	}
	return paths, nil
}

// isNotFound checks whether a Vault API error indicates a 404 response.
func isNotFound(err error) bool {
	var respErr *vaultapi.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}
