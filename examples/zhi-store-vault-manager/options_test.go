package main

import "testing"

func TestParseAppConfigs(t *testing.T) {
	opts := map[string]any{
		"apps": []any{
			map[string]any{
				"name":     "web-api",
				"auth":     "approle",
				"wrapped":  true,
				"wrap_ttl": "120s",
			},
			map[string]any{
				"name":      "ci",
				"auth":      "token",
				"token_ttl": "1h",
			},
		},
	}

	apps, err := parseAppConfigs(opts)
	if err != nil {
		t.Fatalf("parseAppConfigs: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	webAPI := apps[0]
	if webAPI.Name != "web-api" {
		t.Errorf("name = %s", webAPI.Name)
	}
	if webAPI.Auth != "approle" {
		t.Errorf("auth = %s", webAPI.Auth)
	}
	if !webAPI.Wrapped {
		t.Error("wrapped should be true")
	}
	if webAPI.WrapTTL != "120s" {
		t.Errorf("wrap_ttl = %s", webAPI.WrapTTL)
	}

	ci := apps[1]
	if ci.Auth != "token" {
		t.Errorf("auth = %s", ci.Auth)
	}
	if ci.TokenTTL != "1h" {
		t.Errorf("token_ttl = %s", ci.TokenTTL)
	}
}

func TestParseManagerConfig_TLSFields(t *testing.T) {
	opts := map[string]any{
		"ca_cert":     "/path/to/ca.pem",
		"client_cert": "/path/to/client.pem",
		"client_key":  "/path/to/client-key.pem",
		"skip_verify": "true",
	}

	cfg, err := parseManagerConfig(opts)
	if err != nil {
		t.Fatalf("parseManagerConfig: %v", err)
	}
	if cfg.CACert != "/path/to/ca.pem" {
		t.Errorf("CACert = %q, want /path/to/ca.pem", cfg.CACert)
	}
	if cfg.ClientCert != "/path/to/client.pem" {
		t.Errorf("ClientCert = %q, want /path/to/client.pem", cfg.ClientCert)
	}
	if cfg.ClientKey != "/path/to/client-key.pem" {
		t.Errorf("ClientKey = %q, want /path/to/client-key.pem", cfg.ClientKey)
	}
	if !cfg.SkipVerify {
		t.Error("SkipVerify should be true")
	}
}

func TestChildVaultOptions_IncludesTLS(t *testing.T) {
	cfg := &managerConfig{
		Addr:       "https://vault:8200",
		Mount:      "secret",
		Prefix:     "zhi",
		CACert:     "/ca.pem",
		ClientCert: "/client.pem",
		ClientKey:  "/client-key.pem",
		SkipVerify: true,
	}

	opts := childVaultOptions(cfg)
	if opts["ca_cert"] != "/ca.pem" {
		t.Errorf("ca_cert = %v", opts["ca_cert"])
	}
	if opts["client_cert"] != "/client.pem" {
		t.Errorf("client_cert = %v", opts["client_cert"])
	}
	if opts["client_key"] != "/client-key.pem" {
		t.Errorf("client_key = %v", opts["client_key"])
	}
	if opts["skip_verify"] != "true" {
		t.Errorf("skip_verify = %v", opts["skip_verify"])
	}
}

func TestChildVaultOptions_OmitsTLSWhenEmpty(t *testing.T) {
	cfg := &managerConfig{
		Addr:   "https://vault:8200",
		Mount:  "secret",
		Prefix: "zhi",
	}

	opts := childVaultOptions(cfg)
	if _, ok := opts["ca_cert"]; ok {
		t.Error("ca_cert should not be set when empty")
	}
	if _, ok := opts["skip_verify"]; ok {
		t.Error("skip_verify should not be set when false")
	}
}

func TestChildVaultEnv_IncludesTLS(t *testing.T) {
	cfg := &managerConfig{
		Addr:       "https://vault:8200",
		Namespace:  "engineering",
		CACert:     "/ca.pem",
		ClientCert: "/client.pem",
		ClientKey:  "/client-key.pem",
		SkipVerify: true,
	}

	env := childVaultEnv(cfg)
	if env["VAULT_ADDR"] != "https://vault:8200" {
		t.Errorf("VAULT_ADDR = %s", env["VAULT_ADDR"])
	}
	if env["VAULT_CACERT"] != "/ca.pem" {
		t.Errorf("VAULT_CACERT = %s", env["VAULT_CACERT"])
	}
	if env["VAULT_CLIENT_CERT"] != "/client.pem" {
		t.Errorf("VAULT_CLIENT_CERT = %s", env["VAULT_CLIENT_CERT"])
	}
	if env["VAULT_CLIENT_KEY"] != "/client-key.pem" {
		t.Errorf("VAULT_CLIENT_KEY = %s", env["VAULT_CLIENT_KEY"])
	}
	if env["VAULT_SKIP_VERIFY"] != "true" {
		t.Errorf("VAULT_SKIP_VERIFY = %s", env["VAULT_SKIP_VERIFY"])
	}
	if env["VAULT_NAMESPACE"] != "engineering" {
		t.Errorf("VAULT_NAMESPACE = %s", env["VAULT_NAMESPACE"])
	}
}

func TestChildVaultEnv_OmitsTLSWhenEmpty(t *testing.T) {
	cfg := &managerConfig{
		Addr: "http://vault:8200",
	}

	env := childVaultEnv(cfg)
	if _, ok := env["VAULT_CACERT"]; ok {
		t.Error("VAULT_CACERT should not be set")
	}
	if _, ok := env["VAULT_SKIP_VERIFY"]; ok {
		t.Error("VAULT_SKIP_VERIFY should not be set")
	}
	if _, ok := env["VAULT_NAMESPACE"]; ok {
		t.Error("VAULT_NAMESPACE should not be set when empty")
	}
}

func TestChildVaultEnv_IncludesMountAndPrefix(t *testing.T) {
	cfg := &managerConfig{
		Addr:   "https://vault:8200",
		Mount:  "kv",
		Prefix: "myapp",
	}

	env := childVaultEnv(cfg)
	if env["ZHI_VAULT_MOUNT"] != "kv" {
		t.Errorf("ZHI_VAULT_MOUNT = %q, want kv", env["ZHI_VAULT_MOUNT"])
	}
	if env["ZHI_VAULT_PREFIX"] != "myapp" {
		t.Errorf("ZHI_VAULT_PREFIX = %q, want myapp", env["ZHI_VAULT_PREFIX"])
	}
}

func TestChildVaultEnv_IncludesDefaultMountAndPrefix(t *testing.T) {
	cfg := &managerConfig{
		Addr:   "https://vault:8200",
		Mount:  "secret",
		Prefix: "zhi",
	}

	env := childVaultEnv(cfg)
	if env["ZHI_VAULT_MOUNT"] != "secret" {
		t.Errorf("ZHI_VAULT_MOUNT = %q, want secret", env["ZHI_VAULT_MOUNT"])
	}
	if env["ZHI_VAULT_PREFIX"] != "zhi" {
		t.Errorf("ZHI_VAULT_PREFIX = %q, want zhi", env["ZHI_VAULT_PREFIX"])
	}
}

func TestParseAppConfigs_Defaults(t *testing.T) {
	opts := map[string]any{
		"apps": []any{
			map[string]any{
				"name": "myapp",
				"auth": "approle",
			},
		},
	}

	apps, err := parseAppConfigs(opts)
	if err != nil {
		t.Fatalf("parseAppConfigs: %v", err)
	}
	app := apps[0]
	if !app.Wrapped {
		t.Error("wrapped should default to true for approle")
	}
	if app.WrapTTL != "120s" {
		t.Errorf("wrap_ttl should default to 120s, got %s", app.WrapTTL)
	}
}
