package vaultmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func TestStore_PutValues_FiltersEphemeral(t *testing.T) {
	mock := &mockStorePlugin{}
	s := &Store{
		DelegatingPlugin: store.NewDelegatingPlugin(mock),
	}

	values := map[string]config.Value{
		"database/host": {Val: "localhost"},
		"vault/credentials/web-api/role-id": {
			Val:      "role-123",
			Metadata: labels.NewBuilder().Ephemeral().Build(),
		},
	}

	err := s.PutValues(context.Background(), "tree1", values, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	if len(mock.putValues) != 1 {
		t.Errorf("expected 1 value forwarded, got %d", len(mock.putValues))
	}
	if _, ok := mock.putValues["vault/credentials/web-api/role-id"]; ok {
		t.Error("ephemeral value should have been filtered")
	}
	if _, ok := mock.putValues["database/host"]; !ok {
		t.Error("non-ephemeral value should have been forwarded")
	}
}

// mockStorePlugin captures calls for testing.
type mockStorePlugin struct {
	store.DelegatingPlugin
	putValues        map[string]config.Value
	getValues        map[string]config.Value
	loginCalled      bool
	loginMethod      string
	loginCredentials map[string]string
}

func (m *mockStorePlugin) PutValues(_ context.Context, _ string, values map[string]config.Value, _ *store.PutOptions) error {
	m.putValues = values
	return nil
}

func (m *mockStorePlugin) GetValues(_ context.Context, _ string, paths []string) (map[string]config.Value, error) {
	result := make(map[string]config.Value)
	for _, p := range paths {
		if v, ok := m.getValues[p]; ok {
			result[p] = v
		}
	}
	return result, nil
}

func (m *mockStorePlugin) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{Auth: true, AccessControl: true}, nil
}

func (m *mockStorePlugin) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	return nil, nil
}

func (m *mockStorePlugin) Login(_ context.Context, method string, creds map[string]string) (*store.Credential, error) {
	m.loginCalled = true
	m.loginMethod = method
	m.loginCredentials = creds
	return &store.Credential{Token: creds["token"]}, nil
}

func TestStore_Login_AuthenticatesChild(t *testing.T) {
	childToken := "s.child-scoped-token"
	var policyWritten string

	vaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v1/")

		switch {
		case path == "auth/token/lookup-self" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"policies": []string{"root"},
				},
			})

		case strings.HasPrefix(path, "sys/policies/acl/") && r.Method == http.MethodPut:
			var body struct {
				Policy string `json:"policy"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			policyWritten = body.Policy
			w.WriteHeader(http.StatusNoContent)

		case path == "auth/token/create" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"auth": map[string]any{
					"client_token": childToken,
				},
			})

		default:
			t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, fmt.Sprintf("unexpected: %s %s", r.Method, path), http.StatusNotFound)
		}
	}))
	defer vaultServer.Close()

	mock := &mockStorePlugin{}
	admin := NewAdminClient(vaultServer.URL, "", "", vaultServer.Client())

	cfg := &Config{
		Addr:   vaultServer.URL,
		Mount:  "secret",
		Prefix: "zhi",
	}

	s := &Store{
		DelegatingPlugin: store.NewDelegatingPlugin(mock),
		Admin:            admin,
		CredManager:      nil,
		Apps:             nil,
		Cfg:              cfg,
		Log:              hclog.NewNullLogger(),
	}

	cred, err := s.Login(context.Background(), "token", map[string]string{"token": "s.admin-token"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if cred.Token != "s.admin-token" {
		t.Errorf("admin credential token = %q, want s.admin-token", cred.Token)
	}

	if !mock.loginCalled {
		t.Fatal("child Login was not called")
	}
	if mock.loginMethod != "token" {
		t.Errorf("child login method = %q, want token", mock.loginMethod)
	}
	if mock.loginCredentials["token"] != childToken {
		t.Errorf("child token = %q, want %q", mock.loginCredentials["token"], childToken)
	}

	if policyWritten == "" {
		t.Fatal("no policy was written")
	}
	if !strings.Contains(policyWritten, "secret/data/zhi/*") {
		t.Errorf("policy missing data path, got:\n%s", policyWritten)
	}
	if !strings.Contains(policyWritten, "secret/metadata/zhi/*") {
		t.Errorf("policy missing metadata path, got:\n%s", policyWritten)
	}
}

func TestBuildChildStorePolicy(t *testing.T) {
	policy := BuildChildStorePolicy("secret", "zhi")

	expected := []string{
		`secret/data/zhi/*`,
		`secret/metadata/zhi/*`,
		`secret/destroy/zhi/*`,
	}
	for _, path := range expected {
		if !strings.Contains(policy, path) {
			t.Errorf("policy missing %q, got:\n%s", path, policy)
		}
	}

	// Custom mount/prefix.
	policy = BuildChildStorePolicy("kv", "myapp")
	if !strings.Contains(policy, "kv/data/myapp/*") {
		t.Errorf("policy missing kv/data/myapp/*, got:\n%s", policy)
	}
}
