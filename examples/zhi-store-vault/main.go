// zhi-store-vault is an example zhi store plugin backed by HashiCorp Vault's
// KV v2 secrets engine. It demonstrates how to use the vault store provider
// as an external plugin binary.
//
// Configuration is read from environment variables:
//
//	VAULT_ADDR       — Vault server address (default: http://127.0.0.1:8200)
//	VAULT_TOKEN      — Initial Vault token
//	ZHI_VAULT_MOUNT  — KV v2 mount point (default: secret)
//	ZHI_VAULT_PREFIX — Key prefix within the mount (default: zhi)
//	VAULT_NAMESPACE  — Vault namespace (optional, enterprise only)
package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/store/vault"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-store-vault",
		Level:  level,
		Output: os.Stderr,
	})
	logger.Info("starting vault store plugin")

	cfg := vault.DefaultConfig()
	if mount := os.Getenv("ZHI_VAULT_MOUNT"); mount != "" {
		cfg.Mount = mount
	}
	if prefix := os.Getenv("ZHI_VAULT_PREFIX"); prefix != "" {
		cfg.Prefix = prefix
	}

	s, err := vault.New(cfg)
	if err != nil {
		logger.Error("failed to create vault store", "error", err)
		os.Exit(1)
	}

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"store": &store.GRPCPlugin{Impl: s},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
