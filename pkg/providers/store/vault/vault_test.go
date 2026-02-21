package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// --- Unit tests using httptest mock ---

// mockVault simulates a Vault KV v2 server for unit testing.
type mockVault struct {
	mu             sync.Mutex
	secrets        map[string][]secretVersion // path -> versions (1-indexed)
	policies       map[string]string          // policy name -> HCL
	secretMetadata map[string]secretMeta      // path -> per-secret metadata config
}

// secretMeta holds per-secret metadata configured via POST to /metadata/.
type secretMeta struct {
	MaxVersions        int    `json:"max_versions"`
	DeleteVersionAfter string `json:"delete_version_after"`
}

type secretVersion struct {
	data      map[string]any
	destroyed bool
	deleted   bool
}

func newMockVault() *mockVault {
	return &mockVault{
		secrets:        make(map[string][]secretVersion),
		policies:       make(map[string]string),
		secretMetadata: make(map[string]secretMeta),
	}
}

func (m *mockVault) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		path := strings.TrimPrefix(r.URL.Path, "/v1/")

		// Auth endpoints.
		if path == "auth/token/lookup-self" {
			writeJSON(w, map[string]any{
				"data": map[string]any{
					"ttl":         3600,
					"renewable":   false,
					"expire_time": "",
				},
			})
			return
		}

		if strings.HasPrefix(path, "auth/userpass/login/") {
			writeJSON(w, map[string]any{
				"auth": map[string]any{
					"client_token":   "userpass-token",
					"lease_duration": 3600,
					"renewable":      false,
					"policies":       []string{"default"},
					"metadata":       map[string]string{"username": "testuser"},
				},
			})
			return
		}

		if path == "auth/approle/login" {
			writeJSON(w, map[string]any{
				"auth": map[string]any{
					"client_token":   "approle-token",
					"lease_duration": 3600,
					"renewable":      false,
					"policies":       []string{"default"},
					"metadata":       map[string]string{},
				},
			})
			return
		}

		// Policy endpoints.
		if strings.HasPrefix(path, "sys/policies/acl") {
			m.handlePolicy(w, r, path)
			return
		}

		// KV v2 endpoints.
		if strings.Contains(path, "/data/") {
			m.handleData(w, r, path)
			return
		}
		if strings.Contains(path, "/metadata/") {
			m.handleMetadata(w, r, path)
			return
		}
		if strings.Contains(path, "/destroy/") {
			m.handleDestroy(w, r, path)
			return
		}

		http.NotFound(w, r)
	})
}

