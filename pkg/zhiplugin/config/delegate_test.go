package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// mockConfigPlugin is a minimal config.Plugin for testing delegation.
type mockConfigPlugin struct {
	listPaths []string
	getValue  config.Value
	getFound  bool
	setErr    error
	valResult []config.ValidationResult
}

func (m *mockConfigPlugin) List(_ context.Context) ([]string, error) {
	return m.listPaths, nil
}

func (m *mockConfigPlugin) Get(_ context.Context, _ string) (config.Value, bool, error) {
	return m.getValue, m.getFound, nil
}

func (m *mockConfigPlugin) Set(_ context.Context, _ string, _ config.Value) error {
	return m.setErr
}

func (m *mockConfigPlugin) Validate(_ context.Context, _ string, _ config.TreeReader) ([]config.ValidationResult, error) {
	return m.valResult, nil
}

func TestConfigDelegatingPlugin_Delegates(t *testing.T) {
	t.Parallel()
	mock := &mockConfigPlugin{
		listPaths: []string{"a/b", "c/d"},
		getValue:  config.Value{Val: "hello"},
		getFound:  true,
		valResult: []config.ValidationResult{{Severity: config.Info, Message: "ok"}},
	}

	dp := config.NewDelegatingPlugin(mock)
	ctx := context.Background()

	// List
	paths, err := dp.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("List returned %d paths, want 2", len(paths))
	}

	// Get
	v, found, err := dp.Get(ctx, "a/b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Error("Get: expected found=true")
	}
	if v.Val != "hello" {
		t.Errorf("Get: Val=%v, want %q", v.Val, "hello")
	}

	// Set
	if err := dp.Set(ctx, "a/b", config.Value{Val: "x"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Validate
	results, err := dp.Validate(ctx, "a/b", nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Validate returned %d results, want 1", len(results))
	}
}

func TestConfigDelegatingPlugin_SetError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("write failed")
	mock := &mockConfigPlugin{setErr: wantErr}
	dp := config.NewDelegatingPlugin(mock)

	err := dp.Set(context.Background(), "x", config.Value{Val: 1})
	if !errors.Is(err, wantErr) {
		t.Errorf("Set error = %v, want %v", err, wantErr)
	}
}

func TestConfigDelegatingPlugin_EmbedOverride(t *testing.T) {
	t.Parallel()
	mock := &mockConfigPlugin{listPaths: []string{"original"}}

	type custom struct {
		config.DelegatingPlugin
	}

	c := &custom{DelegatingPlugin: config.NewDelegatingPlugin(mock)}
	// Override List to add custom behavior.
	// We test this by calling the embedded base and checking it works.
	paths, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(paths) != 1 || paths[0] != "original" {
		t.Errorf("List = %v, want [original]", paths)
	}
}

func TestConfigNewDelegatingPlugin_PanicsOnNil(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil base")
		}
	}()
	config.NewDelegatingPlugin(nil)
}
