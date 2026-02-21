package pluginopts

import (
	"testing"
)

func TestOptionsFromEnv(t *testing.T) {
	t.Setenv("ZHI_PLUGIN_OPTIONS", `{"addr":"0.0.0.0:9090","token":"secret","count":42,"debug":true}`)

	opts := Options()
	if opts == nil {
		t.Fatal("Options() returned nil")
	}
	if opts["addr"] != "0.0.0.0:9090" {
		t.Errorf("addr = %v, want 0.0.0.0:9090", opts["addr"])
	}
}

func TestOptionsEmpty(t *testing.T) {
	t.Setenv("ZHI_PLUGIN_OPTIONS", "")

	opts := Options()
	if opts != nil {
		t.Errorf("Options() = %v, want nil for empty env", opts)
	}
}

func TestOptionsInvalidJSON(t *testing.T) {
	t.Setenv("ZHI_PLUGIN_OPTIONS", "not-json")

	opts := Options()
	if opts != nil {
		t.Errorf("Options() = %v, want nil for invalid JSON", opts)
	}
}

func TestStringFromOptions(t *testing.T) {
	opts := map[string]any{"addr": "1.2.3.4:80"}
	got := String(opts, "addr", "ZHI_TEST_ADDR", "default")
	if got != "1.2.3.4:80" {
		t.Errorf("String() = %q, want 1.2.3.4:80", got)
	}
}

func TestStringFallbackToEnv(t *testing.T) {
	t.Setenv("ZHI_TEST_ADDR", "from-env")

	opts := map[string]any{"other": "val"}
	got := String(opts, "addr", "ZHI_TEST_ADDR", "default")
	if got != "from-env" {
		t.Errorf("String() = %q, want from-env", got)
	}
}

func TestStringFallbackToDefault(t *testing.T) {
	got := String(nil, "addr", "", "fallback")
	if got != "fallback" {
		t.Errorf("String() = %q, want fallback", got)
	}
}

func TestStringAlias_FirstKeyMatches(t *testing.T) {
	opts := map[string]any{"cacert": "/path/to/ca.pem"}
	got := StringAlias(opts, []string{"cacert", "ca_cert"}, "VAULT_CACERT", "")
	if got != "/path/to/ca.pem" {
		t.Errorf("StringAlias() = %q, want /path/to/ca.pem", got)
	}
}

func TestStringAlias_SecondKeyMatches(t *testing.T) {
	opts := map[string]any{"ca_cert": "/path/to/ca.pem"}
	got := StringAlias(opts, []string{"cacert", "ca_cert"}, "VAULT_CACERT", "")
	if got != "/path/to/ca.pem" {
		t.Errorf("StringAlias() = %q, want /path/to/ca.pem", got)
	}
}

func TestStringAlias_FirstKeyTakesPrecedence(t *testing.T) {
	opts := map[string]any{"cacert": "/first", "ca_cert": "/second"}
	got := StringAlias(opts, []string{"cacert", "ca_cert"}, "", "")
	if got != "/first" {
		t.Errorf("StringAlias() = %q, want /first (first key should take precedence)", got)
	}
}

func TestStringAlias_FallbackToEnv(t *testing.T) {
	t.Setenv("VAULT_CACERT", "/from-env")
	opts := map[string]any{"other": "val"}
	got := StringAlias(opts, []string{"cacert", "ca_cert"}, "VAULT_CACERT", "")
	if got != "/from-env" {
		t.Errorf("StringAlias() = %q, want /from-env", got)
	}
}

func TestStringAlias_FallbackToDefault(t *testing.T) {
	got := StringAlias(nil, []string{"cacert", "ca_cert"}, "", "default-val")
	if got != "default-val" {
		t.Errorf("StringAlias() = %q, want default-val", got)
	}
}

func TestBool(t *testing.T) {
	opts := map[string]any{"debug": true}
	if !Bool(opts, "debug", false) {
		t.Error("Bool(debug) = false, want true")
	}
	if Bool(opts, "missing", false) {
		t.Error("Bool(missing) = true, want false")
	}
	if !Bool(nil, "any", true) {
		t.Error("Bool(nil, any, true) = false, want true")
	}
}

func TestInt(t *testing.T) {
	opts := map[string]any{"count": float64(42)}
	if got := Int(opts, "count", 0); got != 42 {
		t.Errorf("Int(count) = %d, want 42", got)
	}
	if got := Int(opts, "missing", 10); got != 10 {
		t.Errorf("Int(missing) = %d, want 10", got)
	}
}

func TestMap(t *testing.T) {
	opts := map[string]any{
		"auth": map[string]any{
			"method": "token",
		},
	}
	m := Map(opts, "auth")
	if m == nil {
		t.Fatal("Map(auth) returned nil")
	}
	if m["method"] != "token" {
		t.Errorf("auth.method = %v, want token", m["method"])
	}
	if Map(opts, "missing") != nil {
		t.Error("Map(missing) should be nil")
	}
}

func TestStringMap(t *testing.T) {
	opts := map[string]any{
		"credentials": map[string]any{
			"token":   "hvs.xxx",
			"role_id": "abc",
			"count":   float64(5), // non-string, should be skipped
		},
	}
	m := StringMap(opts, "credentials")
	if m == nil {
		t.Fatal("StringMap(credentials) returned nil")
	}
	if m["token"] != "hvs.xxx" {
		t.Errorf("token = %q, want hvs.xxx", m["token"])
	}
	if m["role_id"] != "abc" {
		t.Errorf("role_id = %q, want abc", m["role_id"])
	}
	if _, ok := m["count"]; ok {
		t.Error("count should be skipped (not a string)")
	}
	if StringMap(opts, "missing") != nil {
		t.Error("StringMap(missing) should be nil")
	}
}
