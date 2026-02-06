# Phase 2: Core Interaction

**Goal**: Users can edit configuration values, validate the tree, save changes, and manage components -- all through server-rendered forms with HTMX-powered inline updates.

At the end of this phase, the web UI is a functional replacement for the TUI's core workflow: browse → edit → validate → save.

## Deliverables

### 2.1 Value Editor

**Inline editing** -- clicking "edit" on a tree node replaces the display with an edit form:

**Handler: `GET /tree/edit/{path...}`** (HTMX partial)
- Load current value for the path
- Return `fragments/value_form.html` with input field pre-filled

**Handler: `POST /tree/values/{path...}`**
- Parse form value
- Call `ctrl.SetValue(ctx, path, value)`
- Run validation: `ctrl.Validate(ctx)`
- On success: return `fragments/value_display.html` with updated value
- On validation error: return `fragments/value_form.html` with error messages inline

**Template: `value_form.html`**
```html
<form hx-post="/tree/values/{{ .Path }}"
      hx-target="closest .tree-value"
      hx-swap="outerHTML">
    {{ csrfField }}
    <input type="text" name="value" value="{{ .Value.Val }}"
           class="value-input" autofocus
           hx-post="/tree/values/{{ .Path }}/validate"
           hx-trigger="input changed delay:500ms"
           hx-target="#validation-inline-{{ .PathID }}"
           hx-swap="innerHTML">
    <div id="validation-inline-{{ .PathID }}" class="validation-inline"></div>
    <div class="editor-meta">
        <span class="meta-type">{{ .ValueType }}</span>
        {{ if .Value.Metadata }}
            {{ range $k, $v := .Value.Metadata }}
                <span class="meta-tag">{{ $k }}: {{ $v }}</span>
            {{ end }}
        {{ end }}
    </div>
    <div class="editor-actions">
        <button type="submit" class="btn btn-primary">Save</button>
        <button type="button" class="btn btn-ghost"
                hx-get="/tree/display/{{ .Path }}"
                hx-target="closest .tree-value"
                hx-swap="outerHTML">Cancel</button>
    </div>
</form>
```

**Type-aware inputs:**
- String values: text input
- Integer/float values: number input with step
- Boolean values: toggle switch / checkbox
- Multi-line strings: textarea (auto-expand)
- Values with metadata `ui.options`: select dropdown

**Tasks:**
- [ ] Create `handlers/editor.go` with GET (edit form) and POST (save value) handlers
- [ ] Create `fragments/value_form.html` with type-aware input rendering
- [ ] Create `fragments/value_display.html` for read-only value display
- [ ] Implement value type detection (string, number, bool) for appropriate input types
- [ ] Implement metadata-driven input hints (`ui.placeholder`, `ui.options`, `ui.multiline`)
- [ ] Handle HTMX partial swap for inline editing
- [ ] Handle cancel action (restore display mode)
- [ ] Write tests for value parsing and setting

### 2.2 Live Validation

**Handler: `POST /tree/values/{path...}/validate`** (HTMX partial)
- Set the value temporarily
- Run `ctrl.Validate(ctx)`
- Filter results relevant to the edited path
- Return `fragments/validation_badge.html`

**Handler: `POST /validate`** (full validation)
- Run `ctrl.Validate(ctx)`
- Return full validation results page or fragment

**Page: `pages/validation.html`**
- Groups results by severity (Blocking, Warning, Info)
- Each result shows: path, message, severity icon
- Clicking a path links back to the tree view for that node

**Tasks:**
- [ ] Create `handlers/validation.go` with inline and full validation handlers
- [ ] Create `fragments/validation_badge.html` for inline validation indicator
- [ ] Create `pages/validation.html` for full validation results view
- [ ] Implement severity grouping (Blocking → Warning → Info)
- [ ] Implement severity-colored badges with icons
- [ ] Link validation paths to tree view edit mode
- [ ] Show validation count in sidebar navigation (e.g., "Validate (3)")
- [ ] Write tests for validation result rendering

### 2.3 Save Tree

**Handler: `POST /tree/save`**
- Parse optional tree ID from form
- Call `ctrl.SaveTree(ctx, id)`
- On success: redirect to `/tree` with success notification
- On error: redirect to `/tree` with error notification

**Notification delivery:**
HTMX supports response headers that trigger client-side events:
```go
w.Header().Set("HX-Trigger", `{"showNotification": {"type": "success", "message": "Tree saved successfully"}}`)
```

Alpine.js listens for this event and renders a toast notification.

**Tasks:**
- [ ] Create `handlers/save.go` with POST handler
- [ ] Implement save form in the topbar / tree view
- [ ] Implement notification delivery via HX-Trigger headers
- [ ] Create `fragments/notification_toast.html` for toast rendering
- [ ] Create `static/js/app/notifications.ts` for toast display/dismiss logic
- [ ] Add "unsaved changes" indicator in the topbar
- [ ] Write tests for save flow

### 2.4 Component Management

