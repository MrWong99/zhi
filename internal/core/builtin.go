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
// provider. It reads the following options from the options map:
//   - address: Vault server address (default: VAULT_ADDR env)
//   - token: Vault authentication token (default: VAULT_TOKEN env / ~/.vault-token)
//   - mount: KV v2 mount path (default: "secret")
//   - prefix: path prefix for tree IDs (default: "zhi/")
func NewVaultStoreProvider(_ string, options map[string]any) (store.Plugin, error) {
	opts := vaultstore.Options{}
	if options != nil {
		if v, ok := options["address"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("vault: \"address\" option must be a string, got %T", v)
			}
			opts.Address = s
		}
		if v, ok := options["token"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("vault: \"token\" option must be a string, got %T", v)
			}
			opts.Token = s
		}
		if v, ok := options["mount"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("vault: \"mount\" option must be a string, got %T", v)
			}
			opts.Mount = s
		}
		if v, ok := options["prefix"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("vault: \"prefix\" option must be a string, got %T", v)
			}
			opts.Prefix = s
		}
	}
	logger.Debug("loading vault store provider",
		"address", opts.Address,
		"mount", opts.Mount,
		"prefix", opts.Prefix,
	)
	return vaultstore.New(opts)
}
