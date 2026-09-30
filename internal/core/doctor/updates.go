package doctor

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MrWong99/zhi/pkg/sharing/lockfile"
	"github.com/MrWong99/zhi/pkg/sharing/semver"
)

type updatesAvailableCheck struct{}

func (updatesAvailableCheck) ID() string         { return "updates.available" }
func (updatesAvailableCheck) Category() Category { return CategoryUpdates }
func (updatesAvailableCheck) Description() string {
	return "Installed plugin versions are the latest published on the marketplace."
}

func (c updatesAvailableCheck) Run(ctx context.Context, env *Environment) []Result {
	if env == nil || env.Marketplace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "marketplace client not configured (enable with --check updates)",
		}}
	}
	if env.Workspace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "no workspace loaded",
		}}
	}

	lockPath := filepath.Join(env.Workspace.Dir, "zhi-plugins.lock")
	lf, err := lockfile.LoadFile(lockPath)
	if err != nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusSkipped,
			Message: "no zhi-plugins.lock found",
			Detail:  err.Error(),
		}}
	}

	if len(lf.Plugins) == 0 {
		return []Result{{
			CheckID: c.ID(),
			Name:    c.ID(),
			Status:  StatusOK,
			Message: "no plugins locked",
		}}
	}

	results := make([]Result, 0, len(lf.Plugins))
	for _, p := range lf.Plugins {
		name := p.Type + "/" + p.Name
		publisher, short, ok := parseOCIRef(p.Ref)
		if !ok {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusSkipped,
				Message: "could not parse OCI ref for marketplace lookup",
				Detail:  p.Ref,
			})
			continue
		}

		detail, err := env.Marketplace.GetPlugin(ctx, p.Type, publisher, short)
		if err != nil {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusWarning,
				Message: "marketplace lookup failed",
				Detail:  err.Error(),
			})
			continue
		}
		if len(detail.Versions) == 0 {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusSkipped,
				Message: "no published versions",
			})
			continue
		}

		// Versions are ordered newest-first per the marketplace API contract.
		current := trimVersion(p.Ref)
		latest := detail.Versions[0].Version
		if current == "" {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusSkipped,
				Message: "installed version could not be determined",
			})
			continue
		}

		newer, cmpErr := semver.IsNewer(current, latest)
		if cmpErr != nil {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusSkipped,
				Message: "version comparison failed",
				Detail:  cmpErr.Error(),
			})
			continue
		}
		if newer {
			results = append(results, Result{
				CheckID: c.ID(),
				Name:    name,
				Status:  StatusWarning,
				Message: fmt.Sprintf("update available: %s → %s", current, latest),
				Hint:    "run `zhi plugin update " + short + "`",
			})
			continue
		}
		results = append(results, Result{
			CheckID: c.ID(),
			Name:    name,
			Status:  StatusOK,
			Message: "up to date (" + current + ")",
		})
	}
	return results
}

// parseOCIRef pulls the publisher and short name from a ref like
// "ghcr.io/mrwong99/zhi/zhi-store-memory:v0.0.9". We treat the first path
// segment after the host as the publisher and the last segment (before the
// tag, stripping any "zhi-<type>-" prefix) as the short name.
func parseOCIRef(ref string) (publisher, short string, ok bool) {
	ref = strings.TrimPrefix(ref, "oci://")
	// Drop the tag.
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		ref = ref[:i]
	}
	parts := strings.Split(ref, "/")
	if len(parts) < 3 {
		return "", "", false
	}
	publisher = parts[1]
	last := parts[len(parts)-1]
	// zhi-<type>-<name> → <name>; otherwise use the raw last segment.
	if strings.HasPrefix(last, "zhi-") {
		rest := strings.TrimPrefix(last, "zhi-")
		for _, t := range []string{"config-", "transform-", "store-", "ui-"} {
			if strings.HasPrefix(rest, t) {
				short = strings.TrimPrefix(rest, t)
				return publisher, short, short != ""
			}
		}
	}
	return publisher, last, last != ""
}

// trimVersion extracts a trailing :vX.Y.Z tag from an OCI ref.
func trimVersion(ref string) string {
	i := strings.LastIndex(ref, ":")
	if i < 0 {
		return ""
	}
	return strings.TrimPrefix(ref[i+1:], "v")
}