func (m *mockVault) handleData(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/data/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	secretPath := parts[1]

	switch r.Method {
	case http.MethodGet:
		versions := m.secrets[secretPath]
		if len(versions) == 0 {
			writeVaultError(w, http.StatusNotFound, "secret not found")
			return
		}

		targetVersion := len(versions)
		if v := r.URL.Query().Get("version"); v != "" {
			vi, err := strconv.Atoi(v)
			if err != nil || vi < 1 || vi > len(versions) {
				writeVaultError(w, http.StatusNotFound, "version not found")
				return
			}
			targetVersion = vi
		}

		sv := versions[targetVersion-1]
		deletionTime := ""
		if sv.deleted {
			deletionTime = "2024-01-01T00:00:00Z"
		}

		writeJSON(w, map[string]any{
			"data": map[string]any{
				"data": sv.data,
				"metadata": map[string]any{
					"version":       targetVersion,
					"destroyed":     sv.destroyed,
					"deletion_time": deletionTime,
					"created_time":  "2024-01-01T00:00:00Z",
				},
			},
		})

	case http.MethodPost:
		var body struct {
			Data    map[string]any `json:"data"`
			Options struct {
				CAS int `json:"cas"`
			} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeVaultError(w, http.StatusBadRequest, err.Error())
			return
		}

		if body.Options.CAS > 0 {
			current := len(m.secrets[secretPath])
			if current != body.Options.CAS {
				writeVaultError(w, http.StatusPreconditionFailed,
					"check-and-set parameter did not match the current version")
				return
			}
		}

		m.secrets[secretPath] = append(m.secrets[secretPath], secretVersion{
			data: body.Data,
		})
		newVersion := len(m.secrets[secretPath])
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"version": newVersion,
			},
		})

	case http.MethodDelete:
		versions := m.secrets[secretPath]
		if len(versions) > 0 {
			versions[len(versions)-1].deleted = true
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *mockVault) handleMetadata(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/metadata/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	secretPath := parts[1]

	switch r.Method {
	case http.MethodGet:
		versions := m.secrets[secretPath]
		if len(versions) == 0 {
			writeVaultError(w, http.StatusNotFound, "no metadata found")
			return
		}

		versionsMeta := make(map[string]any)
		for i, sv := range versions {
			dt := ""
			if sv.deleted {
				dt = "2024-01-01T00:00:00Z"
			}
			versionsMeta[strconv.Itoa(i+1)] = map[string]any{
				"created_time":  "2024-01-01T00:00:00Z",
				"deletion_time": dt,
				"destroyed":     sv.destroyed,
			}
		}
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"current_version": len(versions),
				"versions":        versionsMeta,
			},
		})

	case http.MethodPost:
		// Handle per-secret metadata configuration (max_versions, delete_version_after).
		var body struct {
			MaxVersions        int    `json:"max_versions"`
			DeleteVersionAfter string `json:"delete_version_after"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeVaultError(w, http.StatusBadRequest, err.Error())
			return
		}
		meta := m.secretMetadata[secretPath]
		if body.MaxVersions > 0 {
			meta.MaxVersions = body.MaxVersions
		}
		if body.DeleteVersionAfter != "" {
			meta.DeleteVersionAfter = body.DeleteVersionAfter
		}
		m.secretMetadata[secretPath] = meta
		w.WriteHeader(http.StatusNoContent)

	case "LIST":
		m.handleList(w, secretPath)

	case http.MethodDelete:
		// Remove all versions matching this path or with this prefix.
		// Returns success even if nothing was deleted (Vault behavior).
		for key := range m.secrets {
			if key == secretPath || strings.HasPrefix(key, secretPath+"/") {
				delete(m.secrets, key)
			}
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *mockVault) handleList(w http.ResponseWriter, prefix string) {
	prefix = strings.TrimSuffix(prefix, "/") + "/"
	seen := make(map[string]bool)
	for key := range m.secrets {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := strings.TrimPrefix(key, prefix)
		if idx := strings.Index(rest, "/"); idx >= 0 {
			seen[rest[:idx+1]] = true
		} else {
			seen[rest] = true
		}
	}

	if len(seen) == 0 {
		writeVaultError(w, http.StatusNotFound, "no keys found")
		return
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	writeJSON(w, map[string]any{
		"data": map[string]any{
			"keys": keys,
		},
	})
}

func (m *mockVault) handleDestroy(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.SplitN(path, "/destroy/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	secretPath := parts[1]

	var body struct {
		Versions []int `json:"versions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeVaultError(w, http.StatusBadRequest, err.Error())
		return
	}

	versions := m.secrets[secretPath]
	for _, v := range body.Versions {
		if v >= 1 && v <= len(versions) {
			versions[v-1].destroyed = true
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (m *mockVault) handlePolicy(w http.ResponseWriter, r *http.Request, path string) {
	name := strings.TrimPrefix(path, "sys/policies/acl")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")

	switch r.Method {
	case http.MethodGet:
		if name == "" {
			http.NotFound(w, r)
			return
		}
		policy, ok := m.policies[name]
		if !ok {
			writeVaultError(w, http.StatusNotFound, "policy not found")
			return
		}
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"name":   name,
				"policy": policy,
			},
		})

	case http.MethodPut:
		var body struct {
			Policy string `json:"policy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeVaultError(w, http.StatusBadRequest, err.Error())
			return
		}
		m.policies[name] = body.Policy
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		delete(m.policies, name)
		w.WriteHeader(http.StatusNoContent)

	case "LIST":
		keys := make([]string, 0, len(m.policies))
		for k := range m.policies {
			keys = append(keys, k)
		}
		writeJSON(w, map[string]any{
			"data": map[string]any{
				"keys": keys,
			},
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func writeVaultError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []string{msg},
	})
}

func newTestStore(t *testing.T) (*Store, *mockVault) {
	t.Helper()
	mock := newMockVault()
	srv := httptest.NewServer(mock.handler())
	t.Cleanup(srv.Close)

	s, err := New(Config{
		Address: srv.URL,
		Token:   "test-token",
		Mount:   "secret",
		Prefix:  "zhi",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(s.Stop)
	return s, mock
}

// --- Capabilities ---

func TestCapabilities(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	caps, err := s.Capabilities(ctx)
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.Versioning != store.VersioningValue {
		t.Errorf("Versioning = %v, want VersioningValue", caps.Versioning)
	}
	if caps.Encryption != store.EncryptionActive {
		t.Errorf("Encryption = %v, want EncryptionActive", caps.Encryption)
	}
	if !caps.Auth {
		t.Error("Auth should be true")
	}
	if !caps.AccessControl {
		t.Error("AccessControl should be true")
	}
}

// --- Authentication ---

func TestAuthMethods(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	methods, err := s.AuthMethods(ctx)
	if err != nil {
		t.Fatalf("AuthMethods: %v", err)
	}
	if len(methods) == 0 {
		t.Fatal("expected at least one auth method")
	}

	expectedTypes := map[string]bool{
		"token": false, "userpass": false, "approle": false,
		"ldap": false, "kubernetes": false, "oidc": false,
	}
	for _, m := range methods {
		if _, ok := expectedTypes[m.Type]; ok {
			expectedTypes[m.Type] = true
		}
	}
	for typ, found := range expectedTypes {
		if !found {
			t.Errorf("missing auth method: %s", typ)
		}
	}
}

func TestLoginToken(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	cred, err := s.Login(ctx, "token", map[string]string{"token": "my-token"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if cred.Token != "my-token" {
		t.Errorf("Token = %q, want %q", cred.Token, "my-token")
	}
}

func TestLoginUserpass(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	cred, err := s.Login(ctx, "userpass", map[string]string{
		"username": "testuser",
		"password": "testpass",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if cred.Token != "userpass-token" {
		t.Errorf("Token = %q, want %q", cred.Token, "userpass-token")
	}
}

func TestLoginAppRole(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	cred, err := s.Login(ctx, "approle", map[string]string{
		"role_id":   "my-role",
		"secret_id": "my-secret",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if cred.Token != "approle-token" {
		t.Errorf("Token = %q, want %q", cred.Token, "approle-token")
	}
}

func TestLoginMissingCredentials(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_, err := s.Login(ctx, "token", map[string]string{})
	if err == nil {
		t.Fatal("expected error for empty token")
	}

	_, err = s.Login(ctx, "userpass", map[string]string{"username": "user"})
	if err == nil {
		t.Fatal("expected error for missing password")
	}

	_, err = s.Login(ctx, "approle", map[string]string{"role_id": "role"})
	if err == nil {
		t.Fatal("expected error for missing secret_id")
	}
}

func TestLoginUnsupportedMethod(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_, err := s.Login(ctx, "unknown-method", map[string]string{})
	if err == nil {
		t.Fatal("expected error for unsupported method")
	}
}

// --- Value operations ---

func TestPutAndGetValues(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	values := map[string]config.Value{
		"db/host": {Val: "localhost"},
		"db/port": {Val: float64(5432)},
	}
	if err := s.PutValues(ctx, "prod", values, nil); err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := s.GetValues(ctx, "prod", []string{"db/host", "db/port"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d values, want 2", len(got))
	}
	if got["db/host"].Val != "localhost" {
		t.Errorf("db/host = %v, want %q", got["db/host"].Val, "localhost")
	}
	if got["db/port"].Val != float64(5432) {
		t.Errorf("db/port = %v, want 5432", got["db/port"].Val)
	}
}

func TestGetValuesMissing(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetValues(ctx, "nope", []string{"missing/key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d values, want 0", len(got))
	}
}

func TestMetadataPreserved(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	err := s.PutValues(ctx, "meta", map[string]config.Value{
		"key": {Val: "value", Metadata: map[string]any{"description": "test key"}},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := s.GetValues(ctx, "meta", []string{"key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if got["key"].Metadata["description"] != "test key" {
		t.Errorf("description = %v, want %q", got["key"].Metadata["description"], "test key")
	}
}

func TestPutOverwrite(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "old"}}, nil)
	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "new"}}, nil)

	got, _ := s.GetValues(ctx, "app", []string{"key"})
	if got["key"].Val != "new" {
		t.Errorf("key = %v, want %q", got["key"].Val, "new")
	}
}

func TestDeleteValues(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "test", map[string]config.Value{
		"a/x": {Val: "one"},
		"a/y": {Val: "two"},
	}, nil)

	if err := s.DeleteValues(ctx, "test", []string{"a/x"}); err != nil {
		t.Fatalf("DeleteValues: %v", err)
	}

	got, err := s.GetValues(ctx, "test", []string{"a/x", "a/y"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if _, found := got["a/x"]; found {
		t.Error("a/x should have been deleted")
	}
	if got["a/y"].Val != "two" {
		t.Errorf("a/y = %v, want %q", got["a/y"].Val, "two")
	}
}

// --- Tree management ---

func TestListTrees(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "one", map[string]config.Value{"a/b": {Val: "x"}}, nil)
	_ = s.PutValues(ctx, "two", map[string]config.Value{"a/b": {Val: "y"}}, nil)

	ids, err := s.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("got %d ids, want 2", len(ids))
	}
}

func TestListTreesEmpty(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	ids, err := s.ListTrees(ctx)
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("got %d ids, want 0", len(ids))
	}
}

func TestDeleteTree(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "temp", map[string]config.Value{
		"a/b": {Val: "x"},
		"c/d": {Val: "y"},
	}, nil)

	if err := s.DeleteTree(ctx, "temp"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	got, err := s.GetValues(ctx, "temp", []string{"a/b", "c/d"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if len(got) != 0 {
		t.Error("tree should be empty after DeleteTree")
	}
}

// --- CAS ---

func TestCASConflict(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	err := s.PutValues(ctx, "app", map[string]config.Value{
		"key": {Val: "first"},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	// Correct CAS succeeds.
	err = s.PutValues(ctx, "app", map[string]config.Value{
		"key": {Val: "second"},
	}, &store.PutOptions{CASVersions: map[string]string{"key": "1"}})
	if err != nil {
		t.Fatalf("PutValues (correct CAS): %v", err)
	}

	// Wrong CAS fails.
	err = s.PutValues(ctx, "app", map[string]config.Value{
		"key": {Val: "third"},
	}, &store.PutOptions{CASVersions: map[string]string{"key": "1"}})
	if err == nil {
		t.Fatal("expected CAS conflict error")
	}
	if !store.IsCASConflict(err) {
		t.Errorf("expected ErrCASConflict, got %T: %v", err, err)
	}
}

// --- Value-level versioning ---

func TestListValueVersions(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		err := s.PutValues(ctx, "app", map[string]config.Value{
			"db/host": {Val: fmt.Sprintf("host%d", i)},
		}, nil)
		if err != nil {
			t.Fatalf("PutValues v%d: %v", i, err)
		}
	}

	versions, err := s.ListValueVersions(ctx, "app", "db/host")
	if err != nil {
		t.Fatalf("ListValueVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("got %d versions, want 3", len(versions))
	}
	if versions[0] != "3" {
		t.Errorf("first version = %q, want %q (newest first)", versions[0], "3")
	}
}

func TestGetValueVersion(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "v1"}}, nil)
	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "v2"}}, nil)

	val, found, err := s.GetValueVersion(ctx, "app", "key", "1")
	if err != nil {
		t.Fatalf("GetValueVersion: %v", err)
	}
	if !found {
		t.Fatal("expected version 1 to be found")
	}
	if val.Val != "v1" {
		t.Errorf("version 1 = %v, want %q", val.Val, "v1")
	}

	val, found, err = s.GetValueVersion(ctx, "app", "key", "2")
	if err != nil {
		t.Fatalf("GetValueVersion: %v", err)
	}
	if !found {
		t.Fatal("expected version 2 to be found")
	}
	if val.Val != "v2" {
		t.Errorf("version 2 = %v, want %q", val.Val, "v2")
	}
}

func TestRollbackValue(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "original"}}, nil)
	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "updated"}}, nil)

	if err := s.RollbackValue(ctx, "app", "key", "1"); err != nil {
		t.Fatalf("RollbackValue: %v", err)
	}

	got, err := s.GetValues(ctx, "app", []string{"key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if got["key"].Val != "original" {
		t.Errorf("after rollback, key = %v, want %q", got["key"].Val, "original")
	}
}

func TestDeleteValueVersion(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "v1"}}, nil)
	_ = s.PutValues(ctx, "app", map[string]config.Value{"key": {Val: "v2"}}, nil)

	if err := s.DeleteValueVersion(ctx, "app", "key", "1"); err != nil {
		t.Fatalf("DeleteValueVersion: %v", err)
	}

	versions, err := s.ListValueVersions(ctx, "app", "key")
	if err != nil {
		t.Fatalf("ListValueVersions: %v", err)
	}
	for _, v := range versions {
		if v == "1" {
			t.Error("version 1 should be destroyed")
		}
	}
}

// --- Tree-level versioning (not supported) ---

func TestTreeVersioningNotSupported(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_, err := s.ListTreeVersions(ctx, "any")
	if !store.IsNotSupported(err) {
		t.Errorf("ListTreeVersions: expected ErrNotSupported, got %v", err)
	}

	_, err = s.GetTreeVersion(ctx, "any", "v1", nil)
	if !store.IsNotSupported(err) {
		t.Errorf("GetTreeVersion: expected ErrNotSupported, got %v", err)
	}

	if err := s.RollbackTree(ctx, "any", "v1"); !store.IsNotSupported(err) {
		t.Errorf("RollbackTree: expected ErrNotSupported, got %v", err)
	}

	if err := s.DeleteTreeVersion(ctx, "any", "v1"); !store.IsNotSupported(err) {
		t.Errorf("DeleteTreeVersion: expected ErrNotSupported, got %v", err)
	}
}

// --- Encryption (no-ops) ---

func TestEncryptionNoOps(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	if err := s.InitEncryption(ctx, []byte("pass")); err != nil {
		t.Errorf("InitEncryption should succeed (no-op), got: %v", err)
	}
	if err := s.RotateEncryption(ctx, []byte("old"), []byte("new")); err != nil {
		t.Errorf("RotateEncryption should succeed (no-op), got: %v", err)
	}
}

// --- Access control ---

func TestGrantAndListAccess(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	perms := []store.Permission{
		{Path: "db/host", Actions: []store.Action{store.ActionRead, store.ActionWrite}},
		{Path: "app/", Actions: []store.Action{store.ActionRead}},
	}

	if err := s.GrantAccess(ctx, "prod", "alice", perms); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	access, err := s.ListAccess(ctx, "prod")
	if err != nil {
		t.Fatalf("ListAccess: %v", err)
	}
	if _, ok := access["alice"]; !ok {
		t.Fatal("expected alice in access map")
	}
}

func TestRevokeAccess(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	perms := []store.Permission{
		{Path: "db/host", Actions: []store.Action{store.ActionRead}},
		{Path: "db/port", Actions: []store.Action{store.ActionRead}},
	}
	_ = s.GrantAccess(ctx, "prod", "bob", perms)

	if err := s.RevokeAccess(ctx, "prod", "bob", []string{"db/host"}); err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}

	access, err := s.ListAccess(ctx, "prod")
	if err != nil {
		t.Fatalf("ListAccess: %v", err)
	}
	if bobPerms, ok := access["bob"]; ok {
		for _, p := range bobPerms {
			if p.Path == "db/host" {
				t.Error("db/host should have been revoked")
			}
		}
	}
}

func TestRevokeAllAccess(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	_ = s.GrantAccess(ctx, "prod", "charlie", []store.Permission{
		{Path: "db/host", Actions: []store.Action{store.ActionRead}},
	})

	if err := s.RevokeAccess(ctx, "prod", "charlie", nil); err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}

	access, err := s.ListAccess(ctx, "prod")
	if err != nil {
		t.Fatalf("ListAccess: %v", err)
	}
	if _, ok := access["charlie"]; ok {
		t.Error("charlie should have no access after full revocation")
	}
}

// --- Policy generation ---

func TestGeneratePolicy(t *testing.T) {
	perms := []store.Permission{
		{Path: "db/host", Actions: []store.Action{store.ActionRead, store.ActionWrite}},
		{Path: "app/", Actions: []store.Action{store.ActionRead}},
	}

	policy := generatePolicy("secret", "zhi", "prod", perms)
	if !strings.Contains(policy, `"secret/data/zhi/prod/app/*"`) {
		t.Errorf("policy should contain wildcard path, got:\n%s", policy)
	}
	if !strings.Contains(policy, `"read"`) {
		t.Errorf("policy should contain read capability, got:\n%s", policy)
	}
	if !strings.Contains(policy, `"create"`) {
		t.Errorf("policy should contain create capability for write action, got:\n%s", policy)
	}
}

func TestParsePolicyPermissions(t *testing.T) {
	perms := []store.Permission{
		{Path: "db/host", Actions: []store.Action{store.ActionRead, store.ActionWrite}},
	}
	policy := generatePolicy("secret", "zhi", "prod", perms)
	parsed := parsePolicyPermissions(policy, "secret", "zhi", "prod")

	if len(parsed) == 0 {
		t.Fatal("expected at least one permission from parsed policy")
	}

	var found bool
	for _, p := range parsed {
		if p.Path == "db/host" {
			found = true
			if len(p.Actions) == 0 {
				t.Error("expected actions for db/host")
			}
		}
	}
	if !found {
		t.Error("expected db/host in parsed permissions")
	}
}

// --- Constructor ---

func TestNewRequiresAddress(t *testing.T) {
	_, err := New(Config{Address: ""})
	if err == nil {
		t.Error("expected error for empty address")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Mount != "secret" {
		t.Errorf("Mount = %q, want %q", cfg.Mount, "secret")
	}
	if cfg.Prefix != "zhi" {
		t.Errorf("Prefix = %q, want %q", cfg.Prefix, "zhi")
	}
}

// --- Integration tests with a real Vault dev server ---

func TestIntegrationWithVaultDev(t *testing.T) {
	addr := os.Getenv("VAULT_ADDR")
	token := os.Getenv("VAULT_TOKEN")
	if addr == "" || token == "" {
		t.Skip("Skipping: set VAULT_ADDR and VAULT_TOKEN (e.g. vault server -dev)")
	}

	s, err := New(Config{
		Address: addr,
		Token:   token,
		Mount:   "secret",
		Prefix:  "zhi-test",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Stop()
	ctx := context.Background()

	// Validate token.
	cred, err := s.Login(ctx, "token", map[string]string{"token": token})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if cred.Token != token {
		t.Errorf("Token = %q, want %q", cred.Token, token)
	}

	// Put and get.
	err = s.PutValues(ctx, "integration", map[string]config.Value{
		"db/host": {Val: "localhost", Metadata: map[string]any{"env": "test"}},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := s.GetValues(ctx, "integration", []string{"db/host"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if got["db/host"].Val != "localhost" {
		t.Errorf("db/host = %v, want %q", got["db/host"].Val, "localhost")
	}

	// Cleanup.
	_ = s.DeleteTree(ctx, "integration")
}

// --- Store label tests ---

func TestWriteonlyLabel(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	meta := labels.NewBuilder().Writeonly().Build()
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"secret/key": {Val: "super-secret", Metadata: meta},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := s.GetValues(ctx, "app", []string{"secret/key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}

	val, ok := got["secret/key"]
	if !ok {
		t.Fatal("expected secret/key to be present in results")
	}
	if val.Val != nil {
		t.Errorf("write-only value should be nil, got %v", val.Val)
	}
	// Metadata should still be preserved so the label is visible.
	if !labels.IsWriteonly(val.Metadata) {
		t.Error("metadata should preserve store.writeonly label")
	}
}

func TestWriteonlyLabelNotSetReturnsValue(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	err := s.PutValues(ctx, "app", map[string]config.Value{
		"normal/key": {Val: "visible"},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := s.GetValues(ctx, "app", []string{"normal/key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if got["normal/key"].Val != "visible" {
		t.Errorf("non-writeonly value should be returned, got %v", got["normal/key"].Val)
	}
}

func TestNoVersionLabel(t *testing.T) {
	s, mock := newTestStore(t)
	ctx := context.Background()

	meta := labels.NewBuilder().NoVersion().Build()
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"ephemeral/key": {Val: "data", Metadata: meta},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	// Verify the mock recorded max_versions=1 on the metadata.
	sm, ok := mock.secretMetadata["zhi/app/ephemeral/key"]
	if !ok {
		t.Fatal("expected secret metadata to be set for noversion path")
	}
	if sm.MaxVersions != 1 {
		t.Errorf("max_versions = %d, want 1", sm.MaxVersions)
	}
}

func TestTTLLabel(t *testing.T) {
	s, mock := newTestStore(t)
	ctx := context.Background()

	meta := labels.NewBuilder().TTL(3600).Build()
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"temp/key": {Val: "expires", Metadata: meta},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	sm, ok := mock.secretMetadata["zhi/app/temp/key"]
	if !ok {
		t.Fatal("expected secret metadata to be set for TTL path")
	}
	if sm.DeleteVersionAfter != "3600s" {
		t.Errorf("delete_version_after = %q, want %q", sm.DeleteVersionAfter, "3600s")
	}
}

func TestMaxVersionsLabel(t *testing.T) {
	s, mock := newTestStore(t)
	ctx := context.Background()

	meta := labels.NewBuilder().MaxVersions(5).Build()
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"versioned/key": {Val: "data", Metadata: meta},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	sm, ok := mock.secretMetadata["zhi/app/versioned/key"]
	if !ok {
		t.Fatal("expected secret metadata to be set for maxversions path")
	}
	if sm.MaxVersions != 5 {
		t.Errorf("max_versions = %d, want 5", sm.MaxVersions)
	}
}

func TestNoVersionOverridesMaxVersions(t *testing.T) {
	s, mock := newTestStore(t)
	ctx := context.Background()

	// When both noversion and maxversions are set, noversion wins.
	meta := labels.NewBuilder().NoVersion().MaxVersions(10).Build()
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"conflict/key": {Val: "data", Metadata: meta},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	sm, ok := mock.secretMetadata["zhi/app/conflict/key"]
	if !ok {
		t.Fatal("expected secret metadata to be set")
	}
	if sm.MaxVersions != 1 {
		t.Errorf("max_versions = %d, want 1 (noversion should override)", sm.MaxVersions)
	}
}

func TestCASFromVersion(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	// Write version 1.
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"cas/key": {Val: "first"},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues v1: %v", err)
	}

	// Read back to get the Version field populated.
	got, err := s.GetValues(ctx, "app", []string{"cas/key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	v := got["cas/key"]
	if v.Version != "1" {
		t.Fatalf("expected Version %q, got %q", "1", v.Version)
	}

	// Write version 2 using the Version from the read (automatic CAS).
	v.Val = "second"
	err = s.PutValues(ctx, "app", map[string]config.Value{
		"cas/key": v,
	}, nil)
	if err != nil {
		t.Fatalf("PutValues with Version CAS: %v", err)
	}

	// Write version 3 with stale Version=1 (should fail).
	err = s.PutValues(ctx, "app", map[string]config.Value{
		"cas/key": {Val: "third", Version: "1"},
	}, nil)
	if err == nil {
		t.Fatal("expected CAS conflict error from stale Version")
	}
	if !store.IsCASConflict(err) {
		t.Errorf("expected ErrCASConflict, got %T: %v", err, err)
	}
}

func TestCASOptionsOverrideVersion(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	// Write version 1.
	_ = s.PutValues(ctx, "app", map[string]config.Value{
		"cas/key": {Val: "first"},
	}, nil)

	// Value.Version says 5 (wrong), but PutOptions says 1 (correct).
	// PutOptions should take precedence.
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"cas/key": {Val: "second", Version: "5"},
	}, &store.PutOptions{CASVersions: map[string]string{"cas/key": "1"}})
	if err != nil {
		t.Fatalf("PutValues with CAS option override: %v", err)
	}

	got, _ := s.GetValues(ctx, "app", []string{"cas/key"})
	if got["cas/key"].Val != "second" {
		t.Errorf("value should be updated, got %v", got["cas/key"].Val)
	}
}

func TestGetValuesPopulatesVersion(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	// Write two versions.
	_ = s.PutValues(ctx, "app", map[string]config.Value{
		"ver/key": {Val: "v1"},
	}, nil)
	_ = s.PutValues(ctx, "app", map[string]config.Value{
		"ver/key": {Val: "v2"},
	}, nil)

	got, err := s.GetValues(ctx, "app", []string{"ver/key"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	v := got["ver/key"]
	if v.Version != "2" {
		t.Errorf("Version = %q, want %q", v.Version, "2")
	}
	if v.Val != "v2" {
		t.Errorf("Val = %v, want %q", v.Val, "v2")
	}
}

func TestNoLabelsSkipsMetadataCall(t *testing.T) {
	s, mock := newTestStore(t)
	ctx := context.Background()

	// Write a value with no store labels. No metadata POST should be made.
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"plain/key": {Val: "data"},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	if _, ok := mock.secretMetadata["zhi/app/plain/key"]; ok {
		t.Error("expected no secret metadata to be set for plain value")
	}
}

func TestCombinedLabels(t *testing.T) {
	s, mock := newTestStore(t)
	ctx := context.Background()

	// Combine TTL and maxversions labels.
	meta := labels.NewBuilder().TTL(7200).MaxVersions(3).Build()
	err := s.PutValues(ctx, "app", map[string]config.Value{
		"combined/key": {Val: "data", Metadata: meta},
	}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	sm, ok := mock.secretMetadata["zhi/app/combined/key"]
	if !ok {
		t.Fatal("expected secret metadata to be set")
	}
	if sm.MaxVersions != 3 {
		t.Errorf("max_versions = %d, want 3", sm.MaxVersions)
	}
	if sm.DeleteVersionAfter != "7200s" {
		t.Errorf("delete_version_after = %q, want %q", sm.DeleteVersionAfter, "7200s")
	}
}

// --- TLS configuration tests ---

func TestDefaultConfigReadsVaultEnvVars(t *testing.T) {
	// Save and restore env.
	envVars := []string{
		"VAULT_ADDR", "VAULT_TOKEN", "VAULT_NAMESPACE",
		"VAULT_CACERT", "VAULT_CLIENT_CERT", "VAULT_CLIENT_KEY", "VAULT_SKIP_VERIFY",
	}
	saved := make(map[string]string)
	for _, k := range envVars {
		saved[k] = os.Getenv(k)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	os.Setenv("VAULT_ADDR", "https://vault.example.com:8200")
	os.Setenv("VAULT_TOKEN", "hvs.test")
	os.Setenv("VAULT_NAMESPACE", "myns")
	os.Setenv("VAULT_CACERT", "/path/to/ca.pem")
	os.Setenv("VAULT_CLIENT_CERT", "/path/to/cert.pem")
	os.Setenv("VAULT_CLIENT_KEY", "/path/to/key.pem")
	os.Setenv("VAULT_SKIP_VERIFY", "true")

	cfg := DefaultConfig()

	if cfg.Address != "https://vault.example.com:8200" {
		t.Errorf("Address = %q, want %q", cfg.Address, "https://vault.example.com:8200")
	}
	if cfg.Token != "hvs.test" {
		t.Errorf("Token = %q, want %q", cfg.Token, "hvs.test")
	}
	if cfg.Namespace != "myns" {
		t.Errorf("Namespace = %q, want %q", cfg.Namespace, "myns")
	}
	if cfg.CACert != "/path/to/ca.pem" {
		t.Errorf("CACert = %q, want %q", cfg.CACert, "/path/to/ca.pem")
	}
	if cfg.ClientCert != "/path/to/cert.pem" {
		t.Errorf("ClientCert = %q, want %q", cfg.ClientCert, "/path/to/cert.pem")
	}
	if cfg.ClientKey != "/path/to/key.pem" {
		t.Errorf("ClientKey = %q, want %q", cfg.ClientKey, "/path/to/key.pem")
	}
	if !cfg.SkipVerify {
		t.Error("SkipVerify should be true")
	}
}

func TestDefaultConfigSkipVerifyVariants(t *testing.T) {
	saved := os.Getenv("VAULT_SKIP_VERIFY")
	t.Cleanup(func() {
		if saved == "" {
			os.Unsetenv("VAULT_SKIP_VERIFY")
		} else {
			os.Setenv("VAULT_SKIP_VERIFY", saved)
		}
	})

	tests := []struct {
		envVal string
		want   bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run("VAULT_SKIP_VERIFY="+tt.envVal, func(t *testing.T) {
			if tt.envVal == "" {
				os.Unsetenv("VAULT_SKIP_VERIFY")
			} else {
				os.Setenv("VAULT_SKIP_VERIFY", tt.envVal)
			}
			cfg := DefaultConfig()
			if cfg.SkipVerify != tt.want {
				t.Errorf("SkipVerify = %v, want %v", cfg.SkipVerify, tt.want)
			}
		})
	}
}

func TestNewWithTLSSkipVerify(t *testing.T) {
	mock := newMockVault()
	srv := httptest.NewServer(mock.handler())
	t.Cleanup(srv.Close)

	s, err := New(Config{
		Address:    srv.URL,
		Token:      "test",
		SkipVerify: true,
	})
	if err != nil {
		t.Fatalf("New with SkipVerify: %v", err)
	}
	t.Cleanup(s.Stop)

	// Verify the client works.
	ctx := context.Background()
	_, err = s.Capabilities(ctx)
	if err != nil {
		t.Fatalf("Capabilities with SkipVerify: %v", err)
	}
}

func TestNewWithBadCACertPath(t *testing.T) {
	_, err := New(Config{
		Address: "https://vault.example.com:8200",
		Token:   "test",
		CACert:  "/nonexistent/ca.pem",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent CA cert")
	}
	if !strings.Contains(err.Error(), "configuring vault TLS") {
		t.Errorf("error = %q, want to contain 'configuring vault TLS'", err.Error())
	}
}

func TestNewWithBadClientCertPath(t *testing.T) {
	_, err := New(Config{
		Address:    "https://vault.example.com:8200",
		Token:      "test",
		ClientCert: "/nonexistent/cert.pem",
		ClientKey:  "/nonexistent/key.pem",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent client cert")
	}
}

func TestBuildHTTPClientDefault(t *testing.T) {
	// No TLS config should return a plain client with timeout.
	client, err := buildHTTPClient(Config{Address: "http://localhost:8200"})
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", client.Timeout)
	}
}

func TestEncodeValueError(t *testing.T) {
	// A channel is not JSON-marshalable.
	v := config.Value{Val: make(chan int)}
	_, err := encodeValue(v)
	if err == nil {
		t.Fatal("expected error for unmarshalable value, got nil")
	}
	if !strings.Contains(err.Error(), "marshaling value") {
		t.Errorf("error = %q, want to contain %q", err, "marshaling value")
	}

	// Unmarshalable metadata.
	v2 := config.Value{
		Val:      "ok",
		Metadata: map[string]any{"bad": make(chan int)},
	}
	_, err = encodeValue(v2)
	if err == nil {
		t.Fatal("expected error for unmarshalable metadata, got nil")
	}
	if !strings.Contains(err.Error(), "marshaling metadata") {
		t.Errorf("error = %q, want to contain %q", err, "marshaling metadata")
	}

	// Valid value should succeed.
	v3 := config.Value{Val: "hello", Metadata: map[string]any{"key": "val"}}
	encoded, err := encodeValue(v3)
	if err != nil {
		t.Fatalf("encodeValue: %v", err)
	}
	if encoded["zhi_val"] != `"hello"` {
		t.Errorf("zhi_val = %q, want %q", encoded["zhi_val"], `"hello"`)
	}
}

func TestExtractQueryParam(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		param  string
		want   string
	}{
		{
			name:   "basic",
			rawURL: "https://example.com/auth?state=abc123&code=xyz",
			param:  "state",
			want:   "abc123",
		},
		{
			name:   "url encoded value",
			rawURL: "https://example.com/auth?state=abc%3D123&code=xyz",
			param:  "state",
			want:   "abc=123",
		},
		{
			name:   "substring param name",
			rawURL: "https://example.com/auth?substate=wrong&state=correct",
			param:  "state",
			want:   "correct",
		},
		{
			name:   "missing param",
			rawURL: "https://example.com/auth?code=xyz",
			param:  "state",
			want:   "",
		},
		{
			name:   "with fragment",
			rawURL: "https://example.com/auth?state=abc#fragment",
			param:  "state",
			want:   "abc",
		},
		{
			name:   "invalid URL",
			rawURL: "://not-a-url",
			param:  "state",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractQueryParam(tt.rawURL, tt.param)
			if got != tt.want {
				t.Errorf("extractQueryParam(%q, %q) = %q, want %q", tt.rawURL, tt.param, got, tt.want)
			}
		})
	}
}

func TestBuildHTTPClientWithSkipVerify(t *testing.T) {
	client, err := buildHTTPClient(Config{
		Address:    "https://vault.example.com:8200",
		SkipVerify: true,
	})
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}
