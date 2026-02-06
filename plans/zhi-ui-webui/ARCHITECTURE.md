# Technical Architecture

## Plugin Integration

`zhi-ui-webui` is an external UI plugin. It implements the `ui.Plugin` interface and communicates with the zhi engine via gRPC (hashicorp/go-plugin). It can also be registered as a built-in plugin compiled directly into the zhi binary.

```go
// Capabilities: no TTY needed, marketplace support enabled
func (w *webUI) Capabilities(_ context.Context) (ui.Capabilities, error) {
    return ui.Capabilities{
        RequiresTTY:         false,
        SupportsMarketplace: true,
    }, nil
}

// Run: starts the HTTP server, blocks until context is cancelled
func (w *webUI) Run(ctx context.Context, controller ui.Controller) error {
    srv := NewServer(controller, w.config)
    return srv.ListenAndServe(ctx)
}
```

### Deployment Modes

1. **External plugin** (default): built as a standalone binary (`zhi-ui-webui`), discovered from `~/.zhi/plugins/`. Communicates with zhi over gRPC.
2. **Built-in plugin**: compiled into the zhi binary via build tag `webui`. Registered alongside the TUI in `internal/ui/`. Avoids gRPC overhead for local use.

## Project Structure

```
pkg/providers/ui/webui/
├── plugin.go                  # ui.Plugin implementation (Run, Capabilities)
├── server.go                  # HTTP server setup, graceful shutdown
├── config.go                  # Server configuration (addr, TLS, etc.)
├── routes.go                  # Route registration
├── middleware.go              # Security middleware stack
├── handlers/
│   ├── tree.go                # GET /tree, GET /tree/filter
│   ├── editor.go              # GET /tree/edit/:path, POST /tree/values/:path
│   ├── validation.go          # POST /validate, POST /tree/values/:path/validate
│   ├── save.go                # POST /tree/save
│   ├── components.go          # GET /components, POST /components/:name/toggle
│   ├── export.go              # GET /export, POST /export, GET /export/preview
│   ├── apply.go               # GET /apply, POST /apply/run (SSE)
│   ├── marketplace.go         # GET /marketplace, GET /marketplace/:pub/:name
│   ├── plugins.go             # GET /plugins, POST /plugins/install, etc.
│   └── shortcuts.go           # GET /shortcuts (keyboard shortcut overlay)
├── templates/
│   ├── layout.html            # Base layout: <html>, <head>, sidebar, main
│   ├── partials/
│   │   ├── sidebar.html       # Navigation sidebar
│   │   ├── topbar.html        # Breadcrumb + action bar
│   │   ├── notification.html  # Toast notification component
│   │   └── pagination.html    # Pagination component
│   ├── pages/
│   │   ├── tree.html          # Tree browser view
│   │   ├── editor.html        # Value editor panel
│   │   ├── components.html    # Component manager
│   │   ├── validation.html    # Validation results
│   │   ├── export.html        # Export templates + preview
│   │   ├── apply.html         # Apply command runner
│   │   ├── marketplace.html   # Marketplace browser
│   │   ├── plugins.html       # Installed plugins
│   │   ├── plugin_detail.html # Single plugin detail
│   │   └── shortcuts.html     # Keyboard shortcuts reference
│   └── fragments/
│       ├── tree_node.html     # Single tree node (for HTMX partial swap)
│       ├── tree_content.html  # Tree content area (for HTMX filtering)
│       ├── value_display.html # Inline value display
│       ├── value_form.html    # Inline edit form
│       ├── validation_badge.html  # Inline validation indicator
│       ├── component_card.html    # Single component card
│       ├── export_result.html     # Export execution result
│       ├── apply_output.html      # Apply streaming output line
│       ├── marketplace_card.html  # Marketplace search result
│       └── notification_toast.html # Single notification
├── static/
│   ├── css/
│   │   ├── reset.css          # Minimal CSS reset
│   │   ├── tokens.css         # CSS custom properties (colors, spacing, type)
│   │   ├── layout.css         # Sidebar + main grid layout
│   │   ├── components.css     # Reusable UI components (buttons, forms, cards)
│   │   ├── tree.css           # Tree-specific styles
│   │   ├── editor.css         # Editor-specific styles
│   │   └── themes/
│   │       ├── light.css      # Light theme token overrides
│   │       └── dark.css       # Dark theme token overrides
│   ├── js/
│   │   ├── vendor/
│   │   │   ├── htmx.min.js       # HTMX library (~14kB gzip)
│   │   │   ├── htmx-sse.js       # HTMX SSE extension
│   │   │   └── alpine.min.js     # Alpine.js (~15kB gzip)
│   │   └── app/
│   │       ├── shortcuts.ts      # Keyboard shortcut manager
│   │       ├── theme.ts          # Theme toggle (dark/light persistence)
│   │       ├── apply-stream.ts   # SSE consumer for apply output
│   │       └── notifications.ts  # Toast notification manager
│   └── icons/
│       └── *.svg              # Lucide icon SVGs (inline in templates)
├── embed.go                   # go:embed directives for static + templates
└── webui_test.go              # Integration tests

examples/zhi-ui-webui/
├── main.go                    # External plugin entry point
└── main_test.go               # Plugin integration tests
```

