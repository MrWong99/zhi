package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminClient_PutPolicy(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	err := c.putPolicy(context.Background(), "zhi-ws-myapp", `path "secret/data/zhi/*" { capabilities = ["read"] }`)
	if err != nil {
		t.Fatalf("putPolicy: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/v1/sys/policies/acl/zhi-ws-myapp" {
		t.Errorf("path = %s, want /v1/sys/policies/acl/zhi-ws-myapp", gotPath)
	}
	if gotBody["policy"] != `path "secret/data/zhi/*" { capabilities = ["read"] }` {
		t.Errorf("unexpected policy body: %v", gotBody)
	}
}

func TestAdminClient_DeletePolicy(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	err := c.deletePolicy(context.Background(), "zhi-ws-myapp")
	if err != nil {
		t.Fatalf("deletePolicy: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "/v1/sys/policies/acl/zhi-ws-myapp" {
		t.Errorf("path = %s", gotPath)
	}
}

func TestAdminClient_ListPolicies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"keys": []string{"default", "root", "zhi-ws-app1", "zhi-ws-app2", "other-policy"},
			},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	policies, err := c.listPolicies(context.Background(), "zhi-ws-")
	if err != nil {
		t.Fatalf("listPolicies: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %d: %v", len(policies), policies)
	}
}

func TestAdminClient_ReadAppRole(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/auth/approle/role/zhi-myapp" {
			t.Errorf("path = %s, want /v1/auth/approle/role/zhi-myapp", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"token_policies":     []string{"zhi-ws-myapp"},
				"token_ttl":          3600,
				"secret_id_ttl":      0,
				"secret_id_num_uses": 0,
			},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	data, err := c.readAppRole(context.Background(), "zhi-myapp")
	if err != nil {
		t.Fatalf("readAppRole: %v", err)
	}
	policies, ok := data["token_policies"]
	if !ok {
		t.Fatal("expected token_policies in response data")
	}
	policiesSlice, ok := policies.([]any)
	if !ok || len(policiesSlice) != 1 {
		t.Errorf("unexpected token_policies: %v", policies)
	}
}

func TestAdminClient_WriteAppRole(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	params := map[string]any{
		"token_policies": []string{"zhi-ws-myapp"},
		"token_ttl":      "1h",
	}
	err := c.writeAppRole(context.Background(), "zhi-myapp", params)
	if err != nil {
		t.Fatalf("writeAppRole: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/auth/approle/role/zhi-myapp" {
		t.Errorf("path = %s, want /v1/auth/approle/role/zhi-myapp", gotPath)
	}
	if gotBody["token_ttl"] != "1h" {
		t.Errorf("unexpected body: %v", gotBody)
	}
}

func TestAdminClient_ReadRoleID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/auth/approle/role/zhi-myapp/role-id" {
			t.Errorf("path = %s, want /v1/auth/approle/role/zhi-myapp/role-id", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"role_id": "test-role-id",
			},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	roleID, err := c.readRoleID(context.Background(), "zhi-myapp")
	if err != nil {
		t.Fatalf("readRoleID: %v", err)
	}
	if roleID != "test-role-id" {
		t.Errorf("roleID = %s, want test-role-id", roleID)
	}
}

func TestAdminClient_GenerateWrappedSecretID(t *testing.T) {
	var gotWrapTTL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/approle/role/zhi-myapp/secret-id" {
			t.Errorf("path = %s, want /v1/auth/approle/role/zhi-myapp/secret-id", r.URL.Path)
		}
		gotWrapTTL = r.Header.Get("X-Vault-Wrap-TTL")
		json.NewEncoder(w).Encode(map[string]any{
			"wrap_info": map[string]any{
				"token":            "wrapped-token",
				"ttl":              120,
				"creation_time":    "2024-01-01T00:00:00Z",
				"wrapped_accessor": "accessor-123",
			},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	token, err := c.generateWrappedSecretID(context.Background(), "zhi-myapp", "120s")
	if err != nil {
		t.Fatalf("generateWrappedSecretID: %v", err)
	}
	if token != "wrapped-token" {
		t.Errorf("token = %s, want wrapped-token", token)
	}
	if gotWrapTTL != "120s" {
		t.Errorf("X-Vault-Wrap-TTL = %s, want 120s", gotWrapTTL)
	}
}

func TestAdminClient_CreateToken(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/token/create" {
			t.Errorf("path = %s, want /v1/auth/token/create", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{
				"client_token": "new-token",
				"accessor":     "accessor-456",
				"policies":     []string{"zhi-ws-myapp"},
			},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	token, err := c.createToken(context.Background(), []string{"zhi-ws-myapp"}, "1h")
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}
	if token != "new-token" {
		t.Errorf("token = %s, want new-token", token)
	}
	if gotBody["ttl"] != "1h" {
		t.Errorf("unexpected ttl in body: %v", gotBody)
	}
}

func TestAdminClient_CreateScopedToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/token/create" {
			t.Errorf("path = %s, want /v1/auth/token/create", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{
				"client_token": "scoped-token",
				"policies":     []string{},
			},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	token, err := c.createScopedToken(context.Background(), `path "secret/*" { capabilities = ["read"] }`, "30m")
	if err != nil {
		t.Fatalf("createScopedToken: %v", err)
	}
	if token != "scoped-token" {
		t.Errorf("token = %s, want scoped-token", token)
	}
}

func TestAdminClient_LoginToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/auth/token/lookup-self" {
			t.Errorf("path = %s, want /v1/auth/token/lookup-self", r.URL.Path)
		}
		if r.Header.Get("X-Vault-Token") != "admin-token-123" {
			t.Errorf("X-Vault-Token = %s, want admin-token-123", r.Header.Get("X-Vault-Token"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"policies": []string{"root", "default"},
			},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "", "")
	auth, err := c.login(context.Background(), "token", map[string]string{
		"token": "admin-token-123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if auth.ClientToken != "admin-token-123" {
		t.Errorf("client_token = %s, want admin-token-123", auth.ClientToken)
	}
	if len(auth.Policies) != 2 {
		t.Errorf("expected 2 policies, got %d: %v", len(auth.Policies), auth.Policies)
	}
	// Verify the token was persisted on the client.
	if c.getToken() != "admin-token-123" {
		t.Errorf("client token not updated after login")
	}
}

func TestAdminClient_ErrorHandling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []string{"permission denied"},
		})
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "bad-token", "")
	err := c.putPolicy(context.Background(), "test", "policy data")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if got := err.Error(); got != "vault: permission denied (403)" {
		t.Errorf("error = %q, want vault: permission denied (403)", got)
	}
}

