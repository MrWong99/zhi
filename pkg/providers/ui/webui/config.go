package webui

import (
	"os"
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
	return cfg
}
