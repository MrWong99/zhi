package webui

import (
	"io/fs"
	"net/http"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// registerRoutes sets up all HTTP routes on the server's mux.
func (s *Server) registerRoutes() {
	// Static file serving from embedded FS.
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("embedded static FS missing: " + err.Error())
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/",
		cacheControl(http.FileServerFS(staticSub)),
	))

	// Root redirect.
	s.mux.HandleFunc("GET /{$}", s.handleRoot)

	// Tree view.
	s.mux.HandleFunc("GET /tree", s.handleTree)

	// 404 catch-all. The standard mux routes unmatched paths to the
	// longest matching pattern. With Go 1.22+ patterns, we register a
	// wildcard that catches everything else.
	s.mux.HandleFunc("GET /", s.handleNotFound)
}

// handleRoot redirects to /tree.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/tree", http.StatusFound)
}

// handleTree renders the configuration tree page (or fragment for HTMX).
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tree, err := s.ctrl.LoadTree(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load configuration tree: "+err.Error())
		return
	}

	components, err := s.ctrl.ListComponents(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load components: "+err.Error())
		return
	}

	wsName, _ := s.ctrl.WorkspaceName(ctx)

	filter := r.URL.Query().Get("filter")

	// Apply filter if present.
	var reader config.TreeReader = tree
	if filter != "" {
		reader = filterTree(tree, filter)
	}

	nodes := buildNestedTree(reader, components)

	data := pageData{
		WorkspaceName: wsName,
		ActiveNav:     "tree",
		Nonce:         nonceFromCtx(ctx),
		CSRFToken:     csrfFromCtx(ctx),
		TreeNodes:     nodes,
		Filter:        filter,
		Breadcrumbs: []breadcrumb{
			{Label: "Configuration", Href: "/tree"},
		},
	}

	if isHTMX(r) {
		if err := s.engine.renderFragment(w, "tree_content", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := s.engine.renderPage(w, "tree", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleNotFound renders a 404 error page.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.renderError(w, r, http.StatusNotFound, "")
}

// renderError renders an error page.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	ctx := r.Context()
	wsName, _ := s.ctrl.WorkspaceName(ctx)

	data := pageData{
		WorkspaceName: wsName,
		Nonce:         nonceFromCtx(ctx),
		CSRFToken:     csrfFromCtx(ctx),
		StatusCode:    code,
		StatusText:    http.StatusText(code),
		Message:       message,
	}

	w.WriteHeader(code)
	if err := s.engine.renderPage(w, "error", data); err != nil {
		http.Error(w, http.StatusText(code), code)
	}
}

// cacheControl wraps a handler to set Cache-Control for static assets.
func cacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		h.ServeHTTP(w, r)
	})
}
