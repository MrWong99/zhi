package main

import (
	"context"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func TestVaultManagerStore_PutValues_FiltersEphemeral(t *testing.T) {
	mock := &mockStorePlugin{}
	s := &vaultManagerStore{
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
	putValues map[string]config.Value
	getValues map[string]config.Value
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

func (m *mockStorePlugin) Login(_ context.Context, _ string, _ map[string]string) (*store.Credential, error) {
	return &store.Credential{}, nil
}
