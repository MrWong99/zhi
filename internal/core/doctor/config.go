package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

// envVarPattern detects literal ${VAR} expansions that zhi currently stores
// verbatim (no shell-style substitution is performed).
var envVarPattern = regexp.MustCompile(`\$\{[^}]+\}`)

// envVarHint is attached to every env-var expansion warning so users know
// how to track the feature request.
const envVarHint = "env-var expansion is not implemented in zhi; these strings are stored literally. See https://github.com/MrWong99/zhi/issues (file a request)"

type configValidatesCheck struct{}

func (configValidatesCheck) ID() string         { return "config.validates" }
func (configValidatesCheck) Category() Category { return CategoryConfig }
func (configValidatesCheck) Description() string {
	return "engine.Validate returns no blocking results across the full tree."
}

func (c configValidatesCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || env.Engine == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "engine not initialized",
		}}
	}

	tree, err := env.Engine.LoadTree(ctx)
	if err != nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusError,
			Message: "could not load config tree",
			Detail:  err.Error(),
			Hint:    "run `zhi validate` for detail",
		}}
	}

	// Transform failures are non-fatal — fall back to the raw tree so
	// validation still runs and users hear about the transform issue.
	var out []Result
	if err := env.Engine.TransformForDisplay(ctx, tree); err != nil {
		out = append(out, Result{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusWarning,
			Message: "transform for display failed",
			Detail:  err.Error(),
		})
	}

	results, err := env.Engine.Validate(ctx, tree)
	if err != nil {
		out = append(out, Result{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusError,
			Message: "validation failed",
			Detail:  err.Error(),
			Hint:    "run `zhi validate` for detail",
		})
		return out
	}

	var blocking, warning, info int
	var blockingMsgs []string
	for _, r := range results {
		switch r.Severity {
		case config.Blocking:
			blocking++
			if len(blockingMsgs) < 3 {
				blockingMsgs = append(blockingMsgs, r.Message)
			}
		case config.Warning:
			warning++
		case config.Info:
			info++
		}
	}

	switch {
	case blocking > 0:
		out = append(out, Result{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusError,
			Message: fmt.Sprintf("%d blocking, %d warning, %d info", blocking, warning, info),
			Detail:  strings.Join(blockingMsgs, "; "),
			Hint:    "run `zhi validate` for detail",
		})
	case warning > 0:
		out = append(out, Result{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusWarning,
			Message: fmt.Sprintf("%d blocking, %d warning, %d info", blocking, warning, info),
			Hint:    "run `zhi validate` for detail",
		})
	default:
		out = append(out, Result{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusOK,
			Message: "no validation findings",
		})
	}
	return out
}

type configNoDeprecatedCheck struct{}

func (configNoDeprecatedCheck) ID() string         { return "config.no_deprecated_fields" }
func (configNoDeprecatedCheck) Category() Category { return CategoryConfig }
func (configNoDeprecatedCheck) Description() string {
	return "No configuration value carries a core.deprecated label."
}

func (c configNoDeprecatedCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || env.Engine == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "engine not initialized",
		}}
	}

	tree, err := env.Engine.LoadTree(ctx)
	if err != nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "tree load failed, see config.validates",
			Detail:  err.Error(),
		}}
	}

	var out []Result
	paths := tree.List()
	for _, path := range paths {
		v, ok := tree.Get(path)
		if !ok {
			continue
		}
		raw, present := v.Metadata[labels.LabelCoreDeprecated]
		if !present {
			continue
		}
		msg := fmt.Sprintf("%v", raw)
		if msg == "" {
			continue
		}
		out = append(out, Result{
			CheckID: c.ID(),
			Name:    path,
			Status:  StatusWarning,
			Message: "deprecated",
			Hint:    msg,
		})
	}

	if len(out) == 0 {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusOK,
			Message: "no deprecated fields in use",
		}}
	}
	return out
}

type configEnvVarCheck struct{}

func (configEnvVarCheck) ID() string         { return "config.env_var_expansion" }
func (configEnvVarCheck) Category() Category { return CategoryConfig }
func (configEnvVarCheck) Description() string {
	return "Detect literal ${VAR} expressions — zhi does not yet expand them."
}

func (c configEnvVarCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || env.Workspace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "workspace not loaded",
		}}
	}

	type match struct {
		location string
		expr     string
	}
	var matches []match

	// Scan the raw workspace file if we can find it.
	path, ok := findWorkspaceFile(env.Workspace.Dir)
	if ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return []Result{{
				CheckID: c.ID(),
				Name:    c.ID(),
				Status:  StatusSkipped,
				Message: "could not read workspace file",
				Detail:  err.Error(),
			}}
		}
		base := filepath.Base(path)
		for i, line := range strings.Split(string(data), "\n") {
			for _, m := range envVarPattern.FindAllString(line, -1) {
				matches = append(matches, match{
					location: fmt.Sprintf("%s:%d: %s", base, i+1, m),
					expr:     m,
				})
			}
		}
	}

	// Also inspect any string-typed values in the loaded tree.
	if env.Engine != nil {
		if tree, err := env.Engine.LoadTree(ctx); err == nil {
			for _, p := range tree.List() {
				v, ok := tree.Get(p)
				if !ok {
					continue
				}
				s, ok := v.Val.(string)
				if !ok {
					continue
				}
				for _, m := range envVarPattern.FindAllString(s, -1) {
					matches = append(matches, match{
						location: fmt.Sprintf("%s: %s", p, m),
						expr:     m,
					})
				}
			}
		}
	}

	if len(matches) == 0 {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusOK,
			Message: "no literal ${VAR} patterns detected",
		}}
	}

	// Collapse into a single summary — per-match results are too noisy
	// when a workspace leans heavily on placeholders.
	previewCount := 5
	if previewCount > len(matches) {
		previewCount = len(matches)
	}
	preview := make([]string, previewCount)
	for i := 0; i < previewCount; i++ {
		preview[i] = matches[i].location
	}
	return []Result{{
		CheckID: c.ID(),
		Name:    c.ID(),
		Status:  StatusWarning,
		Message: fmt.Sprintf("%d literal ${VAR} patterns detected", len(matches)),
		Detail:  strings.Join(preview, "\n"),
		Hint:    envVarHint,
	}}
}

type configNoConflictsCheck struct{}

func (configNoConflictsCheck) ID() string         { return "config.no_conflicts" }
func (configNoConflictsCheck) Category() Category { return CategoryConfig }
func (configNoConflictsCheck) Description() string {
	return "Component dependencies resolve without cycles or overlaps."
}

func (c configNoConflictsCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || env.Engine == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "engine not initialized",
		}}
	}

	errs := env.Engine.Components().ValidateDependencies()
	if len(errs) == 0 {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusOK,
			Message: "component dependencies are consistent",
		}}
	}

	out := make([]Result, 0, len(errs))
	for _, e := range errs {
		out = append(out, Result{
			CheckID: c.ID(),
			Name:    "components",
			Status:  StatusError,
			Message: e.Error(),
		})
	}
	return out
}

// findWorkspaceFile returns the on-disk path of the workspace config file in
// dir. Mirrors the candidate order used by core.LoadWorkspace so the raw file
// scan sees exactly what the loader did.
func findWorkspaceFile(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	for _, name := range []string{"zhi.yaml", "zhi.yml", "zhi.json"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}
