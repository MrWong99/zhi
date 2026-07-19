package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// Engine is the central orchestrator. It holds references to resolved
// providers and exposes high-level operations on the configuration tree.
type Engine struct {
	registry         *Registry
	workspace        *WorkspaceConfig
	configPlugin     config.Plugin
	transformPlugins []transform.Plugin
	storePlugin      store.Plugin // nil if no store is configured
	components       *ComponentManager
	session          *SessionManager // nil if no store is configured
}

// NewEngine resolves all providers from the workspace config using the
// registry, initializes the ComponentManager, and returns a ready engine.
func NewEngine(registry *Registry, workspace *WorkspaceConfig) (*Engine, error) {
	e := &Engine{
		registry:  registry,
		workspace: workspace,
	}

	// Resolve config provider.
	Logger().Info("resolving config provider", "provider", workspace.Config.Provider)
	Logger().Debug("config provider options", "provider", workspace.Config.Provider, "options", workspace.Config.Options)
	cp, err := registry.ConfigProvider(workspace.Dir, workspace.Config.Provider, workspace.Config.Options)
	if err != nil {
		return nil, fmt.Errorf("resolving config provider: %w", err)
	}
	e.configPlugin = cp

	// Resolve transform providers.
	for _, t := range workspace.Transform {
		Logger().Info("resolving transform provider", "provider", t.Provider)
		Logger().Debug("transform provider options", "provider", t.Provider, "options", t.Options)
		tp, err := registry.TransformProvider(workspace.Dir, t.Provider, t.Options)
		if err != nil {
			return nil, fmt.Errorf("resolving transform provider %q: %w", t.Provider, err)
		}
		e.transformPlugins = append(e.transformPlugins, tp)
	}

	// Resolve store provider (optional).
	if workspace.Store.Provider != "" {
		Logger().Info("resolving store provider", "provider", workspace.Store.Provider)
		Logger().Debug("store provider options", "provider", workspace.Store.Provider, "options", workspace.Store.Options)
		sp, err := registry.StoreProvider(workspace.Dir, workspace.Store.Provider, workspace.Store.Options)
		if err != nil {
			return nil, fmt.Errorf("resolving store provider: %w", err)
		}
		e.storePlugin = sp
	} else if workspace.Dir != "" {
		// No store configured — use built-in jsonfile store as fallback so
		// that "zhi set" persists values instead of silently discarding them.
		Logger().Warn("no store provider configured, using built-in plaintext JSON file store as fallback",
			"directory", filepath.Join(workspace.Dir, DefaultStoreDir))
		Logger().Warn("values will be stored in PLAINTEXT on disk — do not use for secrets in production")
		sp, err := NewJSONFileStoreProvider(workspace.Dir, nil)
		if err != nil {
			return nil, fmt.Errorf("initializing fallback file store: %w", err)
		}
		e.storePlugin = sp
	}

	// Initialize session manager for authenticated stores.
	if e.storePlugin != nil {
		e.session = NewSessionManager(e.storePlugin)

		// Attempt auto-login from workspace store options (fallback auth).
		if method, creds := extractStoreAuth(workspace.Store.Options); method != "" {
			Logger().Info("attempting auto-login from workspace config", "method", method)
			if _, err := e.session.Login(context.Background(), method, creds); err != nil {
				Logger().Warn("auto-login from workspace config failed", "method", method, "error", err)
			} else {
				Logger().Info("auto-login from workspace config succeeded", "method", method)
			}
		}
	}

	// Initialize component manager.
	cm, err := NewComponentManager(workspace.Components)
	if err != nil {
		return nil, fmt.Errorf("initializing components: %w", err)
	}
	e.components = cm

	Logger().Info("engine initialized",
		"configProvider", workspace.Config.Provider,
		"transformCount", len(e.transformPlugins),
		"hasStore", e.storePlugin != nil)

	return e, nil
}

