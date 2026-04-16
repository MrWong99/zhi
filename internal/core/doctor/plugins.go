package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/lockfile"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/semver"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
	"github.com/MrWong99/zhi/pkg/zhiplugin/launch"
)

// defaultAliveTimeout is used when env.Timeout is zero so alive probes
// still have a deterministic per-call deadline.
const defaultAliveTimeout = 5 * time.Second

type pluginsReferencedInstalledCheck struct{}

func (pluginsReferencedInstalledCheck) ID() string         { return "plugins.referenced_installed" }
func (pluginsReferencedInstalledCheck) Category() Category { return CategoryPlugins }
func (pluginsReferencedInstalledCheck) Description() string {
	return "Every provider referenced in zhi.yaml resolves to a built-in or discovered plugin."
}

func (c pluginsReferencedInstalledCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || env.Workspace == nil {
		return []Result{skipped(c.ID(), c.ID(), "no workspace loaded")}
	}
	if env.Registry == nil {
		return []Result{skipped(c.ID(), c.ID(), "no registry available")}
	}

	ws := env.Workspace
	// Build lookup sets so we never accidentally launch a plugin here.
	configNames := env.Registry.ListConfig()
	transformNames := env.Registry.ListTransform()
	storeNames := env.Registry.ListStore()
	uiNames := env.Registry.ListUI()

	var refs []providerRef
	if ws.Config.Provider != "" {
		refs = append(refs, providerRef{kind: "config", name: ws.Config.Provider, names: configNames})
	}
	for _, t := range ws.Transform {
		if t.Provider == "" {
			continue
		}
		refs = append(refs, providerRef{kind: "transform", name: t.Provider, names: transformNames})
	}
	if ws.Store.Provider != "" {
		refs = append(refs, providerRef{kind: "store", name: ws.Store.Provider, names: storeNames})
	}
	if ws.UI.Provider != "" {
		refs = append(refs, providerRef{kind: "ui", name: ws.UI.Provider, names: uiNames})
	}

	if len(refs) == 0 {
		return []Result{skipped(c.ID(), c.ID(), "no providers referenced")}
	}

	results := make([]Result, 0, len(refs))
	for _, ref := range refs {
		name := ref.kind + "/" + ref.name
		if slices.Contains(ref.names, ref.name) {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusOK,
				Message: "resolved",
			})
			continue
		}
		results = append(results, Result{
			CheckID: c.ID(),
			Name:    name,
			Status:  StatusError,
			Message: "plugin not installed",
			Detail:  fmt.Sprintf("no %s provider registered as %q", ref.kind, ref.name),
			Hint:    "install with `zhi plugin install oci://...` or set the provider name correctly",
		})
	}
	return results
}

type pluginsManifestReadableCheck struct{}

func (pluginsManifestReadableCheck) ID() string         { return "plugins.manifest_readable" }
func (pluginsManifestReadableCheck) Category() Category { return CategoryPlugins }
func (pluginsManifestReadableCheck) Description() string {
	return "Each discovered plugin has a readable zhi-plugin.yaml manifest."
}

func (c pluginsManifestReadableCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || len(env.Discovered) == 0 {
		return []Result{skipped(c.ID(), c.ID(), "no discovered plugins")}
	}

	metaStore := metadata.NewStore(metadata.DefaultMetadataDir())
	results := make([]Result, 0, len(env.Discovered))
	for _, p := range env.Discovered {
		name := string(p.Type) + "/" + p.Name
		// Published plugins include a zhi-plugin.yaml next to the binary;
		// OCI-installed plugins don't — their metadata lives under
		// ~/.zhi/metadata/ instead. Try the local manifest first, then
		// fall back to the install metadata store.
		path := manifestPathFor(p)
		m, err := manifest.LoadFile(path)
		switch {
		case err == nil:
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusOK,
				Message: m.Name + " v" + m.Version,
				Detail:  "source: " + path,
			})
		case errors.Is(err, os.ErrNotExist):
			meta, mErr := metaStore.Load(p.Name)
			if mErr == nil && meta != nil {
				results = append(results, Result{
					CheckID: c.ID(),
					Name:    name,
					Status:  StatusOK,
					Message: meta.Name + " v" + meta.Version,
					Detail:  "source: install metadata (" + metadata.DefaultMetadataDir() + ")",
				})
				continue
			}
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusWarning,
				Message: "no manifest or install metadata",
				Detail:  "looked at " + path,
				Hint:    "plugin will still work but version-compat checks are skipped",
			})
		default:
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusError,
				Message: "manifest unreadable",
				Detail:  err.Error(),
			})
		}
	}
	return results
}

