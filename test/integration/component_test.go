package integration

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// TestComponentFilteringInWorkspace tests that component filtering works
// end-to-end with a real workspace: disabled components' paths are excluded
// from filtered trees and exports.
func TestComponentFilteringInWorkspace(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: database
    paths:
      - database/
    mandatory: true
  - name: cache
    paths:
      - cache/
  - name: monitoring
    paths:
      - monitoring/
`,
		map[string]string{
			"database.yaml": `database:
  host:
    val: localhost
  port:
    val: 5432
`,
			"cache.yaml": `cache:
  host:
    val: redis.local
  ttl:
    val: 300
`,
			"monitoring.yaml": `monitoring:
  enabled:
    val: true
  endpoint:
    val: https://metrics.example.com
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	cm := eng.Components()
	ctx := context.Background()

	// Initially: database is mandatory (enabled), cache and monitoring are disabled.
	if !cm.IsEnabled("database") {
		t.Error("mandatory component 'database' should be enabled")
	}
	if cm.IsEnabled("cache") {
		t.Error("non-mandatory component 'cache' should be disabled initially")
	}
	if cm.IsEnabled("monitoring") {
		t.Error("non-mandatory component 'monitoring' should be disabled initially")
	}

	// Load filtered tree - only database paths should be included.
	filtered, err := eng.FilteredTree(ctx)
	if err != nil {
		t.Fatalf("FilteredTree: %v", err)
	}
	paths := filtered.List()
	sort.Strings(paths)

	// database/ paths should be present. cache/ and monitoring/ should not.
	for _, p := range paths {
		if strings.HasPrefix(p, "cache/") || strings.HasPrefix(p, "monitoring/") {
			t.Errorf("disabled component path %q should not be in filtered tree", p)
		}
	}
	if _, ok := filtered.Get("database/host"); !ok {
		t.Error("database/host should be in filtered tree")
	}

	// Enable cache and verify it appears.
	if err := cm.Enable("cache"); err != nil {
		t.Fatalf("Enable cache: %v", err)
	}
	filtered2, err := eng.FilteredTree(ctx)
	if err != nil {
		t.Fatalf("FilteredTree (2): %v", err)
	}
	if _, ok := filtered2.Get("cache/host"); !ok {
		t.Error("cache/host should be in filtered tree after enabling cache")
	}
	if _, ok := filtered2.Get("monitoring/enabled"); ok {
		t.Error("monitoring/enabled should NOT be in filtered tree (still disabled)")
	}
}

// TestComponentDependencies verifies that enabling a component auto-enables
// its dependencies and disabling a dependency is blocked.
func TestComponentDependencies(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: database
    paths:
      - database/
    mandatory: true
  - name: cache
    paths:
      - cache/
    dependencies:
      - database
  - name: session
    paths:
      - session/
    dependencies:
      - cache
`,
		map[string]string{
			"database.yaml": `database:
  host:
    val: localhost
`,
			"cache.yaml": `cache:
  host:
    val: redis.local
`,
			"session.yaml": `session:
  ttl:
    val: 3600
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	cm := eng.Components()

	// Enable session, which depends on cache, which depends on database.
	if err := cm.Enable("session"); err != nil {
		t.Fatalf("Enable session: %v", err)
	}

	// Both cache and database should be auto-enabled.
	if !cm.IsEnabled("cache") {
		t.Error("cache should be auto-enabled as session's dependency")
	}
	if !cm.IsEnabled("database") {
		t.Error("database should be auto-enabled as cache's dependency")
	}
	if !cm.IsEnabled("session") {
		t.Error("session should be enabled")
	}

	// Try to disable cache (session depends on it) - should fail.
	err = cm.Disable("cache")
	if err == nil {
		t.Fatal("should not be able to disable cache while session is enabled")
	}
	if !strings.Contains(err.Error(), "required by") {
		t.Errorf("error should mention 'required by', got: %v", err)
	}

	// Disable session first, then cache should be disableable.
	if err := cm.Disable("session"); err != nil {
		t.Fatalf("Disable session: %v", err)
	}
	if err := cm.Disable("cache"); err != nil {
		t.Fatalf("Disable cache: %v", err)
	}
	if cm.IsEnabled("cache") {
		t.Error("cache should be disabled")
	}
}

// TestComponentMandatoryCannotBeDisabled verifies that mandatory components
// cannot be disabled.
func TestComponentMandatoryCannotBeDisabled(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: core
    paths:
      - core/
    mandatory: true
`,
		map[string]string{
			"core.yaml": `core:
  key:
    val: essential
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	err = eng.Components().Disable("core")
	if err == nil {
		t.Fatal("should not be able to disable mandatory component")
	}
	if !strings.Contains(err.Error(), "mandatory") {
		t.Errorf("error should mention mandatory, got: %v", err)
	}
}

