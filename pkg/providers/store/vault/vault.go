// Package vault implements a zhi store plugin backed by HashiCorp Vault's
// KV v2 secret engine. Each configuration tree is stored as a single secret
// under a configurable mount and path prefix. The flat key-value entries of
// the tree are JSON-encoded and stored as the secret's data map.
//
// Because Vault KV v2 natively versions secrets, this store supports
// versioning out of the box. Vault also encrypts data at rest, so
// EncryptionStatus always reports EncryptionActive.
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

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

// secretPath returns the full Vault path for a tree ID.
func (s *Store) secretPath(id string) string {
	return s.prefix + id
}

// entryValue is the JSON structure persisted per tree entry inside a
// Vault secret's data map.
type entryValue struct {
	Val      any            `json:"value"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Save persists a configuration tree as a Vault KV v2 secret. Each call
// creates a new secret version.
func (s *Store) Save(ctx context.Context, id string, tree config.TreeReader) error {
	data := make(map[string]any, len(tree.List()))
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if !ok {
			continue
		}
		encoded, err := json.Marshal(entryValue{Val: v.Val, Metadata: v.Metadata})
		if err != nil {
			return fmt.Errorf("vault: encoding %q: %w", path, err)
		}
		data[path] = string(encoded)
	}

	_, err := s.client.KVv2(s.mount).Put(ctx, s.secretPath(id), data)
	if err != nil {
		return fmt.Errorf("vault: saving tree %q: %w", id, err)
	}
	return nil
}

// Load retrieves the latest version of a configuration tree from Vault.
func (s *Store) Load(ctx context.Context, id string) (*config.Tree, bool, error) {
	secret, err := s.client.KVv2(s.mount).Get(ctx, s.secretPath(id))
	if err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("vault: loading tree %q: %w", id, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, false, nil
	}
	tree, err := dataToTree(secret.Data)
	if err != nil {
		return nil, false, err
	}
	return tree, true, nil
}

// Delete removes the configuration tree and all its versions from Vault.
func (s *Store) Delete(ctx context.Context, id string) error {
	err := s.client.KVv2(s.mount).DeleteMetadata(ctx, s.secretPath(id))
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("vault: deleting tree %q: %w", id, err)
	}
	return nil
}

// ListTrees returns all stored tree IDs under the configured prefix.
func (s *Store) ListTrees(ctx context.Context) ([]string, error) {
	// KV v2 list goes through the metadata/ prefix.
	secret, err := s.client.Logical().ListWithContext(ctx,
		s.mount+"/metadata/"+s.prefix)
	if err != nil {
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
			// Skip sub-directories (they end with /).
			if str != "" && str[len(str)-1] != '/' {
				ids = append(ids, str)
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// SupportsVersioning returns true because Vault KV v2 natively versions
// secrets.
func (s *Store) SupportsVersioning(_ context.Context) (bool, error) {
	return true, nil
}

// ListVersions returns version identifiers for the given tree, newest
// first.
func (s *Store) ListVersions(ctx context.Context, id string) ([]string, error) {
	meta, err := s.client.KVv2(s.mount).GetMetadata(ctx, s.secretPath(id))
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("vault: listing versions for %q: %w", id, err)
	}
	if meta == nil {
		return nil, nil
	}

	var versions []string
	for v, vm := range meta.Versions {
		if vm.DeletionTime.IsZero() && !vm.Destroyed {
			versions = append(versions, v)
		}
	}

	// Sort descending (newest first).
	sort.Slice(versions, func(i, j int) bool {
		vi, _ := strconv.Atoi(versions[i])
		vj, _ := strconv.Atoi(versions[j])
		return vi > vj
	})

	return versions, nil
}

// LoadVersion retrieves a specific version of a configuration tree.
func (s *Store) LoadVersion(ctx context.Context, id string, version string) (*config.Tree, bool, error) {
	ver, err := strconv.Atoi(version)
	if err != nil {
		return nil, false, fmt.Errorf("vault: invalid version %q: %w", version, err)
	}
	secret, err := s.client.KVv2(s.mount).GetVersion(ctx, s.secretPath(id), ver)
	if err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("vault: loading version %q of %q: %w", version, id, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, false, nil
	}
	tree, err := dataToTree(secret.Data)
	if err != nil {
		return nil, false, err
	}
	return tree, true, nil
}

// DeleteVersion soft-deletes a single version of a configuration tree.
func (s *Store) DeleteVersion(ctx context.Context, id string, version string) error {
	ver, err := strconv.Atoi(version)
	if err != nil {
		return fmt.Errorf("vault: invalid version %q: %w", version, err)
	}
	err = s.client.KVv2(s.mount).DeleteVersions(ctx, s.secretPath(id), []int{ver})
	if err != nil {
		return fmt.Errorf("vault: deleting version %q of %q: %w", version, id, err)
	}
	return nil
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

// dataToTree converts a Vault KV v2 data map back into a config.Tree.
func dataToTree(data map[string]any) (*config.Tree, error) {
	tree := config.NewTree()
	for path, raw := range data {
		str, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("vault: unexpected type %T for path %q", raw, path)
		}
		var ev entryValue
		if err := json.Unmarshal([]byte(str), &ev); err != nil {
			return nil, fmt.Errorf("vault: decoding %q: %w", path, err)
		}
		if err := tree.Set(path, &config.Value{Val: ev.Val, Metadata: ev.Metadata}); err != nil {
			return nil, fmt.Errorf("vault: setting %q: %w", path, err)
		}
	}
	return tree, nil
}

// isNotFound checks whether a Vault API error indicates a 404 response.
func isNotFound(err error) bool {
	var respErr *vaultapi.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}