type pluginsVersionCompatibleCheck struct{}

func (pluginsVersionCompatibleCheck) ID() string         { return "plugins.version_compatible" }
func (pluginsVersionCompatibleCheck) Category() Category { return CategoryPlugins }
func (pluginsVersionCompatibleCheck) Description() string {
	return "Installed plugin manifests declare a minZhiVersion compatible with the current zhi."
}

func (c pluginsVersionCompatibleCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || len(env.Discovered) == 0 {
		return []Result{skipped(c.ID(), c.ID(), "no discovered plugins")}
	}

	results := make([]Result, 0, len(env.Discovered))
	for _, p := range env.Discovered {
		name := string(p.Type) + "/" + p.Name
		path := manifestPathFor(p)
		m, err := manifest.LoadFile(path)
		if err != nil {
			// Previous check already warned; stay silent here.
			continue
		}
		if m.MinZhiVersion == "" {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusOK,
				Message: "no minimum specified",
			})
			continue
		}
		// Dev builds have no meaningful semver; surface a warning instead
		// of failing parse.
		if env.ZhiVersion == "" || env.ZhiVersion == "dev" {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusWarning,
				Message: "running a dev build; compatibility not enforced",
				Detail:  fmt.Sprintf("plugin requires zhi >= %s", m.MinZhiVersion),
				Hint:    "running a dev build; version compatibility not enforced",
			})
			continue
		}
		ok, err := semver.Matches(env.ZhiVersion, ">="+m.MinZhiVersion)
		if err != nil {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusWarning,
				Message: "unable to compare versions",
				Detail:  err.Error(),
				Hint:    "running a dev build; version compatibility not enforced",
			})
			continue
		}
		if !ok {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusError,
				Message: fmt.Sprintf("plugin requires zhi >= %s but you have %s", m.MinZhiVersion, env.ZhiVersion),
				Hint:    "upgrade zhi or install an older version of the plugin",
			})
			continue
		}
		results = append(results, Result{
			CheckID: c.ID(),
			Name:    name,
			Status:  StatusOK,
			Message: fmt.Sprintf("zhi %s satisfies >= %s", env.ZhiVersion, m.MinZhiVersion),
		})
	}
	return results
}

type pluginsAliveCheck struct{}

func (pluginsAliveCheck) ID() string         { return "plugins.alive" }
func (pluginsAliveCheck) Category() Category { return CategoryPlugins }
func (pluginsAliveCheck) Description() string {
	return "Each referenced plugin launches and responds to a cheap RPC within timeout."
}