### Why `pkg/providers/ui/webui/`?

Following the existing pattern where built-in providers live in `pkg/providers/`:
- `pkg/providers/config/structuredfile/` -- built-in config provider
- `pkg/providers/store/vault/` -- built-in store provider
- `pkg/providers/ui/webui/` -- built-in web UI provider

The `examples/zhi-ui-webui/` directory contains the external plugin wrapper that imports and uses the core package.

## Template Engine

### Template Organization

Templates use Go `html/template` with a hierarchical structure:

```go
type templateEngine struct {
    templates map[string]*template.Template
    funcMap   template.FuncMap
}

func newTemplateEngine() *templateEngine {
    funcMap := template.FuncMap{
        // Formatting
        "json":       jsonMarshal,
        "pathSegments": splitPath,
        "pathParent":   parentPath,
        "pathBase":     basePath,
        "timeAgo":      timeAgo,
        "truncate":     truncate,

        // Tree helpers
        "treeToNested":  buildNestedTree,
        "isExpandable":  hasChildren,
        "childPaths":    filterChildren,

        // Security
        "csrfField":  csrfHiddenInput,
        "csrfToken":  csrfTokenValue,
        "nonce":      cspNonce,

        // UI helpers
        "severityClass": validationSeverityCSS,
        "severityIcon":  validationSeverityIcon,
        "componentBadge": componentBadgeHTML,
        "activeNav":     isActiveNavItem,
        "icon":          inlineSVGIcon,
    }
    // ...
}
```

### Rendering Pipeline

```go
// Full page render (initial load or non-HTMX request)
func (e *templateEngine) renderPage(w http.ResponseWriter, page string, data any) {
    // Renders: layout.html → includes sidebar, topbar → embeds page content
}

// Fragment render (HTMX partial swap)
func (e *templateEngine) renderFragment(w http.ResponseWriter, fragment string, data any) {
    // Renders only the named fragment template
}

// Handler pattern
func (h *treeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    tree, _ := h.ctrl.LoadTree(r.Context())
    data := buildTreeViewData(tree, r.URL.Query().Get("filter"))

    if isHTMX(r) {
        h.engine.renderFragment(w, "tree_content", data)
        return
    }
    h.engine.renderPage(w, "tree", data)
}
```

### Nested Tree Construction

The flat key-value tree from the config plugin is converted into a nested structure for rendering:

```go
type treeNode struct {
    Segment    string       // e.g., "database"
    FullPath   string       // e.g., "database/host"
    Value      *config.Value // nil for intermediate nodes
    Children   []*treeNode
    Component  string       // owning component name
    Disabled   bool         // component disabled
    Depth      int
}

func buildNestedTree(tree *config.Tree, components []ComponentInfo) []*treeNode {
    // Convert flat paths to hierarchical tree for template rendering
}
```

## Data Flow

### Read Flow (Tree View)

