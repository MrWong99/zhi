package ui

import (
	"os"

	mcpplugin "github.com/MrWong99/zhi/internal/ui/mcp"

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

	// Register the MCP stdio plugin -- it runs an MCP server over
	// stdin/stdout for LLM clients (Claude Desktop, Claude Code, etc.).
	_ = reg.RegisterUI("mcp-stdio", NewMCPStdioProvider)
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
		if v, ok := options["tls_cert"].(string); ok {
			cfg.TLSCert = v
		}
		if v, ok := options["tls_key"].(string); ok {
			cfg.TLSKey = v
		}
		if v, ok := options["tls_client_ca"].(string); ok {
			cfg.TLSClientCA = v
		}
		if v, ok := options["tls_min_version"].(string); ok {
			cfg.TLSMinVersion = v
		}
		if v, ok := options["tls_cipher_suites"].([]string); ok {
			cfg.TLSCipherSuites = v
		}
	}
	return webui.New(cfg), nil
}

// NewMCPStdioProvider is a UIFactory that creates an MCP stdio plugin.
// It saves the original os.Stdout and redirects it to os.Stderr so that
// stray writes (log output, fmt.Println, etc.) do not corrupt the MCP
// JSON-RPC stream on stdout.
//
// Supported options:
//   - read_only: Disable mutation tools (default: false)
//   - version:   Override the MCP server version string
func NewMCPStdioProvider(_ string, options map[string]any) (zhiui.Plugin, error) {
	// Save original stdout for the MCP transport and redirect os.Stdout
	// to stderr to protect the MCP stream.
	origStdout := os.Stdout
	os.Stdout = os.Stderr

	plugin := &mcpplugin.StdioPlugin{
		Stdin:  os.Stdin,
		Stdout: origStdout,
	}

	if options != nil {
		if v, ok := options["read_only"].(bool); ok && v {
			plugin.ReadOnly = true
		}
		if v, ok := options["version"].(string); ok {
			plugin.Version = v
		}
	}
	return plugin, nil
}