func (c pluginsAliveCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || env.Workspace == nil {
		return []Result{skipped(c.ID(), c.ID(), "no workspace loaded")}
	}
	if env.Registry == nil {
		return []Result{skipped(c.ID(), c.ID(), "no registry available")}
	}

	ws := env.Workspace
	// Index discovered externals by type+name for quick path lookup.
	discovered := make(map[string]core.PluginInfo, len(env.Discovered))
	for _, p := range env.Discovered {
		discovered[string(p.Type)+"/"+p.Name] = p
	}

	builtinConfig := builtinSet(env.Registry.ListConfigProviders())
	builtinTransform := builtinSet(env.Registry.ListTransformProviders())
	builtinStore := builtinSet(env.Registry.ListStoreProviders())
	builtinUI := builtinSet(env.Registry.ListUIProviders())

	probeTimeout := defaultAliveTimeout
	if env.Timeout > 0 {
		// Leave headroom so two sub-operations fit inside Runner's deadline.
		probeTimeout = env.Timeout / 2
		if probeTimeout <= 0 {
			probeTimeout = env.Timeout
		}
	}

	var results []Result
	if ws.Config.Provider != "" {
		results = append(results, c.probe(ctx, env, "config", ws.Config.Provider, discovered, builtinConfig, probeTimeout))
	}
	for _, t := range ws.Transform {
		if t.Provider == "" {
			continue
		}
		results = append(results, c.probe(ctx, env, "transform", t.Provider, discovered, builtinTransform, probeTimeout))
	}
	if ws.Store.Provider != "" {
		results = append(results, c.probe(ctx, env, "store", ws.Store.Provider, discovered, builtinStore, probeTimeout))
	}
	if ws.UI.Provider != "" {
		// UI plugins frequently need a TTY, so a liveness launch is risky.
		results = append(results, Result{
			CheckID: c.ID(),
			Name:    "ui/" + ws.UI.Provider,
			Status:  StatusSkipped,
			Message: "UI plugin liveness is not probed",
		})
		_ = builtinUI // reserved for future use
	}

	if len(results) == 0 {
		return []Result{skipped(c.ID(), c.ID(), "no providers referenced")}
	}
	return results
}

func (c pluginsAliveCheck) probe(ctx context.Context, env *Environment, kind, name string, discovered map[string]core.PluginInfo, builtins map[string]bool, timeout time.Duration) Result {
	label := kind + "/" + name
	if builtins[name] {
		// In-process factory: no RPC to make, but report OK for symmetry.
		return Result{
			CheckID: pluginsAliveCheck{}.ID(),
			Name:    label,
			Status:  StatusOK,
			Message: "built-in provider (in-process)",
		}
	}
	info, ok := discovered[label]
	if !ok {
		return Result{
			CheckID: pluginsAliveCheck{}.ID(),
			Name:    label,
			Status:  StatusError,
			Message: "plugin binary not discovered",
			Hint:    "run `zhi plugin install` or check workspace plugin directories",
		}
	}

	// Sub-context for a single probe — keep it independent of the parent
	// context so cancelling it does not affect the enclosing check.
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	switch kind {
	case "config":
		p, cleanup, err := launch.LaunchConfig(info.Path)
		if err != nil {
			return rpcFailure(label, "plugin did not respond", err)
		}
		defer cleanup()
		if _, err := p.List(probeCtx); err != nil {
			return rpcFailure(label, "plugin did not respond", err)
		}
	case "transform":
		p, cleanup, err := launch.LaunchTransform(info.Path)
		if err != nil {
			return rpcFailure(label, "plugin did not respond", err)
		}
		defer cleanup()
		if _, err := p.ValidatePolicy(probeCtx); err != nil {
			return rpcFailure(label, "plugin did not respond", err)
		}
	case "store":
		p, cleanup, err := launch.LaunchStore(info.Path)
		if err != nil {
			return rpcFailure(label, "plugin did not respond", err)
		}
		defer cleanup()
		if _, err := p.Capabilities(probeCtx); err != nil {
			return rpcFailure(label, "plugin did not respond", err)
		}
	default:
		return Result{
			CheckID: pluginsAliveCheck{}.ID(),
			Name:    label,
			Status:  StatusSkipped,
			Message: "unknown plugin kind",
		}
	}

	latency := time.Since(start).Round(time.Millisecond)
	return Result{
		CheckID: pluginsAliveCheck{}.ID(),
		Name:    label,
		Status:  StatusOK,
		Message: fmt.Sprintf("responded in %s", latency),
	}
}

type pluginsSignatureVerifiedCheck struct{}

