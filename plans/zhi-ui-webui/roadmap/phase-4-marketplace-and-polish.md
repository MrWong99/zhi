# Phase 4: Marketplace & Polish

**Goal**: Integrate marketplace browsing and plugin management, polish the design with animations and transitions, improve accessibility, and add responsive behavior.

At the end of this phase, the web UI is feature-complete and provides a polished experience across devices.

## Deliverables

### 4.1 Marketplace Browser

**Page: `pages/marketplace.html`**
- Search bar with query input
- Filter chips: plugin type (config, transform, store, ui, workspace)
- Sort dropdown (relevance, downloads, rating, recently updated)
- Verified-only toggle
- Paginated results grid

**Handler: `GET /marketplace`**
- Parse query parameters: `q`, `type`, `sort`, `verified`, `page`
- Call `ctrl.SearchMarketplace(ctx, query)`
- Render results

**Handler: `GET /marketplace?...`** (HTMX partial for search/filter)
- Same logic, returns `fragments/marketplace_grid.html`

**Search interaction:**
```html
<form hx-get="/marketplace"
      hx-target="#marketplace-results"
      hx-swap="innerHTML"
      hx-push-url="true"
      hx-trigger="submit, input[name=q] changed delay:400ms">
    <input type="search" name="q" value="{{ .Query }}" placeholder="Search plugins...">

    <div class="filter-chips">
        {{ range .Types }}
            <label class="chip {{ if eq . $.ActiveType }}active{{ end }}">
                <input type="radio" name="type" value="{{ . }}"
                       {{ if eq . $.ActiveType }}checked{{ end }}>
                {{ . }}
            </label>
        {{ end }}
    </div>

    <select name="sort" hx-trigger="change">
        <option value="relevance">Relevance</option>
        <option value="downloads">Downloads</option>
        <option value="rating">Rating</option>
        <option value="updated">Updated</option>
    </select>
</form>
```

**Result card: `fragments/marketplace_card.html`**
```
┌─────────────────────────────────────────┐
│  ✓ zhi-store-vault                      │
│    by hashicorp          [store] [v1.2] │
│                                         │
│    HashiCorp Vault KV v2 backend        │
│    for secure secret storage            │
│                                         │
│    ★★★★☆ (42)  ↓ 1.2k  [Install]       │
└─────────────────────────────────────────┘
```

**Tasks:**
- [ ] Create `handlers/marketplace.go` with search and detail handlers
- [ ] Create `pages/marketplace.html` with search, filters, and result grid
- [ ] Create `fragments/marketplace_card.html` for individual result cards
- [ ] Implement type filter chips with HTMX-driven updates
- [ ] Implement sort dropdown
- [ ] Implement verified-only toggle
- [ ] Implement pagination
- [ ] Show installed/update-available badges on results
- [ ] Handle empty results gracefully
- [ ] Handle marketplace unavailable gracefully (show message)
- [ ] Write tests for search query construction

### 4.2 Plugin Detail Page

**Page: `pages/plugin_detail.html`**
- Full plugin information: name, publisher, description, long description
- Version history table
- Dependencies list
- Rating display and rating form
- Install/Update/Uninstall actions
- Platform compatibility badges

**Handler: `GET /marketplace/{publisher}/{name}`**
- Call `ctrl.GetMarketplaceDetail(ctx, publisher, name)`
- Render detail page

**Rating form:**
```html
<form hx-post="/marketplace/{{ .Publisher }}/{{ .Name }}/rate"
      hx-target="#rating-section"
      hx-swap="outerHTML">
    {{ csrfField }}
    <fieldset class="star-rating">
        {{ range $i := seq 1 5 }}
            <input type="radio" name="score" value="{{ $i }}" id="star-{{ $i }}">
            <label for="star-{{ $i }}">★</label>
        {{ end }}
    </fieldset>
    <textarea name="comment" placeholder="Optional review..."></textarea>
    <button type="submit" class="btn btn-primary">Submit Rating</button>
</form>
```

**Tasks:**
- [ ] Create `pages/plugin_detail.html` with full detail layout
- [ ] Render version history table
- [ ] Render dependency list
- [ ] Implement star rating display (CSS-only stars)
- [ ] Implement rating form
- [ ] Show platform compatibility badges
- [ ] Handle install/update/uninstall actions (see 4.3)

### 4.3 Plugin Install / Update / Uninstall

**Handler: `POST /plugins/install`**
- Parse plugin reference from form
- Call `ctrl.InstallPlugin(ctx, ref)`
- On success: return success notification
- On error: return error notification

**Handler: `POST /plugins/{name}/uninstall`**
- Confirmation dialog (Alpine.js `<dialog>`)
- Call `ctrl.UninstallPlugin(ctx, name, type)`
- On success: remove from list or show notification

