# Design Philosophy & Visual Language

## Core Philosophy: "Terminal Elegance in the Browser"

The Web UI bridges the precision and directness of a terminal interface with the visual richness of a modern browser. Every element serves a purpose. There is no decoration without function.

### Guiding Principles

1. **Information density over whitespace** -- configuration management is data-heavy; optimize for showing many paths and values at once, not for marketing page aesthetics.
2. **Keyboard-first, mouse-friendly** -- power users navigate with shortcuts; casual users click. Both paths are first-class.
3. **Immediate feedback** -- validation results appear inline as you edit, not after a round-trip to a separate page. HTMX partial swaps make this possible without JavaScript complexity.
4. **Contextual actions** -- operations appear where they make sense. Edit buttons next to values, validate next to the form, save in the toolbar. No hidden menus.
5. **Progressive disclosure** -- show the tree first; expand into details on demand. Metadata, validation history, and component info are revealed, not dumped.

## Visual Language

### Color System

A purposeful, security-aware color palette built on CSS custom properties for theming:

```
Light Theme (default):
  --bg-primary:     #fafafa      // Page background
  --bg-surface:     #ffffff      // Cards, panels
  --bg-elevated:    #f5f5f5      // Hover states, secondary panels
  --text-primary:   #1a1a2e      // Primary text (near-black, not pure black)
  --text-secondary: #64648c      // Muted text, labels
  --accent:         #6366f1      // Primary actions (indigo -- distinct, professional)
  --accent-hover:   #4f46e5      // Hover state
  --success:        #10b981      // Validation pass, save success
  --warning:        #f59e0b      // Validation warnings
  --danger:         #ef4444      // Blocking errors, destructive actions
  --info:           #3b82f6      // Info severity, hints
  --border:         #e5e5ef      // Subtle borders

Dark Theme:
  --bg-primary:     #0f0f1a      // Deep dark background
  --bg-surface:     #1a1a2e      // Cards, panels
  --bg-elevated:    #252540      // Hover, secondary
  --text-primary:   #e5e5ef      // Light text
  --text-secondary: #9090b0      // Muted
  --accent:         #818cf8      // Lighter indigo for dark bg
  --border:         #2a2a45      // Subtle dark borders
```

### Typography

```
--font-ui:    'Inter', system-ui, sans-serif    // UI chrome, labels, buttons
--font-mono:  'JetBrains Mono', 'Fira Code', monospace  // Paths, values, code
--font-size-sm:   0.8125rem   // 13px -- secondary labels
--font-size-base: 0.875rem    // 14px -- body text (dense)
--font-size-lg:   1rem        // 16px -- section headers
--font-size-xl:   1.25rem     // 20px -- page titles
```

Configuration paths and values are always rendered in monospace. UI chrome uses the sans-serif stack. The base font size is intentionally smaller (14px) to maximize information density.

### Spacing & Grid

- **4px base unit** -- all spacing is a multiple of 4px
- **Sidebar**: fixed 280px, collapsible to icon-only (48px)
- **Main content**: fluid, max-width 1200px for readability
- **Tree items**: 36px row height for comfortable clicking with dense layout
- **Cards**: 12px padding, 6px border-radius, subtle shadow

### Iconography

Use a small, consistent icon set. Prefer:
- **Lucide Icons** (open source, clean line style, tree-shakeable SVGs)
- Inline SVGs embedded in templates (no icon font, no external requests)
- 16x16 and 20x20 sizes only

### Component Patterns

#### The Sidebar

```
┌────────────────────┬─────────────────────────────────────┐
│  ◆ zhi             │                                     │
│  workspace-name    │                                     │
│                    │                                     │
│  ▸ Tree            │        Main Content Area            │
│    Editor          │                                     │
│    Components      │                                     │
│  ──────────        │                                     │
│  ▸ Validate        │                                     │
│    Export          │                                     │
│    Apply           │                                     │
│  ──────────        │                                     │
│  ▸ Marketplace     │                                     │
│    Plugins         │                                     │
│  ──────────        │                                     │
│  ◐ Dark/Light      │                                     │
│  ⌨ Shortcuts       │                                     │
│  ⓘ About           │                                     │
└────────────────────┴─────────────────────────────────────┘
```

The sidebar is the primary navigation. It uses full-page navigation (standard `<a>` links) with HTMX boosting for instant transitions.

#### Tree View

