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

	// Auth routes (not behind requireAuth).
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /login/interactive", s.handleLoginInteractive)
	s.mux.HandleFunc("POST /logout", s.handleLogout)
	s.mux.HandleFunc("GET /auth/status", s.handleAuthStatus)

	// authWrap applies requireAuth middleware to a handler function.
	auth := s.requireAuth
	authWrap := func(h http.HandlerFunc) http.Handler { return auth(h) }

	// Root redirect.
	s.mux.Handle("GET /{$}", authWrap(s.handleRoot))

	// Tree view.
	s.mux.Handle("GET /tree", authWrap(s.handleTree))

	// Value editor
	s.mux.Handle("GET /tree/edit/{path...}", authWrap(s.handleEditForm))
	s.mux.Handle("POST /tree/values/{path...}", authWrap(s.handleSaveValue))
	s.mux.Handle("GET /tree/display/{path...}", authWrap(s.handleDisplayValue))

	// Inline validation
	s.mux.Handle("POST /validate/inline/{path...}", authWrap(s.handleInlineValidation))

	// Full validation
	s.mux.Handle("GET /validation", authWrap(s.handleValidationPage))
	s.mux.Handle("POST /validate", authWrap(s.handleFullValidation))

	// Save tree
	s.mux.Handle("POST /tree/save", authWrap(s.handleSaveTree))

	// Components
	s.mux.Handle("GET /components", authWrap(s.handleComponentsPage))
	s.mux.Handle("POST /components/{name}/toggle", authWrap(s.handleComponentToggle))

	// Export
	s.mux.Handle("GET /export", authWrap(s.handleExportPage))
	s.mux.Handle("POST /export/preview", authWrap(s.handleExportPreview))
	s.mux.Handle("POST /export", authWrap(s.handleExport))
	s.mux.Handle("POST /export/all", authWrap(s.handleExportAll))

	// Apply
	s.mux.Handle("GET /apply", authWrap(s.handleApplyPage))
	s.mux.Handle("POST /apply/run", authWrap(s.handleApplyRun))

	// Keyboard shortcuts
	s.mux.Handle("GET /shortcuts", authWrap(s.handleShortcutsPage))

	// Marketplace
	s.mux.Handle("GET /marketplace", authWrap(s.handleMarketplacePage))
	s.mux.Handle("GET /marketplace/{type}/{publisher}/{name}", authWrap(s.handlePluginDetail))
	s.mux.Handle("POST /marketplace/{type}/{publisher}/{name}/rate", authWrap(s.handleRatePlugin))

	// Installed plugins
	s.mux.Handle("GET /plugins", authWrap(s.handlePluginsPage))
	s.mux.Handle("POST /plugins/install", authWrap(s.handleInstallPlugin))
	s.mux.Handle("POST /plugins/update-all", authWrap(s.handleUpdateAllPlugins))
	s.mux.Handle("POST /plugins/{name}/uninstall", authWrap(s.handleUninstallPlugin))
	s.mux.Handle("POST /plugins/{name}/update", authWrap(s.handleUpdatePlugin))

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

	// Apply filter if present. Keep a reference to the full tree
	// so buildNestedTree can evaluate ui.showIf conditions.
	var reader config.TreeReader = tree
	if filter != "" {
		reader = filterTree(tree, filter)
	}

	nodes := buildNestedTree(reader, tree, components)

	data := pageData{
		WorkspaceName: wsName,
		ActiveNav:     "tree",
		Nonce:         nonceFromCtx(ctx),
		CSRFToken:     csrfFromCtx(ctx),
		Authenticated: authenticatedFromCtx(ctx),
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
		Authenticated: authenticatedFromCtx(ctx),
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
		Authenticated: authenticatedFromCtx(ctx),
		StatusCode:    code,
		StatusText:    http.StatusText(code),
		Message:       message,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
