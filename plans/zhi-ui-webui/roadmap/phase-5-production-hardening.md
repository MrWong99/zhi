# Phase 5: Production Hardening

**Goal**: Harden the web UI for production use with comprehensive testing, performance optimization, security auditing, and documentation.

At the end of this phase, the web UI is ready for release as a first-class zhi UI plugin.

## Deliverables

### 5.1 Comprehensive Test Suite

**Unit tests** for every handler:

```go
// Example: tree handler test
func TestTreeHandler_LoadsTreeAndRendersHTML(t *testing.T) {
    ctrl := &mockController{
        tree: testTree("database/host", "localhost", "database/port", 5432),
        components: []ui.ComponentInfo{{Name: "database", Enabled: true}},
    }
    srv := newTestServer(ctrl)

    resp := srv.GET("/tree")
    assert.Equal(t, 200, resp.Code)
    assert.Contains(t, resp.Body.String(), "database/host")
    assert.Contains(t, resp.Body.String(), "localhost")
}

func TestTreeHandler_HTMXPartialSwap(t *testing.T) {
    ctrl := &mockController{tree: testTree(...)}
    srv := newTestServer(ctrl)

    resp := srv.GETWithHeaders("/tree?filter=database", map[string]string{
        "HX-Request": "true",
    })
    assert.Equal(t, 200, resp.Code)
    // Should only contain the tree content fragment, not the full page
    assert.NotContains(t, resp.Body.String(), "<html>")
    assert.Contains(t, resp.Body.String(), "database/host")
}
```

**Test categories:**
- Handler tests (HTTP request → response)
- Template rendering tests (data → HTML correctness)
- Middleware tests (CSRF, security headers, compression)
- Integration tests (full server with mock controller)
- Browser automation tests (optional, using chromedp or similar)

**Tasks:**
- [ ] Create `testutil_test.go` with mock controller, test server helpers
- [ ] Write handler tests for every route (tree, editor, validation, save, components, export, apply, marketplace, plugins)
- [ ] Write HTMX partial swap tests for every fragment endpoint
- [ ] Write middleware tests (CSRF accept/reject, headers, compression)
- [ ] Write template rendering tests (verify correct HTML output)
- [ ] Write SSE streaming tests for apply handler
- [ ] Write integration tests with full server lifecycle (start → request → shutdown)
- [ ] Achieve 80%+ code coverage on handler and middleware packages
- [ ] Add benchmark tests for template rendering

### 5.2 Performance Optimization

**Template caching:**
```go
// Templates are parsed once at startup and cached
// In dev mode, templates are re-parsed on every request
type templateEngine struct {
    cache   map[string]*template.Template
    devMode bool
}
```

**Response optimization:**
- Gzip compression for all text responses
- `Cache-Control` headers for static assets (versioned with content hash)
- `ETag` headers for template responses
- Minimal CSS/JS payload (no unused styles)

**Server tuning:**
- Connection pool limits
- Read/write timeouts calibrated for SSE streaming
- Idle timeout for connection cleanup

**Tasks:**
- [ ] Implement template caching with dev-mode bypass
- [ ] Add content-hash versioning to static asset URLs (`/static/css/app.abc123.css`)
- [ ] Configure `Cache-Control: immutable, max-age=31536000` for hashed assets
- [ ] Add ETag headers for dynamic pages
- [ ] Profile template rendering latency, optimize hot paths
- [ ] Minimize CSS (remove unused rules)
- [ ] Minimize vendor JS (verify minified versions)
- [ ] Benchmark page load time (target: < 100ms TTFB for cached tree)
- [ ] Test with concurrent users (e.g., 10 concurrent connections)
- [ ] Add `X-Response-Time` header for monitoring

### 5.3 Security Audit

Systematic verification of all security controls from [SECURITY.md](../SECURITY.md):

