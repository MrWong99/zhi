---
title: "fix: Web UI bugs and improvements from QS session"
type: fix
status: completed
date: 2026-02-21
---

# fix: Web UI bugs and improvements from QS session

## Overview

QS testing of the zhi Web UI (`zhi-home-server` workspace, Vault-backed store) revealed 2 critical bugs, 1 medium bug, 2 inconsistencies, and 5 improvement suggestions. The critical bugs break core functionality: login error handling and value persistence.

## Phase 1: Critical Bugs

### 1.1 Login failure renders raw HTML source

**Severity:** Critical
**Files:**
- `pkg/providers/ui/webui/auth_handlers.go:122-138`
- `pkg/providers/ui/webui/middleware.go:302-365` (gzip middleware)
- `pkg/providers/ui/webui/templates.go:150-167` (renderPage)

**Root cause analysis:**

In `handleLogin`, the failure path calls `w.WriteHeader(http.StatusUnauthorized)` (line 134) **before** `renderPage` sets `Content-Type: text/html` (line 164 of templates.go). While Go's `net/http` buffers headers until the first `Write()` call, the gzip middleware's `WriteHeader` override (middleware.go:331) reads `Content-Type` at the time `WriteHeader` is called — when it's still empty.

Server-side curl testing confirms the response headers are correct (Content-Type: text/html, status 401). This means the raw-HTML rendering the user observed is likely a **browser-side** issue:

1. **Most likely:** The gzip middleware sets `Content-Encoding: gzip` when `Content-Type` is empty (at WriteHeader time), but the actual Content-Type is set later. Some browsers may process headers in a way that the Content-Type arrives too late or gets lost after the gzip encoding header is already committed.
2. **Alternative:** A browser or extension quirk with 401 status + gzip + Content-Type header ordering.

**Fix:**

Move Content-Type setting before `WriteHeader` in the login handler, and restructure the gzip middleware to not commit encoding before Content-Type is known:

```go
// auth_handlers.go - handleLogin failure path
w.Header().Set("Cache-Control", "no-store")
w.Header().Set("Content-Type", "text/html; charset=utf-8") // Set BEFORE WriteHeader
w.WriteHeader(http.StatusUnauthorized)
if err := s.engine.renderPage(w, "login", data); err != nil {
    s.renderError(w, r, http.StatusInternalServerError, err.Error())
}
```

This same pattern exists in **all error rendering paths** — not just login:
- `renderError` in `routes.go:177-195` (404, 500 pages)
- `recoveryMiddleware` in `middleware.go:183` (panic recovery)

Consider adding a `renderPageWithStatus` helper to fix all callers at once:

```go
func (s *Server) renderPageWithStatus(w http.ResponseWriter, name string, code int, data any) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(code)
    if err := s.engine.renderPage(w, name, data); err != nil {
        http.Error(w, http.StatusText(code), code)
    }
}
```

