package main

import "testing"

func TestParseConfig_FromPluginOptions(t *testing.T) {
	opts := map[string]any{
		"addr":        "https://vault.example.com:8200",
		"token":       "s.mytoken",
		"mount":       "kv",
		"prefix":      "myapp",
		"namespace":   "engineering",
		"ca_cert":     "/etc/ssl/ca.pem",
		"client_cert": "/etc/ssl/client.pem",
		"client_key":  "/etc/ssl/client-key.pem",
		"skip_verify": "true",
	}

	cfg := parseConfig(opts)

	if cfg.Address != "https://vault.example.com:8200" {
		t.Errorf("Address = %q, want https://vault.example.com:8200", cfg.Address)
	}
	if cfg.Token != "s.mytoken" {
		t.Errorf("Token = %q, want s.mytoken", cfg.Token)
	}
	if cfg.Mount != "kv" {
		t.Errorf("Mount = %q, want kv", cfg.Mount)
	}
	if cfg.Prefix != "myapp" {
		t.Errorf("Prefix = %q, want myapp", cfg.Prefix)
	}
	if cfg.Namespace != "engineering" {
		t.Errorf("Namespace = %q, want engineering", cfg.Namespace)
	}
	if cfg.CACert != "/etc/ssl/ca.pem" {
		t.Errorf("CACert = %q", cfg.CACert)
	}
	if cfg.ClientCert != "/etc/ssl/client.pem" {
		t.Errorf("ClientCert = %q", cfg.ClientCert)
	}
	if cfg.ClientKey != "/etc/ssl/client-key.pem" {
		t.Errorf("ClientKey = %q", cfg.ClientKey)
	}
	if !cfg.SkipVerify {
		t.Error("SkipVerify should be true")
	}
}

func TestParseConfig_FallsBackToEnvVars(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://env-vault:8200")
	t.Setenv("VAULT_TOKEN", "s.envtoken")
	t.Setenv("ZHI_VAULT_MOUNT", "secrets")
	t.Setenv("ZHI_VAULT_PREFIX", "envprefix")
	t.Setenv("VAULT_NAMESPACE", "prod")

	// Empty opts — should fall back to env vars.
	cfg := parseConfig(map[string]any{})

	if cfg.Address != "https://env-vault:8200" {
		t.Errorf("Address = %q, want https://env-vault:8200", cfg.Address)
	}
	if cfg.Token != "s.envtoken" {
		t.Errorf("Token = %q, want s.envtoken", cfg.Token)
	}
	if cfg.Mount != "secrets" {
		t.Errorf("Mount = %q, want secrets", cfg.Mount)
	}
	if cfg.Prefix != "envprefix" {
		t.Errorf("Prefix = %q, want envprefix", cfg.Prefix)
	}
	if cfg.Namespace != "prod" {
		t.Errorf("Namespace = %q, want prod", cfg.Namespace)
	}
}

func TestParseConfig_Defaults(t *testing.T) {
	// Clear any env vars that might interfere.
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "")
	t.Setenv("ZHI_VAULT_MOUNT", "")
	t.Setenv("ZHI_VAULT_PREFIX", "")
	t.Setenv("VAULT_NAMESPACE", "")
	t.Setenv("VAULT_SKIP_VERIFY", "")

	cfg := parseConfig(map[string]any{})

	if cfg.Address != "http://127.0.0.1:8200" {
		t.Errorf("Address = %q, want http://127.0.0.1:8200", cfg.Address)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty", cfg.Token)
	}
	if cfg.Mount != "secret" {
		t.Errorf("Mount = %q, want secret", cfg.Mount)
	}
	if cfg.Prefix != "zhi" {
		t.Errorf("Prefix = %q, want zhi", cfg.Prefix)
	}
	if cfg.Namespace != "" {
		t.Errorf("Namespace = %q, want empty", cfg.Namespace)
	}
	if cfg.SkipVerify {
		t.Error("SkipVerify should be false by default")
	}
}
