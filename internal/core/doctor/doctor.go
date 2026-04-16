package doctor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
)

// DefaultTimeout is the per-check wall-clock deadline used when the
// caller does not specify one.
const DefaultTimeout = 10 * time.Second

// EnvironmentOptions configures BuildEnvironment.
type EnvironmentOptions struct {
	// WorkspacePath is the directory to load the workspace from.
	WorkspacePath string
	// ZhiVersion is the running zhi version; passed in by the caller
	// because the build-time version lives in the cli package and the
	// doctor package must not import it (cycle).
	ZhiVersion string
	// Deep enables slow / expensive checks (signature verification etc.).
	Deep bool
	// Timeout is the per-check deadline. Zero means DefaultTimeout.
	Timeout time.Duration
	// MarketplaceURL overrides the marketplace endpoint. Empty =
	// marketplace.DefaultURL. Only consulted when Updates is true.
	MarketplaceURL string
	// Updates enables the updates category (requires network).
	Updates bool
}

// BuildEnvironment loads the workspace, discovers plugins, and builds an
// Engine. Failures are captured on the returned Environment rather than
// returned as errors so checks can still run and report them.
func BuildEnvironment(opts EnvironmentOptions) *Environment {
	env := &Environment{
		WorkspacePath: opts.WorkspacePath,
		ZhiVersion:    opts.ZhiVersion,
		Deep:          opts.Deep,
		Timeout:       opts.Timeout,
	}
	if env.Timeout == 0 {
		env.Timeout = DefaultTimeout
	}
	if opts.Updates {
		url := opts.MarketplaceURL
		if url == "" {
			url = marketplace.DefaultURL
		}
		env.Marketplace = marketplace.NewClient(url)
	}

	env.Registry = core.DefaultRegistry()

	ws, err := core.LoadWorkspace(opts.WorkspacePath)
	if err != nil {
		env.WorkspaceErr = err
		return env
	}
	env.Workspace = ws

	// Discover external plugins. Errors here are non-fatal — the
	// registry will still contain built-ins.
	_ = env.Registry.RefreshExternal(core.DiscoveryConfig{
		Directories: ws.PluginDirectories(),
	})

	// Snapshot discovered plugins so checks can see exactly what was
	// found without re-running discovery.
	env.Discovered, _ = core.Discover(core.DiscoveryConfig{
		Directories: ws.PluginDirectories(),
	})

	eng, err := core.NewEngine(env.Registry, ws)
	if err != nil {
		env.EngineErr = err
		return env
	}
	env.Engine = eng

	return env
}

// Close releases resources held by the Environment — at present, any
// plugin processes launched while constructing the engine.
func (e *Environment) Close() {
	if e == nil || e.Engine == nil {
		return
	}
	e.Engine.Close()
}

// Runner executes a filtered set of Checks and aggregates their
// results into a Report.
type Runner struct {
	checks     []Check
	categories []Category
}

// NewRunner builds a Runner that will execute the given checks when
// their category is in the filter. An empty filter selects every
// category listed in AllCategories.
func NewRunner(checks []Check, filter []Category) *Runner {
	if len(filter) == 0 {
		filter = DefaultCategories
	}
	return &Runner{checks: checks, categories: filter}
}

// Run executes every selected check and returns the aggregated Report.
// Checks are invoked sequentially within a category so output order is
// stable; categories themselves run in the order declared by filter.
//
// Each check receives a context derived from ctx with env.Timeout as
// the deadline. Panics inside a check are recovered and surfaced as a
// StatusError Result so one bad check does not take down the whole run.
func (r *Runner) Run(ctx context.Context, env *Environment) Report {
	var report Report
	for _, cat := range r.categories {
		group := CategoryGroup{Category: cat}
		for _, check := range r.checks {
			if check.Category() != cat {
				continue
			}
			group.Results = append(group.Results, runCheck(ctx, env, check)...)
		}
		report.Groups = append(report.Groups, group)
	}
	return report
}

// runCheck wraps a single Check invocation with timeout + panic recovery.
func runCheck(parent context.Context, env *Environment, c Check) (out []Result) {
	defer func() {
		if rec := recover(); rec != nil {
			out = append(out, Result{
				CheckID: c.ID(),
				Name:    c.ID(),
				Status:  StatusError,
				Message: fmt.Sprintf("check panicked: %v", rec),
			})
		}
	}()

	ctx := parent
	if env.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, env.Timeout)
		defer cancel()
	}

	results := c.Run(ctx, env)
	// Honour context cancellation — if the check ignored the deadline
	// but the context did expire we emit a warning alongside any
	// returned results, so users know the run was cut short.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		results = append(results, Result{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusWarning,
			Message: fmt.Sprintf("check exceeded %s deadline", env.Timeout),
		})
	}
	return results
}

// AllChecks returns every registered Check, in the order they should
// appear for each category.
func AllChecks() []Check {
	return []Check{
		// Workspace
		&workspaceExistsCheck{},
		&workspaceParsesCheck{},
		&workspaceValidatesCheck{},
		&workspacePluginDirsCheck{},
		&workspaceFileStoreCheck{},

		// Plugins
		&pluginsReferencedInstalledCheck{},
		&pluginsManifestReadableCheck{},
		&pluginsVersionCompatibleCheck{},
		&pluginsAliveCheck{},
		&pluginsSignatureVerifiedCheck{},

		// Store
		&storeCapabilitiesCheck{},
		&storeListTreesCheck{},
		&vaultReachableCheck{},
		&vaultUnsealedCheck{},
		&vaultTokenValidCheck{},
		&vaultTokenTTLCheck{},
		&vaultMountAccessibleCheck{},

		// Config
		&configValidatesCheck{},
		&configNoDeprecatedCheck{},
		&configEnvVarCheck{},
		&configNoConflictsCheck{},

		// Updates
		&updatesAvailableCheck{},
	}
}

// ParseCategories turns a list of user-supplied strings into Category
// values, preserving order and rejecting unknown names. An empty input
// returns DefaultCategories.
func ParseCategories(names []string) ([]Category, error) {
	if len(names) == 0 {
		return DefaultCategories, nil
	}
	out := make([]Category, 0, len(names))
	for _, raw := range names {
		c := Category(raw)
		if !slices.Contains(AllCategories, c) {
			return nil, fmt.Errorf("unknown check category %q (valid: %s)", raw, categoriesString())
		}
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out, nil
}

func categoriesString() string {
	s := ""
	for i, c := range AllCategories {
		if i > 0 {
			s += "|"
		}
		s += string(c)
	}
	return s
}