func (pluginsSignatureVerifiedCheck) ID() string         { return "plugins.signature_verified" }
func (pluginsSignatureVerifiedCheck) Category() Category { return CategoryPlugins }
func (pluginsSignatureVerifiedCheck) Description() string {
	return "Installed plugin binaries match their lockfile digests (deep mode only)."
}

func (c pluginsSignatureVerifiedCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || !env.Deep {
		return []Result{skipped(c.ID(), c.ID(), "enable with --deep")}
	}
	if env.Workspace == nil {
		return []Result{skipped(c.ID(), c.ID(), "no workspace loaded")}
	}
	if len(env.Discovered) == 0 {
		return []Result{skipped(c.ID(), c.ID(), "no discovered plugins")}
	}

	lockPath := filepath.Join(env.Workspace.Dir, "zhi-plugins.lock")
	lf, err := lockfile.LoadFile(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Result{skipped(c.ID(), c.ID(), "no zhi-plugins.lock found")}
		}
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusError,
			Message: "lockfile unreadable",
			Detail:  err.Error(),
		}}
	}

	platformKey := runtime.GOOS + "/" + runtime.GOARCH
	results := make([]Result, 0, len(lf.Plugins))
	for _, locked := range lf.Plugins {
		label := locked.Type + "/" + locked.Name
		info, ok := findDiscovered(env.Discovered, locked.Name, locked.Type)
		if !ok {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    label,
				Status:  StatusWarning,
				Message: "locked plugin not discovered",
				Detail:  "no binary matching lockfile entry",
			})
			continue
		}
		expected, has := locked.Platforms[platformKey]
		if !has {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    label,
				Status:  StatusWarning,
				Message: "no digest recorded for this platform",
				Detail:  "platform " + platformKey + " not in lockfile entry",
			})
			continue
		}
		if err := verify.VerifyBinaryDigest(info.Path, expected); err != nil {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    label,
				Status:  StatusError,
				Message: "digest mismatch",
				Detail:  err.Error(),
				Hint:    "reinstall with `zhi plugin install --force`",
			})
			continue
		}
		results = append(results, Result{
			CheckID: c.ID(),
			Name:    label,
			Status:  StatusOK,
			Message: "digest matches lockfile",
		})
	}
	return results
}

// --- helpers ---

// providerRef bundles a workspace provider reference with the set of
// known names for that plugin type, so the check can do a lookup without
// launching external plugins.
type providerRef struct {
	kind  string
	name  string
	names []string
}

// manifestPathFor returns the conventional manifest location next to a
// plugin binary. The binary layout is either flat (~/.zhi/plugins/zhi-<type>-<name>)
// or subdirectory-based (~/.zhi/plugins/<type>/<name>); in both cases the
// manifest is expected as a sibling file.
func manifestPathFor(p core.PluginInfo) string {
	return filepath.Join(filepath.Dir(p.Path), "zhi-plugin.yaml")
}

// builtinSet returns the set of provider names marked as built-in in the
// registry info slice. External providers have Source equal to their
// binary path rather than "built-in".
func builtinSet(infos []core.ProviderInfo) map[string]bool {
	out := make(map[string]bool, len(infos))
	for _, info := range infos {
		if info.Source == "built-in" {
			out[info.Name] = true
		}
	}
	return out
}

func findDiscovered(plugins []core.PluginInfo, name, typ string) (core.PluginInfo, bool) {
	for _, p := range plugins {
		if p.Name == name && string(p.Type) == typ {
			return p, true
		}
	}
	return core.PluginInfo{}, false
}

func skipped(checkID, name, message string) Result {
	return Result{
		CheckID: checkID,
		Name:    name,
		Status:  StatusSkipped,
		Message: message,
	}
}

func rpcFailure(name, message string, err error) Result {
	return Result{
		CheckID: pluginsAliveCheck{}.ID(),
		Name:    name,
		Status:  StatusError,
		Message: message,
		Detail:  err.Error(),
	}
}
