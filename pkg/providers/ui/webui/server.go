package webui

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/MrWong99/zhi/internal/tlsutil"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// Server is the Web UI HTTP server.
type Server struct {
	ctrl    ui.Controller
	config  Config
	engine  *templateEngine
	mux     *http.ServeMux
	httpSrv *http.Server

	// mu protects boundAddr which is set once the listener is ready.
	mu        sync.Mutex
	boundAddr string
	ready     chan struct{}
}

// NewServer creates a new Web UI server.
func NewServer(ctrl ui.Controller, cfg Config) (*Server, error) {
	engine, err := newTemplateEngine(cfg.DevMode, cfg.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("initializing template engine: %w", err)
	}

	s := &Server{
		ctrl:   ctrl,
		config: cfg,
		engine: engine,
		mux:    http.NewServeMux(),
		ready:  make(chan struct{}),
	}
	s.registerRoutes()
	return s, nil
}

// Addr returns the address the server is listening on. It blocks until
// the server is ready or ctx is cancelled.
func (s *Server) Addr(ctx context.Context) (string, error) {
	select {
	case <-s.ready:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.boundAddr, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ListenAndServe starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	// Determine TLS config first so the middleware chain can know whether
	// the server is running over HTTPS.
	tlsCfg := &tlsutil.Config{
		CertFile:     s.config.TLSCert,
		KeyFile:      s.config.TLSKey,
		ClientCAFile: s.config.TLSClientCA,
		MinVersion:   s.config.TLSMinVersion,
		CipherSuites: s.config.TLSCipherSuites,
	}

	// Build the middleware chain.
	csrf := newCSRFMiddleware(tlsCfg.Enabled())
	handler := s.mux
	var h http.Handler = handler

	// Apply middleware in reverse order (outermost first).
	h = compressMiddleware(h)
	h = etagMiddleware(h)
	h = csrf.middleware(h)
	h = securityHeadersMiddleware(h)
	h = recoveryMiddleware(s.engine)(h)
	h = loggingMiddleware(h)
	h = responseTimeMiddleware(h)
	h = requestIDMiddleware(h)

	s.httpSrv = &http.Server{
		Addr:              s.config.Addr,
		Handler:           h,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    8 << 10, // 8KB
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	ln, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.config.Addr, err)
	}

	// Wrap with TLS if configured.
	if tlsCfg.Enabled() {
		tc, err := tlsCfg.TLSConfig()
		if err != nil {
			ln.Close()
			return fmt.Errorf("configuring TLS: %w", err)
		}
		ln = tls.NewListener(ln, tc)
	}

	actual := ln.Addr().String()
	s.mu.Lock()
	s.boundAddr = actual
	s.mu.Unlock()
	close(s.ready)

	scheme := "http"
	if tlsCfg.Enabled() {
		scheme = "https"
	}
	url := scheme + "://" + actual
	log.Printf("zhi webui: listening on %s", url)

	if s.config.DevMode {
		log.Printf("zhi webui: dev mode enabled")
		if s.config.TemplateDir != "" {
			log.Printf("zhi webui: template dir = %s", s.config.TemplateDir)
		}
	}

	if s.config.AutoOpen {
		go openBrowser(url)
	}

	var cancelOnError context.CancelFunc
	ctx, cancelOnError = context.WithCancel(ctx)

	var wg sync.WaitGroup
	wg.Go(func() {
		if err := s.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server error: %v", err)
			cancelOnError()
		}
	})

	// Block until context is cancelled.
	<-ctx.Done()

	// Graceful shutdown with 5-second timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	wg.Wait()
	return nil
}

// openBrowser attempts to open the given URL in the default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	_ = cmd.Start()
}
