# Phase 1: Foundation

**Goal**: A running web server that renders the configuration tree as a read-only HTML page with navigation chrome, a design system, and security middleware in place.

At the end of this phase, a user can run `zhi ui webui` (or the external plugin), open a browser, and browse the configuration tree visually.

## Deliverables

### 1.1 Project Scaffolding

**Create the package structure:**

```
pkg/providers/ui/webui/
├── plugin.go          # ui.Plugin implementation
├── server.go          # HTTP server lifecycle
├── config.go          # Configuration struct + env parsing
├── routes.go          # Route table
├── middleware.go      # Middleware chain
├── templates.go       # Template engine
├── embed.go           # go:embed directives
├── static/            # CSS, JS vendor files, icons
└── templates/         # HTML templates
```

**Create the external plugin wrapper:**

```
examples/zhi-ui-webui/
├── main.go            # goplugin.Serve entry point
└── main_test.go       # Integration test
```

**Tasks:**
- [ ] Create `pkg/providers/ui/webui/plugin.go` implementing `ui.Plugin`
- [ ] Create `examples/zhi-ui-webui/main.go` as external plugin entry point
- [ ] Add `build-webui` target to Makefile
- [ ] Add `webui` to the built-in UI driver registry (behind build tag)

### 1.2 HTTP Server

**`server.go`** -- the core HTTP server with graceful shutdown:

```go
type Server struct {
    ctrl     ui.Controller
    config   Config
    engine   *templateEngine
    mux      *http.ServeMux
    httpSrv  *http.Server
}

func NewServer(ctrl ui.Controller, cfg Config) *Server
func (s *Server) ListenAndServe(ctx context.Context) error
```

**Behavior:**
- Bind to configured address (default `127.0.0.1:8080`)
- Print the URL to stderr: `zhi webui: listening on http://127.0.0.1:8080`
- Optionally auto-open the browser (`xdg-open` / `open` / `start`)
- Block until context is cancelled
- Graceful shutdown with 5-second timeout

**Tasks:**
- [ ] Implement `Server` struct with `ListenAndServe`
- [ ] Implement `Config` struct with environment variable parsing
- [ ] Implement browser auto-open (platform-aware: Linux, macOS, Windows)
- [ ] Implement graceful shutdown on context cancellation
- [ ] Write tests for server lifecycle (start, serve, shutdown)

### 1.3 Security Middleware

Implement the middleware stack from [SECURITY.md](../SECURITY.md):

**Tasks:**
- [ ] `requestID` -- generate unique request ID, add to context and response header
- [ ] `logging` -- log method, path, status, duration to stderr
- [ ] `recovery` -- catch panics, log stack trace, return 500 error page
- [ ] `securityHeaders` -- CSP, X-Frame-Options, X-Content-Type-Options, etc.
- [ ] `csrf` -- generate and validate CSRF tokens (double-submit cookie)
- [ ] `compress` -- gzip compression for HTML/CSS/JS responses
- [ ] Chain middleware in correct order in `routes.go`
- [ ] Write tests for CSRF validation (accept valid, reject missing/invalid)
- [ ] Write tests for security headers presence

### 1.4 Design System (CSS)

Create the visual foundation. No build tools needed -- plain CSS with custom properties.

**Files:**
```
static/css/
├── reset.css       # Minimal reset (box-sizing, margin, font inheritance)
├── tokens.css      # CSS custom properties (all design tokens)
├── layout.css      # Sidebar + main content grid
├── components.css  # Buttons, forms, cards, badges, tables
├── tree.css        # Tree-specific styles
└── themes/
    ├── light.css   # Light theme overrides (default)
    └── dark.css    # Dark theme overrides
```

**Tasks:**
- [ ] Create `reset.css` with minimal normalize
- [ ] Create `tokens.css` with color, typography, spacing, and shadow tokens
- [ ] Create `layout.css` with sidebar/main grid and responsive breakpoints
- [ ] Create `components.css` with button, form input, card, badge, and table styles
- [ ] Create `tree.css` with tree node, expand/collapse, and path highlighting
- [ ] Create `light.css` and `dark.css` theme variants
- [ ] Verify all styles work without JavaScript

