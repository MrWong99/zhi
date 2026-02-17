package core

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// stubConfig is a minimal config.Plugin for testing.
type stubConfig struct{}

func (s *stubConfig) List(context.Context) ([]string, error) { return nil, nil }
func (s *stubConfig) Get(context.Context, string) (config.Value, bool, error) {
	return config.Value{}, false, nil
}
func (s *stubConfig) Set(context.Context, string, config.Value) error { return nil }
func (s *stubConfig) Validate(context.Context, string, config.TreeReader) ([]config.ValidationResult, error) {
	return nil, nil
}

// stubTransform is a minimal transform.Plugin for testing.
type stubTransform struct{}

func (s *stubTransform) BeforeDisplay(context.Context, *config.Tree) error { return nil }
func (s *stubTransform) AfterSave(context.Context, *config.Tree) error     { return nil }
func (s *stubTransform) ValidatePolicy(context.Context) (transform.ValidatePolicy, error) {
	return transform.ValidateBeforeTransform, nil
}

// stubStore is a minimal store.Plugin for testing.
type stubStore struct{}

func (s *stubStore) Capabilities(context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{}, nil
}
func (s *stubStore) AuthMethods(context.Context) ([]store.AuthMethod, error) { return nil, nil }
func (s *stubStore) Login(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, nil
}
func (s *stubStore) LoginInteractive(context.Context, string, map[string]string) (*store.InteractiveChallenge, error) {
	return nil, fmt.Errorf("interactive login not supported")
}
func (s *stubStore) LoginInteractiveCallback(context.Context, string, map[string]string) (*store.Credential, error) {
	return nil, fmt.Errorf("interactive login not supported")
}
func (s *stubStore) ListTrees(context.Context) ([]string, error) { return nil, nil }
func (s *stubStore) DeleteTree(context.Context, string) error    { return nil }
func (s *stubStore) GetValues(context.Context, string, []string) (map[string]config.Value, error) {
	return nil, nil
}
func (s *stubStore) PutValues(context.Context, string, map[string]config.Value, *store.PutOptions) error {
	return nil
}
func (s *stubStore) DeleteValues(context.Context, string, []string) error       { return nil }
func (s *stubStore) ListTreeVersions(context.Context, string) ([]string, error) { return nil, nil }
func (s *stubStore) GetTreeVersion(context.Context, string, string, []string) (map[string]config.Value, error) {
	return nil, nil
}
func (s *stubStore) RollbackTree(context.Context, string, string) error      { return nil }
func (s *stubStore) DeleteTreeVersion(context.Context, string, string) error { return nil }
func (s *stubStore) ListValueVersions(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (s *stubStore) GetValueVersion(context.Context, string, string, string) (config.Value, bool, error) {
	return config.Value{}, false, nil
}
func (s *stubStore) RollbackValue(context.Context, string, string, string) error      { return nil }
func (s *stubStore) DeleteValueVersion(context.Context, string, string, string) error { return nil }
func (s *stubStore) InitEncryption(context.Context, []byte) error                     { return nil }
func (s *stubStore) RotateEncryption(context.Context, []byte, []byte) error           { return nil }
func (s *stubStore) GrantAccess(context.Context, string, string, []store.Permission) error {
	return nil
}
func (s *stubStore) RevokeAccess(context.Context, string, string, []string) error { return nil }
func (s *stubStore) ListAccess(context.Context, string) (map[string][]store.Permission, error) {
	return nil, nil
}

func TestRegistryRegisterAndResolve(t *testing.T) {
	r := NewRegistry()

	err := r.RegisterConfig("test-config", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})
	if err != nil {
		t.Fatalf("RegisterConfig: %v", err)
	}

	err = r.RegisterTransform("test-transform", func(string, map[string]any) (transform.Plugin, error) {
		return &stubTransform{}, nil
	})
	if err != nil {
		t.Fatalf("RegisterTransform: %v", err)
	}

	err = r.RegisterStore("test-store", func(string, map[string]any) (store.Plugin, error) {
		return &stubStore{}, nil
	})
	if err != nil {
		t.Fatalf("RegisterStore: %v", err)
	}

	t.Run("resolve config", func(t *testing.T) {
		p, err := r.ConfigProvider(".", "test-config", nil)
		if err != nil {
			t.Fatalf("ConfigProvider: %v", err)
		}
		if p == nil {
			t.Fatal("ConfigProvider returned nil")
		}
	})

	t.Run("resolve transform", func(t *testing.T) {
		p, err := r.TransformProvider(".", "test-transform", nil)
		if err != nil {
			t.Fatalf("TransformProvider: %v", err)
		}
		if p == nil {
			t.Fatal("TransformProvider returned nil")
		}
	})

	t.Run("resolve store", func(t *testing.T) {
		p, err := r.StoreProvider(".", "test-store", nil)
		if err != nil {
			t.Fatalf("StoreProvider: %v", err)
		}
		if p == nil {
			t.Fatal("StoreProvider returned nil")
		}
	})
}

