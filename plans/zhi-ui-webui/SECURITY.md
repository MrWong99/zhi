# Security Model

## Threat Model

`zhi-ui-webui` serves a local web interface for configuration management. The primary threats are:

| Threat | Vector | Impact |
|--------|--------|--------|
| **Cross-site request forgery** | Malicious site triggers mutations via user's browser | Unauthorized config changes, data exfiltration |
| **Cross-site scripting** | Injected config values rendered as HTML | Session hijack, data theft |
| **Unauthorized access** | Network exposure beyond localhost | Full config access by unauthorized parties |
| **Man-in-the-middle** | Unencrypted HTTP on non-loopback | Credential/config interception |
| **Server-side template injection** | Crafted config values breaking template boundaries | Code execution on server |
| **Path traversal** | Malicious path values in URL parameters | File system access |
| **Denial of service** | Resource exhaustion via large requests or concurrent connections | UI unavailability |

## Security Controls

### 1. Localhost Binding (Default)

By default, the server binds to `127.0.0.1:8080` (IPv4 loopback only). This prevents network exposure.

```go
const defaultAddr = "127.0.0.1:8080"
```

If the user configures a non-loopback address, the server:
1. Prints a prominent warning to stderr
2. Requires explicit confirmation via `ZHI_WEBUI_ALLOW_REMOTE=true`
3. Enforces TLS if a non-loopback address is used (fails to start without cert/key)

### 2. CSRF Protection

All state-changing requests (POST, PUT, DELETE) require a valid CSRF token.

**Implementation:**
- Tokens are generated per-session using HMAC-SHA256 with a random secret
- The secret is auto-generated at startup (32 bytes from `crypto/rand`)
- Tokens are embedded in forms via a template helper: `{{ csrfField }}`
- HTMX requests include the token via `hx-headers` configured in the base layout
- Token validation happens in middleware before any handler executes

```go
// Template integration
<meta name="csrf-token" content="{{ csrfToken }}">

// HTMX global config (in layout.html)
<body hx-headers='{"X-CSRF-Token": "{{ csrfToken }}"}'>
```

**Double-submit cookie pattern:**
- Token stored in a `__Host-csrf` cookie (SameSite=Strict, HttpOnly, Secure if TLS)
- Token also submitted in `X-CSRF-Token` header or `_csrf` form field
- Server verifies both match

### 3. Content Security Policy (CSP)

A strict CSP prevents XSS even if a template escaping bug exists:

```
Content-Security-Policy:
    default-src 'none';
    script-src 'nonce-{random}';
    style-src 'self' 'nonce-{random}';
    img-src 'self' data:;
    connect-src 'self';
    font-src 'self';
    form-action 'self';
    frame-ancestors 'none';
    base-uri 'none';
    upgrade-insecure-requests;
```

- **Nonce-based script loading**: each response includes a unique nonce; only scripts with matching nonce execute
- **No `unsafe-inline`**: all styles use nonces or external sheets
- **No `unsafe-eval`**: HTMX and Alpine.js work without `eval()`
- **`connect-src 'self'`**: prevents data exfiltration to external origins
- **`frame-ancestors 'none'`**: prevents clickjacking

**Nonce propagation:**
```go
func cspMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        nonce := generateNonce() // 16 bytes, base64
        ctx := context.WithValue(r.Context(), nonceKey, nonce)
        w.Header().Set("Content-Security-Policy", buildCSP(nonce))
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// In templates:
<script nonce="{{ nonce }}">...</script>
```

### 4. Additional Security Headers

```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
        if isTLS(r) {
            w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
        }
        next.ServeHTTP(w, r)
    })
}
```

### 5. Template Security

Go `html/template` automatically escapes values in HTML context. Additional precautions:

- **All config values are escaped by default** -- Go `html/template` handles this
- **No raw HTML rendering** -- never use `template.HTML()` for user-provided data
- **Path validation** -- URL path parameters are validated against the zhi path regex `[a-z][a-z0-9._-]*[a-z0-9]` before use
- **JSON encoding** -- values displayed as JSON use `json.Marshal` (which escapes `<`, `>`, `&`)

### 6. Request Limits

```go
// Middleware: limit request body size
func maxBodySize(limit int64) func(http.Handler) http.Handler {
    // Wraps r.Body with http.MaxBytesReader
    // Default: 1MB for regular requests, 10MB for file uploads
}

// Read timeout: 10 seconds
// Write timeout: 30 seconds (longer for SSE streaming)
// Idle timeout: 120 seconds
// Max header size: 8KB
```

### 7. Path Traversal Prevention

All path parameters extracted from URLs are validated:

```go
var validPathSegment = regexp.MustCompile(`^[a-z][a-z0-9._-]*[a-z0-9]$`)

func validateConfigPath(path string) error {
    segments := strings.Split(path, "/")
    for _, seg := range segments {
        if seg == ".." || seg == "." || !validPathSegment.MatchString(seg) {
            return fmt.Errorf("invalid path segment: %q", seg)
        }
    }
    return nil
}
```

### 8. Rate Limiting

Mutation endpoints are rate-limited to prevent accidental rapid-fire submissions:

```go
// Per-endpoint rate limits (token bucket)
// POST /tree/values/*:   10 req/sec
// POST /tree/save:        2 req/sec
// POST /apply/run:        1 req/sec
// POST /marketplace/*:    5 req/sec
```

Rate limiting is lenient (these are local tools, not public APIs) but prevents runaway HTMX retry loops.

### 9. Session Management

Since this is a local tool (not a multi-user web app), sessions are simple:

- A session cookie is set on first visit with a random ID
- Sessions are stored in-memory (no persistence needed)
- Session stores: CSRF token, theme preference, tree filter state
- Cookie attributes: `HttpOnly`, `SameSite=Strict`, `Secure` (if TLS), `Path=/`
- No authentication required by default (localhost access is the auth boundary)

### 10. TLS Support

For non-localhost deployments or security-conscious local use:

```go
// Auto-generate self-signed cert for local development
if config.TLSCert == "" && config.TLSKey == "" && config.Addr == defaultAddr {
    // Option: generate ephemeral self-signed cert
    // Prints fingerprint to stderr for user verification
}

// Production: user-provided cert and key
srv.TLSConfig = &tls.Config{
    MinVersion: tls.VersionTLS12,
    CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
    CipherSuites: []uint16{
        tls.TLS_AES_128_GCM_SHA256,
        tls.TLS_AES_256_GCM_SHA384,
        tls.TLS_CHACHA20_POLY1305_SHA256,
    },
}
```

## Security Checklist for Development

- [ ] All POST/PUT/DELETE handlers check CSRF token
- [ ] No `template.HTML()` used with user-provided data
- [ ] CSP nonce propagated to all `<script>` and `<style>` tags
- [ ] Path parameters validated before use in controller calls
- [ ] Error messages don't leak internal paths or stack traces
- [ ] Static file server restricted to embedded FS (no directory traversal)
- [ ] Graceful shutdown closes connections within timeout
- [ ] Non-loopback binding requires explicit opt-in and TLS
- [ ] Rate limits tested under concurrent requests
- [ ] SSE endpoint has connection timeout to prevent resource leaks