// LoadTree assembles a configuration tree by calling List() and then Get()
// for each path on the config provider.
func (e *Engine) LoadTree(ctx context.Context) (*config.Tree, error) {
	Logger().Info("loading configuration tree")
	paths, err := e.configPlugin.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing config paths: %w", err)
	}
	Logger().Debug("config paths discovered", "count", len(paths))

	tree := config.NewTree()
	for _, path := range paths {
		v, found, err := e.configPlugin.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("getting config value at %q: %w", path, err)
		}
		if !found {
			continue
		}
		if err := tree.Set(path, &v); err != nil {
			return nil, fmt.Errorf("setting tree value at %q: %w", path, err)
		}
	}

	// Load stored tree and merge if available. A store read failure (store
	// unreachable, Vault sealed, auth expired, corrupt store file) must abort
	// rather than silently falling back to config-provider defaults — otherwise
	// export/apply would overwrite real config with defaults and exit 0. A
	// genuinely empty store (first run) returns ok=false with a nil error and
	// is not treated as a failure. Workspaces without a store skip this entirely.
	if e.storePlugin != nil {
		storedTree, ok, err := e.LoadStoredTree(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading stored values: %w", err)
		}
		if ok {
			for _, path := range tree.List() {
				storedVal, found := storedTree.Get(path)
				if !found {
					continue
				}
				currentVal, found := tree.GetPtr(path) // pointer so we can modify it directly
				if !found {
					continue // should not happen since we used "List()"
				}
				// Check stored value has same type as current value, attempting
				// lossless numeric conversion when types differ (e.g. float64 from
				// JSON store vs int from YAML config).
				if reflect.TypeOf(storedVal.Val) != reflect.TypeOf(currentVal.Val) {
					converted, ok := tryNumericConvert(storedVal.Val, reflect.TypeOf(currentVal.Val))
					if !ok {
						Logger().Warn("stored value has different type and won't be updated",
							"path", path, "storedType", reflect.TypeOf(storedVal.Val), "newType", reflect.TypeOf(currentVal.Val))
						continue
					}
					storedVal.Val = converted
				}
				currentVal.Val = storedVal.Val
				Logger().Debug("updated value from stored value", "path", path)
			}
		}
	}

	return tree, nil
}

// TransformForDisplay applies all transform plugins' BeforeDisplay
// operations to the tree in order.
func (e *Engine) TransformForDisplay(ctx context.Context, tree *config.Tree) error {
	for i, tp := range e.transformPlugins {
		Logger().Debug("applying BeforeDisplay transform", "index", i)
		if err := tp.BeforeDisplay(ctx, tree); err != nil {
			return fmt.Errorf("transform BeforeDisplay: %w", err)
		}
	}
	return nil
}

// TransformForSave applies all transform plugins' AfterSave operations
// to the tree in order.
func (e *Engine) TransformForSave(ctx context.Context, tree *config.Tree) error {
	for i, tp := range e.transformPlugins {
		Logger().Debug("applying AfterSave transform", "index", i)
		if err := tp.AfterSave(ctx, tree); err != nil {
			return fmt.Errorf("transform AfterSave: %w", err)
		}
	}
	return nil
}

// Validate runs config provider validation for each path in the tree and
// also validates component dependency constraints. The returned results
// include both config validation findings and component dependency errors
// (reported as Blocking severity under the special path
// "_components/<name>").
func (e *Engine) Validate(ctx context.Context, tree *config.Tree) ([]config.ValidationResult, error) {
	Logger().Debug("starting validation")
	var results []config.ValidationResult

	for _, path := range tree.List() {
		vr, err := e.configPlugin.Validate(ctx, path, tree)
		if err != nil {
			return nil, fmt.Errorf("validating %q: %w", path, err)
		}
		results = append(results, vr...)
	}

	// Validate component dependency constraints.
	for _, depErr := range e.components.ValidateDependencies() {
		results = append(results, config.ValidationResult{
			Severity: config.Blocking,
			Message:  depErr.Error(),
		})
	}

	Logger().Info("validation complete", "resultCount", len(results))
	return results, nil
}

