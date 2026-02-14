package webui

import (
	"os"
	"strings"
)

// Config holds the Web UI server configuration.
type Config struct {
	// Addr is the listen address. Default: "127.0.0.1:8080".
	Addr string
	// AutoOpen controls whether the browser is opened automatically.
	AutoOpen bool
	// DevMode enables development features like template reloading.
	DevMode bool
	// TemplateDir overrides the embedded template directory with a
	// filesystem path. Only effective when DevMode is true.
	TemplateDir string

	// TLSCert is the path to the server TLS certificate PEM file.
	TLSCert string
	// TLSKey is the path to the server TLS private key PEM file.
	TLSKey string
	// TLSClientCA is the path to a CA PEM file for client certificate
	// verification. When set, mutual TLS is enabled.
	TLSClientCA string
	// TLSMinVersion is the minimum TLS version ("1.2" or "1.3").
	TLSMinVersion string
	// TLSCipherSuites is a list of allowed TLS cipher suite names.
	TLSCipherSuites []string
}

// DefaultConfig returns a Config with sensible defaults, overridden by
// environment variables where set.
func DefaultConfig() Config {
	cfg := Config{
		Addr:     "127.0.0.1:8080",
		AutoOpen: true,
	}
	if v := os.Getenv("ZHI_WEBUI_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("ZHI_WEBUI_AUTO_OPEN"); v == "false" || v == "0" {
		cfg.AutoOpen = false
	}
	if v := os.Getenv("ZHI_WEBUI_DEV"); v == "true" || v == "1" {
		cfg.DevMode = true
	}
	if v := os.Getenv("ZHI_WEBUI_TEMPLATE_DIR"); v != "" {
		cfg.TemplateDir = v
	}
	if v := os.Getenv("ZHI_WEBUI_TLS_CERT"); v != "" {
		cfg.TLSCert = v
	}
	if v := os.Getenv("ZHI_WEBUI_TLS_KEY"); v != "" {
		cfg.TLSKey = v
	}
	if v := os.Getenv("ZHI_WEBUI_TLS_CLIENT_CA"); v != "" {
		cfg.TLSClientCA = v
	}
	if v := os.Getenv("ZHI_WEBUI_TLS_MIN_VERSION"); v != "" {
		cfg.TLSMinVersion = v
	}
	if v := os.Getenv("ZHI_WEBUI_TLS_CIPHER_SUITES"); v != "" {
		for name := range strings.SplitSeq(v, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.TLSCipherSuites = append(cfg.TLSCipherSuites, name)
			}
		}
	}
	return cfg
}
