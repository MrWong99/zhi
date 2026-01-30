package core

import (
	"context"
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

func (s *stubStore) Save(context.Context, string, config.TreeReader) error    { return nil }
func (s *stubStore) Load(context.Context, string) (*config.Tree, bool, error) { return nil, false, nil }
func (s *stubStore) Delete(context.Context, string) error                     { return nil }
func (s *stubStore) ListTrees(context.Context) ([]string, error)              { return nil, nil }
func (s *stubStore) SupportsVersioning(context.Context) (bool, error)         { return false, nil }
func (s *stubStore) ListVersions(context.Context, string) ([]string, error)   { return nil, nil }
func (s *stubStore) LoadVersion(context.Context, string, string) (*config.Tree, bool, error) {
	return nil, false, nil
}
func (s *stubStore) DeleteVersion(context.Context, string, string) error { return nil }
func (s *stubStore) EncryptionStatus(context.Context) (store.EncryptionStatus, error) {
	return store.EncryptionNone, nil
}
func (s *stubStore) InitEncryption(context.Context, []byte) error           { return nil }
func (s *stubStore) RotateEncryption(context.Context, []byte, []byte) error { return nil }

func TestRegistryRegisterAndResolve(t *testing.T) {
	r := NewRegistry()

	err := r.RegisterConfig("test-config", func(map[string]any) (config.Plugin, error) {
		return &stubConfig{}, nil
	})
	if err != nil {
		t.Fatalf("RegisterConfig: %v", err)
	}

	err = r.RegisterTransform("test-transform", func(map[string]any) (transform.Plugin, error) {
		return &stubTransform{}, nil
	})
	if err != nil {
		t.Fatalf("RegisterTransform: %v", err)
	}

	err = r.RegisterStore("test-store", func(map[string]any) (store.Plugin, error) {
		return &stubStore{}, nil
	})
	if err != nil {
		t.Fatalf("RegisterStore: %v", err)
	}

	t.Run("resolve config", func(t *testing.T) {
		p, err := r.ConfigProvider("test-config", nil)
		if err != nil {
			t.Fatalf("ConfigProvider: %v", err)
		}
		if p == nil {
			t.Fatal("ConfigProvider returned nil")
		}
	})

	t.Run("resolve transform", func(t *testing.T) {
		p, err := r.TransformProvider("test-transform", nil)
		if err != nil {
			t.Fatalf("TransformProvider: %v", err)
		}
		if p == nil {
			t.Fatal("TransformProvider returned nil")
		}
	})

	t.Run("resolve store", func(t *testing.T) {
		p, err := r.StoreProvider("test-store", nil)
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

	_, err := r.ConfigProvider("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown config provider")
	}

	_, err = r.TransformProvider("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown transform provider")
	}

	_, err = r.StoreProvider("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown store provider")
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	r := NewRegistry()
	factory := func(map[string]any) (config.Plugin, error) { return &stubConfig{}, nil }

	if err := r.RegisterConfig("dup", factory); err != nil {
		t.Fatalf("first RegisterConfig: %v", err)
	}
	if err := r.RegisterConfig("dup", factory); err == nil {
		t.Error("expected error for duplicate config registration")
	}

	tfactory := func(map[string]any) (transform.Plugin, error) { return &stubTransform{}, nil }
	if err := r.RegisterTransform("dup", tfactory); err != nil {
		t.Fatalf("first RegisterTransform: %v", err)
	}
	if err := r.RegisterTransform("dup", tfactory); err == nil {
		t.Error("expected error for duplicate transform registration")
	}

	sfactory := func(map[string]any) (store.Plugin, error) { return &stubStore{}, nil }
	if err := r.RegisterStore("dup", sfactory); err != nil {
		t.Fatalf("first RegisterStore: %v", err)
	}
	if err := r.RegisterStore("dup", sfactory); err == nil {
		t.Error("expected error for duplicate store registration")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterConfig("beta", func(map[string]any) (config.Plugin, error) { return &stubConfig{}, nil })
	_ = r.RegisterConfig("alpha", func(map[string]any) (config.Plugin, error) { return &stubConfig{}, nil })

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
