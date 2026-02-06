# Phase 3: Export & Apply

**Goal**: Users can preview and execute exports, and run apply commands with real-time streaming output in the browser.

At the end of this phase, the full configuration lifecycle is available: browse → edit → validate → save → export → apply.

## Deliverables

### 3.1 Export Template List

**Page: `pages/export.html`**
- List all configured export templates
- Show built-in format options (JSON, YAML, TOML, dotenv)
- Each template shows: name, format, output path, preview button, export button

**Handler: `GET /export`**
- Call `ctrl.ExportTemplates(ctx)` to get configured templates
- Add built-in format options
- Render template list

**Template layout:**
```
┌─────────────────────────────────────────────────────────────┐
│  Export                                                      │
│  ─────────────────────────────────────────────────────────── │
│                                                              │
│  Templates                                                   │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  docker-compose           YAML template                 │ │
│  │  Output: ./docker-compose.override.yml                  │ │
│  │  [Preview]  [Export]                                     │ │
│  ├─────────────────────────────────────────────────────────┤ │
│  │  env-file                 dotenv format                 │ │
│  │  Output: ./.env    Prefix: app/env                      │ │
│  │  [Preview]  [Export]                                     │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                              │
│  Quick Export (built-in formats)                             │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐             │
│  │  JSON  │ │  YAML  │ │  TOML  │ │  dotenv  │             │
│  └────────┘ └────────┘ └────────┘ └──────────┘             │
└─────────────────────────────────────────────────────────────┘
```

**Tasks:**
- [ ] Create `handlers/export.go` with template list handler
- [ ] Create `pages/export.html` with template list and quick export buttons
- [ ] Style export template cards with format indicators

### 3.2 Export Preview

**Handler: `POST /export/preview`** (HTMX partial)
- Parse export request (template name, format, prefix, dry-run flag)
- Call `ctrl.Export(ctx, req)` with DryRun=true
- Return rendered preview in a code block with syntax highlighting

**Preview panel:**
```html
<div class="export-preview" id="export-preview">
    <div class="preview-header">
        <span class="preview-title">Preview: {{ .Name }}</span>
        <button class="btn btn-sm btn-ghost"
                hx-post="/export"
                hx-include="[name='template'], [name='format']"
                hx-target="#export-result">
            Export to file
        </button>
    </div>
    <pre class="code-block"><code>{{ .Content }}</code></pre>
</div>
```

**Tasks:**
- [ ] Implement preview handler returning dry-run export content
- [ ] Create `fragments/export_preview.html` with code block rendering
- [ ] Add basic syntax highlighting via CSS classes (key/value coloring for YAML, JSON, TOML)
- [ ] Support preview for custom templates and built-in formats

### 3.3 Export Execution

**Handler: `POST /export`**
- Parse export request from form
- Call `ctrl.Export(ctx, req)` with DryRun=false
- Return result fragment with success/error status

**Result fragment:**
```html
<div class="export-result {{ if .Error }}error{{ else }}success{{ end }}">
    {{ if .Error }}
        <span class="icon">✗</span>
        <span>Export failed: {{ .Error }}</span>
    {{ else }}
        <span class="icon">✓</span>
        <span>Exported <strong>{{ .Name }}</strong> to <code>{{ .OutputPath }}</code></span>
    {{ end }}
</div>
```

**Tasks:**
- [ ] Implement export execution handler
- [ ] Create `fragments/export_result.html` for success/error display
- [ ] Add "Export All" button that exports all configured templates
- [ ] Show notification on export completion
- [ ] Write tests for export flow

### 3.4 Apply Command Runner

This is the most complex UI feature: running external commands with real-time streamed output.

**Page: `pages/apply.html`**
- Show configured apply targets
- "Run" button to start execution
- Real-time scrolling output terminal
- Exit code display on completion

**Architecture:**

```
Browser                          Server
  │                                │
  │  POST /apply/run               │
  │  (target=docker-compose)       │
  │ ─────────────────────────────► │
  │                                │ ctrl.Apply(ctx, target, handler)
  │  SSE: event: output            │     │
  │  data: {"line":"..."}          │     │ handler(event) called per line
  │ ◄──────────────────────────────│ ◄───┘
  │  SSE: event: output            │
  │  data: {"line":"..."}          │
  │ ◄──────────────────────────────│
  │  SSE: event: done              │
  │  data: {"exit_code":0}         │
  │ ◄──────────────────────────────│
  │                                │
  │  EventSource closes            │
```

