package vaultmanager

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
		case r.Method == http.MethodGet && path == "/v1/auth/token/lookup-self":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"policies": []string{"root"}},
			})

		case r.Method == http.MethodPut && strings.HasPrefix(path, "/v1/sys/policies/acl/"):
			name := strings.TrimPrefix(path, "/v1/sys/policies/acl/")
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			policiesWritten[name] = body["policy"].(string)
			w.WriteHeader(http.StatusNoContent)

		case r.Method == "LIST" && path == "/v1/sys/policies/acl":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []string{}},
			})

		case r.Method == http.MethodGet && strings.HasPrefix(path, "/v1/auth/approle/role/") && !strings.HasSuffix(path, "/role-id") && !strings.HasSuffix(path, "/secret-id"):
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"errors": []string{"no role"}})

		case r.Method == http.MethodPost && strings.HasPrefix(path, "/v1/auth/approle/role/") && !strings.HasSuffix(path, "/secret-id"):
			name := strings.TrimPrefix(path, "/v1/auth/approle/role/")
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			approlesWritten[name] = body
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodGet && strings.HasSuffix(path, "/role-id"):
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"role_id": "test-role-id-123"},
			})

		case r.Method == http.MethodPost && strings.HasSuffix(path, "/secret-id"):
			json.NewEncoder(w).Encode(map[string]any{
				"wrap_info": map[string]any{
					"token":            "hvs.wrapped-token-abc",
					"ttl":              120,
					"creation_time":    "2026-02-20T00:00:00Z",
					"wrapped_accessor": "acc-123",
				},
			})

		case r.Method == http.MethodPost && path == "/v1/auth/token/create-orphan":
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

	admin := NewAdminClient(srv.URL, "", "", nil)
	_, err := admin.Login(context.Background(), "token", map[string]string{"token": "root-token"})
	if err != nil {
		t.Fatalf("admin Login: %v", err)
	}

	cm := NewCredManager(admin, "secret", "zhi", "testws", nil)

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

	apps := []AppConfig{
		{Name: "web-api", Auth: "approle", Wrapped: true, WrapTTL: "120s"},
		{Name: "worker", Auth: "approle", Wrapped: true, WrapTTL: "120s"},
		{Name: "ci", Auth: "token", TokenTTL: "1h"},
	}

	err = cm.ReconcilePolicies(context.Background(), values, apps)
	if err != nil {
		t.Fatalf("ReconcilePolicies: %v", err)
	}
	if len(policiesWritten) != 3 {
		t.Errorf("expected 3 policies, got %d: %v", len(policiesWritten), policiesWritten)
	}

	webAPIPolicy := policiesWritten["zhi-testws-web-api"]
	if !strings.Contains(webAPIPolicy, "database/host") {
		t.Error("web-api policy missing database/host")
	}
	if !strings.Contains(webAPIPolicy, "database/password") {
		t.Error("web-api policy missing database/password")
	}

	err = cm.EnsureAppRoles(context.Background(), apps)
	if err != nil {
		t.Fatalf("EnsureAppRoles: %v", err)
	}
	if len(approlesWritten) != 2 {
		t.Errorf("expected 2 approles, got %d", len(approlesWritten))
	}

	creds, err := cm.GenerateCredentials(context.Background(), apps)
	if err != nil {
		t.Fatalf("GenerateCredentials: %v", err)
	}
	if len(creds) != 3 {
		t.Errorf("expected 3 credential sets, got %d", len(creds))
	}

	webAPICreds := creds["web-api"]
	if webAPICreds.RoleID != "test-role-id-123" {
		t.Errorf("role_id = %s", webAPICreds.RoleID)
	}
	if !webAPICreds.Wrapped {
		t.Error("expected wrapped=true for wrapped AppRole credentials")
	}
	if webAPICreds.SecretID != "hvs.wrapped-token-abc" {
		t.Errorf("secret_id = %s", webAPICreds.SecretID)
	}

	ciCreds := creds["ci"]
	if ciCreds.Token != "hvs.child-token-xyz" {
		t.Errorf("token = %s", ciCreds.Token)
	}

	enriched := InjectCredentials(values, creds)

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

	if _, ok := enriched["database/host"]; !ok {
		t.Error("original value database/host missing")
	}

	authMethod, ok := enriched["vault/credentials/web-api/auth-method"]
	if !ok {
		t.Fatal("missing auth-method")
	}
	if authMethod.Val != "approle" {
		t.Errorf("auth-method = %v", authMethod.Val)
	}
}
