package config_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestMergedPlugin_List(t *testing.T) {
	t.Parallel()
	a := &mockConfigPlugin{listPaths: []string{"x", "y"}}
	b := &mockConfigPlugin{listPaths: []string{"z"}}

	mp, err := config.MergedPlugin(
		config.MountedPlugin{Impl: a, Prefix: "app-a/"},
		config.MountedPlugin{Impl: b, Prefix: "app-b/"},
	)
	if err != nil {
		t.Fatalf("MergedPlugin: %v", err)
	}

	paths, err := mp.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	sort.Strings(paths)
	want := []string{"app-a/x", "app-a/y", "app-b/z"}
	if len(paths) != len(want) {
		t.Fatalf("List returned %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestMergedPlugin_GetRoutes(t *testing.T) {
	t.Parallel()
	a := &mockConfigPlugin{getValue: config.Value{Val: "from-a"}, getFound: true}
	b := &mockConfigPlugin{getValue: config.Value{Val: "from-b"}, getFound: true}

	mp, err := config.MergedPlugin(
		config.MountedPlugin{Impl: a, Prefix: "a/"},
		config.MountedPlugin{Impl: b, Prefix: "b/"},
	)
	if err != nil {
		t.Fatalf("MergedPlugin: %v", err)
	}

	v, found, err := mp.Get(context.Background(), "a/key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found || v.Val != "from-a" {
		t.Errorf("Get(a/key) = %v, %v; want from-a, true", v.Val, found)
	}

	v, found, err = mp.Get(context.Background(), "b/key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found || v.Val != "from-b" {
		t.Errorf("Get(b/key) = %v, %v; want from-b, true", v.Val, found)
	}
}

func TestMergedPlugin_GetUnknownPath(t *testing.T) {
	t.Parallel()
	a := &mockConfigPlugin{}

	mp, err := config.MergedPlugin(
		config.MountedPlugin{Impl: a, Prefix: "a/"},
	)
	if err != nil {
		t.Fatalf("MergedPlugin: %v", err)
	}

	_, found, err := mp.Get(context.Background(), "unknown/path")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Error("Get should return found=false for unknown prefix")
	}
}

func TestMergedPlugin_SetRoutes(t *testing.T) {
	t.Parallel()
	a := &mockConfigPlugin{}

	mp, err := config.MergedPlugin(
		config.MountedPlugin{Impl: a, Prefix: "a/"},
	)
	if err != nil {
		t.Fatalf("MergedPlugin: %v", err)
	}

	err = mp.Set(context.Background(), "a/key", config.Value{Val: "v"})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	err = mp.Set(context.Background(), "unknown/key", config.Value{Val: "v"})
	if err == nil {
		t.Error("Set should error for unknown prefix")
	}
}

func TestMergedPlugin_OverlappingPrefixes(t *testing.T) {
	t.Parallel()
	a := &mockConfigPlugin{}

	_, err := config.MergedPlugin(
		config.MountedPlugin{Impl: a, Prefix: "app/"},
		config.MountedPlugin{Impl: a, Prefix: "app/sub/"},
	)
	if err == nil {
		t.Fatal("expected error for overlapping prefixes")
	}
}

func TestMergedPlugin_EmptyPrefix(t *testing.T) {
	t.Parallel()
	a := &mockConfigPlugin{}

	_, err := config.MergedPlugin(
		config.MountedPlugin{Impl: a, Prefix: ""},
	)
	if err == nil {
		t.Fatal("expected error for empty prefix")
	}
}

func TestMergedPlugin_NilImpl(t *testing.T) {
	t.Parallel()

	_, err := config.MergedPlugin(
		config.MountedPlugin{Impl: nil, Prefix: "a/"},
	)
	if err == nil {
		t.Fatal("expected error for nil Impl")
	}
}

func TestMergedPlugin_NoChildren(t *testing.T) {
	t.Parallel()
	_, err := config.MergedPlugin()
	if err == nil {
		t.Fatal("expected error for no children")
	}
}

func TestMergedPlugin_ListError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("list failed")
	a := &errorConfigPlugin{listErr: wantErr}

	mp, err := config.MergedPlugin(
		config.MountedPlugin{Impl: a, Prefix: "a/"},
	)
	if err != nil {
		t.Fatalf("MergedPlugin: %v", err)
	}

	_, err = mp.List(context.Background())
	if err == nil {
		t.Fatal("expected error from child List")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapping %v", err, wantErr)
	}
}

// errorConfigPlugin returns errors from configured methods.
type errorConfigPlugin struct {
	listErr error
}

func (e *errorConfigPlugin) List(_ context.Context) ([]string, error) {
	return nil, e.listErr
}
func (e *errorConfigPlugin) Get(_ context.Context, _ string) (config.Value, bool, error) {
	return config.Value{}, false, nil
}
func (e *errorConfigPlugin) Set(_ context.Context, _ string, _ config.Value) error {
	return nil
}
func (e *errorConfigPlugin) Validate(_ context.Context, _ string, _ config.TreeReader) ([]config.ValidationResult, error) {
	return nil, nil
}