```
Browser GET /tree
    │
    ▼
server.routes → treeHandler.ServeHTTP
    │
    ├── ctrl.LoadTree(ctx)  ──→  gRPC ──→  Engine.LoadTree()
    │                                         │
    │                                         ├── configPlugin.List()
    │                                         ├── configPlugin.Get(path) × N
    │                                         └── transformForDisplay()
    │
    ├── ctrl.ListComponents(ctx)  ──→  ComponentManager.States()
    │
    ├── buildNestedTree(tree, components)
    │
    └── templateEngine.renderPage("tree", data)
            │
            ▼
        HTML response → Browser renders
```

### Write Flow (Edit Value)

```
Browser POST /tree/values/database/host
    │  form data: value=newhost
    │  header: X-CSRF-Token=...
    │
    ▼
csrfMiddleware.validate()
    │
    ▼
editorHandler.handleUpdate
    │
    ├── parse form value
    ├── ctrl.SetValue(ctx, "database/host", value)  ──→  gRPC ──→  Engine
    ├── ctrl.Validate(ctx)  ──→  gRPC ──→  Engine
    │
    ├── if validation OK:
    │     renderFragment("value_display", updatedData)
    │     + HX-Trigger: showNotification(success)
    │
    └── if validation failed:
          renderFragment("value_form", dataWithErrors)
```

### Streaming Flow (Apply)

```
Browser GET /apply/run  (SSE connection via EventSource)
    │
    ▼
applyHandler.handleStream
    │
    ├── Set headers: Content-Type: text/event-stream
    ├── ctrl.Apply(ctx, target, func(event) {
    │       fmt.Fprintf(w, "event: output\ndata: %s\n\n", json(event))
    │       flusher.Flush()
    │   })
    │
    ├── on each event:
    │     event: output
    │     data: {"line":"Pulling image...","stream":"stdout"}
    │
    └── on completion:
          event: done
          data: {"exit_code":0}
```

## Asset Embedding

All static assets are embedded into the binary using Go's `embed` directive:

```go
//go:embed templates/* static/*
var assets embed.FS

// Templates are parsed from embedded FS at startup
// Static files are served via http.FileServer(http.FS(...))
```

This produces a single binary with no external file dependencies, matching zhi's distribution model.

### TypeScript Build

TypeScript files in `static/js/app/` are compiled to JavaScript at build time:

```makefile
build-webui-ts:
    npx esbuild static/js/app/*.ts \
        --bundle --minify --sourcemap \
        --outdir=static/js/dist/ \
        --format=esm --target=es2020
```

The compiled JS is then embedded alongside other static assets. The build uses `esbuild` for fast compilation with no complex bundler configuration.

## Server Configuration

```go
type Config struct {
    // Network
    Addr       string  // Listen address (default: "127.0.0.1:8080")
    TLSCert    string  // TLS certificate path (optional)
    TLSKey     string  // TLS private key path (optional)

    // Security
    CSRFSecret []byte  // CSRF token signing key (auto-generated if empty)
    AllowOrigins []string // CORS origins (default: same-origin only)

    // Behavior
    OpenBrowser bool   // Auto-open browser on start (default: true)
    ReadOnly    bool   // Disable mutations (view-only mode)

    // Development
    DevMode     bool   // Serve files from disk instead of embed (hot reload)
}
```

Configuration is passed via environment variables prefixed with `ZHI_WEBUI_`:
- `ZHI_WEBUI_ADDR` -- listen address
- `ZHI_WEBUI_TLS_CERT` / `ZHI_WEBUI_TLS_KEY` -- TLS config
- `ZHI_WEBUI_OPEN_BROWSER` -- auto-open (true/false)
- `ZHI_WEBUI_READ_ONLY` -- view-only mode
- `ZHI_WEBUI_DEV` -- development mode

## Middleware Stack

Requests flow through middleware in order:

```
Request
  → requestID         (unique ID for tracing)
  → logging           (structured request/response logging)
  → recovery          (panic recovery with error page)
  → securityHeaders   (CSP, HSTS, X-Frame-Options, etc.)
  → csrf              (token validation on POST/PUT/DELETE)
  → compress          (gzip response compression)
  → static            (serve embedded CSS/JS/icons)
  → Handler
```

See [SECURITY.md](SECURITY.md) for details on each security middleware.