// SetValue stores a value at the given path via the config provider.
// The path is validated before being sent to the plugin.
func (e *Engine) SetValue(ctx context.Context, path string, value config.Value) error {
	Logger().Debug("setting value", "path", path)
	if err := config.ValidatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	return e.configPlugin.Set(ctx, path, value)
}

// SaveTree persists a tree to the store provider by extracting all values
// and writing them with PutValues. The tree is stored under the workspace's
// derived TreeID. Returns an error if no store is configured.
func (e *Engine) SaveTree(ctx context.Context, tree *config.Tree) error {
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	values := make(map[string]config.Value)
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if ok {
			values[path] = v
		}
	}
	Logger().Info("saving tree", "valueCount", len(values))
	Logger().Debug("saving tree details", "treeID", e.TreeID())
	return e.storePlugin.PutValues(ctx, e.TreeID(), values, nil)
}

// LoadStoredTree loads a tree from the store provider. The config plugin's
// path list is used so the store knows exactly which values to fetch.
// The tree is loaded using the workspace's derived TreeID.
// Returns an error if no store is configured.
func (e *Engine) LoadStoredTree(ctx context.Context) (*config.Tree, bool, error) {
	Logger().Debug("loading stored tree", "treeID", e.TreeID())
	if e.storePlugin == nil {
		return nil, false, fmt.Errorf("no store provider configured")
	}
	// Obtain the expected paths from the config plugin.
	paths, err := e.configPlugin.List(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("listing config paths for store load: %w", err)
	}
	values, err := e.storePlugin.GetValues(ctx, e.TreeID(), paths)
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}
	tree := config.NewTree()
	for path, v := range values {
		if desiredType := labels.GetSemanticType(v.Metadata); desiredType != "" {
			var err error
			switch desiredType {
			case "string":
				err = v.TryConvert(reflect.TypeFor[string]())
			case "int":
				err = v.TryConvert(reflect.TypeFor[int]())
			case "uint":
				err = v.TryConvert(reflect.TypeFor[uint]())
			case "float":
				err = v.TryConvert(reflect.TypeFor[float64]())
			case "bool":
				err = v.TryConvert(reflect.TypeFor[bool]())
			default:
				err = fmt.Errorf("unsupported semantic type: %s", desiredType)
			}
			if err != nil {
				Logger().Warn("could not convert value to desired type", "path", path, "error", err)
			}
		}
		if err := tree.Set(path, &v); err != nil {
			return nil, false, fmt.Errorf("setting stored value at %q: %w", path, err)
		}
	}
	return tree, true, nil
}

