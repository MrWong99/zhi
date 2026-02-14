package ui_test

import (
	"context"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/ui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

func TestControllerAdapter_StoreAuthMethods(t *testing.T) {
	ctrl := setupTestController(t)
	adapter := &ui.ControllerAdapter{Inner: ctrl}
	ctx := context.Background()

	methods, err := adapter.StoreAuthMethods(ctx)
	if err != nil {
		t.Fatalf("StoreAuthMethods: %v", err)
	}
	// Default mock store has no auth methods.
	if len(methods) != 0 {
		t.Errorf("expected 0 methods, got %d", len(methods))
	}
}

func TestControllerAdapter_StoreAuthStatus(t *testing.T) {
	ctrl := setupTestController(t)
	adapter := &ui.ControllerAdapter{Inner: ctrl}
	ctx := context.Background()

	session, err := adapter.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestControllerAdapter_StoreLogout(t *testing.T) {
	ctrl := setupTestController(t)
	adapter := &ui.ControllerAdapter{Inner: ctrl}
	ctx := context.Background()

	if err := adapter.StoreLogout(ctx); err != nil {
		t.Fatalf("StoreLogout: %v", err)
	}

	session, err := adapter.StoreAuthStatus(ctx)
	if err != nil {
		t.Fatalf("StoreAuthStatus: %v", err)
	}
	if session.Status != zhiui.StoreSessionUnauthenticated {
		t.Errorf("expected unauthenticated, got %q", session.Status)
	}
}

func TestControllerAdapter_StoreLogin(t *testing.T) {
	// Use the auth mock store so Login actually returns a credential.
	reg := core.NewRegistry()
	mc := newMockConfig()
	ms := &authMockStore{mockStore: newMockStore()}

	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return mc, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterTransform("mock", func(string, map[string]any) (transform.Plugin, error) {
		return &mockTransform{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterStore("mock", func(string, map[string]any) (store.Plugin, error) {
		return ms, nil
	}); err != nil {
		t.Fatal(err)
	}

	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
		Transform: []core.ProviderRef{
			{Provider: "mock"},
		},
		Store: core.ProviderRef{Provider: "mock"},
		Dir:   t.TempDir(),
	}

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatal(err)
	}

	inner := ui.NewUIController(eng)
	adapter := &ui.ControllerAdapter{Inner: inner}
	ctx := context.Background()

	session, err := adapter.StoreLogin(ctx, "userpass", map[string]string{
		"username": "admin",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("StoreLogin: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if session.Status != zhiui.StoreSessionAuthenticated {
		t.Errorf("expected authenticated, got %q", session.Status)
	}
}
