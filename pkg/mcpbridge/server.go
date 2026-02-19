package mcpbridge

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// Option configures the MCP server created by NewMCPServer.
type Option func(*serverConfig)

type serverConfig struct {
	version  string
	readOnly bool
}

// WithVersion overrides the version string reported by the MCP server.
func WithVersion(v string) Option {
	return func(c *serverConfig) { c.version = v }
}

// WithReadOnly disables all mutation tools (set_value, save, apply,
// enable/disable components, marketplace install/uninstall, store login/logout).
func WithReadOnly(ro bool) Option {
	return func(c *serverConfig) { c.readOnly = ro }
}

// NewMCPServer creates a fully configured MCP server with all tools,
// resources, and prompts registered against the given Controller. The
// Controller is wrapped in a SafeController to serialize concurrent access.
func NewMCPServer(ctrl ui.Controller, opts ...Option) *mcp.Server {
	cfg := serverConfig{version: "dev"}
	for _, o := range opts {
		o(&cfg)
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "zhi", Version: cfg.version},
		nil,
	)

	safe := NewSafeController(ctrl)
	RegisterResources(server, safe)
	RegisterPrompts(server, safe)
	RegisterTools(server, safe, cfg.readOnly)

	return server
}
