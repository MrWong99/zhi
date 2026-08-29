package doctor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// jsonStoreEngine builds a core.Engine wired up to the built-in jsonfile
// store under t.TempDir(). That exercises the real Engine code paths
// without depending on network I/O or a gRPC plugin process.
func jsonStoreEngine(t *testing.T) (*core.Engine, *core.WorkspaceConfig) {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.RegisterConfig("mock", func(string, map[string]any) (config.Plugin, error) {
		return newMockConfigPlugin(nil), nil
	}); err != nil {
		t.Fatalf("RegisterConfig: %v", err)
	}
	if err := reg.RegisterStore("jsonfile", core.NewJSONFileStoreProvider); err != nil {
		t.Fatalf("RegisterStore: %v", err)
	}
	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "mock"},
		Store:   core.ProviderRef{Provider: "jsonfile"},
		Dir:     t.TempDir(),
	}
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(eng.Close)
	return eng, ws
}

func TestStoreCapabilities_OK(t *testing.T) {
	eng, ws := jsonStoreEngine(t)
	env := &Environment{Workspace: ws, Engine: eng, Timeout: time.Second}

	got := storeCapabilitiesCheck{}.Run(context.Background(), env)
	if len(got) != 1 {
		t.Fatalf("want 1 result, got %d", len(got))
	}
	if got[0].Status != StatusOK {
		t.Fatalf("status = %s, want OK (msg=%s detail=%s)", got[0].Status, got[0].Message, got[0].Detail)
	}
	if !strings.Contains(got[0].Message, "versioning=") || !strings.Contains(got[0].Message, "encryption=") {
		t.Errorf("message = %q, want versioning+encryption summary", got[0].Message)
	}
}

func TestStoreCapabilities_NoStoreConfigured(t *testing.T) {
	env := &Environment{
		Workspace: &core.WorkspaceConfig{},
		Timeout:   time.Second,
	}
	got := storeCapabilitiesCheck{}.Run(context.Background(), env)
	if got[0].Status != StatusSkipped {
		t.Errorf("status = %s, want Skipped", got[0].Status)
	}
	if !strings.Contains(got[0].Message, "no store") {
		t.Errorf("message = %q, want mention of no store", got[0].Message)
	}
}

func TestStoreCapabilities_NoEngine(t *testing.T) {
	env := &Environment{
		Workspace: &core.WorkspaceConfig{
			Store: core.ProviderRef{Provider: "jsonfile"},
		},
		Timeout: time.Second,
	}
	got := storeCapabilitiesCheck{}.Run(context.Background(), env)
	if got[0].Status != StatusSkipped {
		t.Errorf("status = %s, want Skipped", got[0].Status)
	}
}

func TestStoreListTrees_OK(t *testing.T) {
	eng, ws := jsonStoreEngine(t)
	env := &Environment{Workspace: ws, Engine: eng, Timeout: time.Second}

	got := storeListTreesCheck{}.Run(context.Background(), env)
	if got[0].Status != StatusOK {
		t.Fatalf("status = %s, want OK (msg=%s detail=%s)", got[0].Status, got[0].Message, got[0].Detail)
	}
	if !strings.Contains(got[0].Message, "tree(s)") {
		t.Errorf("message = %q, want tree count", got[0].Message)
	}
}

func TestStoreListTrees_NoEngine(t *testing.T) {
	env := &Environment{
		Workspace: &core.WorkspaceConfig{
			Store: core.ProviderRef{Provider: "jsonfile"},
		},
		Timeout: time.Second,
	}
	got := storeListTreesCheck{}.Run(context.Background(), env)
	if got[0].Status != StatusSkipped {
		t.Errorf("status = %s, want Skipped", got[0].Status)
	}
}