**Handler: `POST /apply/run`** (SSE stream)
```go
func (h *applyHandler) handleRun(w http.ResponseWriter, r *http.Request) {
    target := r.FormValue("target")

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher := w.(http.Flusher)

    handler := func(event ui.ApplyEvent) {
        data, _ := json.Marshal(event)
        fmt.Fprintf(w, "event: output\ndata: %s\n\n", data)
        flusher.Flush()
    }

    result, err := h.ctrl.Apply(r.Context(), target, handler)

    // Send completion event
    doneData, _ := json.Marshal(map[string]any{
        "exit_code": result.ExitCode,
        "error":     errStr(err, result),
    })
    fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
    flusher.Flush()
}
```

**Client-side: `static/js/app/apply-stream.ts`**

The TypeScript module manages the SSE connection and renders output:

```typescript
class ApplyStream {
    private source: EventSource | null = null;
    private output: HTMLElement;

    start(target: string): void {
        this.output.innerHTML = '';
        this.setStatus('running');

        this.source = new EventSource(`/apply/run?target=${encodeURIComponent(target)}`);

        this.source.addEventListener('output', (e: MessageEvent) => {
            const event = JSON.parse(e.data);
            this.appendLine(event.line, event.stream);
            this.autoScroll();
        });

        this.source.addEventListener('done', (e: MessageEvent) => {
            const result = JSON.parse(e.data);
            this.setStatus(result.exit_code === 0 ? 'success' : 'failed');
            this.showExitCode(result.exit_code, result.error);
            this.source?.close();
        });

        this.source.addEventListener('error', () => {
            this.setStatus('error');
            this.source?.close();
        });
    }

    stop(): void {
        this.source?.close();
        this.setStatus('cancelled');
    }
}
```

**Terminal-style output display:**
```html
<div class="apply-terminal">
    <div class="terminal-header">
        <span class="terminal-title">Apply: {{ .Target }}</span>
        <span class="terminal-status" id="apply-status">Ready</span>
    </div>
    <div class="terminal-output" id="apply-output">
        <!-- Lines appended here by TypeScript -->
    </div>
    <div class="terminal-footer">
        <span id="apply-exit-code"></span>
    </div>
</div>
```

**Styling:**
- Dark background terminal aesthetic (even in light theme)
- Monospace font
- stdout in white/light gray, stderr in red/amber
- Auto-scroll to bottom, but stop auto-scroll if user scrolls up
- Status indicator: running (blue pulse), success (green), failed (red)

**Tasks:**
- [ ] Create `handlers/apply.go` with SSE streaming handler
- [ ] Create `pages/apply.html` with target selector and terminal output
- [ ] Create `static/js/app/apply-stream.ts` with EventSource consumer
- [ ] Style terminal output with monospace dark theme
- [ ] Implement stdout/stderr color differentiation
- [ ] Implement auto-scroll with "user scrolled up" detection
- [ ] Show status indicator (running/success/failed/cancelled)
- [ ] Show exit code on completion
- [ ] Implement cancel button (closes EventSource, could signal context cancellation)
- [ ] Handle connection errors gracefully (reconnect or show error)
- [ ] Add timeout for long-running applies (configurable)
- [ ] Write tests for SSE output format

### 3.5 Export Before Apply

If the workspace is configured with `pre-export: true`, the UI should run exports before apply:

**Tasks:**
- [ ] Detect `pre-export` configuration
- [ ] Show "Export & Apply" combined action when pre-export is enabled
- [ ] Execute exports before starting the apply stream
- [ ] Show export results before apply output
- [ ] Handle export failures (abort apply, show error)

### 3.6 Navigation Shortcuts (Phase 3 additions)

**New shortcuts:**
- `g e` -- navigate to Export
- `g a` -- navigate to Apply

**Tasks:**
- [ ] Register new shortcuts in the ShortcutManager
- [ ] Update shortcuts overlay page

## Acceptance Criteria

- [ ] Export templates listed with format indicators
- [ ] Preview shows rendered content in a code block
- [ ] Export writes to configured output path and shows success
- [ ] "Export All" exports all configured templates
- [ ] Apply streams output in real-time to a terminal-style display
- [ ] stdout and stderr distinguished by color
- [ ] Exit code shown on completion
- [ ] Cancel button stops the apply
- [ ] Pre-export runs before apply when configured
- [ ] SSE connection handles errors gracefully
- [ ] All forms include CSRF tokens
- [ ] `make test` and `make lint` pass
