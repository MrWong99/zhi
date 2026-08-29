package doctor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MrWong99/zhi/internal/core"
)

// vaultServerOpts drives the fake Vault mux built by newVaultServer.
type vaultServerOpts struct {
	sealed       bool
	initialized  bool
	lookupStatus int // 0 => 200 with body; otherwise return this code
	ttl          int
	expireTime   string
	listStatus   int      // 0 => 200; otherwise return this code
	listKeys     []string // keys returned on a successful LIST
}

func newVaultServer(t *testing.T, opts vaultServerOpts) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// sys/seal-status is unauthenticated and returns fields at the top
	// level (not wrapped in "data").
	mux.HandleFunc("/v1/sys/seal-status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if opts.sealed {
			// Match real Vault's 503 when sealed. Body still decodes.
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sealed":      opts.sealed,
			"initialized": opts.initialized,
		})
	})

	mux.HandleFunc("/v1/auth/token/lookup-self", func(w http.ResponseWriter, r *http.Request) {
		if opts.lookupStatus != 0 {
			w.WriteHeader(opts.lookupStatus)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []string{"permission denied"},
			})
			return
		}
		body := map[string]any{
			"data": map[string]any{
				"ttl":         opts.ttl,
				"expire_time": opts.expireTime,
				"renewable":   false,
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	})

	// LIST /v1/secret/metadata/zhi/
	mux.HandleFunc("/v1/secret/metadata/zhi/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "LIST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if opts.listStatus != 0 {
			w.WriteHeader(opts.listStatus)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []string{"forbidden"},
			})
			return
		}
		keys := opts.listKeys
		if keys == nil {
			keys = []string{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"keys": keys},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// vaultEnv builds an Environment whose workspace points at srv.
func vaultEnv(srv *httptest.Server, token string) *Environment {
	ws := &core.WorkspaceConfig{
		Store: core.ProviderRef{
			Provider: "vault",
			Options: map[string]any{
				"addr":   srv.URL,
				"mount":  "secret",
				"prefix": "zhi",
				"auth": map[string]any{
					"credentials": map[string]any{
						"token": token,
					},
				},
			},
		},
	}
	return &Environment{
		Workspace: ws,
		Timeout:   5 * time.Second,
	}
}

// runAll runs a list of checks back-to-back, collecting their results
// keyed by check ID.
func runAll(t *testing.T, env *Environment, checks ...Check) map[string]Result {
	t.Helper()
	out := make(map[string]Result, len(checks))
	for _, c := range checks {
		results := c.Run(context.Background(), env)
		if len(results) != 1 {
			t.Fatalf("check %s: want 1 result, got %d (%+v)", c.ID(), len(results), results)
		}
		out[c.ID()] = results[0]
	}
	return out
}

func TestVaultChecks_AllOK(t *testing.T) {
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      false,
		initialized: true,
		ttl:         4 * 3600,
		expireTime:  time.Now().Add(4 * time.Hour).Format(time.RFC3339),
		listKeys:    []string{"prod/", "staging/"},
	})
	env := vaultEnv(srv, "test-token")

	res := runAll(t, env,
		vaultReachableCheck{},
		vaultUnsealedCheck{},
		vaultTokenValidCheck{},
		vaultTokenTTLCheck{},
		vaultMountAccessibleCheck{},
	)

	for id, r := range res {
		if r.Status != StatusOK {
			t.Errorf("%s: status = %s, want OK (msg=%s detail=%s)", id, r.Status, r.Message, r.Detail)
		}
	}
	if !strings.Contains(res["store.vault.mount_accessible"].Message, "2 tree(s)") {
		t.Errorf("mount_accessible message = %q, want 2 tree(s)", res["store.vault.mount_accessible"].Message)
	}
}

func TestVaultChecks_Sealed(t *testing.T) {
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      true,
		initialized: true,
	})
	env := vaultEnv(srv, "test-token")

	reach := runAll(t, env, vaultReachableCheck{})["store.vault.reachable"]
	if reach.Status != StatusOK {
		t.Errorf("reachable: status = %s, want OK", reach.Status)
	}

	unsealed := runAll(t, env, vaultUnsealedCheck{})["store.vault.unsealed"]
	if unsealed.Status != StatusError {
		t.Fatalf("unsealed: status = %s, want Error", unsealed.Status)
	}
	if !strings.Contains(unsealed.Hint, "unseal") {
		t.Errorf("unsealed hint = %q, want mention of unseal", unsealed.Hint)
	}
}

func TestVaultChecks_NotInitialized(t *testing.T) {
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      false,
		initialized: false,
	})
	env := vaultEnv(srv, "test-token")

	unsealed := runAll(t, env, vaultUnsealedCheck{})["store.vault.unsealed"]
	if unsealed.Status != StatusError {
		t.Fatalf("unsealed: status = %s, want Error", unsealed.Status)
	}
	if !strings.Contains(unsealed.Hint, "init") {
		t.Errorf("unsealed hint = %q, want mention of init", unsealed.Hint)
	}
}

func TestVaultChecks_ExpiredToken(t *testing.T) {
	srv := newVaultServer(t, vaultServerOpts{
		sealed:       false,
		initialized:  true,
		lookupStatus: http.StatusForbidden,
	})
	env := vaultEnv(srv, "bad-token")

	got := runAll(t, env, vaultTokenValidCheck{}, vaultTokenTTLCheck{})
	valid := got["store.vault.token_valid"]
	if valid.Status != StatusError {
		t.Errorf("token_valid: status = %s, want Error", valid.Status)
	}
	// When lookup fails, the TTL check can't do anything useful — it
	// skips and tells the user where to look.
	ttl := got["store.vault.token_ttl"]
	if ttl.Status != StatusSkipped {
		t.Errorf("token_ttl: status = %s, want Skipped", ttl.Status)
	}
}

