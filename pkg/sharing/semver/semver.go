// Package semver provides semver version parsing, comparison, and constraint
// matching for the zhi plugin update system. It wraps github.com/Masterminds/semver/v3
// with zhi-specific convenience functions for update scope classification.
package semver

import (
	"fmt"
	"strings"

	sv "github.com/Masterminds/semver/v3"
)

// UpdateScope classifies the magnitude of a version change.
type UpdateScope string

const (
	// ScopePatch indicates a patch-level update (e.g. 1.2.0 → 1.2.1).
	ScopePatch UpdateScope = "patch"
	// ScopeMinor indicates a minor-level update (e.g. 1.2.0 → 1.3.0).
	ScopeMinor UpdateScope = "minor"
	// ScopeMajor indicates a major-level update (e.g. 1.2.0 → 2.0.0).
	ScopeMajor UpdateScope = "major"
	// ScopeNone indicates no version change.
	ScopeNone UpdateScope = "none"
)

// Parse parses a semver version string. Leading "v" is accepted.
func Parse(version string) (*sv.Version, error) {
	v, err := sv.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("parsing version %q: %w", version, err)
	}
	return v, nil
}

// ParseConstraint parses a version constraint string. Supported formats:
//   - "1.2.0"      exact version
//   - ">=1.2.0"    minimum version
//   - "~1.2.0"     patch updates only (1.2.x)
//   - "^1.2.0"     minor updates (1.x.y)
//   - ">=1.0,<2.0" range
func ParseConstraint(constraint string) (*sv.Constraints, error) {
	// Masterminds/semver uses ", " for AND and " || " for OR.
	// We normalise comma-separated constraints to the expected format.
	normalised := strings.ReplaceAll(constraint, ",", ", ")
	c, err := sv.NewConstraint(normalised)
	if err != nil {
		return nil, fmt.Errorf("parsing constraint %q: %w", constraint, err)
	}
	return c, nil
}

// Matches returns true if the given version satisfies the constraint.
func Matches(version string, constraint string) (bool, error) {
	v, err := Parse(version)
	if err != nil {
		return false, err
	}
	c, err := ParseConstraint(constraint)
	if err != nil {
		return false, err
	}
	return c.Check(v), nil
}

// CompareVersions returns the UpdateScope between two version strings.
// The from version should be older than the to version.
func CompareVersions(from, to string) (UpdateScope, error) {
	vFrom, err := Parse(from)
	if err != nil {
		return ScopeNone, err
	}
	vTo, err := Parse(to)
	if err != nil {
		return ScopeNone, err
	}

	if vFrom.Equal(vTo) {
		return ScopeNone, nil
	}

	if vTo.Major() != vFrom.Major() {
		return ScopeMajor, nil
	}
	if vTo.Minor() != vFrom.Minor() {
		return ScopeMinor, nil
	}
	return ScopePatch, nil
}

// IsAutoUpdateAllowed checks whether an update from fromVersion to toVersion
// is permitted under the given autoInstallScope.
func IsAutoUpdateAllowed(fromVersion, toVersion, autoInstallScope string) (bool, error) {
	scope, err := CompareVersions(fromVersion, toVersion)
	if err != nil {
		return false, err
	}

	switch UpdateScope(autoInstallScope) {
	case ScopePatch:
		return scope == ScopePatch, nil
	case ScopeMinor:
		return scope == ScopePatch || scope == ScopeMinor, nil
	case UpdateScope("all"):
		return true, nil
	default:
		return false, nil
	}
}

// LatestMatching returns the highest version from candidates that satisfies
// the constraint. Returns empty string if none match.
func LatestMatching(candidates []string, constraint string) (string, error) {
	c, err := ParseConstraint(constraint)
	if err != nil {
		return "", err
	}

	var best *sv.Version
	var bestStr string
	for _, s := range candidates {
		v, err := Parse(s)
		if err != nil {
			continue
		}
		if !c.Check(v) {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best = v
			bestStr = s
		}
	}
	return bestStr, nil
}

// Latest returns the highest version from the candidates list.
// Returns empty string if the list is empty or no valid versions are found.
func Latest(candidates []string) string {
	var best *sv.Version
	var bestStr string
	for _, s := range candidates {
		v, err := Parse(s)
		if err != nil {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best = v
			bestStr = s
		}
	}
	return bestStr
}

// IsNewer returns true if candidate is a newer version than current.
func IsNewer(current, candidate string) (bool, error) {
	vCurrent, err := Parse(current)
	if err != nil {
		return false, err
	}
	vCandidate, err := Parse(candidate)
	if err != nil {
		return false, err
	}
	return vCandidate.GreaterThan(vCurrent), nil
}
