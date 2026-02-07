package ui

import (
	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/providers/ui/webui"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// RegisterBuiltins registers all builtin UI drivers (registered via
// [Register]) into the core [core.Registry] as UI plugin factories.
// This bridges the legacy per-package init() registration with the new
// plugin system.
//
// It also registers UI plugins that implement [zhiui.Plugin] directly
// (such as the web UI) without the UIDriver adapter layer.
//
// It should be called after all UI driver init() functions have run (i.e.
// after importing the driver packages) and before resolving UI providers.
func RegisterBuiltins(reg *core.Registry) {
	driversMu.RLock()
	defer driversMu.RUnlock()

	for name, factory := range drivers {
		f := factory // capture loop variable
		_ = reg.RegisterUI(name, func(_ string, _ map[string]any) (zhiui.Plugin, error) {
			return &BuiltinAdapter{Driver: f()}, nil
		})
	}

	// Register the web UI directly -- it implements zhiui.Plugin and does
	// not require TTY access, so it bypasses the BuiltinAdapter.
	_ = reg.RegisterUI("webui", NewWebUIProvider)
}

// NewWebUIProvider is a UIFactory that creates a webui plugin. It reads
// configuration from the options map and environment variables.
//
// Supported options:
//   - addr:         Listen address (default: ZHI_WEBUI_ADDR or "127.0.0.1:8080")
//   - auto_open:    Open browser automatically (default: true)
//   - dev_mode:     Enable development features (default: ZHI_WEBUI_DEV or false)
//   - template_dir: Filesystem path for template reloading in dev mode
func NewWebUIProvider(_ string, options map[string]any) (zhiui.Plugin, error) {
	cfg := webui.DefaultConfig()
	if options != nil {
		if v, ok := options["addr"].(string); ok {
			cfg.Addr = v
		}
		if v, ok := options["auto_open"].(bool); ok {
			cfg.AutoOpen = v
		}
		if v, ok := options["dev_mode"].(bool); ok {
			cfg.DevMode = v
		}
		if v, ok := options["template_dir"].(string); ok {
			cfg.TemplateDir = v
		}
	}
	return webui.New(cfg), nil
}
