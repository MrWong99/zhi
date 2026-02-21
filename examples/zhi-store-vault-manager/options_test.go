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
