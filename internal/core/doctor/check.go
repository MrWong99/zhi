// Package doctor implements health-check diagnostics for a zhi workspace.
//
// The package exposes a small set of primitives — Check, Result, Runner,
// Report — and a handful of Check implementations grouped into categories
// (workspace, plugins, store, config, updates). The cli/doctor command
// drives the Runner and renders the Report. Checks are self-contained:
// they inspect the Environment they are handed and return findings. The
// Runner tags each finding with the owning check's Category so Result
// instances never carry a category field of their own.
package doctor

import (
	"context"
	"time"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
)

// Category identifies a group of related checks. The CLI filters by
// category and the Report groups findings by category.
type Category string

const (
	CategoryWorkspace Category = "workspace"
	CategoryPlugins   Category = "plugins"
	CategoryStore     Category = "store"
	CategoryConfig    Category = "config"
	CategoryUpdates   Category = "updates"
)

// AllCategories lists every category in the order they are rendered.
// Updates runs last and only when explicitly requested.
var AllCategories = []Category{
	CategoryWorkspace,
	CategoryPlugins,
	CategoryStore,
	CategoryConfig,
	CategoryUpdates,
}

// DefaultCategories is the set of categories that run when the user does
// not pass --check. Updates requires network access and is opt-in.
var DefaultCategories = []Category{
	CategoryWorkspace,
	CategoryPlugins,
	CategoryStore,
	CategoryConfig,
}

// Status is the outcome of a single check finding.
type Status string

const (
	// StatusOK — the check passed.
	StatusOK Status = "ok"
	// StatusWarning — a non-blocking issue that deserves attention.
	StatusWarning Status = "warning"
	// StatusError — a blocking issue; the workspace is unhealthy.
	StatusError Status = "error"
	// StatusSkipped — the check did not apply in this environment.
	StatusSkipped Status = "skipped"
)

// Result is a single finding emitted by a Check. The owning Check
// determines the Category — Result deliberately does not carry a
// Category field (that would duplicate Check.Category()). Grouping is
// the Report's job.
type Result struct {
	CheckID string `json:"check_id"`
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Hint    string `json:"hint,omitempty"`
	Fixable bool   `json:"fixable,omitempty"`
}

// Check is a single diagnostic. The Runner invokes Run and labels the
// returned results with Category() before adding them to the Report.
type Check interface {
	ID() string
	Category() Category
	Description() string
	Run(ctx context.Context, env *Environment) []Result
}

// Fixer is an optional Check extension that knows how to repair state
// it reported on. The CLI calls Fix when --fix is passed and the
// associated Result has Fixable=true.
type Fixer interface {
	Check
	Fix(ctx context.Context, env *Environment, r Result) error
}

// Environment bundles everything a Check might need. Fields may be nil
// when doctor is running in degraded mode — for example, Engine is nil
// if engine construction failed, but Workspace and Discovered may still
// be populated so workspace and plugin checks can still run.
type Environment struct {
	WorkspacePath string
	Workspace     *core.WorkspaceConfig
	WorkspaceErr  error
	Registry      *core.Registry
	Engine        *core.Engine
	EngineErr     error
	Discovered    []core.PluginInfo
	ZhiVersion    string
	Marketplace   *marketplace.Client
	Deep          bool
	// Timeout is the Runner's per-check wall-clock deadline. Checks may
	// consult this for their own internal sub-timeouts, but they must
	// also honour the context passed to Run — the Runner installs a
	// derived context with this deadline.
	Timeout time.Duration
}

// CategoryGroup bundles results from checks that share a category.
type CategoryGroup struct {
	Category Category `json:"category"`
	Results  []Result `json:"results"`
}

// Report is the aggregated output of a Runner.Run invocation. Groups
// preserves the order in which categories ran.
type Report struct {
	Groups []CategoryGroup `json:"groups"`
}

// Counts returns the total number of results with each status across
// the whole report.
func (r Report) Counts() (ok, warning, errCount, skipped int) {
	for _, g := range r.Groups {
		for _, res := range g.Results {
			switch res.Status {
			case StatusOK:
				ok++
			case StatusWarning:
				warning++
			case StatusError:
				errCount++
			case StatusSkipped:
				skipped++
			}
		}
	}
	return
}

// Total returns the total number of non-skipped results.
func (r Report) Total() int {
	ok, warn, errCount, _ := r.Counts()
	return ok + warn + errCount
}

// HasError reports whether any result has StatusError.
func (r Report) HasError() bool {
	_, _, e, _ := r.Counts()
	return e > 0
}

// ExitCode maps a Report to a process exit code:
//
//	0 — no errors (warnings allowed).
//	1 — at least one error result.
func (r Report) ExitCode() int {
	if r.HasError() {
		return 1
	}
	return 0
}