func TestRegistryUnknownProvider(t *testing.T) {
	r := NewRegistry()

	_, err := r.ConfigProvider(".", "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown config provider")
	}

	_, err = r.TransformProvider(".", "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown transform provider")
	}

	_, err = r.StoreProvider(".", "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown store provider")
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	r := NewRegistry()
	factory := func(string, map[string]any) (config.Plugin, error) { return &stubConfig{}, nil }

	if err := r.RegisterConfig("dup", factory); err != nil {
		t.Fatalf("first RegisterConfig: %v", err)
	}
	if err := r.RegisterConfig("dup", factory); err == nil {
		t.Error("expected error for duplicate config registration")
	}

	tfactory := func(string, map[string]any) (transform.Plugin, error) { return &stubTransform{}, nil }
	if err := r.RegisterTransform("dup", tfactory); err != nil {
		t.Fatalf("first RegisterTransform: %v", err)
	}
	if err := r.RegisterTransform("dup", tfactory); err == nil {
		t.Error("expected error for duplicate transform registration")
	}

	sfactory := func(string, map[string]any) (store.Plugin, error) { return &stubStore{}, nil }
	if err := r.RegisterStore("dup", sfactory); err != nil {
		t.Fatalf("first RegisterStore: %v", err)
	}
	if err := r.RegisterStore("dup", sfactory); err == nil {
		t.Error("expected error for duplicate store registration")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterConfig("beta", func(string, map[string]any) (config.Plugin, error) { return &stubConfig{}, nil })
	_ = r.RegisterConfig("alpha", func(string, map[string]any) (config.Plugin, error) { return &stubConfig{}, nil })

	got := r.ListConfig()
	want := []string{"alpha", "beta"}
	if !slices.Equal(got, want) {
		t.Errorf("ListConfig() = %v, want %v", got, want)
	}

	if got := r.ListTransform(); len(got) != 0 {
		t.Errorf("ListTransform() = %v, want empty", got)
	}
	if got := r.ListStore(); len(got) != 0 {
		t.Errorf("ListStore() = %v, want empty", got)
	}
}

func TestDefaultRegistryHasStructuredfile(t *testing.T) {
	r := DefaultRegistry()
	got := r.ListConfig()
	want := []string{"structuredfile"}
	if !slices.Equal(got, want) {
		t.Errorf("DefaultRegistry ListConfig() = %v, want %v", got, want)
	}
}

