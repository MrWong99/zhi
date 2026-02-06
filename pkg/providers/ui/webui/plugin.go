// Package webui implements a browser-based UI plugin for zhi. It serves
// an HTML interface for browsing and managing configuration trees.
package webui

import (
	"context"
	"fmt"
	"sync"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// WebUI implements the ui.Plugin interface.
type WebUI struct {
	config Config

	mu     sync.Mutex
	server *Server
	ready  chan struct{} // closed when server is assigned
}

// New creates a new WebUI plugin with the given configuration.
func New(cfg Config) *WebUI {
	return &WebUI{
		config: cfg,
		ready:  make(chan struct{}),
	}
}

// Run starts the web server and blocks until the context is cancelled.
func (w *WebUI) Run(ctx context.Context, controller ui.Controller) error {
	srv, err := NewServer(controller, w.config)
	if err != nil {
		return fmt.Errorf("creating webui server: %w", err)
	}
	w.mu.Lock()
	w.server = srv
	w.mu.Unlock()
	close(w.ready)
	return srv.ListenAndServe(ctx)
}

// Capabilities reports that the web UI does not require TTY access.
func (w *WebUI) Capabilities(_ context.Context) (ui.Capabilities, error) {
	return ui.Capabilities{
		RequiresTTY:         false,
		SupportsMarketplace: true,
	}, nil
}

// Addr returns the address the server is listening on. It blocks until
// the server is ready or ctx is cancelled.
func (w *WebUI) Addr(ctx context.Context) (string, error) {
	select {
	case <-w.ready:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	w.mu.Lock()
	srv := w.server
	w.mu.Unlock()
	if srv == nil {
		return "", fmt.Errorf("server not started")
	}
	return srv.Addr(ctx)
}
