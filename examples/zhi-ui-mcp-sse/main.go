// zhi-ui-mcp-sse is a zhi UI plugin that exposes the configuration engine
// as an MCP (Model Context Protocol) server over HTTP using Streamable HTTP
// transport. It allows MCP-compatible LLM clients (Claude Desktop, Cursor,
// etc.) to manage configurations remotely.
//
// Configuration is read from the workspace options (zhi.yaml) passed via
// ZHI_PLUGIN_OPTIONS, falling back to environment variables:
//
//	ui:
//	  provider: mcp-sse
//	  options:
//	    addr: "0.0.0.0:9090"
//	    token: "my-secret"
//
// Supported options / environment variable fallbacks:
//   - addr  / ZHI_MCP_ADDR  — listen address (default: "127.0.0.1:8091")
//   - token / ZHI_MCP_TOKEN — Bearer token for authentication (empty = dev mode)
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/mcpbridge"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/pluginopts"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// mcpSSE implements ui.Plugin by serving an MCP server over HTTP.
type mcpSSE struct {
	addr      string
	authToken string

	mu        sync.Mutex
	boundAddr string
	ready     chan struct{} // closed when the server is listening
}

func newMCPSSE() *mcpSSE {
	opts := pluginopts.Options()
	return &mcpSSE{
		addr:      pluginopts.String(opts, "addr", "ZHI_MCP_ADDR", "127.0.0.1:8091"),
		authToken: pluginopts.String(opts, "token", "ZHI_MCP_TOKEN", ""),
		ready:     make(chan struct{}),
	}
}

// Addr returns the address the server is listening on. It blocks until
// the server is ready or ctx is cancelled.
func (m *mcpSSE) Addr(ctx context.Context) (string, error) {
	select {
	case <-m.ready:
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.boundAddr, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (m *mcpSSE) Capabilities(_ context.Context) (ui.Capabilities, error) {
	return ui.Capabilities{
		RequiresTTY:         false,
		SupportsMarketplace: true,
	}, nil
}

func (m *mcpSSE) Run(ctx context.Context, controller ui.Controller) error {
	server := mcpbridge.NewMCPServer(controller)

	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, nil)

	httpMux := http.NewServeMux()
	httpMux.Handle("/mcp", m.authMiddleware(handler))

	srv := &http.Server{
		Addr:    m.addr,
		Handler: httpMux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	ln, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", m.addr, err)
	}

	actual := ln.Addr().String()
	m.mu.Lock()
	m.boundAddr = actual
	m.mu.Unlock()
	close(m.ready)

	var wg sync.WaitGroup
	wg.Go(func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server error: %v", err)
		}
	})

	log.Printf("zhi MCP SSE server listening on %s", actual)

	<-ctx.Done()

	shutdownCtx := context.Background()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	wg.Wait()
	return nil
}

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-ui-mcp-sse",
		Level:  level,
		Output: os.Stderr,
	})
	logger.Info("starting MCP SSE UI plugin")

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ui": &ui.GRPCPlugin{Impl: newMCPSSE()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
