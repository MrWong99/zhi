package webui

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// authMethodData is the view model for an auth method on the login page.
type authMethodData struct {
	Type        string
	Description string
	Fields      []authFieldData
	Interactive bool
}

// authFieldData is the view model for a single credential field.
type authFieldData struct {
	Name        string
	Description string
	Required    bool
	Secret      bool
}

// toAuthMethodData converts controller auth methods to view models.
func toAuthMethodData(methods []ui.StoreAuthMethod) []authMethodData {
	out := make([]authMethodData, 0, len(methods))
	for _, m := range methods {
		fields := make([]authFieldData, 0, len(m.Fields))
		for _, f := range m.Fields {
			fields = append(fields, authFieldData{
				Name:        f.Name,
				Description: f.Description,
				Required:    f.Required,
				Secret:      f.Secret,
			})
		}
		out = append(out, authMethodData{
			Type:        m.Type,
			Description: m.Description,
			Fields:      fields,
			Interactive: m.Interactive,
		})
	}
	return out
}

// handleLoginPage renders the login page.
// GET /login
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// If already authenticated or no auth required, redirect to /tree.
	session, err := s.ctrl.StoreAuthStatus(ctx)
	if err == nil && (session.Status == ui.StoreSessionAuthenticated || session.Status == ui.StoreSessionNone) {
		http.Redirect(w, r, "/tree", http.StatusSeeOther)
		return
	}

	methods, err := s.ctrl.StoreAuthMethods(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load auth methods: "+err.Error())
		return
	}

	// No auth methods means the store doesn't require auth — skip login.
	if len(methods) == 0 {
		http.Redirect(w, r, "/tree", http.StatusSeeOther)
		return
	}

	selected := r.URL.Query().Get("method")
	if selected == "" && len(methods) > 0 {
		selected = methods[0].Type
	}

	wsName, _ := s.ctrl.WorkspaceName(ctx)
	data := pageData{
		WorkspaceName:  wsName,
		Nonce:          nonceFromCtx(ctx),
		CSRFToken:      csrfFromCtx(ctx),
		AuthMethods:    toAuthMethodData(methods),
		SelectedMethod: selected,
	}

	w.Header().Set("Cache-Control", "no-store")
	if err := s.engine.renderPage(w, "login", data); err != nil {
		s.renderError(w, r, http.StatusInternalServerError, err.Error())
	}
}

// handleLogin processes login form submission.
// POST /login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	method := r.FormValue("method")
	if method == "" {
		s.renderError(w, r, http.StatusBadRequest, "No auth method specified")
		return
	}

	// Collect all credential fields from the form.
	methods, err := s.ctrl.StoreAuthMethods(ctx)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load auth methods: "+err.Error())
		return
	}

	credentials := make(map[string]string)
	for _, m := range methods {
		if m.Type == method {
			for _, f := range m.Fields {
				credentials[f.Name] = r.FormValue("cred_" + f.Name)
			}
			break
		}
	}

	_, loginErr := s.ctrl.StoreLogin(ctx, method, credentials)
	if loginErr != nil {
		// Preserve non-secret credential values for re-rendering.
		prevCreds := make(map[string]string)
		for _, m := range methods {
			if m.Type == method {
				for _, f := range m.Fields {
					if !f.Secret {
						if v := r.FormValue("cred_" + f.Name); v != "" {
							prevCreds[f.Name] = v
						}
					}
				}
				break
			}
		}

		// Re-render login page with error.
		wsName, _ := s.ctrl.WorkspaceName(ctx)
		data := pageData{
			WorkspaceName:       wsName,
			Nonce:               nonceFromCtx(ctx),
			CSRFToken:           csrfFromCtx(ctx),
			AuthMethods:         toAuthMethodData(methods),
			SelectedMethod:      method,
			LoginError:          loginErr.Error(),
			PreviousCredentials: prevCreds,
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		if err := s.engine.renderPage(w, "login", data); err != nil {
			s.renderError(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}

	http.Redirect(w, r, "/tree", http.StatusSeeOther)
}

// handleLoginInteractive starts an interactive (browser-based) auth flow.
// POST /login/interactive
func (s *Server) handleLoginInteractive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"invalid form data"}`, http.StatusBadRequest)
		return
	}

	method := r.FormValue("method")
	if method == "" {
		http.Error(w, `{"error":"no auth method specified"}`, http.StatusBadRequest)
		return
	}

	params := make(map[string]string)
	// Collect method-specific parameters from the form.
	methods, err := s.ctrl.StoreAuthMethods(ctx)
	if err == nil {
		for _, m := range methods {
			if m.Type == method {
				for _, f := range m.Fields {
					if v := r.FormValue("cred_" + f.Name); v != "" {
						params[f.Name] = v
					}
				}
				break
			}
		}
	}

	challenge, err := s.ctrl.StoreLoginInteractive(ctx, method, params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]string{
		"challenge_id": challenge.ChallengeID,
		"auth_url":     challenge.AuthURL,
		"expires_at":   challenge.ExpiresAt,
	})
}

// handleLogout clears the session and redirects to /login.
// POST /logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	_ = s.ctrl.StoreLogout(r.Context())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleAuthStatus returns current auth status as JSON.
// GET /auth/status
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, err := s.ctrl.StoreAuthStatus(ctx)
	if err != nil {
		http.Error(w, `{"status":"error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(`{"status":"` + string(session.Status) + `"}`))
}

// requireAuth middleware checks StoreAuthStatus and redirects to /login
// when authentication is needed. It stores the authentication state in
// the request context so templates can conditionally render UI elements.
//
// When the session is unauthenticated, we probe the store's auth methods
// to distinguish "no auth required" (SessionNone) from "needs login".
// This handles stores like jsonfile that report Auth=false but where the
// SessionManager hasn't yet transitioned from its default state.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session, err := s.ctrl.StoreAuthStatus(ctx)
		if err != nil {
			redirectToLogin(w, r)
			return
		}
		switch session.Status {
		case ui.StoreSessionNone:
			next.ServeHTTP(w, r)
		case ui.StoreSessionAuthenticated:
			ctx = context.WithValue(ctx, authenticatedCtxKey, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		case ui.StoreSessionUnauthenticated:
			// Check if the store actually requires auth. Stores without
			// auth support (e.g. jsonfile) return no methods, meaning
			// login is not needed — treat as SessionNone.
			methods, mErr := s.ctrl.StoreAuthMethods(ctx)
			if mErr == nil && len(methods) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			redirectToLogin(w, r)
		default:
			// expired
			redirectToLogin(w, r)
		}
	})
}

// redirectToLogin sends the client to the login page. For HTMX requests
// it uses the HX-Redirect header; for full-page requests it uses HTTP 303.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
