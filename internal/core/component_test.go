package core

import (
	"slices"
	"sort"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestComponentManagerBasic(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}},
		{Name: "monitoring", Paths: []string{"monitoring/"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	// Mandatory components start enabled.
	if !cm.IsEnabled("database") {
		t.Error("mandatory component 'database' should be enabled")
	}
	// Non-mandatory components start disabled.
	if cm.IsEnabled("redis") {
		t.Error("non-mandatory component 'redis' should start disabled")
	}
	if cm.IsEnabled("monitoring") {
		t.Error("non-mandatory component 'monitoring' should start disabled")
	}
}

func TestComponentManagerEnable(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}},
		{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	// Enabling redis should auto-enable database.
	if err := cm.Enable("redis"); err != nil {
		t.Fatalf("Enable('redis'): %v", err)
	}
	if !cm.IsEnabled("redis") {
		t.Error("redis should be enabled")
	}
	if !cm.IsEnabled("database") {
		t.Error("database should be auto-enabled as dependency of redis")
	}
}

func TestComponentManagerEnableUnknown(t *testing.T) {
	cm, err := NewComponentManager(nil)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}
	if err := cm.Enable("nonexistent"); err == nil {
		t.Error("expected error for unknown component")
	}
}

func TestComponentManagerDisable(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}},
		{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	_ = cm.Enable("redis") // also enables database

	// Cannot disable database while redis depends on it.
	if err := cm.Disable("database"); err == nil {
		t.Error("expected error disabling database while redis is enabled")
	}

	// Disable redis first, then database.
	if err := cm.Disable("redis"); err != nil {
		t.Fatalf("Disable('redis'): %v", err)
	}
	if err := cm.Disable("database"); err != nil {
		t.Fatalf("Disable('database'): %v", err)
	}
}

func TestComponentManagerDisableMandatory(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	if err := cm.Disable("database"); err == nil {
		t.Error("expected error disabling mandatory component")
	}
}

func TestComponentManagerDisableUnknown(t *testing.T) {
	cm, err := NewComponentManager(nil)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}
	if err := cm.Disable("nonexistent"); err == nil {
		t.Error("expected error for unknown component")
	}
}

func TestComponentManagerCircularDependency(t *testing.T) {
	defs := []ComponentDef{
		{Name: "a", Paths: []string{"a/"}, Dependencies: []string{"b"}},
		{Name: "b", Paths: []string{"b/"}, Dependencies: []string{"c"}},
		{Name: "c", Paths: []string{"c/"}, Dependencies: []string{"a"}},
	}
	_, err := NewComponentManager(defs)
	if err == nil {
		t.Error("expected error for circular dependencies")
	}
}

func TestComponentManagerSelfDependency(t *testing.T) {
	defs := []ComponentDef{
		{Name: "a", Paths: []string{"a/"}, Dependencies: []string{"a"}},
	}
	_, err := NewComponentManager(defs)
	if err == nil {
		t.Error("expected error for self-dependency")
	}
}

func TestComponentManagerDuplicateNames(t *testing.T) {
	defs := []ComponentDef{
		{Name: "dup", Paths: []string{"a/"}},
		{Name: "dup", Paths: []string{"b/"}},
	}
	_, err := NewComponentManager(defs)
	if err == nil {
		t.Error("expected error for duplicate component names")
	}
}

func TestComponentManagerUnknownDependency(t *testing.T) {
	defs := []ComponentDef{
		{Name: "a", Paths: []string{"a/"}, Dependencies: []string{"nonexistent"}},
	}
	_, err := NewComponentManager(defs)
	if err == nil {
		t.Error("expected error for unknown dependency")
	}
}

func TestComponentManagerOverlappingPaths(t *testing.T) {
	defs := []ComponentDef{
		{Name: "a", Paths: []string{"shared/"}},
		{Name: "b", Paths: []string{"shared/sub/"}},
	}
	_, err := NewComponentManager(defs)
	if err == nil {
		t.Error("expected error for overlapping path prefixes")
	}
}

func TestComponentManagerOverlappingPathsExact(t *testing.T) {
	defs := []ComponentDef{
		{Name: "a", Paths: []string{"shared/"}},
		{Name: "b", Paths: []string{"shared/"}},
	}
	_, err := NewComponentManager(defs)
	if err == nil {
		t.Error("expected error for identical path prefixes across components")
	}
}

func TestComponentManagerFilterTree(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	tree := config.NewTree()
	_ = tree.Set("database/host", &config.Value{Val: "localhost"})
	_ = tree.Set("database/port", &config.Value{Val: 5432})
	_ = tree.Set("redis/host", &config.Value{Val: "localhost"})
	_ = tree.Set("app/name", &config.Value{Val: "myapp"}) // unmanaged

	filtered := cm.FilterTree(tree)
	paths := filtered.List()
	sort.Strings(paths)

	// database is mandatory (enabled), redis is disabled, app/name is unmanaged.
	want := []string{"app/name", "database/host", "database/port"}
	if !slices.Equal(paths, want) {
		t.Errorf("FilterTree paths = %v, want %v", paths, want)
	}

	// Enable redis and filter again.
	_ = cm.Enable("redis")
	filtered2 := cm.FilterTree(tree)
	paths2 := filtered2.List()
	sort.Strings(paths2)

	want2 := []string{"app/name", "database/host", "database/port", "redis/host"}
	if !slices.Equal(paths2, want2) {
		t.Errorf("FilterTree paths after enable = %v, want %v", paths2, want2)
	}
}