**Page: `pages/components.html`**
- List all components with their state (enabled/disabled/mandatory)
- Show path prefixes owned by each component
- Show dependencies between components
- Toggle components via form POST

**Handler: `GET /components`**
- Call `ctrl.ListComponents(ctx)`
- Render component list

**Handler: `POST /components/{name}/toggle`**
- If currently enabled: call `ctrl.DisableComponent(ctx, name)`
- If currently disabled: call `ctrl.EnableComponent(ctx, name)`
- On success: return updated component card (HTMX partial)
- On error (e.g., mandatory component, dependency conflict): return card with error

**Template: `fragments/component_card.html`**
```html
<div class="component-card {{ if .Enabled }}enabled{{ end }} {{ if .Mandatory }}mandatory{{ end }}"
     id="component-{{ .Name }}">
    <div class="component-header">
        <h3>{{ .Name }}</h3>
        {{ if .Mandatory }}
            <span class="badge badge-info">Mandatory</span>
        {{ else }}
            <form hx-post="/components/{{ .Name }}/toggle"
                  hx-target="#component-{{ .Name }}"
                  hx-swap="outerHTML">
                {{ csrfField }}
                <button type="submit" class="toggle-btn {{ if .Enabled }}active{{ end }}">
                    {{ if .Enabled }}Disable{{ else }}Enable{{ end }}
                </button>
            </form>
        {{ end }}
    </div>
    <p class="component-desc">{{ .Description }}</p>
    <div class="component-paths">
        <strong>Paths:</strong>
        {{ range .Paths }}<code>{{ . }}</code>{{ end }}
    </div>
    {{ if .Dependencies }}
        <div class="component-deps">
            <strong>Depends on:</strong>
            {{ range .Dependencies }}<a href="#component-{{ . }}">{{ . }}</a>{{ end }}
        </div>
    {{ end }}
</div>
```

**Tasks:**
- [ ] Create `handlers/components.go` with GET and POST toggle handlers
- [ ] Create `pages/components.html` with component grid/list layout
- [ ] Create `fragments/component_card.html` for individual component rendering
- [ ] Implement toggle with dependency resolution feedback
- [ ] Show enabled dependency chain when enabling a component
- [ ] Show error when trying to disable a mandatory component
- [ ] Show warning when disabling a component that other components depend on
- [ ] Reflect component state changes in the tree view (muted disabled paths)
- [ ] Write tests for component toggle logic

### 2.5 Keyboard Shortcuts (Phase 2 Subset)

Implement the core keyboard shortcut manager with navigation shortcuts:

**File: `static/js/app/shortcuts.ts`**

```typescript
interface Shortcut {
    keys: string;       // e.g., "g t", "ctrl+s", "/"
    description: string;
    handler: () => void;
}

class ShortcutManager {
    register(shortcut: Shortcut): void;
    unregister(keys: string): void;
    enable(): void;
    disable(): void; // Disable when input is focused
}
```

**Phase 2 shortcuts:**
- `/` -- focus filter input
- `g t` -- navigate to Tree
- `g c` -- navigate to Components
- `g v` -- navigate to Validation
- `Ctrl+S` -- save tree
- `Esc` -- close edit form / cancel
- `?` -- show shortcut overlay

**Tasks:**
- [ ] Create `static/js/app/shortcuts.ts` with ShortcutManager class
- [ ] Implement multi-key sequence support (e.g., `g t`)
- [ ] Implement modifier key support (e.g., `Ctrl+S`)
- [ ] Auto-disable shortcuts when input/textarea is focused
- [ ] Compile TypeScript with esbuild and embed result
- [ ] Create shortcut overlay page (`pages/shortcuts.html`)
- [ ] Write tests for shortcut parsing and matching

### 2.6 Theme Toggle

**File: `static/js/app/theme.ts`**

```typescript
// Read preference from localStorage, fallback to system preference
// Toggle by swapping data-theme attribute on <html>
// Persist choice to localStorage
```

**Tasks:**
- [ ] Create `static/js/app/theme.ts` with theme detection and toggle
- [ ] Add `data-theme` attribute support in CSS (light/dark switching)
- [ ] Add theme toggle button in sidebar footer
- [ ] Respect `prefers-color-scheme` media query as default
- [ ] Persist preference in localStorage

## Acceptance Criteria

- [ ] Clicking "edit" on a tree value opens an inline form
- [ ] Submitting the form saves the value and shows updated display
- [ ] Validation runs inline (500ms debounce) and on full page
- [ ] Validation results grouped by severity with clear visual indicators
- [ ] "Save" persists the tree and shows success/error notification
- [ ] Components can be toggled (except mandatory ones)
- [ ] Enabling a component auto-enables its dependencies
- [ ] Disabling a component warns about dependents
- [ ] Keyboard shortcuts navigate between views
- [ ] `Ctrl+S` saves the tree from any view
- [ ] Dark/light theme toggles and persists
- [ ] All forms include CSRF tokens
- [ ] All HTMX requests include CSRF header
- [ ] `make test` and `make lint` pass
