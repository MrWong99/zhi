package webui

import (
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
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

// recoveryMiddleware catches panics, logs the stack trace, and returns 500.
func recoveryMiddleware(engine *templateEngine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("PANIC: %v\n%s", rec, debug.Stack())
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
}

func newCSRFMiddleware() *csrfMiddleware {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return &csrfMiddleware{secret: secret}
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
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")

		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, writer: gz}, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController.
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
