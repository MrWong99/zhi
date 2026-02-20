// Package pluginopts provides helpers for external zhi plugins to read
// workspace configuration options passed by the host via the
// ZHI_PLUGIN_OPTIONS environment variable.
//
// When a workspace config (zhi.yaml) includes options for a provider:
//
//	ui:
//	  provider: mcp-sse
//	  options:
//	    addr: "0.0.0.0:9090"
//	    token: "my-secret"
//
// The host JSON-encodes the options map and sets ZHI_PLUGIN_OPTIONS before
// launching the external plugin process. This package provides typed
// accessors so plugin authors can easily read those values with fallbacks
// to environment variables or defaults.
package pluginopts

import (
	"encoding/json"
	"os"
)

// Options reads and parses the plugin options from the ZHI_PLUGIN_OPTIONS
// environment variable. Returns nil if the variable is unset or empty.
func Options() map[string]any {
	raw := os.Getenv("ZHI_PLUGIN_OPTIONS")
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

// String returns a string option value for key. If the key is absent or
// not a string, it falls back to the environment variable envKey, then
// to defaultVal.
func String(opts map[string]any, key, envKey, defaultVal string) string {
	if opts != nil {
		if v, ok := opts[key].(string); ok {
			return v
		}
	}
	if envKey != "" {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
	}
	return defaultVal
}

// Bool returns a boolean option value for key. If the key is absent or
// not a bool, it returns defaultVal.
func Bool(opts map[string]any, key string, defaultVal bool) bool {
	if opts != nil {
		if v, ok := opts[key].(bool); ok {
			return v
		}
	}
	return defaultVal
}

// Int returns an integer option value for key. JSON numbers are decoded
// as float64, so this function converts accordingly. If the key is absent
// or not a number, it returns defaultVal.
func Int(opts map[string]any, key string, defaultVal int) int {
	if opts != nil {
		if v, ok := opts[key].(float64); ok {
			return int(v)
		}
	}
	return defaultVal
}

// Map returns a nested map option value for key. If the key is absent or
// not a map, it returns nil.
func Map(opts map[string]any, key string) map[string]any {
	if opts == nil {
		return nil
	}
	v, ok := opts[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}

// StringMap extracts a map[string]string from a nested map option.
// JSON maps decode as map[string]any, so each value is type-asserted to
// string. Non-string values are silently skipped.
func StringMap(opts map[string]any, key string) map[string]string {
	nested := Map(opts, key)
	if nested == nil {
		return nil
	}
	result := make(map[string]string, len(nested))
	for k, v := range nested {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}
