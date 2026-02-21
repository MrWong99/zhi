package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		case r.Method == http.MethodPut && startsWith(r.URL.Path, "/v1/sys/policies/acl/"):
			mu.Lock()
			policiesWritten = append(policiesWritten, lastSegment(r.URL.Path))
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

	admin := newAdminClient(srv.URL, "token", "", nil)
	cm := &credManager{
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
	apps := []appConfig{{Name: "web-api", Auth: "approle"}}

	err := cm.reconcilePolicies(context.Background(), values, apps)
	if err != nil {
		t.Fatalf("reconcilePolicies: %v", err)
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

	creds := map[string]*appCredentials{
		"web-api": {
			AuthMethod:      "approle",
			RoleID:          "role-123",
			WrappedSecretID: "hvs.wrapped",
		},
	}

	result := injectCredentials(values, creds)

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
}