```
┌─────────────────────────────────────────────────────┐
│  Configuration Tree                    [Filter...] │
│  ─────────────────────────────────────────────────── │
│  ▾ database/                         [component]   │
│    ├─ host          "localhost"       [edit]        │
│    ├─ port          5432              [edit]        │
│    └─ name          "mydb"            [edit]        │
│  ▾ app/                                            │
│    ├─ debug         true              [edit]        │
│    └─ log-level     "info"            [edit]        │
│  ▸ tls/                              [disabled]    │
└─────────────────────────────────────────────────────┘
```

- Paths rendered as a collapsible tree (server-side with `<details>`/`<summary>`)
- Values shown inline in monospace
- Component badges show which component owns each path group
- Disabled components shown muted with a badge
- Filter input uses HTMX to re-render the tree with matching paths

#### Inline Editing

```
┌─────────────────────────────────────────────────────┐
│  database/host                                      │
│  ─────────────────────────────────────────────────── │
│  ┌────────────────────────────────────────────────┐ │
│  │ localhost                                      │ │
│  └────────────────────────────────────────────────┘ │
│  Type: string  │  Component: database               │
│                                                     │
│  Metadata:                                          │
│    description: "Database hostname"                 │
│    ui.placeholder: "Enter hostname or IP"           │
│                                                     │
│  Validation: ✓ Valid                                │
│                                                     │
│  [Save Value]  [Cancel]                             │
└─────────────────────────────────────────────────────┘
```

Editing happens inline or in a detail panel (not a modal). The form POSTs to the server; HTMX swaps the response into the page. Validation runs on blur via HTMX trigger.

#### Notifications & Toasts

Non-blocking notifications slide in from the top-right corner:
- **Success** (green accent): "Tree saved successfully"
- **Warning** (amber): "3 validation warnings"
- **Error** (red): "Failed to save: store unreachable"
- Auto-dismiss after 5 seconds, click to dismiss immediately
- Implemented with Alpine.js for show/hide transitions

## Interaction Patterns

### Navigation

| Pattern | Implementation |
|---------|---------------|
| Page navigation | Standard `<a href>` links with HTMX `hx-boost` for SPA-like speed |
| Tree expand/collapse | `<details>`/`<summary>` elements (no JS needed, accessible by default) |
| Inline editing | HTMX `hx-get` loads edit form, `hx-post` submits, swaps response |
| Filtering | HTMX `hx-get` with `hx-trigger="input changed delay:300ms"` |
| Modals/Dialogs | Alpine.js `x-show` with `<dialog>` element for confirmation dialogs |

### Keyboard Shortcuts

Global shortcuts managed by a small TypeScript module:

| Shortcut | Action |
|----------|--------|
| `/` | Focus filter/search input |
| `g t` | Go to Tree view |
| `g c` | Go to Components view |
| `g v` | Go to Validation view |
| `g e` | Go to Export view |
| `g a` | Go to Apply view |
| `g m` | Go to Marketplace view |
| `Ctrl+S` | Save tree |
| `Esc` | Close panel / cancel edit |
| `?` | Show keyboard shortcut overlay |

### Responsive Behavior

- **Desktop (>1024px)**: Sidebar + main content side by side
- **Tablet (768-1024px)**: Collapsible sidebar (icon-only by default)
- **Mobile (<768px)**: Sidebar becomes a hamburger menu; single-column layout

The UI is usable on mobile but optimized for desktop -- configuration management is inherently a desktop-focused task.

## SSR + HTMX Patterns

### Full Page Render

Every route returns a complete HTML document on first load. This ensures:
- Works with JavaScript disabled (read-only mode)
- Search engines can index (not relevant for local, but good practice)
- Browser back/forward works natively

### Partial Swaps

HTMX requests include `HX-Request: true` header. The server detects this and returns only the changed fragment:

```go
func (s *server) handleTreeView(w http.ResponseWriter, r *http.Request) {
    data := s.loadTreeData(r.Context())
    if r.Header.Get("HX-Request") == "true" {
        s.renderPartial(w, "tree-content", data)
        return
    }
    s.renderFull(w, "tree", data)
}
```

### Form Submission Pattern

```html
<form hx-post="/tree/values/database/host"
      hx-target="#value-editor"
      hx-swap="outerHTML">
    <input name="value" value="{{ .Value }}">
    <button type="submit">Save</button>
</form>
```

On submit, the server validates and returns either:
- The updated view fragment (success)
- The form with validation errors highlighted (failure)

No client-side validation logic needed.

### Live Validation

```html
<input name="value"
       hx-post="/tree/values/database/port/validate"
       hx-trigger="input changed delay:500ms"
       hx-target="#validation-result"
       hx-swap="innerHTML">
<div id="validation-result"></div>
```

Validation feedback appears inline after 500ms of inactivity. The server runs the full validation pipeline and returns an HTML fragment with the result.