### 1.5 Template Engine

Set up the Go `html/template` engine with helpers and layout composition.

**Tasks:**
- [ ] Create `templates.go` with template parsing, function map, and render methods
- [ ] Implement `renderPage(w, name, data)` for full page renders
- [ ] Implement `renderFragment(w, name, data)` for HTMX partial renders
- [ ] Implement HTMX detection (`HX-Request` header check)
- [ ] Register template functions: `csrfField`, `csrfToken`, `nonce`, `icon`, `activeNav`, `json`, `pathSegments`
- [ ] Create `layout.html` base template with `<head>`, sidebar, main content block
- [ ] Create `sidebar.html` partial with navigation links
- [ ] Create `topbar.html` partial with breadcrumb and action buttons
- [ ] Write tests for template rendering (full page and fragment modes)

### 1.6 Static Asset Embedding

**Tasks:**
- [ ] Create `embed.go` with `//go:embed` directives for `templates/` and `static/`
- [ ] Serve static files at `/static/` via `http.FileServer`
- [ ] Set `Cache-Control` headers for static assets (1 year for hashed files, no-cache for dev)
- [ ] Add HTMX and Alpine.js vendor files to `static/js/vendor/`
- [ ] Add Lucide icon SVGs for common icons (folder, file, edit, check, x, alert, search, etc.)

### 1.7 Tree View (Read-Only)

The first functional page: browse the configuration tree.

**Handler: `GET /tree`**
- Load tree via `ctrl.LoadTree(ctx)`
- Load components via `ctrl.ListComponents(ctx)`
- Build nested tree structure for rendering
- Render `pages/tree.html`

**Template features:**
- Collapsible tree using `<details>`/`<summary>` (no JS required)
- Path segments rendered as breadcrumbs
- Values displayed inline in monospace
- Component badges on path groups
- Disabled components shown muted
- Filter input that re-renders tree via HTMX (`GET /tree?filter=database`)

**Handler: `GET /tree?filter=...`** (HTMX partial)
- Same data loading
- Filter paths by prefix/substring
- Return only `fragments/tree_content.html`

**Tasks:**
- [ ] Implement `buildNestedTree()` to convert flat paths to hierarchical nodes
- [ ] Create `handlers/tree.go` with GET handler
- [ ] Create `pages/tree.html` with collapsible tree layout
- [ ] Create `fragments/tree_content.html` for HTMX filter updates
- [ ] Create `fragments/tree_node.html` for individual node rendering
- [ ] Implement path filtering (prefix match + substring match)
- [ ] Show component ownership badges on tree nodes
- [ ] Show disabled component indicators
- [ ] Style tree nodes with proper indentation and icons
- [ ] Write handler tests with mock controller

### 1.8 Workspace Info

**Handler: `GET /`** (redirects to `/tree`)

Display workspace name in the sidebar header.

**Tasks:**
- [ ] Create root handler that redirects to `/tree`
- [ ] Call `ctrl.WorkspaceName(ctx)` and include in layout data
- [ ] Display workspace name in sidebar header

### 1.9 Error Pages

Create error page templates for common HTTP errors:

**Tasks:**
- [ ] Create `pages/error.html` generic error template
- [ ] Render 404 for unknown routes
- [ ] Render 500 for internal errors (from recovery middleware)
- [ ] Render 400 for bad requests (invalid paths, etc.)
- [ ] Style error pages consistently with the design system

## Acceptance Criteria

- [ ] `make build-webui` produces a working binary
- [ ] Running the plugin starts an HTTP server on `127.0.0.1:8080`
- [ ] Browsing to the URL shows the configuration tree
- [ ] Tree nodes expand and collapse without JavaScript
- [ ] Filtering the tree updates results inline (with HTMX)
- [ ] Security headers present on all responses (CSP, X-Frame-Options, etc.)
- [ ] CSRF tokens embedded in page for future form use
- [ ] Light and dark themes selectable
- [ ] All tests pass with `make test`
- [ ] `make lint` passes
