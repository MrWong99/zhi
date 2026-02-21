package main

import (
	"fmt"

	"github.com/MrWong99/zhi/pkg/zhiplugin/pluginopts"
)

// appConfig holds the configuration for a single managed application.
type appConfig struct {
	Name          string
	Auth          string   // "approle" or "token"
	Wrapped       bool     // Use response-wrapping for secret_id (default: true for approle)
	WrapTTL       string   // Wrapping token TTL (default: "120s")
	TokenTTL      string   // Token TTL for token auth (default: "1h")
	TokenPolicies []string // Additional policies beyond the generated one
}

// managerConfig holds the full meta-plugin configuration.
type managerConfig struct {
	Addr      string
	Mount     string
	Prefix    string
	Namespace string

	// TLS fields for Vault connection.
	CACert     string
	ClientCert string
	ClientKey  string
	SkipVerify bool

	Apps []appConfig

	Workspace string
}

func parseManagerConfig(opts map[string]any) (*managerConfig, error) {
	skipVerify := pluginopts.String(opts, "skip_verify", "VAULT_SKIP_VERIFY", "")
	cfg := &managerConfig{
		Addr:       pluginopts.String(opts, "addr", "VAULT_ADDR", "http://127.0.0.1:8200"),
		Mount:      pluginopts.String(opts, "mount", "ZHI_VAULT_MOUNT", "secret"),
		Prefix:     pluginopts.String(opts, "prefix", "ZHI_VAULT_PREFIX", "zhi"),
		Namespace:  pluginopts.String(opts, "namespace", "VAULT_NAMESPACE", ""),
		CACert:     pluginopts.String(opts, "ca_cert", "VAULT_CACERT", ""),
		ClientCert: pluginopts.String(opts, "client_cert", "VAULT_CLIENT_CERT", ""),
		ClientKey:  pluginopts.String(opts, "client_key", "VAULT_CLIENT_KEY", ""),
		SkipVerify: skipVerify == "true" || skipVerify == "1",
		Workspace:  pluginopts.String(opts, "workspace", "", "default"),
	}

	apps, err := parseAppConfigs(opts)
	if err != nil {
		return nil, err
	}
	cfg.Apps = apps

	return cfg, nil
}

func parseAppConfigs(opts map[string]any) ([]appConfig, error) {
	appsRaw, ok := opts["apps"]
	if !ok {
		return nil, nil
	}

	appsList, ok := appsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("apps must be a list")
	}

	var apps []appConfig
	for i, raw := range appsList {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("apps[%d] must be an object", i)
		}

		name, _ := m["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("apps[%d]: name is required", i)
		}

		auth, _ := m["auth"].(string)
		if auth == "" {
			auth = "approle"
		}
		if auth != "approle" && auth != "token" {
			return nil, fmt.Errorf("apps[%d]: auth must be 'approle' or 'token', got %q", i, auth)
		}

		app := appConfig{
			Name: name,
			Auth: auth,
		}

		switch auth {
		case "approle":
			app.Wrapped = true // default
			if w, ok := m["wrapped"].(bool); ok {
				app.Wrapped = w
			}
			app.WrapTTL = "120s" // default
			if ttl, ok := m["wrap_ttl"].(string); ok {
				app.WrapTTL = ttl
			}
		case "token":
			app.TokenTTL = "1h" // default
			if ttl, ok := m["token_ttl"].(string); ok {
				app.TokenTTL = ttl
			}
			if policies, ok := m["token_policies"].([]any); ok {
				for _, p := range policies {
					if s, ok := p.(string); ok {
						app.TokenPolicies = append(app.TokenPolicies, s)
					}
				}
			}
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// childVaultOptions returns the options map to forward to the child vault plugin.
func childVaultOptions(cfg *managerConfig) map[string]any {
	opts := map[string]any{
		"addr":   cfg.Addr,
		"mount":  cfg.Mount,
		"prefix": cfg.Prefix,
	}
	if cfg.Namespace != "" {
		opts["namespace"] = cfg.Namespace
	}
	if cfg.CACert != "" {
		opts["ca_cert"] = cfg.CACert
	}
	if cfg.ClientCert != "" {
		opts["client_cert"] = cfg.ClientCert
	}
	if cfg.ClientKey != "" {
		opts["client_key"] = cfg.ClientKey
	}
	if cfg.SkipVerify {
		opts["skip_verify"] = "true"
	}
	return opts
}

// childVaultEnv returns the environment variables to pass to the child vault plugin.
func childVaultEnv(cfg *managerConfig) map[string]string {
	env := map[string]string{
		"VAULT_ADDR": cfg.Addr,
	}
	if cfg.CACert != "" {
		env["VAULT_CACERT"] = cfg.CACert
	}
	if cfg.ClientCert != "" {
		env["VAULT_CLIENT_CERT"] = cfg.ClientCert
	}
	if cfg.ClientKey != "" {
		env["VAULT_CLIENT_KEY"] = cfg.ClientKey
	}
	if cfg.SkipVerify {
		env["VAULT_SKIP_VERIFY"] = "true"
	}
	if cfg.Namespace != "" {
		env["VAULT_NAMESPACE"] = cfg.Namespace
	}
	return env
}