// TestComponentDependencyValidation verifies that the validation system
// catches broken dependency constraints.
func TestComponentDependencyValidation(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: database
    paths:
      - database/
  - name: app
    paths:
      - app/
    dependencies:
      - database
`,
		map[string]string{
			"database.yaml": `database:
  host:
    val: localhost
`,
			"app.yaml": `app:
  name:
    val: myapp
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	cm := eng.Components()

	// Enable app (auto-enables database).
	if err := cm.Enable("app"); err != nil {
		t.Fatalf("Enable app: %v", err)
	}

	// Validate should have no dependency errors.
	ctx := context.Background()
	tree, err := eng.LoadTree(ctx)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	results, err := eng.Validate(ctx, tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, r := range results {
		if r.Severity == config.Blocking && strings.Contains(r.Message, "dependency") {
			t.Errorf("unexpected dependency error: %s", r.Message)
		}
	}
}

// TestComponentCircularDependencyRejected verifies that circular dependencies
// are caught at workspace construction time.
func TestComponentCircularDependencyRejected(t *testing.T) {
	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "structuredfile"},
		Components: []core.ComponentDef{
			{Name: "a", Paths: []string{"a/"}, Dependencies: []string{"b"}},
			{Name: "b", Paths: []string{"b/"}, Dependencies: []string{"c"}},
			{Name: "c", Paths: []string{"c/"}, Dependencies: []string{"a"}},
		},
	}

	reg := core.DefaultRegistry()
	_, err := core.NewEngine(reg, ws)
	if err == nil {
		t.Fatal("NewEngine should reject circular dependencies")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention circular dependency, got: %v", err)
	}
}

// TestComponentOverlappingPathsRejected verifies that components with
// overlapping path prefixes are caught.
func TestComponentOverlappingPathsRejected(t *testing.T) {
	ws := &core.WorkspaceConfig{
		Version: "1",
		Config:  core.ProviderRef{Provider: "structuredfile"},
		Components: []core.ComponentDef{
			{Name: "database", Paths: []string{"database/"}},
			{Name: "database-extra", Paths: []string{"database/advanced/"}},
		},
	}

	reg := core.DefaultRegistry()
	_, err := core.NewEngine(reg, ws)
	if err == nil {
		t.Fatal("NewEngine should reject overlapping component path prefixes")
	}
	if !strings.Contains(err.Error(), "overlapping") {
		t.Errorf("error should mention overlapping, got: %v", err)
	}
}

// TestComponentCascadeDisable verifies that cascade disable works: disabling
// a dependency also disables all dependents.
func TestComponentCascadeDisable(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: database
    paths:
      - database/
  - name: cache
    paths:
      - cache/
    dependencies:
      - database
  - name: session
    paths:
      - session/
    dependencies:
      - cache
`,
		map[string]string{
			"database.yaml": `database:
  host:
    val: localhost
`,
			"cache.yaml": `cache:
  host:
    val: redis
`,
			"session.yaml": `session:
  ttl:
    val: 600
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	cm := eng.Components()

	// Enable everything.
	if err := cm.Enable("session"); err != nil {
		t.Fatalf("Enable session: %v", err)
	}
	if !cm.IsEnabled("database") || !cm.IsEnabled("cache") || !cm.IsEnabled("session") {
		t.Fatal("all components should be enabled")
	}

	// Cascade disable database - should also disable cache and session.
	disabled, err := cm.DisableCascade("database")
	if err != nil {
		t.Fatalf("DisableCascade: %v", err)
	}

	// Should have disabled session, cache, and database (in that order).
	if len(disabled) != 3 {
		t.Fatalf("expected 3 disabled components, got %d: %v", len(disabled), disabled)
	}

	if cm.IsEnabled("database") || cm.IsEnabled("cache") || cm.IsEnabled("session") {
		t.Error("all components should be disabled after cascade")
	}
}

// TestComponentExportFiltering tests that export only includes paths from
// enabled components.
func TestComponentExportFiltering(t *testing.T) {
	wsDir := setupWorkspaceDir(t,
		`version: "1"
config:
  provider: structuredfile
  options:
    directory: ./config
components:
  - name: app
    paths:
      - app/
    mandatory: true
  - name: secrets
    paths:
      - secrets/
`,
		map[string]string{
			"app.yaml": `app:
  name:
    val: production-app
`,
			"secrets.yaml": `secrets:
  api-key:
    val: sk-super-secret-key-12345
`,
		},
		nil,
	)

	ws, err := core.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	reg := core.DefaultRegistry()
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	// secrets is disabled by default. Export should NOT include secrets.
	td, err := core.PrepareTreeData(ctx, eng, false, "")
	if err != nil {
		t.Fatalf("PrepareTreeData: %v", err)
	}

	cfg := core.ExportRunConfig{
		Format: "dotenv",
		DryRun: true,
	}
	result, err := core.Export(ctx, td, cfg)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if strings.Contains(result.Content, "sk-super-secret-key-12345") {
		t.Error("export should NOT contain secret from disabled component")
	}
	if !strings.Contains(result.Content, "production-app") {
		t.Error("export should contain value from enabled component")
	}

	// Enable secrets and verify it's now included.
	eng.Components().Enable("secrets")
	td2, err := core.PrepareTreeData(ctx, eng, false, "")
	if err != nil {
		t.Fatalf("PrepareTreeData (2): %v", err)
	}
	result2, err := core.Export(ctx, td2, cfg)
	if err != nil {
		t.Fatalf("Export (2): %v", err)
	}
	if !strings.Contains(result2.Content, "sk-super-secret-key-12345") {
		t.Error("export should contain secret from now-enabled component")
	}
}
