package webui

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// requestID generates a unique request ID and adds it to the context and
// response header.
func requestIDMiddleware(next http.Handler) http.Handler {
	var counter atomic.Uint64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("%d-%d", time.Now().UnixNano(), counter.Add(1))
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// responseTimeMiddleware records the time taken to process a request and
// sets the X-Response-Time header. Because it wraps the handler, it uses
// a deferred-write wrapper to inject the header before the response body.
func responseTimeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseTimeWriter{ResponseWriter: w, start: start}
		next.ServeHTTP(rw, r)
		// If nothing was written yet, set it anyway for HEAD or empty responses.
		if !rw.headerWritten {
			w.Header().Set("X-Response-Time", time.Since(start).String())
		}
	})
}

type responseTimeWriter struct {
	http.ResponseWriter
	start         time.Time
	headerWritten bool
}

func (w *responseTimeWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.headerWritten = true
		w.Header().Set("X-Response-Time", time.Since(w.start).String())
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseTimeWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController.
func (w *responseTimeWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Flush implements http.Flusher by delegating to the wrapped ResponseWriter.
// Without this, SSE/streaming responses would never reach the client because
// this wrapper would otherwise hide the underlying Flusher.
func (w *responseTimeWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// etagMiddleware computes a weak ETag for bufferable GET responses and
// returns 304 Not Modified when the client's If-None-Match header
// matches. It skips text/event-stream (SSE) and text/html (which
// contain per-request CSP nonces) to avoid unnecessary buffering.
func etagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		rec := &etagRecorder{
			ResponseWriter: w,
			buf:            &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(rec, r)

		ct := rec.Header().Get("Content-Type")
		// Skip SSE, HTML (dynamic nonces), and error responses.
		if strings.HasPrefix(ct, "text/event-stream") ||
			strings.Contains(ct, "text/html") ||
			rec.statusCode >= 400 {
			w.WriteHeader(rec.statusCode)
			_, _ = w.Write(rec.buf.Bytes())
			return
		}

		body := rec.buf.Bytes()
		if len(body) == 0 {
			w.WriteHeader(rec.statusCode)
			return
		}

		hash := sha256.Sum256(body)
		etag := `W/"` + fmt.Sprintf("%x", hash[:8]) + `"`

		w.Header().Set("ETag", etag)

		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(rec.statusCode)
		_, _ = w.Write(body)
	})
}

// etagRecorder buffers the response body for ETag computation.
type etagRecorder struct {
	http.ResponseWriter
	buf         *bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func (r *etagRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.wroteHeader = true
}

func (r *etagRecorder) Write(b []byte) (int, error) {
	return r.buf.Write(b)
}

// loggingMiddleware logs method, path, status, and duration to stderr.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%-7s %-30s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Flush implements http.Flusher by delegating to the wrapped ResponseWriter.
// Without this, SSE/streaming responses would never reach the client because
// this wrapper would otherwise hide the underlying Flusher.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// recoveryMiddleware catches panics, logs the stack trace, and returns 500.
func recoveryMiddleware(engine *templateEngine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("PANIC: %v\n%s", rec, debug.Stack())
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					data := pageData{
						StatusCode: 500,
						StatusText: "Internal Server Error",
						Message:    "An unexpected error occurred.",
					}
					if err := engine.renderPage(w, "error", data); err != nil {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeadersMiddleware sets standard security headers and per-request
// CSP nonce.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := generateNonce()
		ctx := context.WithValue(r.Context(), nonceCtxKey, nonce)

		csp := fmt.Sprintf(
			"default-src 'none'; script-src 'nonce-%s'; style-src 'self' 'nonce-%s'; img-src 'self' data:; connect-src 'self'; font-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'",
			nonce, nonce,
		)

		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateNonce creates a random base64-encoded nonce for CSP.
func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// csrfMiddleware implements double-submit cookie CSRF protection.
type csrfMiddleware struct {
	secret []byte
	secure bool
}

func newCSRFMiddleware(secure bool) *csrfMiddleware {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return &csrfMiddleware{secret: secret, secure: secure}
}

// generateToken creates an HMAC-SHA256 CSRF token.
func (c *csrfMiddleware) generateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	raw := base64.StdEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(raw))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return raw + "." + sig
}

// validateToken verifies the HMAC signature of a CSRF token.
func (c *csrfMiddleware) validateToken(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	raw, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(raw))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

func (c *csrfMiddleware) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensure a CSRF cookie and token are present.
		cookie, err := r.Cookie("_csrf")
		var token string
		if err != nil || !c.validateToken(cookie.Value) {
			token = c.generateToken()
			http.SetCookie(w, &http.Cookie{
				Name:     "_csrf",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				Secure:   c.secure,
			})
		} else {
			token = cookie.Value
		}

		ctx := context.WithValue(r.Context(), csrfCtxKey, token)

		// Validate CSRF on state-changing methods.
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			submitted := r.Header.Get("X-CSRF-Token")
			if submitted == "" {
				submitted = r.FormValue("_csrf")
			}
			if submitted == "" || !c.validateToken(submitted) || submitted != token {
				http.Error(w, "CSRF token invalid", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// compressMiddleware applies gzip compression for text responses.
// It skips compression for text/event-stream (SSE) responses, which
// require unbuffered delivery for real-time streaming.
func compressMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz, err := gzip.NewWriterLevel(w, gzip.DefaultCompression)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w, gz: gz, headerWritten: false}
		defer gw.finish()
		next.ServeHTTP(gw, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz            *gzip.Writer
	headerWritten bool
	skipGzip      bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.headerWritten = true
		ct := w.Header().Get("Content-Type")
		if strings.HasPrefix(ct, "text/event-stream") {
			w.skipGzip = true
		} else {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	if w.skipGzip {
		return w.ResponseWriter.Write(b)
	}
	return w.gz.Write(b)
}

func (w *gzipResponseWriter) finish() {
	if !w.skipGzip {
		_ = w.gz.Close()
	}
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController.
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Flush implements http.Flusher for SSE and streaming responses.
func (w *gzipResponseWriter) Flush() {
	if w.skipGzip {
		if f, ok := w.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
	} else {
		_ = w.gz.Flush()
		if f, ok := w.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
	}
}
