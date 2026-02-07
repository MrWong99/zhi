package webui

import (
	"io/fs"
	"net/http"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// registerRoutes sets up all HTTP routes on the server's mux.
func (s *Server) registerRoutes() {
	// Static file serving from embedded FS. Content-hashed paths
	// (e.g. /static/css/tokens.a1b2c3d4.css) are served with immutable
	// cache headers. Non-hashed paths get a short cache.
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("embedded static FS missing: " + err.Error())
	}
	fileServer := http.FileServerFS(staticSub)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/",
		hashedStaticHandler(fileServer),
	))

	// Root redirect.
	s.mux.HandleFunc("GET /{$}", s.handleRoot)

	// Tree view.
	s.mux.HandleFunc("GET /tree", s.handleTree)

	// Value editor (Phase 2).
	s.mux.HandleFunc("GET /tree/edit/{path...}", s.handleEditForm)
	s.mux.HandleFunc("POST /tree/values/{path...}", s.handleSaveValue)
	s.mux.HandleFunc("GET /tree/display/{path...}", s.handleDisplayValue)

	// Inline validation (Phase 2).
	s.mux.HandleFunc("POST /validate/inline/{path...}", s.handleInlineValidation)

	// Full validation (Phase 2).
	s.mux.HandleFunc("GET /validation", s.handleValidationPage)
	s.mux.HandleFunc("POST /validate", s.handleFullValidation)

	// Save tree (Phase 2).
	s.mux.HandleFunc("POST /tree/save", s.handleSaveTree)

	// Components (Phase 2).
	s.mux.HandleFunc("GET /components", s.handleComponentsPage)
	s.mux.HandleFunc("POST /components/{name}/toggle", s.handleComponentToggle)

	// Export (Phase 3).
	s.mux.HandleFunc("GET /export", s.handleExportPage)
	s.mux.HandleFunc("POST /export/preview", s.handleExportPreview)
	s.mux.HandleFunc("POST /export", s.handleExport)
	s.mux.HandleFunc("POST /export/all", s.handleExportAll)

	// Apply (Phase 3).
	s.mux.HandleFunc("GET /apply", s.handleApplyPage)
	s.mux.HandleFunc("POST /apply/run", s.handleApplyRun)

	// Keyboard shortcuts (Phase 2).
	s.mux.HandleFunc("GET /shortcuts", s.handleShortcutsPage)

	// Marketplace (Phase 4).
	s.mux.HandleFunc("GET /marketplace", s.handleMarketplacePage)
	s.mux.HandleFunc("GET /marketplace/{publisher}/{name}", s.handlePluginDetail)
	s.mux.HandleFunc("POST /marketplace/{publisher}/{name}/rate", s.handleRatePlugin)

	// Installed plugins (Phase 4).
	s.mux.HandleFunc("GET /plugins", s.handlePluginsPage)
	s.mux.HandleFunc("POST /plugins/install", s.handleInstallPlugin)
	s.mux.HandleFunc("POST /plugins/update-all", s.handleUpdateAllPlugins)
	s.mux.HandleFunc("POST /plugins/{name}/uninstall", s.handleUninstallPlugin)
	s.mux.HandleFunc("POST /plugins/{name}/update", s.handleUpdatePlugin)

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

// handleShortcutsPage renders the keyboard shortcuts page.
func (s *Server) handleShortcutsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsName, _ := s.ctrl.WorkspaceName(ctx)

	data := pageData{
		WorkspaceName: wsName,
		ActiveNav:     "shortcuts",
		Nonce:         nonceFromCtx(ctx),
		CSRFToken:     csrfFromCtx(ctx),
		Breadcrumbs: []breadcrumb{
			{Label: "Keyboard Shortcuts", Href: "/shortcuts"},
		},
	}

	if err := s.engine.renderPage(w, "shortcuts", data); err != nil {
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

// hashedStaticHandler strips content hashes from URLs and sets appropriate
// cache headers. Hashed paths get immutable 1-year caching; non-hashed
// paths get a short 1-hour cache.
func hashedStaticHandler(fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original, hashed := isHashedPath(r.URL.Path)
		if hashed {
			r.URL.Path = original
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		fileServer.ServeHTTP(w, r)
	})
}