func TestRegistryListIncludesExternal(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterConfig("builtin", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	r.SetExternalPlugins([]PluginInfo{
		{Name: "ext-config", Type: PluginTypeConfig, Path: "/fake/zhi-config-ext"},
		{Name: "ext-transform", Type: PluginTypeTransform, Path: "/fake/zhi-transform-ext"},
		{Name: "ext-store", Type: PluginTypeStore, Path: "/fake/zhi-store-ext"},
	})

	configs := r.ListConfig()
	if !slices.Contains(configs, "builtin") {
		t.Error("ListConfig missing built-in provider")
	}
	if !slices.Contains(configs, "ext-config") {
		t.Error("ListConfig missing external provider")
	}

	transforms := r.ListTransform()
	if !slices.Contains(transforms, "ext-transform") {
		t.Error("ListTransform missing external provider")
	}

	stores := r.ListStore()
	if !slices.Contains(stores, "ext-store") {
		t.Error("ListStore missing external provider")
	}
}

func TestRegistryListExternalDoesNotDuplicate(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterConfig("shared", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	// External plugin with same name as built-in.
	r.SetExternalPlugins([]PluginInfo{
		{Name: "shared", Type: PluginTypeConfig, Path: "/fake/zhi-config-shared"},
	})

	configs := r.ListConfig()
	count := 0
	for _, name := range configs {
		if name == "shared" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 'shared' in ListConfig, got %d", count)
	}
}

func TestRegistryListProviderInfos(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterConfig("builtin", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	r.SetExternalPlugins([]PluginInfo{
		{Name: "ext", Type: PluginTypeConfig, Path: "/plugins/zhi-config-ext"},
	})

	infos := r.ListConfigProviders()
	if len(infos) != 2 {
		t.Fatalf("got %d infos, want 2", len(infos))
	}

	// First should be built-in (sorted).
	if infos[0].Name != "builtin" || infos[0].Source != "built-in" {
		t.Errorf("infos[0] = %+v, want builtin/built-in", infos[0])
	}
	if infos[1].Name != "ext" || infos[1].Source != "/plugins/zhi-config-ext" {
		t.Errorf("infos[1] = %+v, want ext/external-path", infos[1])
	}
}

func TestRegistryListProviderInfosExternalSameNameAsBuiltin(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterConfig("shared", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	r.SetExternalPlugins([]PluginInfo{
		{Name: "shared", Type: PluginTypeConfig, Path: "/plugins/zhi-config-shared"},
	})

	// Built-in takes precedence; external with same name should not appear.
	infos := r.ListConfigProviders()
	if len(infos) != 1 {
		t.Fatalf("got %d infos, want 1 (external duplicate should be filtered)", len(infos))
	}
	if infos[0].Source != "built-in" {
		t.Errorf("expected built-in source, got %q", infos[0].Source)
	}
}

func TestRegistryBuiltinTakesPrecedenceOverExternal(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterConfig("test", func(string, map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})

	r.SetExternalPlugins([]PluginInfo{
		{Name: "test", Type: PluginTypeConfig, Path: "/fake/zhi-config-test"},
	})

	// Should resolve the built-in, not the external.
	p, err := r.ConfigProvider(".", "test", nil)
	if err != nil {
		t.Fatalf("ConfigProvider: %v", err)
	}
	if p == nil {
		t.Fatal("ConfigProvider returned nil")
	}
}

func TestRegistryExternalFallbackUnknown(t *testing.T) {
	r := NewRegistry()

	// No external plugins set, should fail.
	_, err := r.ConfigProvider(".", "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown config provider without externals")
	}
}

func TestRegistryClose(t *testing.T) {
	r := NewRegistry()

	cleanupCalled := 0
	r.mu.Lock()
	r.cleanups = append(r.cleanups, func() { cleanupCalled++ })
	r.cleanups = append(r.cleanups, func() { cleanupCalled++ })
	r.cachedConfig["test"] = &stubConfig{}
	r.mu.Unlock()

	r.Close()

	if cleanupCalled != 2 {
		t.Errorf("cleanup called %d times, want 2", cleanupCalled)
	}

	r.mu.Lock()
	if len(r.cachedConfig) != 0 {
		t.Error("cachedConfig not cleared after Close")
	}
	if len(r.cleanups) != 0 {
		t.Error("cleanups not cleared after Close")
	}
	r.mu.Unlock()
}

func TestRegistryCloseIdempotent(t *testing.T) {
	r := NewRegistry()

	cleanupCalled := 0
	r.mu.Lock()
	r.cleanups = append(r.cleanups, func() { cleanupCalled++ })
	r.mu.Unlock()

	r.Close()
	r.Close() // should not panic or call cleanup again

	if cleanupCalled != 1 {
		t.Errorf("cleanup called %d times, want 1", cleanupCalled)
	}
}

func TestRegistryRefreshExternal(t *testing.T) {
	r := NewRegistry()

	// RefreshExternal with a non-existent dir should succeed (empty result).
	err := r.RefreshExternal(DiscoveryConfig{Directories: []string{"/nonexistent"}})
	if err != nil {
		t.Fatalf("RefreshExternal: %v", err)
	}

	if len(r.externalPlugins) != 0 {
		t.Errorf("expected 0 external plugins, got %d", len(r.externalPlugins))
	}
}

func TestRegistrySetExternalPlugins(t *testing.T) {
	r := NewRegistry()

	plugins := []PluginInfo{
		{Name: "a", Type: PluginTypeConfig, Path: "/a"},
		{Name: "b", Type: PluginTypeStore, Path: "/b"},
	}
	r.SetExternalPlugins(plugins)

	if len(r.externalPlugins) != 2 {
		t.Errorf("expected 2 external plugins, got %d", len(r.externalPlugins))
	}
}