**Handler: `POST /plugins/{name}/update`**
- Call `ctrl.UpdatePlugin(ctx, name, version)`
- On success: show updated version

**Tasks:**
- [ ] Implement install handler with success/error feedback
- [ ] Implement uninstall handler with confirmation dialog
- [ ] Implement update handler
- [ ] Show install progress indicator
- [ ] Handle installation errors (network, verification failure)

### 4.4 Installed Plugins Page

**Page: `pages/plugins.html`**
- List all installed plugins with version, source, install date
- Show update availability with "Update" button
- "Uninstall" button with confirmation
- "Check for Updates" button

**Handler: `GET /plugins`**
- Call `ctrl.ListInstalledPlugins(ctx)`
- Call `ctrl.CheckUpdates(ctx)` (merged into display)

**Tasks:**
- [ ] Create `handlers/plugins.go` with list and action handlers
- [ ] Create `pages/plugins.html` with installed plugin list
- [ ] Show update badges for plugins with available updates
- [ ] Implement "Update All" button
- [ ] Show last-checked timestamp
- [ ] Write tests for plugin management flows

### 4.5 UI Polish: Transitions & Animations

Add subtle transitions for a polished feel without impacting performance:

**CSS transitions:**
```css
/* HTMX swap transitions */
.htmx-swapping { opacity: 0; transition: opacity 150ms ease-out; }
.htmx-settling { opacity: 1; transition: opacity 150ms ease-in; }

/* Tree expand/collapse */
details[open] > summary::before { transform: rotate(90deg); }
summary::before { transition: transform 150ms ease; }

/* Toast notifications */
.notification-enter { transform: translateX(100%); opacity: 0; }
.notification-active { transform: translateX(0); opacity: 1; transition: all 300ms ease; }
.notification-exit { transform: translateX(100%); opacity: 0; transition: all 200ms ease; }

/* Sidebar hover */
.nav-item { transition: background-color 100ms ease; }

/* Button press */
.btn:active { transform: scale(0.97); }

/* Focus ring */
:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
```

**Tasks:**
- [ ] Add HTMX swap transitions (fade in/out)
- [ ] Add tree expand/collapse arrow animation
- [ ] Add toast notification slide-in/out
- [ ] Add button press feedback
- [ ] Add focus ring styles for keyboard navigation
- [ ] Add sidebar hover effects
- [ ] Add loading spinner for HTMX requests (`htmx:beforeRequest` / `htmx:afterRequest`)
- [ ] Ensure `prefers-reduced-motion` disables all animations

### 4.6 Responsive Design

**Tasks:**
- [ ] Implement responsive breakpoints (desktop > 1024, tablet 768-1024, mobile < 768)
- [ ] Sidebar collapses to icon-only on tablet
- [ ] Sidebar becomes hamburger menu on mobile
- [ ] Tree view switches to single-column on mobile
- [ ] Component cards stack vertically on mobile
- [ ] Export preview full-width on mobile
- [ ] Apply terminal full-width on all sizes
- [ ] Test on common viewport sizes (1440, 1024, 768, 375)

### 4.7 Accessibility

**Tasks:**
- [ ] All interactive elements have visible focus indicators
- [ ] Tree navigation works with arrow keys
- [ ] Modals trap focus and close on Escape
- [ ] ARIA roles on dynamic regions (`role="alert"` for notifications, `role="tree"` for tree view)
- [ ] `aria-live="polite"` on HTMX swap targets for screen reader announcements
- [ ] Color contrast ratios meet WCAG AA (4.5:1 for text, 3:1 for UI elements)
- [ ] All form inputs have associated labels
- [ ] Error messages linked to inputs via `aria-describedby`
- [ ] Skip-to-content link for keyboard users
- [ ] Test with screen reader (basic VoiceOver/NVDA pass)

### 4.8 Marketplace Navigation Shortcuts

**New shortcuts:**
- `g m` -- navigate to Marketplace
- `g p` -- navigate to Installed Plugins

**Tasks:**
- [ ] Register marketplace shortcuts
- [ ] Update shortcuts overlay

## Acceptance Criteria

- [ ] Marketplace search returns and displays results
- [ ] Filters and sort work via HTMX without page reload
- [ ] Plugin detail page shows full information
- [ ] Install/Uninstall/Update actions work with feedback
- [ ] Rating form submits successfully
- [ ] Installed plugins page lists all plugins with update info
- [ ] CSS transitions smooth and subtle
- [ ] `prefers-reduced-motion` respected
- [ ] Responsive on desktop, tablet, and mobile viewports
- [ ] WCAG AA color contrast met
- [ ] Tree navigable by keyboard
- [ ] All forms accessible with screen reader
- [ ] `make test` and `make lint` pass
