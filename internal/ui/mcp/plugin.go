// Package mcp implements the MCP stdio UI plugin for zhi. It runs an MCP
// server over stdin/stdout, allowing LLM clients (Claude Desktop, Claude Code,
// Cursor, etc.) to manage configurations using the Model Context Protocol.
package mcp

import (
	"context"
	"io"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/mcpbridge"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// StdioPlugin implements ui.Plugin by running an MCP server over stdio.
// The Stdin and Stdout fields must be set before calling Run — typically
// the factory saves the original os.Stdout (before redirecting it to
// stderr) and passes it here so that stray writes from other code do not
// corrupt the MCP JSON-RPC stream.
type StdioPlugin struct {
	// Stdin is the reader for MCP input (typically os.Stdin).
	Stdin io.ReadCloser
	// Stdout is the writer for MCP output (typically the original os.Stdout
	// saved before redirection).
	Stdout io.WriteCloser
	// ReadOnly disables mutation tools when true.
	ReadOnly bool
	// Version overrides the MCP server version string.
	Version string
}

// Run starts the MCP server and blocks until the client disconnects or
// ctx is cancelled.
func (p *StdioPlugin) Run(ctx context.Context, controller ui.Controller) error {
	var opts []mcpbridge.Option
	if p.ReadOnly {
		opts = append(opts, mcpbridge.WithReadOnly(true))
	}
	if p.Version != "" {
		opts = append(opts, mcpbridge.WithVersion(p.Version))
	}

	server := mcpbridge.NewMCPServer(controller, opts...)

	transport := &gomcp.IOTransport{
		Reader: p.Stdin,
		Writer: p.Stdout,
	}
	return server.Run(ctx, transport)
}

// Capabilities reports that the MCP stdio plugin does not require TTY
// access and supports marketplace operations.
func (p *StdioPlugin) Capabilities(_ context.Context) (ui.Capabilities, error) {
	return ui.Capabilities{
		RequiresTTY:         false,
		SupportsMarketplace: true,
	}, nil
}
