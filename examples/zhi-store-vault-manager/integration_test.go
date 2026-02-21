package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

func TestIntegration_FullLifecycle(t *testing.T) {
	var (
		mu              sync.Mutex
		policiesWritten = make(map[string]string)
		approlesWritten = make(map[string]map[string]any)
		tokensCreated   int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		path := r.URL.Path

		switch {
		// Token lookup-self (login verification)
		case r.Method == http.MethodGet && path == "/v1/auth/token/lookup-self":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"policies": []string{"root"}},
			})

		// Policy write
		case r.Method == http.MethodPut && strings.HasPrefix(path, "/v1/sys/policies/acl/"):
			name := strings.TrimPrefix(path, "/v1/sys/policies/acl/")
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			policiesWritten[name] = body["policy"].(string)
			w.WriteHeader(http.StatusNoContent)

		// Policy list
		case r.Method == "LIST" && path == "/v1/sys/policies/acl":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []string{}},
			})

		// AppRole read (return 404 = doesn't exist)
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/v1/auth/approle/role/") && !strings.HasSuffix(path, "/role-id") && !strings.HasSuffix(path, "/secret-id"):
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"errors": []string{"no role"}})

		// AppRole write
		case r.Method == http.MethodPost && strings.HasPrefix(path, "/v1/auth/approle/role/") && !strings.HasSuffix(path, "/secret-id"):
			name := strings.TrimPrefix(path, "/v1/auth/approle/role/")
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			approlesWritten[name] = body
			w.WriteHeader(http.StatusNoContent)

		// Role ID read
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/role-id"):
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"role_id": "test-role-id-123"},
			})

		// Wrapped secret ID generation
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/secret-id"):
			json.NewEncoder(w).Encode(map[string]any{
				"wrap_info": map[string]any{
					"token":            "hvs.wrapped-token-abc",
					"ttl":              120,
					"creation_time":    "2026-02-20T00:00:00Z",
					"wrapped_accessor": "acc-123",
				},
			})

		// Token create
		case r.Method == http.MethodPost && path == "/v1/auth/token/create":
			tokensCreated++
			json.NewEncoder(w).Encode(map[string]any{
				"auth": map[string]any{
					"client_token":   "hvs.child-token-xyz",
					"policies":       []string{"default"},
					"lease_duration": 3600,
					"renewable":      true,
				},
			})

		default:
			t.Logf("unhandled: %s %s", r.Method, path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Setup
	admin := newAdminClient(srv.URL, "", "", nil)
	_, err := admin.login(context.Background(), "token", map[string]string{"token": "root-token"})
	if err != nil {
		t.Fatalf("admin login: %v", err)
	}

	cm := newCredManager(admin, "secret", "zhi", "testws", nil)

	values := map[string]config.Value{
		"database/host": {
			Val: "localhost",
			Metadata: map[string]any{
				"vault.app.web-api": "read",
				"vault.app.worker":  "read,write",
			},
		},
		"database/password": {
			Val: "secret123",
			Metadata: map[string]any{
				"vault.app.web-api": "read",
			},
		},
	}

	apps := []appConfig{
		{Name: "web-api", Auth: "approle", Wrapped: true, WrapTTL: "120s"},
		{Name: "worker", Auth: "approle", Wrapped: true, WrapTTL: "120s"},
		{Name: "ci", Auth: "token", TokenTTL: "1h"},
	}

	// 1. Reconcile policies
	err = cm.reconcilePolicies(context.Background(), values, apps)
	if err != nil {
		t.Fatalf("reconcilePolicies: %v", err)
	}
	if len(policiesWritten) != 3 {
		t.Errorf("expected 3 policies, got %d: %v", len(policiesWritten), policiesWritten)
	}

	// Verify web-api policy contains correct paths
	webAPIPolicy := policiesWritten["zhi-testws-web-api"]
	if !strings.Contains(webAPIPolicy, "database/host") {
		t.Error("web-api policy missing database/host")
	}
	if !strings.Contains(webAPIPolicy, "database/password") {
		t.Error("web-api policy missing database/password")
	}

	// 2. Ensure AppRoles
	err = cm.ensureAppRoles(context.Background(), apps)
	if err != nil {
		t.Fatalf("ensureAppRoles: %v", err)
	}
	if len(approlesWritten) != 2 {
		t.Errorf("expected 2 approles, got %d", len(approlesWritten))
	}

	// 3. Generate credentials
	creds, err := cm.generateCredentials(context.Background(), apps)
	if err != nil {
		t.Fatalf("generateCredentials: %v", err)
	}
	if len(creds) != 3 {
		t.Errorf("expected 3 credential sets, got %d", len(creds))
	}

	// Verify AppRole credentials
	webAPICreds := creds["web-api"]
	if webAPICreds.RoleID != "test-role-id-123" {
		t.Errorf("role_id = %s", webAPICreds.RoleID)
	}
	if webAPICreds.WrappedSecretID != "hvs.wrapped-token-abc" {
		t.Errorf("wrapped_secret_id = %s", webAPICreds.WrappedSecretID)
	}

	// Verify token credentials
	ciCreds := creds["ci"]
	if ciCreds.Token != "hvs.child-token-xyz" {
		t.Errorf("token = %s", ciCreds.Token)
	}

	// 4. Inject credentials
	enriched := injectCredentials(values, creds)

	// Verify injection
	roleIDVal, ok := enriched["vault/credentials/web-api/role-id"]
	if !ok {
		t.Fatal("missing injected role-id")
	}
	if roleIDVal.Val != "test-role-id-123" {
		t.Errorf("injected role_id = %v", roleIDVal.Val)
	}
	if !labels.IsHidden(roleIDVal.Metadata) {
		t.Error("injected value should be ui.hidden")
	}
	if !labels.IsWriteonly(roleIDVal.Metadata) {
		t.Error("injected value should be store.writeonly")
	}
	if !labels.IsEphemeral(roleIDVal.Metadata) {
		t.Error("injected value should be store.ephemeral")
	}
	if !labels.IsReadonly(roleIDVal.Metadata) {
		t.Error("injected value should be ui.readonly")
	}

	// Verify original values still present
	if _, ok := enriched["database/host"]; !ok {
		t.Error("original value database/host missing")
	}

	// Verify auth-method injected
	authMethod, ok := enriched["vault/credentials/web-api/auth-method"]
	if !ok {
		t.Fatal("missing auth-method")
	}
	if authMethod.Val != "approle" {
		t.Errorf("auth-method = %v", authMethod.Val)
	}
}