**Tasks:**
- [ ] Verify CSRF protection on all POST/PUT/DELETE endpoints (automated test)
- [ ] Verify CSP headers present and correctly formed on all responses
- [ ] Verify no `template.HTML()` or `template.JS()` used with user data
- [ ] Verify path traversal prevention (fuzz test with `../`, encoded variants)
- [ ] Verify request body limits enforced
- [ ] Verify non-loopback address requires explicit opt-in and TLS
- [ ] Verify session cookie attributes (HttpOnly, SameSite, Secure)
- [ ] Verify error pages don't leak stack traces or internal paths
- [ ] Verify static file server cannot escape embedded FS
- [ ] Run `gosec` static analysis
- [ ] Run `govulncheck` for known vulnerabilities in dependencies
- [ ] Test with browser developer tools: verify no mixed content, no console errors

### 5.4 Error Handling & Resilience

**Graceful degradation:**
- If the controller is unreachable (gRPC error), show an informative error page
- If a single operation fails, show inline error without losing page state
- If SSE connection drops during apply, show reconnect option
- If marketplace is unavailable, show "marketplace unavailable" message

**Tasks:**
- [ ] Create error boundary pattern for handler methods
- [ ] Implement controller-unavailable error page
- [ ] Implement SSE reconnect logic (with backoff)
- [ ] Implement inline error display for HTMX requests (HX-Retarget to error zone)
- [ ] Log all errors with request ID for debugging
- [ ] Test error paths for every handler

### 5.5 Configuration Documentation

Document all configuration options and usage patterns:

**Tasks:**
- [ ] Write `docs/user-guide/web-ui.md` with setup and usage instructions
- [ ] Document environment variables (`ZHI_WEBUI_*`)
- [ ] Document keyboard shortcuts reference
- [ ] Document external plugin deployment
- [ ] Document built-in mode (build tag)
- [ ] Document TLS setup for remote access
- [ ] Add examples to workspace configuration docs

### 5.6 CI Integration

**Tasks:**
- [ ] Add `make build-webui` to CI build matrix
- [ ] Add webui tests to CI test suite
- [ ] Add TypeScript compilation check to CI
- [ ] Add CSS lint (stylelint, optional) to CI
- [ ] Add HTML validation for rendered templates (optional)
- [ ] Cross-compile webui binary for linux/darwin × amd64/arm64

### 5.7 Developer Experience

**Dev mode features:**
- Live template reloading (read from disk instead of embed)
- Verbose logging with request/response details
- Auto-reload on file change (optional, via separate file watcher or browser extension)

**Tasks:**
- [ ] Implement `ZHI_WEBUI_DEV=true` mode for live template reloading
- [ ] Add verbose request logging in dev mode
- [ ] Add template syntax validation at startup (fail fast on parse errors)
- [ ] Add handler registration logging (list all routes at startup)
- [ ] Document dev workflow in a CONTRIBUTING section

### 5.8 Release Preparation

**Tasks:**
- [ ] Add `zhi-ui-webui` to GoReleaser configuration for distribution
- [ ] Create plugin manifest (`zhi-plugin.yaml`) for marketplace listing
- [ ] Test external plugin mode end-to-end (build binary, install to `~/.zhi/plugins/`, run)
- [ ] Test built-in mode end-to-end (build with tag, select via `zhi ui webui`)
- [ ] Write changelog for initial release
- [ ] Tag v0.1.0

## Acceptance Criteria

- [ ] 80%+ code coverage on handlers and middleware
- [ ] All security controls verified with automated tests
- [ ] `gosec` and `govulncheck` pass with no high-severity findings
- [ ] Page TTFB < 100ms for tree view with 100 config paths
- [ ] Static assets cached with content-hash versioning
- [ ] Graceful error handling for all failure modes
- [ ] User documentation complete
- [ ] CI pipeline green with webui tests
- [ ] External plugin mode tested end-to-end
- [ ] Built-in mode tested end-to-end
- [ ] `make check` passes (fmt + vet + lint + test)
