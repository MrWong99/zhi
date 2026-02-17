// Package authcallback provides an ephemeral HTTP server that listens on
// localhost for OAuth2/OIDC identity provider callbacks. The server accepts
// a single request, extracts the query parameters, and shuts down.
package authcallback

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// CallbackResult holds the query parameters received from the IdP callback.
type CallbackResult struct {
	Params map[string]string
	Err    error
}

// CallbackServer is a single-use HTTP server on localhost that captures the
// IdP redirect callback. It binds to 127.0.0.1 only to prevent external
// access.
type CallbackServer struct {
	listener  net.Listener
	result    chan CallbackResult
	server    *http.Server
	closeOnce sync.Once
}

// NewCallbackServer creates a new callback server bound to 127.0.0.1:0
// (random available port).
func NewCallbackServer() (*CallbackServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("binding callback listener: %w", err)
	}

	cs := &CallbackServer{
		listener: ln,
		result:   make(chan CallbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oidc/callback", cs.handleCallback)

	cs.server = &http.Server{Handler: mux}

	go func() {
		if err := cs.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			cs.result <- CallbackResult{Err: fmt.Errorf("callback server: %w", err)}
		}
	}()

	return cs, nil
}

// URL returns the callback URL that should be registered as the redirect URI.
func (s *CallbackServer) URL() string {
	return fmt.Sprintf("http://%s/oidc/callback", s.listener.Addr().String())
}

// Wait blocks until the callback is received or the context is cancelled.
func (s *CallbackServer) Wait(ctx context.Context) (CallbackResult, error) {
	select {
	case r := <-s.result:
		return r, r.Err
	case <-ctx.Done():
		s.Close()
		return CallbackResult{}, ctx.Err()
	}
}

// Close shuts down the callback server. Safe to call multiple times.
func (s *CallbackServer) Close() {
	s.closeOnce.Do(func() {
		s.server.Close()
	})
}

const successHTML = `<!DOCTYPE html>
<html><head><title>Login Successful</title></head>
<body style="font-family:sans-serif;text-align:center;padding:2em">
<h2>Login successful</h2>
<p>You may close this tab and return to the application.</p>
</body></html>`

func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	params := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, successHTML)

	s.result <- CallbackResult{Params: params}

	// Shut down asynchronously after responding.
	go s.Close()
}