**Acceptance criteria:**
- [x] Login with invalid token credentials shows styled login page with red error banner
- [x] Login with invalid userpass credentials shows styled login page with red error banner
- [x] Response Content-Type is `text/html; charset=utf-8` for all error responses (login, 404, 500)
- [x] Works with gzip compression enabled (browser's Accept-Encoding: gzip)
- [x] Verify in Firefox and Chromium
- [x] `renderError` (404/500 pages) also renders correctly with proper Content-Type

---

### 1.2 Inline editor save does not persist values

**Severity:** Critical
**Files:**
- `pkg/providers/ui/webui/editor.go:47-115` (handleSaveValue)
- `pkg/providers/ui/webui/templates/pages/tree.html:19-22` (valueChanged trigger)
- `internal/ui/driver.go:59-70` (UIController.LoadTree)
- `internal/ui/driver.go:97-108` (UIController.SetValue)
- `internal/core/engine.go:99-152` (Engine.LoadTree store merge)

**Root cause (confirmed):**

The save flow has a race between in-memory changes and the tree reload:

1. User edits value, clicks inline Save
2. `handleSaveValue` calls `s.ctrl.SetValue(ctx, path, newValue)` → updates config plugin AND cached `c.tree`
3. Response triggers `HX-Trigger: {"markUnsaved": true, "valueChanged": true}`
4. HTMX catches `valueChanged` on `#tree-content` (tree.html:19-22) → fires `GET /tree`
5. `handleTree` calls `s.ctrl.LoadTree(ctx)` → `engine.LoadTree(ctx)`:
   - Creates NEW tree from config plugin (has new value)
   - Loads stored values from Vault (has OLD value)
   - **Merges stored values into tree** (engine.go:144: `currentVal.Val = storedVal.Val`) → overwrites new value with old stored value
6. `c.tree` is replaced with the stale merged tree
7. User clicks "Save" → `SaveTree` persists the stale tree → old value is saved

The `valueChanged` trigger exists to update `ui.showIf` visibility (where changing one value may show/hide other fields). But the full `LoadTree` reload destroys unsaved in-memory changes.

**Fix — option A (recommended): Track dirty paths in UIController**

Add a dirty-path set to `UIController` that prevents store merge from overwriting unsaved changes:

```go
// internal/ui/driver.go

type UIController struct {
    engine         *core.Engine
    tree           *config.Tree
    dirtyPaths     map[string]bool // paths modified since last save
    marketplace    *marketplace.Client
    marketplaceErr error
}

func (c *UIController) SetValue(ctx context.Context, path string, value config.Value) error {
    if err := c.engine.SetValue(ctx, path, value); err != nil {
        return err
    }
    if c.tree != nil {
        if err := c.tree.Set(path, &value); err != nil {
            return err
        }
    }
    if c.dirtyPaths == nil {
        c.dirtyPaths = make(map[string]bool)
    }
    c.dirtyPaths[path] = true
    return nil
}

func (c *UIController) LoadTree(ctx context.Context) (*config.Tree, error) {
    tree, err := c.engine.LoadTree(ctx)
    if err != nil {
        c.handleAuthError(err)
        return nil, fmt.Errorf("loading tree: %w", err)
    }
    if err := c.engine.TransformForDisplay(ctx, tree); err != nil {
        return nil, fmt.Errorf("transforming tree: %w", err)
    }
    // Restore dirty (unsaved) values that would be overwritten by store merge.
    if c.tree != nil && len(c.dirtyPaths) > 0 {
        for path := range c.dirtyPaths {
            if dirtyVal, ok := c.tree.Get(path); ok {
                _ = tree.Set(path, &dirtyVal)
            }
        }
    }
    c.tree = tree
    return tree, nil
}

func (c *UIController) SaveTree(ctx context.Context) error {
    // ... existing save logic ...
    err := c.engine.SaveTree(ctx, c.tree)
    if err == nil {
        c.dirtyPaths = nil // Clear dirty set after successful save
    }
    return err
}
```

**Fix — option B (simpler but loses showIf updates): Remove valueChanged trigger**

Change `handleSaveValue` to NOT trigger `valueChanged`:

```go
// editor.go:110 - remove valueChanged from trigger
w.Header().Set("HX-Trigger", `{"markUnsaved": true}`)
```

This prevents the full tree reload. The inline save already returns the correct `value_display` fragment via HTMX swap. Downside: `ui.showIf` conditions won't update until manual page refresh.

**Recommendation:** Option A. It preserves showIf functionality while fixing persistence.

**Thread safety note:** `UIController.tree` is accessed by multiple concurrent HTTP handlers (LoadTree replaces it, SetValue mutates it, SaveTree reads it). Add a `sync.RWMutex` to protect access:

```go
type UIController struct {
    mu             sync.RWMutex
    engine         *core.Engine
    tree           *config.Tree
    dirtyPaths     map[string]bool
    marketplace    *marketplace.Client
    marketplaceErr error
}
```

Use `c.mu.Lock()` in SetValue, LoadTree, SaveTree. Use `c.mu.RLock()` in Tree, GetValue, Validate.

**Multi-value editing scenario:** Each `handleSaveValue` triggers `valueChanged` → `LoadTree`. With the dirty path tracking, editing value A then value B works correctly: both A and B are in `dirtyPaths`, so `LoadTree` restores both after the store merge. Without this fix, editing B would overwrite A with the stale stored value.

**Acceptance criteria:**
- [x] Edit a string value (e.g., timezone), click inline Save → value updates in tree view
- [x] Click top-level Save → value persists to Vault
- [x] Reload page → value shows the saved (new) value
- [x] Edit multiple values before saving → all values persist correctly
- [x] showIf conditions still update when dependent values change
- [x] Verify with `curl` against Vault that values are actually stored
- [x] No race conditions when rapidly editing values (concurrent HTTP requests)

---

## Phase 2: Medium Bug + Inconsistencies

### 2.1 Sidebar navigation visible on login page

**Severity:** Medium
**Files:**
- `pkg/providers/ui/webui/templates/layout.html:30-37`
- `pkg/providers/ui/webui/templates/sidebar.html`
- `pkg/providers/ui/webui/templates/topbar.html`

**Root cause:** `layout.html` unconditionally renders `{{template "sidebar" .}}` and `{{template "topbar" .}}`. The login page uses the same layout as authenticated pages.

**Fix:** Add a conditional check using `ActiveNav` (which is empty for the login page):

```html
<!-- layout.html -->
<div class="app">
    {{if .ActiveNav}}
      {{template "sidebar" .}}
      <div class="main">
        {{template "topbar" .}}
        <div class="content" id="main-content" role="main">
          {{block "content" .}}{{end}}
        </div>
      </div>
    {{else}}
      <div class="main main-full">
        <div class="content" id="main-content" role="main">
          {{block "content" .}}{{end}}
        </div>
      </div>
    {{end}}
</div>
```

Add `.main-full` CSS to center content without sidebar offset:

```css
/* layout.css */
.main-full {
    grid-column: 1 / -1;
}
```

**Acceptance criteria:**
- [x] Login page shows no sidebar and no topbar
- [x] Login page content is centered
- [x] After login, sidebar and topbar appear on all pages
- [x] Error pages (404, 500) also hide sidebar when not authenticated

---

### 2.2 Keyboard shortcuts don't match documentation

**Severity:** Low (inconsistency)
**Files:**
- `docs/user-guide/web-ui.md:114-124` (documentation — outdated)
- `pkg/providers/ui/webui/static/js/app.js:130-265` (actual implementation)
- `pkg/providers/ui/webui/templates/pages/shortcuts.html` (matches implementation)

**Root cause:** The implementation was changed from Ctrl-based shortcuts to vim-style `g+letter` sequences, but `docs/user-guide/web-ui.md` was not updated.

**Fix:** Update the docs to match the actual implementation:

```markdown
## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+S` | Save configuration |
| `/` | Focus search/filter |
| `?` | Show keyboard shortcuts |
| `Esc` | Close edit form / Cancel |
| `g t` | Go to Configuration Tree |
| `g c` | Go to Components |
| `g v` | Go to Validation |
| `g e` | Go to Export |
| `g a` | Go to Apply |
| `g m` | Go to Marketplace |
| `g p` | Go to Plugins |
```

**Acceptance criteria:**
- [x] `docs/user-guide/web-ui.md` keyboard shortcuts table matches `app.js` implementation
- [x] Matches `shortcuts.html` page in the UI

---

### 2.3 Marketplace error message is verbose and redundant

**Severity:** Low (inconsistency)
**Files:**
- `pkg/providers/ui/webui/marketplace_handlers.go:29`
- `internal/ui/driver.go:439` (double-wraps with "searching marketplace:")

**Root cause:** Double error wrapping. `SearchMarketplace` in `driver.go:439` wraps with `fmt.Errorf("searching marketplace: %w", err)`. Then `marketplace_handlers.go:29` prepends "Marketplace is currently unavailable: ". If the underlying error also contains "searching marketplace" from the marketplace client, it appears three times.

**Fix:**

1. In `marketplace_handlers.go`, show a user-friendly message and log the technical error:

```go
if err != nil {
    log.Printf("marketplace search error: %v", err)
    data := pageData{
        MarketplaceError: "Marketplace is currently unavailable. Please check your marketplace configuration.",
        // ... rest of fields
    }
}
```

2. Optionally, add a `<details>` element in the template for technical details if the user has dev mode enabled.

**Acceptance criteria:**
- [x] Marketplace error shows a clean, user-friendly message
- [x] No repeated "searching marketplace" in the visible error
- [x] Technical error is logged server-side for debugging

---

## Phase 3: Improvements

### 3.1 Login form should remember the selected authentication method

**Files:**
- `pkg/providers/ui/webui/auth_handlers.go:122-138` (handleLogin failure re-render)
- `pkg/providers/ui/webui/static/js/app.js:283-289` (auth method selector)

**Current behavior:** On login failure, `SelectedMethod` is preserved in the re-rendered page data (line 130). The auth method dropdown correctly shows the selected method after failure. However:
- The JS auth method selector does a full page navigation (`window.location.href = "/login?method=..."`) on change, losing any entered credential values.
- Non-secret credential field values (username, role, address) are not preserved on failure because the template inputs have no `value="..."` attributes and `pageData` doesn't carry previous credential values.

**Fix:**

1. The server-side already preserves `SelectedMethod` — verified in code.
2. Preserve non-secret credential values on login failure by adding them to `pageData` and populating the input `value` attributes (but NOT for `type="password"` fields — security risk).
3. Change the JS auth method selector to use HTMX instead of full page navigation:

```html
<!-- login.html: replace the select with HTMX-powered method switch -->
<select id="auth-method-select" name="method"
        hx-get="/login"
        hx-target=".login-form"
        hx-swap="outerHTML"
        hx-include="[name='method']">
```

Or keep the simple approach: use `hx-get="/login"` with query params to reload just the form fields for the selected method.

**Acceptance criteria:**
- [x] Changing auth method keeps the dropdown selection
- [x] Login failure preserves the selected auth method
- [x] Non-secret credential fields (username, role) retain their values on failure
- [x] Secret fields (password, token) are cleared on failure (security)
- [x] Credential fields update when switching methods

---

### 3.2 Validation warning formatting

**Files:**
- `pkg/providers/ui/webui/templates/fragments/validation_content.html:18`
- `pkg/providers/ui/webui/templates/fragments/value_form.html:89`
- `pkg/providers/ui/webui/templates.go` (template function map)

**Current behavior:** Validation messages are rendered as plain text. Shell commands and code references appear inline without formatting.

**Fix:** Add a template function that detects backtick-wrapped content and wraps it in `<code>` tags:

```go
// templates.go
"formatValidation": formatValidationMessage,

func formatValidationMessage(msg string) template.HTML {
    // Replace `...` with <code>...</code>
    re := regexp.MustCompile("`([^`]+)`")
    escaped := template.HTMLEscapeString(msg)
    formatted := re.ReplaceAllString(escaped, "<code>$1</code>")
    return template.HTML(formatted)
}
```

Update templates to use `{{formatValidation .Message}}` instead of `{{.Message}}`.

**Acceptance criteria:**
- [x] Backtick-wrapped content renders in `<code>` styled inline blocks
- [x] Plain text messages render normally
- [x] No XSS vectors introduced (HTML-escape before code wrapping)

---

### 3.3 Unsaved changes feedback improvement

**Files:**
- `pkg/providers/ui/webui/templates/topbar.html:14-22`
- `pkg/providers/ui/webui/editor.go:108-114`
- `pkg/providers/ui/webui/static/js/app.js:101-128`

**Current behavior:** The dual-save model (inline "Save" edits in memory, top-level "Save" persists) is unclear. The inline Save button and the persist Save button look similar.

**Fix:**

1. Rename the inline editor save button from "Save" to "Apply" (in `value_form.html`)
2. Disable the top-level Save button when there are no unsaved changes:

```javascript
// app.js
function markUnsaved() {
    if (unsavedIndicator) unsavedIndicator.classList.add("visible");
    var saveBtn = document.getElementById("save-tree-btn");
    if (saveBtn) saveBtn.disabled = false;
}
function markSaved() {
    if (unsavedIndicator) unsavedIndicator.classList.remove("visible");
    var saveBtn = document.getElementById("save-tree-btn");
    if (saveBtn) saveBtn.disabled = true;
}
```

3. Add initial `disabled` to save button in topbar when no unsaved changes.
4. Add `window.onbeforeunload` guard when unsaved changes exist:

```javascript
// app.js
var hasUnsaved = false;
function markUnsaved() {
    hasUnsaved = true;
    // ... existing code
}
function markSaved() {
    hasUnsaved = false;
    // ... existing code
}
window.addEventListener("beforeunload", function(e) {
    if (hasUnsaved) {
        e.preventDefault();
    }
});
```

**Acceptance criteria:**
- [x] Inline editor button says "Apply" instead of "Save"
- [x] Top-level Save button is disabled when no unsaved changes
- [x] Top-level Save button enables after an inline edit
- [x] After saving, button returns to disabled state
- [x] Closing the tab with unsaved changes triggers a browser warning

---

### 3.4 Empty string values display as "" instead of placeholder

**Files:**
- `pkg/providers/ui/webui/templates/fragments/tree_node.html:21`
- `pkg/providers/ui/webui/templates/fragments/value_display.html:14`

**Current behavior:** Empty strings show as `""` which is indistinguishable from a meaningful empty string.

**Fix:** Add conditional rendering in the templates:

```html
{{if eq .ValueType "string"}}
  {{if eq .DisplayValue ""}}
    <span class="value-string value-empty">(empty)</span>
  {{else}}
    <span class="value-string">"{{.DisplayValue}}"</span>
  {{end}}
{{end}}
```

Add `.value-empty` CSS for muted/italic styling.

**Acceptance criteria:**
- [x] Empty string values display as "(empty)" in muted style
- [x] Non-empty strings still display with quotes: `"value"`
- [x] The edit form still shows an empty input field for empty strings

---

### 3.5 Mobile tab bar truncation

**Files:**
- `pkg/providers/ui/webui/static/css/polish.css:183-237` (mobile breakpoint)
- `pkg/providers/ui/webui/templates/sidebar.html`

**Current behavior:** On mobile (< 768px), the sidebar becomes a horizontal scrollable tab bar. Labels are truncated when the total width exceeds the viewport.

**Fix:** On mobile, hide text labels and show only icons. The SVG icons are already present in each `<a>` tag:

```css
@media (max-width: 768px) {
    .sidebar-nav a span:not(.sr-only) {
        /* Hide text labels, keep SVGs */
    }
    .sidebar-nav a svg {
        margin: 0; /* Remove right margin when text is hidden */
    }
}
```

Since the current `<a>` tags contain inline SVG + text directly (no wrapper span around the text), add a `<span>` wrapper for the text label in `sidebar.html`, then hide it on mobile:

```html
<a href="/tree" class="{{activeNav . "tree"}}">
  {{icon "folder"}} <span class="nav-label">Configuration</span>
</a>
```

```css
@media (max-width: 768px) {
    .nav-label { display: none; }
}
```

**Acceptance criteria:**
- [x] Mobile viewport shows only icons in the tab bar
- [x] All 8 navigation items fit without horizontal scrolling on 320px width
- [x] Hover/focus states show the full label (tooltip)
- [x] Tablet and desktop viewports unchanged

---

## Implementation Order

1. **Bug 1.2** (value persistence) — highest impact, blocks primary workflow
2. **Bug 1.1** (login 401 raw HTML) — breaks authentication flow
3. **Bug 2.1** (sidebar on login) — visual, straightforward
4. **Bug 2.2** (shortcuts docs) — docs-only change
5. **Bug 2.3** (marketplace error) — simple string cleanup
6. **Improvement 3.3** (unsaved feedback) — UX clarity, pairs with Bug 1.2
7. **Improvement 3.1** (login method memory) — pairs with Bug 1.1
8. **Improvement 3.4** (empty strings) — template change
9. **Improvement 3.2** (validation formatting) — template + Go function
10. **Improvement 3.5** (mobile truncation) — CSS + template

## Testing Strategy

- All existing `webui_test.go` tests must continue passing
- Add test cases for:
  - Login failure returns correct Content-Type with 401 status
  - `SetValue` followed by `LoadTree` preserves the unsaved value
  - `SaveTree` after `SetValue` persists the new value
  - `SaveTree` clears dirty paths
  - Multiple `SetValue` calls before `SaveTree` all persist
- Manual verification using the running `zhi edit --ui webui` session at http://127.0.0.1:8080/
- Browser testing in Firefox and Chromium for Bug 1.1

## References

### Internal
- `pkg/providers/ui/webui/auth_handlers.go` — login/auth flow
- `pkg/providers/ui/webui/editor.go` — value editing, inline save
- `pkg/providers/ui/webui/save.go` — tree persistence
- `internal/ui/driver.go` — UIController (SetValue, LoadTree, SaveTree)
- `internal/core/engine.go:99-152` — Engine.LoadTree with store merge
- `pkg/providers/ui/webui/templates/layout.html` — page layout
- `pkg/providers/ui/webui/static/js/app.js` — keyboard shortcuts, notifications
- `docs/user-guide/web-ui.md` — user-facing documentation

### Institutional Knowledge
- Commit `68ea786`: `zhi set` had the same persistence issue (SetValue without SaveTree)
- Commits `750cc59`, `b5ef30b`: Prior HTMX `hx-select` inheritance bugs caused empty tree content
- Commit `c4eacb1`: Export handlers switched from fragments to HX-Trigger notifications
- Pattern: Use `HX-Trigger` events for transient feedback, not DOM fragments
