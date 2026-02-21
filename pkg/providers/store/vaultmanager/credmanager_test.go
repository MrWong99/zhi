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

func TestCredManager_ReconcilePolicies(t *testing.T) {
	var policiesWritten []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/sys/policies/acl/"):
			mu.Lock()
			parts := strings.Split(r.URL.Path, "/")
			policiesWritten = append(policiesWritten, parts[len(parts)-1])
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "LIST" && r.URL.Path == "/v1/sys/policies/acl":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []string{"zhi-default-old-app"}},
			})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	admin := NewAdminClient(srv.URL, "token", "", nil)
	cm := &CredManager{
		admin:     admin,
		mount:     "secret",
		prefix:    "zhi",
		workspace: "default",
	}

	values := map[string]config.Value{
		"database/host": {
			Val:      "localhost",
			Metadata: map[string]any{"vault.app.web-api": "read"},
		},
	}
	apps := []AppConfig{{Name: "web-api", Auth: "approle"}}

	err := cm.ReconcilePolicies(context.Background(), values, apps)
	if err != nil {
		t.Fatalf("ReconcilePolicies: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(policiesWritten) != 1 || policiesWritten[0] != "zhi-default-web-api" {
		t.Errorf("expected policy zhi-default-web-api, got %v", policiesWritten)
	}
}

func TestCredManager_InjectCredentials(t *testing.T) {
	values := map[string]config.Value{
		"database/host": {Val: "localhost"},
	}

	t.Run("wrapped", func(t *testing.T) {
		creds := map[string]*AppCredentials{
			"web-api": {
				AuthMethod: "approle",
				RoleID:     "role-123",
				SecretID:   "hvs.wrapped",
				Wrapped:    true,
			},
		}

		result := InjectCredentials(values, creds)

		roleID, ok := result["vault/credentials/web-api/role-id"]
		if !ok {
			t.Fatal("missing role-id")
		}
		if roleID.Val != "role-123" {
			t.Errorf("role-id = %v", roleID.Val)
		}
		if !labels.IsHidden(roleID.Metadata) {
			t.Error("role-id should be ui.hidden")
		}
		if !labels.IsEphemeral(roleID.Metadata) {
			t.Error("role-id should be store.ephemeral")
		}

		wrappedSID, ok := result["vault/credentials/web-api/wrapped-secret-id"]
		if !ok {
			t.Fatal("missing wrapped-secret-id")
		}
		if wrappedSID.Val != "hvs.wrapped" {
			t.Errorf("wrapped-secret-id = %v", wrappedSID.Val)
		}
	})

	t.Run("unwrapped", func(t *testing.T) {
		creds := map[string]*AppCredentials{
			"web-api": {
				AuthMethod: "approle",
				RoleID:     "role-123",
				SecretID:   "plain-secret-id",
				Wrapped:    false,
			},
		}

		result := InjectCredentials(values, creds)

		sid, ok := result["vault/credentials/web-api/secret-id"]
		if !ok {
			t.Fatal("missing secret-id for unwrapped mode")
		}
		if sid.Val != "plain-secret-id" {
			t.Errorf("secret-id = %v", sid.Val)
		}

		_, hasWrapped := result["vault/credentials/web-api/wrapped-secret-id"]
		if hasWrapped {
			t.Error("should not have wrapped-secret-id in unwrapped mode")
		}
	})
}