func TestVaultChecks_LowTTL(t *testing.T) {
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      false,
		initialized: true,
		ttl:         300,
		expireTime:  time.Now().Add(5 * time.Minute).Format(time.RFC3339),
	})
	env := vaultEnv(srv, "soon-expiring")

	ttl := runAll(t, env, vaultTokenTTLCheck{})["store.vault.token_ttl"]
	if ttl.Status != StatusWarning {
		t.Fatalf("token_ttl: status = %s, want Warning (msg=%s)", ttl.Status, ttl.Message)
	}
	if !strings.Contains(ttl.Message, "5m") && !strings.Contains(ttl.Message, "300s") {
		t.Errorf("token_ttl message = %q, want duration", ttl.Message)
	}
}

func TestVaultChecks_RootToken(t *testing.T) {
	// ttl=0 is Vault's signal for no-expiry (root-like) tokens.
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      false,
		initialized: true,
		ttl:         0,
	})
	env := vaultEnv(srv, "root")

	ttl := runAll(t, env, vaultTokenTTLCheck{})["store.vault.token_ttl"]
	if ttl.Status != StatusOK {
		t.Fatalf("token_ttl: status = %s, want OK", ttl.Status)
	}
	if !strings.Contains(ttl.Message, "no expiry") {
		t.Errorf("token_ttl message = %q, want mention of no expiry", ttl.Message)
	}
}

func TestVaultChecks_ForbiddenMount(t *testing.T) {
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      false,
		initialized: true,
		listStatus:  http.StatusForbidden,
	})
	env := vaultEnv(srv, "test-token")

	mount := runAll(t, env, vaultMountAccessibleCheck{})["store.vault.mount_accessible"]
	if mount.Status != StatusError {
		t.Fatalf("mount_accessible: status = %s, want Error", mount.Status)
	}
	if !strings.Contains(mount.Hint, "list") {
		t.Errorf("mount_accessible hint = %q, want mention of list", mount.Hint)
	}
}

func TestVaultChecks_EmptyMount(t *testing.T) {
	// A 404 on LIST for a KV v2 mount with no data is normal; the
	// check should still report OK.
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      false,
		initialized: true,
		listStatus:  http.StatusNotFound,
	})
	env := vaultEnv(srv, "test-token")

	mount := runAll(t, env, vaultMountAccessibleCheck{})["store.vault.mount_accessible"]
	if mount.Status != StatusOK {
		t.Fatalf("mount_accessible: status = %s, want OK (msg=%s detail=%s)", mount.Status, mount.Message, mount.Detail)
	}
}

func TestVaultChecks_NoToken(t *testing.T) {
	srv := newVaultServer(t, vaultServerOpts{
		sealed:      false,
		initialized: true,
	})
	env := vaultEnv(srv, "")
	// Also clear VAULT_TOKEN env so the fallback doesn't mask the case.
	t.Setenv("VAULT_TOKEN", "")

	valid := runAll(t, env, vaultTokenValidCheck{})["store.vault.token_valid"]
	if valid.Status != StatusError {
		t.Fatalf("token_valid: status = %s, want Error", valid.Status)
	}
	if !strings.Contains(valid.Message, "no VAULT_TOKEN") {
		t.Errorf("token_valid message = %q, want no-token message", valid.Message)
	}
}

func TestVaultChecks_Unreachable(t *testing.T) {
	// Close the server immediately so the URL refuses connections.
	srv := httptest.NewServer(http.NewServeMux())
	url := srv.URL
	srv.Close()

	ws := &core.WorkspaceConfig{
		Store: core.ProviderRef{
			Provider: "vault",
			Options: map[string]any{
				"addr": url,
			},
		},
	}
	env := &Environment{Workspace: ws, Timeout: 500 * time.Millisecond}

	reach := runAll(t, env, vaultReachableCheck{})["store.vault.reachable"]
	if reach.Status != StatusError {
		t.Fatalf("reachable: status = %s, want Error", reach.Status)
	}
	// The unsealed check should degrade to skipped so the user sees the
	// error in one place only.
	unsealed := runAll(t, env, vaultUnsealedCheck{})["store.vault.unsealed"]
	if unsealed.Status != StatusSkipped {
		t.Errorf("unsealed: status = %s, want Skipped", unsealed.Status)
	}
}

func TestVaultChecks_ProviderNotVault(t *testing.T) {
	ws := &core.WorkspaceConfig{
		Store: core.ProviderRef{Provider: "jsonfile"},
	}
	env := &Environment{Workspace: ws, Timeout: time.Second}

	checks := []Check{
		vaultReachableCheck{},
		vaultUnsealedCheck{},
		vaultTokenValidCheck{},
		vaultTokenTTLCheck{},
		vaultMountAccessibleCheck{},
	}
	got := runAll(t, env, checks...)
	for id, r := range got {
		if r.Status != StatusSkipped {
			t.Errorf("%s: status = %s, want Skipped", id, r.Status)
		}
	}
}

func TestVaultChecks_NilWorkspace(t *testing.T) {
	env := &Environment{}
	r := vaultReachableCheck{}.Run(context.Background(), env)
	if len(r) != 1 || r[0].Status != StatusSkipped {
		t.Errorf("reachable on nil workspace: want Skipped, got %+v", r)
	}
}
