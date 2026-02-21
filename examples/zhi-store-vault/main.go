// zhi-store-vault is an example zhi store plugin backed by HashiCorp Vault's
// KV v2 secrets engine. It demonstrates how to use the vault store provider
// as an external plugin binary.
//
// Configuration is read from ZHI_PLUGIN_OPTIONS (JSON), falling back to
// environment variables:
//
//	addr / VAULT_ADDR       — Vault server address (default: http://127.0.0.1:8200)
//	token / VAULT_TOKEN     — Initial Vault token
//	mount / ZHI_VAULT_MOUNT — KV v2 mount point (default: secret)
//	prefix / ZHI_VAULT_PREFIX — Key prefix within the mount (default: zhi)
//	namespace / VAULT_NAMESPACE — Vault namespace (optional, enterprise only)
//	cacert (or ca_cert) / VAULT_CACERT      — CA certificate path (optional)
//	clientcert (or client_cert) / VAULT_CLIENT_CERT — Client certificate path (optional)
//	clientkey (or client_key) / VAULT_CLIENT_KEY   — Client private key path (optional)
//	skip_verify / VAULT_SKIP_VERIFY — Disable TLS verification (optional)
package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/store/vault"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/pluginopts"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func parseConfig(opts map[string]any) vault.Config {
	skipVerify := pluginopts.String(opts, "skip_verify", "VAULT_SKIP_VERIFY", "")
	return vault.Config{
		Address:    pluginopts.String(opts, "addr", "VAULT_ADDR", "http://127.0.0.1:8200"),
		Token:      pluginopts.String(opts, "token", "VAULT_TOKEN", ""),
		Mount:      pluginopts.String(opts, "mount", "ZHI_VAULT_MOUNT", "secret"),
		Prefix:     pluginopts.String(opts, "prefix", "ZHI_VAULT_PREFIX", "zhi"),
		Namespace:  pluginopts.String(opts, "namespace", "VAULT_NAMESPACE", ""),
		CACert:     pluginopts.StringAlias(opts, []string{"cacert", "ca_cert"}, "VAULT_CACERT", ""),
		ClientCert: pluginopts.StringAlias(opts, []string{"clientcert", "client_cert"}, "VAULT_CLIENT_CERT", ""),
		ClientKey:  pluginopts.StringAlias(opts, []string{"clientkey", "client_key"}, "VAULT_CLIENT_KEY", ""),
		SkipVerify: skipVerify == "true" || skipVerify == "1",
	}
}

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

	opts := pluginopts.Options()
	cfg := parseConfig(opts)

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
