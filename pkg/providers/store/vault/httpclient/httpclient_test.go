package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Request(t *testing.T) {
	var gotPath, gotMethod, gotToken, gotNS string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotToken = r.Header.Get("X-Vault-Token")
		gotNS = r.Header.Get("X-Vault-Namespace")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"key": "value"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token", "ns1", nil)
	resp, err := c.Request(context.Background(), http.MethodGet, "secret/data/test", nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", gotMethod)
	}
	if gotPath != "/v1/secret/data/test" {
		t.Errorf("path = %s", gotPath)
	}
	if gotToken != "test-token" {
		t.Errorf("token = %s", gotToken)
	}
	if gotNS != "ns1" {
		t.Errorf("namespace = %s", gotNS)
	}
	if resp.Data == nil {
		t.Error("expected data in response")
	}
}

func TestClient_Do_ExtraHeaders(t *testing.T) {
	var gotWrapTTL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotWrapTTL = r.Header.Get("X-Vault-Wrap-TTL")
		json.NewEncoder(w).Encode(map[string]any{
			"wrap_info": map[string]any{
				"token": "wrapped-token",
				"ttl":   120,
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token", "", nil)
	resp, err := c.Do(context.Background(), http.MethodPost, "auth/approle/role/test/secret-id", nil,
		map[string]string{"X-Vault-Wrap-TTL": "120s"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotWrapTTL != "120s" {
		t.Errorf("X-Vault-Wrap-TTL = %s, want 120s", gotWrapTTL)
	}
	if resp.WrapInfo == nil {
		t.Fatal("expected wrap_info in response")
	}
	if resp.WrapInfo.Token != "wrapped-token" {
		t.Errorf("wrap_info.token = %s", resp.WrapInfo.Token)
	}
}

func TestClient_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "token", "", nil)
	resp, err := c.Request(context.Background(), http.MethodDelete, "sys/policies/acl/test", nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response for 204")
	}
}

func TestClient_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []string{"permission denied"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "bad-token", "", nil)
	_, err := c.Request(context.Background(), http.MethodGet, "secret/data/test", nil)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("status = %d, want 403", apiErr.StatusCode)
	}
	if apiErr.Message != "permission denied" {
		t.Errorf("message = %q", apiErr.Message)
	}
}

func TestClient_TokenManagement(t *testing.T) {
	c := New("http://localhost", "", "", nil)
	if c.Token() != "" {
		t.Error("expected empty token initially")
	}
	c.SetToken("new-token")
	if c.Token() != "new-token" {
		t.Errorf("token = %s, want new-token", c.Token())
	}
}

func TestClient_List(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"keys": []string{"a", "b"}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token", "", nil)
	resp, err := c.List(context.Background(), "secret/metadata/zhi/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotMethod != "LIST" {
		t.Errorf("method = %s, want LIST", gotMethod)
	}
	keys, err := ParseListKeys(resp)
	if err != nil {
		t.Fatalf("ParseListKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("keys = %v, want 2 entries", keys)
	}
}

func TestClient_RequestWithBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type: application/json")
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "token", "", nil)
	_, err := c.Request(context.Background(), http.MethodPut, "sys/policies/acl/test",
		map[string]any{"policy": "path \"secret/*\" { capabilities = [\"read\"] }"})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if gotBody["policy"] == nil {
		t.Error("expected policy in body")
	}
}

func TestClient_NamespaceAbsentWhenEmpty(t *testing.T) {
	var hasNS bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasNS = r.Header["X-Vault-Namespace"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "token", "", nil)
	_, err := c.Request(context.Background(), http.MethodGet, "test", nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if hasNS {
		t.Error("X-Vault-Namespace should not be set when namespace is empty")
	}
}

func TestParseListKeys_NilResponse(t *testing.T) {
	keys, err := ParseListKeys(nil)
	if err != nil {
		t.Fatalf("ParseListKeys(nil): %v", err)
	}
	if keys != nil {
		t.Errorf("expected nil keys, got %v", keys)
	}
}

func TestParseListKeys_EmptyData(t *testing.T) {
	keys, err := ParseListKeys(&Response{})
	if err != nil {
		t.Fatalf("ParseListKeys(empty): %v", err)
	}
	if keys != nil {
		t.Errorf("expected nil keys, got %v", keys)
	}
}

func TestClient_AuthResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{
				"client_token":   "s.token123",
				"accessor":       "acc-456",
				"policies":       []string{"default", "admin"},
				"lease_duration": 3600,
				"renewable":      true,
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token", "", nil)
	resp, err := c.Request(context.Background(), http.MethodPost, "auth/token/create", nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if resp.Auth == nil {
		t.Fatal("expected auth in response")
	}
	if resp.Auth.ClientToken != "s.token123" {
		t.Errorf("client_token = %s", resp.Auth.ClientToken)
	}
	if len(resp.Auth.Policies) != 2 {
		t.Errorf("policies = %v", resp.Auth.Policies)
	}
}