// ListTrees returns all tree IDs from the store provider. Returns an error
// if no store is configured.
func (e *Engine) ListTrees(ctx context.Context) ([]string, error) {
	if e.storePlugin == nil {
		return nil, fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.ListTrees(ctx)
}

// StoreCapabilities returns the store plugin's capabilities. Returns an
// error if no store is configured.
func (e *Engine) StoreCapabilities(ctx context.Context) (*store.Capabilities, error) {
	if e.storePlugin == nil {
		return nil, fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.Capabilities(ctx)
}

// DeleteTree removes an entire tree from the store. Returns an error if
// no store is configured.
func (e *Engine) DeleteTree(ctx context.Context, id string) error {
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.DeleteTree(ctx, id)
}

// DeleteValues removes specific values from a tree in the store. Returns
// an error if no store is configured.
func (e *Engine) DeleteValues(ctx context.Context, id string, paths []string) error {
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.DeleteValues(ctx, id, paths)
}

// --- Tree-level versioning ---

// ListTreeVersions returns version identifiers for a stored tree,
// ordered newest first. Returns an error if no store is configured.
func (e *Engine) ListTreeVersions(ctx context.Context, id string) ([]string, error) {
	Logger().Debug("listing tree versions", "treeID", id)
	if e.storePlugin == nil {
		return nil, fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.ListTreeVersions(ctx, id)
}

// GetTreeVersion retrieves a specific version of a tree from the store.
// The config plugin's path list determines which values to request.
// Returns an error if no store is configured.
func (e *Engine) GetTreeVersion(ctx context.Context, id string, version string) (*config.Tree, bool, error) {
	Logger().Debug("getting tree version", "treeID", id, "version", version)
	if e.storePlugin == nil {
		return nil, false, fmt.Errorf("no store provider configured")
	}
	paths, err := e.configPlugin.List(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("listing config paths for version load: %w", err)
	}
	values, err := e.storePlugin.GetTreeVersion(ctx, id, version, paths)
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}
	tree := config.NewTree()
	for path, v := range values {
		cp := v
		if err := tree.Set(path, &cp); err != nil {
			return nil, false, fmt.Errorf("setting versioned value at %q: %w", path, err)
		}
	}
	return tree, true, nil
}

// RollbackTree restores a stored tree to a previous version. Returns an
// error if no store is configured.
func (e *Engine) RollbackTree(ctx context.Context, id string, version string) error {
	Logger().Debug("rolling back tree", "treeID", id, "version", version)
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.RollbackTree(ctx, id, version)
}

// DeleteTreeVersion permanently removes a single tree version. Returns
// an error if no store is configured.
func (e *Engine) DeleteTreeVersion(ctx context.Context, id string, version string) error {
	Logger().Debug("deleting tree version", "treeID", id, "version", version)
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.DeleteTreeVersion(ctx, id, version)
}

// --- Value-level versioning ---

// ListValueVersions returns version identifiers for a specific value,
// ordered newest first. Returns an error if no store is configured.
func (e *Engine) ListValueVersions(ctx context.Context, id string, path string) ([]string, error) {
	Logger().Debug("listing value versions", "treeID", id, "path", path)
	if e.storePlugin == nil {
		return nil, fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.ListValueVersions(ctx, id, path)
}

// GetValueVersion retrieves a specific version of a single value from
// the store. Returns an error if no store is configured.
func (e *Engine) GetValueVersion(ctx context.Context, id string, path string, version string) (config.Value, bool, error) {
	Logger().Debug("getting value version", "treeID", id, "path", path, "version", version)
	if e.storePlugin == nil {
		return config.Value{}, false, fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.GetValueVersion(ctx, id, path, version)
}

// RollbackValue restores a value to a previous version. Returns an
// error if no store is configured.
func (e *Engine) RollbackValue(ctx context.Context, id string, path string, version string) error {
	Logger().Debug("rolling back value", "treeID", id, "path", path, "version", version)
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.RollbackValue(ctx, id, path, version)
}

// DeleteValueVersion permanently removes a single value version.
// Returns an error if no store is configured.
func (e *Engine) DeleteValueVersion(ctx context.Context, id string, path string, version string) error {
	Logger().Debug("deleting value version", "treeID", id, "path", path, "version", version)
	if e.storePlugin == nil {
		return fmt.Errorf("no store provider configured")
	}
	return e.storePlugin.DeleteValueVersion(ctx, id, path, version)
}

// Close shuts down the engine, killing all external plugin processes.
func (e *Engine) Close() {
	Logger().Info("closing engine")
	if e.registry != nil {
		e.registry.Close()
	}
}

// Components returns the component manager for UI and CLI access.
func (e *Engine) Components() *ComponentManager {
	return e.components
}

// FilteredTree loads the configuration tree and filters it to only include
// paths belonging to enabled components (plus unmanaged paths).
func (e *Engine) FilteredTree(ctx context.Context) (*config.Tree, error) {
	Logger().Debug("loading filtered tree")
	tree, err := e.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	return e.components.FilterTree(tree), nil
}

// StorePlugin returns the resolved store plugin, or nil if no store is
// configured.
func (e *Engine) StorePlugin() store.Plugin {
	return e.storePlugin
}

// Session returns the session manager for the active store plugin, or
// nil if no store is configured.
func (e *Engine) Session() *SessionManager {
	return e.session
}

// WorkspaceDir returns the directory containing the workspace config file.
func (e *Engine) WorkspaceDir() string {
	if e.workspace == nil {
		return ""
	}
	return e.workspace.Dir
}

// TreeID returns a deterministic identifier derived from the workspace
// directory path. The same workspace always produces the same ID, while
// different workspaces produce different IDs. This is used as the tree
// key when persisting to the store.
func (e *Engine) TreeID() string {
	dir := e.WorkspaceDir()
	if dir == "" {
		return "default"
	}
	h := sha256.Sum256([]byte(dir))
	return hex.EncodeToString(h[:8])
}

// ValidatePath runs config provider validation for a single path in the tree.
// If the path does not exist in the tree, an empty result is returned.
func (e *Engine) ValidatePath(ctx context.Context, path string, tree *config.Tree) ([]config.ValidationResult, error) {
	if _, ok := tree.Get(path); !ok {
		return nil, nil
	}
	return e.configPlugin.Validate(ctx, path, tree)
}

// Workspace returns the workspace configuration.
func (e *Engine) Workspace() *WorkspaceConfig {
	return e.workspace
}

// SetTestWorkspaceDir sets the workspace directory for testing purposes.
// This should only be used in tests.
func (e *Engine) SetTestWorkspaceDir(dir string) {
	if e.workspace == nil {
		e.workspace = &WorkspaceConfig{}
	}
	e.workspace.Dir = dir
}

// BuildApplyRunConfig creates an ApplyRunConfig from the workspace apply
// configuration and the current component state. The target parameter
// selects a named apply target (empty = "default").
func (e *Engine) BuildApplyRunConfig(target string) (ApplyRunConfig, error) {
	Logger().Debug("building apply run config", "target", target)
	ws := e.Workspace()
	if ws == nil {
		return ApplyRunConfig{}, fmt.Errorf("no workspace loaded")
	}

	t, err := ws.Apply.ResolveTarget(target)
	if err != nil {
		return ApplyRunConfig{}, err
	}

	cm := e.Components()
	var enabled, disabled []string
	for _, c := range cm.ListComponents() {
		if c.Enabled {
			enabled = append(enabled, c.Name)
		} else {
			disabled = append(disabled, c.Name)
		}
	}

	return ApplyRunConfig{
		Target:             t,
		WorkspaceDir:       ws.Dir,
		TimeoutOverride:    -1, // use target config
		EnabledComponents:  enabled,
		DisabledComponents: disabled,
	}, nil
}

// extractStoreAuth reads the "auth" key from store options. The expected
// format in zhi.yaml is:
//
//	store:
//	  provider: vault
//	  options:
//	    addr: "http://127.0.0.1:8200"
//	    auth:
//	      method: token
//	      credentials:
//	        token: "hvs.xxxxx"
//
// Returns the method name and credentials map. Returns ("", nil) if no
// auth section is present.
func extractStoreAuth(options map[string]any) (string, map[string]string) {
	if options == nil {
		return "", nil
	}
	authRaw, ok := options["auth"]
	if !ok {
		return "", nil
	}
	auth, ok := authRaw.(map[string]any)
	if !ok {
		return "", nil
	}
	method, _ := auth["method"].(string)
	if method == "" {
		return "", nil
	}
	credsRaw, ok := auth["credentials"]
	if !ok {
		return method, nil
	}
	credsMap, ok := credsRaw.(map[string]any)
	if !ok {
		return method, nil
	}
	creds := make(map[string]string, len(credsMap))
	for k, v := range credsMap {
		if s, ok := v.(string); ok {
			creds[k] = s
		}
	}
	return method, creds
}