func TestAdminClient_VaultTokenHeader(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Vault-Token")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "my-secret-token", "")
	err := c.deletePolicy(context.Background(), "test-policy")
	if err != nil {
		t.Fatalf("deletePolicy: %v", err)
	}
	if gotToken != "my-secret-token" {
		t.Errorf("X-Vault-Token = %q, want %q", gotToken, "my-secret-token")
	}
}

func TestAdminClient_NamespaceHeader(t *testing.T) {
	var gotNamespace string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotNamespace = r.Header.Get("X-Vault-Namespace")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "engineering/team-a")
	err := c.deletePolicy(context.Background(), "test-policy")
	if err != nil {
		t.Fatalf("deletePolicy: %v", err)
	}
	if gotNamespace != "engineering/team-a" {
		t.Errorf("X-Vault-Namespace = %q, want %q", gotNamespace, "engineering/team-a")
	}
}

func TestAdminClient_NamespaceHeaderAbsentWhenEmpty(t *testing.T) {
	var gotNamespace string
	var hasNamespaceHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotNamespace = r.Header.Get("X-Vault-Namespace")
		_, hasNamespaceHeader = r.Header["X-Vault-Namespace"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newAdminClient(srv.URL, "test-token", "")
	err := c.deletePolicy(context.Background(), "test-policy")
	if err != nil {
		t.Fatalf("deletePolicy: %v", err)
	}
	if hasNamespaceHeader {
		t.Errorf("X-Vault-Namespace should not be set when namespace is empty, got %q", gotNamespace)
	}
}
