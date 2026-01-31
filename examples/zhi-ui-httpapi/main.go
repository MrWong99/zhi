// zhi-ui-httpapi is an example zhi UI plugin that exposes the
// configuration engine as an HTTP/JSON API. It demonstrates:
//
//   - Implementing ui.Plugin (Run, Capabilities)
//   - Using the Controller for all core operations
//   - SSE (Server-Sent Events) for streaming Apply output
//   - Blocking Run until context cancellation with graceful shutdown
//
// The listen address defaults to ":8090" and can be overridden with the
// ZHI_HTTP_ADDR environment variable.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// httpUI implements ui.Plugin by serving an HTTP/JSON API.
type httpUI struct {
	addr string

	// mu protects boundAddr which is set once the listener is ready.
	mu        sync.Mutex
	boundAddr string
	ready     chan struct{} // closed when the server is listening
}

func newHTTPUI() *httpUI {
	addr := os.Getenv("ZHI_HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	return &httpUI{addr: addr, ready: make(chan struct{})}
}

// Addr returns the address the server is listening on. It blocks until
// the server is ready or ctx is cancelled.
func (h *httpUI) Addr(ctx context.Context) (string, error) {
	select {
	case <-h.ready:
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.boundAddr, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (h *httpUI) Capabilities(_ context.Context) (ui.Capabilities, error) {
	return ui.Capabilities{RequiresTTY: false}, nil
}

func (h *httpUI) Run(ctx context.Context, controller ui.Controller) error {
	mux := http.NewServeMux()
	s := &server{ctrl: controller}
	s.registerRoutes(mux)

	srv := &http.Server{
		Addr:    h.addr,
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	// If addr is ":0", the OS picks a free port. We use a listener so
	// the actual address is available for tests and logging.
	ln, err := net.Listen("tcp", h.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", h.addr, err)
	}

	actual := ln.Addr().String()
	h.mu.Lock()
	h.boundAddr = actual
	h.mu.Unlock()
	close(h.ready)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server error: %v", err)
		}
	}()

	log.Printf("zhi HTTP API listening on %s", actual)

	// Block until the context is cancelled (host shutting down).
	<-ctx.Done()

	// Graceful shutdown.
	shutdownCtx := context.Background()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	wg.Wait()
	return nil
}

// server holds a reference to the controller and registers HTTP handlers.
type server struct {
	ctrl ui.Controller
}

func (s *server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workspace", s.handleWorkspaceName)
	mux.HandleFunc("GET /api/tree", s.handleLoadTree)
	mux.HandleFunc("PUT /api/tree/values/{path...}", s.handleSetValue)
	mux.HandleFunc("POST /api/tree/validate", s.handleValidate)
	mux.HandleFunc("POST /api/tree/save", s.handleSaveTree)
	mux.HandleFunc("GET /api/export/templates", s.handleExportTemplates)
	mux.HandleFunc("POST /api/export", s.handleExport)
	mux.HandleFunc("POST /api/apply", s.handleApply)
	mux.HandleFunc("GET /api/components", s.handleListComponents)
	mux.HandleFunc("POST /api/components/{name}/enable", s.handleEnableComponent)
	mux.HandleFunc("POST /api/components/{name}/disable", s.handleDisableComponent)
}

// ---------- handlers ----------

func (s *server) handleWorkspaceName(w http.ResponseWriter, r *http.Request) {
	name, err := s.ctrl.WorkspaceName(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name})
}

func (s *server) handleLoadTree(w http.ResponseWriter, r *http.Request) {
	tree, err := s.ctrl.LoadTree(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	entries := ui.TreeToEntries(tree)
	type entryJSON struct {
		Path     string         `json:"path"`
		Value    any            `json:"value"`
		Metadata map[string]any `json:"metadata,omitempty"`
	}
	out := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryJSON{
			Path:     e.Path,
			Value:    e.Value.Val,
			Metadata: e.Value.Metadata,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleSetValue(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, errors.New("path is required"))
		return
	}

	var body struct {
		Value    any            `json:"value"`
		Metadata map[string]any `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	v := config.Value{Val: body.Value, Metadata: body.Metadata}
	if err := s.ctrl.SetValue(r.Context(), path, v); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleValidate(w http.ResponseWriter, r *http.Request) {
	results, err := s.ctrl.Validate(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *server) handleSaveTree(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if body.ID == "" {
		writeError(w, http.StatusBadRequest, errors.New("id is required"))
		return
	}
	if err := s.ctrl.SaveTree(r.Context(), body.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleExportTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := s.ctrl.ExportTemplates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	type tmplJSON struct {
		Name     string `json:"name"`
		Template string `json:"template"`
		Format   string `json:"format"`
		Output   string `json:"output"`
		Prefix   string `json:"prefix,omitempty"`
	}
	out := make([]tmplJSON, 0, len(templates))
	for _, t := range templates {
		out = append(out, tmplJSON{
			Name:     t.Name,
			Template: t.Template,
			Format:   t.Format,
			Output:   t.Output,
			Prefix:   t.Prefix,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleExport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TemplatePath string `json:"template_path"`
		Format       string `json:"format"`
		OutputPath   string `json:"output_path"`
		Prefix       string `json:"prefix"`
		DryRun       bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	req := ui.ExportRequest{
		TemplatePath: body.TemplatePath,
		Format:       body.Format,
		OutputPath:   body.OutputPath,
		Prefix:       body.Prefix,
		DryRun:       body.DryRun,
	}
	result, err := s.ctrl.Export(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"name":        result.Name,
		"content":     result.Content,
		"output_path": result.OutputPath,
	})
}

// handleApply streams apply output as Server-Sent Events (SSE).
//
// Each output line is sent as an "output" event with JSON data containing
// the line and stream ("stdout"/"stderr"). When the command finishes, a
// "done" event is sent with the exit code and any error message.
func (s *server) handleApply(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming not supported"))
		return
	}

	var body struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	handler := func(event ui.ApplyEvent) {
		data, _ := json.Marshal(map[string]string{
			"line":   event.Line,
			"stream": event.Stream,
		})
		_, _ = fmt.Fprintf(w, "event: output\ndata: %s\n\n", data)
		flusher.Flush()
	}

	result, err := s.ctrl.Apply(r.Context(), body.Target, handler)

	doneData := map[string]any{
		"exit_code": 0,
		"error":     "",
	}
	if result != nil {
		doneData["exit_code"] = result.ExitCode
		doneData["error"] = result.Error
	}
	if err != nil {
		doneData["error"] = err.Error()
	}
	data, _ := json.Marshal(doneData)
	_, _ = fmt.Fprintf(w, "event: done\ndata: %s\n\n", data)
	flusher.Flush()
}

func (s *server) handleListComponents(w http.ResponseWriter, r *http.Request) {
	components, err := s.ctrl.ListComponents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	type compJSON struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Enabled      bool     `json:"enabled"`
		Mandatory    bool     `json:"mandatory"`
		Paths        []string `json:"paths,omitempty"`
		Dependencies []string `json:"dependencies,omitempty"`
	}
	out := make([]compJSON, 0, len(components))
	for _, c := range components {
		out = append(out, compJSON{
			Name:         c.Name,
			Description:  c.Description,
			Enabled:      c.Enabled,
			Mandatory:    c.Mandatory,
			Paths:        c.Paths,
			Dependencies: c.Dependencies,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleEnableComponent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	enabled, err := s.ctrl.EnableComponent(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled})
}

func (s *server) handleDisableComponent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.ctrl.DisableComponent(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, err error) {
	// Avoid leaking internal details: only send the top-level error
	// message. Trim gRPC status prefixes for readability.
	msg := err.Error()
	if i := strings.LastIndex(msg, "desc = "); i >= 0 {
		msg = msg[i+len("desc = "):]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

func main() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ui": &ui.GRPCPlugin{Impl: newHTTPUI()},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
