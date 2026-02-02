package core

import (
	"fmt"
	"path/filepath"

	"github.com/MrWong99/zhi/pkg/providers/config/structuredfile"
	vaultstore "github.com/MrWong99/zhi/pkg/providers/store/vault"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// NewStructuredFileProvider is a ConfigFactory that creates a structuredfile
// config provider. It reads the "directory" option from the options map;
// if absent, it uses the structuredfile default directory.
func NewStructuredFileProvider(workspace string, options map[string]any) (config.Plugin, error) {
	dir := structuredfile.DefaultDir
	if options != nil {
		if d, ok := options["directory"]; ok {
			ds, ok := d.(string)
			if !ok {
				return nil, fmt.Errorf("structuredfile: \"directory\" option must be a string, got %T", d)
			}
			dir = ds
		}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	logger.Debug("loading structuredfile provider", "dir", dir)
	return structuredfile.NewFromDir(dir)
}

// NewVaultStoreProvider is a StoreFactory that creates a Vault KV v2 store
// provider. It reads connection settings from the options map, falling back
// to environment variables and sensible defaults.
//
// Supported options:
//   - address:   Vault server URL (default: VAULT_ADDR or "http://127.0.0.1:8200")
//   - token:     Initial Vault token (default: VAULT_TOKEN)
//   - mount:     KV v2 mount point (default: "secret")
//   - prefix:    Key prefix within the mount (default: "zhi")
//   - namespace: Vault namespace (default: VAULT_NAMESPACE)
func NewVaultStoreProvider(_ string, options map[string]any) (store.Plugin, error) {
	cfg := vaultstore.DefaultConfig()
	if options != nil {
		if v, ok := options["address"].(string); ok {
			cfg.Address = v
		}
		if v, ok := options["token"].(string); ok {
			cfg.Token = v
		}
		if v, ok := options["mount"].(string); ok {
			cfg.Mount = v
		}
		if v, ok := options["prefix"].(string); ok {
			cfg.Prefix = v
		}
		if v, ok := options["namespace"].(string); ok {
			cfg.Namespace = v
		}
	}
	logger.Debug("loading vault store provider", "address", cfg.Address, "mount", cfg.Mount, "prefix", cfg.Prefix)
	return vaultstore.New(cfg)
}