func TestComponentManagerPathBelongsToComponent(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}},
		{Name: "redis", Paths: []string{"redis/"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	tests := []struct {
		path      string
		wantComp  string
		wantOwned bool
	}{
		{"database/host", "database", true},
		{"database/port", "database", true},
		{"redis/host", "redis", true},
		{"app/name", "", false},
		{"other/path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			comp, owned := cm.PathBelongsToComponent(tt.path)
			if comp != tt.wantComp || owned != tt.wantOwned {
				t.Errorf("PathBelongsToComponent(%q) = (%q, %v), want (%q, %v)",
					tt.path, comp, owned, tt.wantComp, tt.wantOwned)
			}
		})
	}
}

func TestComponentManagerEnabledPaths(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/config/", "redis/data/"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	got := cm.EnabledPaths()
	sort.Strings(got)
	want := []string{"database/"}
	if !slices.Equal(got, want) {
		t.Errorf("EnabledPaths() = %v, want %v", got, want)
	}

	_ = cm.Enable("redis")
	got2 := cm.EnabledPaths()
	sort.Strings(got2)
	want2 := []string{"database/", "redis/config/", "redis/data/"}
	if !slices.Equal(got2, want2) {
		t.Errorf("EnabledPaths() = %v, want %v", got2, want2)
	}
}

func TestComponentManagerListComponents(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	states := cm.ListComponents()
	if len(states) != 2 {
		t.Fatalf("got %d components, want 2", len(states))
	}

	db := states[0]
	if db.Name != "database" || !db.Enabled || !db.Mandatory {
		t.Errorf("database state = %+v, want enabled+mandatory", db)
	}

	redis := states[1]
	if redis.Name != "redis" || redis.Enabled || redis.Mandatory {
		t.Errorf("redis state = %+v, want disabled+not-mandatory", redis)
	}
	if !slices.Equal(redis.Dependencies, []string{"database"}) {
		t.Errorf("redis dependencies = %v, want [database]", redis.Dependencies)
	}
}

func TestComponentManagerValidateDependencies(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}},
		{Name: "redis", Paths: []string{"redis/"}, Dependencies: []string{"database"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	// Both disabled — no violations.
	if errs := cm.ValidateDependencies(); len(errs) != 0 {
		t.Errorf("expected no errors when all disabled, got %v", errs)
	}

	// Force an inconsistent state: redis enabled, database disabled.
	cm.state["redis"] = true
	cm.state["database"] = false
	errs := cm.ValidateDependencies()
	if len(errs) != 1 {
		t.Fatalf("expected 1 dependency violation, got %d: %v", len(errs), errs)
	}
}

func TestComponentManagerStateSaveLoad(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
		{Name: "redis", Paths: []string{"redis/"}},
		{Name: "monitoring", Paths: []string{"monitoring/"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	_ = cm.Enable("redis")
	saved := cm.SaveState()

	// Create a fresh manager and load state.
	cm2, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}
	cm2.LoadState(saved)

	if !cm2.IsEnabled("database") {
		t.Error("database should be enabled (mandatory)")
	}
	if !cm2.IsEnabled("redis") {
		t.Error("redis should be enabled (loaded from state)")
	}
	if cm2.IsEnabled("monitoring") {
		t.Error("monitoring should be disabled (loaded from state)")
	}
}

func TestComponentManagerLoadStateMandatoryForced(t *testing.T) {
	defs := []ComponentDef{
		{Name: "database", Paths: []string{"database/"}, Mandatory: true},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	// Try to load state that disables a mandatory component.
	cm.LoadState(map[string]bool{"database": false})
	if !cm.IsEnabled("database") {
		t.Error("mandatory component should remain enabled even when loaded state says disabled")
	}
}

func TestComponentManagerTransitiveDependencies(t *testing.T) {
	defs := []ComponentDef{
		{Name: "a", Paths: []string{"a/"}},
		{Name: "b", Paths: []string{"b/"}, Dependencies: []string{"a"}},
		{Name: "c", Paths: []string{"c/"}, Dependencies: []string{"b"}},
	}
	cm, err := NewComponentManager(defs)
	if err != nil {
		t.Fatalf("NewComponentManager: %v", err)
	}

	// Enabling c should transitively enable b and a.
	if err := cm.Enable("c"); err != nil {
		t.Fatalf("Enable('c'): %v", err)
	}
	if !cm.IsEnabled("a") {
		t.Error("a should be transitively enabled")
	}
	if !cm.IsEnabled("b") {
		t.Error("b should be transitively enabled")
	}
	if !cm.IsEnabled("c") {
		t.Error("c should be enabled")
	}
}

func TestComponentManagerEmptyDefinitions(t *testing.T) {
	cm, err := NewComponentManager(nil)
	if err != nil {
		t.Fatalf("NewComponentManager(nil): %v", err)
	}

	states := cm.ListComponents()
	if len(states) != 0 {
		t.Errorf("expected 0 components, got %d", len(states))
	}

	paths := cm.EnabledPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %v", paths)
	}

	// FilterTree with no components should return all paths.
	tree := config.NewTree()
	_ = tree.Set("app/name", &config.Value{Val: "test"})
	filtered := cm.FilterTree(tree)
	if fPaths := filtered.List(); len(fPaths) != 1 {
		t.Errorf("expected 1 path in filtered tree, got %d", len(fPaths))
	}
}
