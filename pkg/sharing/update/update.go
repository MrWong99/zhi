// Package update implements plugin update detection, caching, and automatic
// update logic for the zhi plugin system. It queries the marketplace for
// available versions, compares against installed metadata, and manages an
// update check cache to avoid excessive API calls.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/semver"
)

// DefaultCheckInterval is how long update check results are cached.
const DefaultCheckInterval = 24 * time.Hour

// UpdateInfo describes an available update for a single plugin.
type UpdateInfo struct {
	// Name is the plugin's short name.
	Name string `json:"name"`
	// Installed is the currently installed version.
	Installed string `json:"installed"`
	// Available is the latest available version.
	Available string `json:"available"`
	// Scope is the update magnitude: patch, minor, major.
	Scope semver.UpdateScope `json:"scope"`
	// HasAdvisory indicates a security advisory exists for the installed version.
	HasAdvisory bool `json:"hasAdvisory,omitempty"`
	// Changelog is the changelog text for the available version.
	Changelog string `json:"changelog,omitempty"`
	// Pinned indicates the plugin is pinned and should be skipped during --all updates.
	Pinned bool `json:"pinned,omitempty"`
}

// CheckResult holds the results of an update check across all plugins.
type CheckResult struct {
	// CheckedAt is when the check was performed.
	CheckedAt time.Time `json:"checkedAt"`
	// Updates lists available updates.
	Updates []UpdateInfo `json:"updates"`
}

// Cache stores update check results on disk to avoid excessive API calls.
type Cache struct {
	path     string
	interval time.Duration
}

// NewCache creates a cache at the given path with the specified check interval.
func NewCache(path string, interval time.Duration) *Cache {
	return &Cache{path: path, interval: interval}
}

// DefaultCachePath returns the default update check cache path.
func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zhi", "cache", "update-check.json")
}

// Load reads a cached check result. Returns nil if the cache is missing,
// expired, or invalid.
func (c *Cache) Load() *CheckResult {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil
	}
	var result CheckResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	if time.Since(result.CheckedAt) > c.interval {
		return nil
	}
	return &result
}

// Save writes a check result to the cache file.
func (c *Cache) Save(result *CheckResult) error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling check result: %w", err)
	}
	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		return fmt.Errorf("writing cache: %w", err)
	}
	return nil
}

// Checker performs update checks against the marketplace.
type Checker struct {
	metadataStore *metadata.Store
	marketplace   *marketplace.Client
	cache         *Cache
}

// NewChecker creates an update checker.
func NewChecker(metaStore *metadata.Store, mc *marketplace.Client, cache *Cache) *Checker {
	return &Checker{
		metadataStore: metaStore,
		marketplace:   mc,
		cache:         cache,
	}
}

// CheckAll checks all installed plugins for available updates. If useCache
// is true and a valid cache exists, it returns the cached result.
func (c *Checker) CheckAll(ctx context.Context, useCache bool) (*CheckResult, error) {
	if useCache && c.cache != nil {
		if cached := c.cache.Load(); cached != nil {
			return cached, nil
		}
	}

	plugins, err := c.metadataStore.List()
	if err != nil {
		return nil, fmt.Errorf("listing installed plugins: %w", err)
	}

	if len(plugins) == 0 {
		result := &CheckResult{CheckedAt: time.Now()}
		if c.cache != nil {
			_ = c.cache.Save(result)
		}
		return result, nil
	}

	// Check updates in parallel.
	type updateResult struct {
		info *UpdateInfo
		err  error
	}

	results := make([]updateResult, len(plugins))
	var wg sync.WaitGroup
	for i, p := range plugins {
		wg.Add(1)
		go func(idx int, plugin *metadata.InstalledPlugin) {
			defer wg.Done()
			info, err := c.checkPlugin(ctx, plugin)
			results[idx] = updateResult{info: info, err: err}
		}(i, p)
	}
	wg.Wait()

	var updates []UpdateInfo
	for _, r := range results {
		if r.err != nil {
			continue // skip plugins we can't check
		}
		if r.info != nil {
			updates = append(updates, *r.info)
		}
	}

	result := &CheckResult{
		CheckedAt: time.Now(),
		Updates:   updates,
	}
	if c.cache != nil {
		_ = c.cache.Save(result)
	}
	return result, nil
}

// CheckPlugin checks a single plugin for available updates.
func (c *Checker) CheckPlugin(ctx context.Context, name string) (*UpdateInfo, error) {
	plugin, err := c.metadataStore.Load(name)
	if err != nil {
		return nil, fmt.Errorf("loading metadata for %s: %w", name, err)
	}
	if plugin == nil {
		return nil, fmt.Errorf("plugin %q is not installed", name)
	}
	return c.checkPlugin(ctx, plugin)
}

// checkPlugin queries the marketplace for the latest version of a plugin
// and returns an UpdateInfo if a newer version is available.
func (c *Checker) checkPlugin(ctx context.Context, plugin *metadata.InstalledPlugin) (*UpdateInfo, error) {
	if plugin.Publisher == "" {
		return nil, nil // can't check without publisher
	}

	versions, err := c.marketplace.ListVersions(ctx, plugin.Type, plugin.Publisher, plugin.Name)
	if err != nil {
		return nil, fmt.Errorf("listing versions for %s: %w", plugin.Name, err)
	}

	if len(versions.Versions) == 0 {
		return nil, nil
	}

	// Collect version strings.
	var versionStrings []string
	changelogMap := make(map[string]string)
	for _, v := range versions.Versions {
		versionStrings = append(versionStrings, v.Version)
		if v.Changelog != "" {
			changelogMap[v.Version] = v.Changelog
		}
	}

	latest := semver.Latest(versionStrings)
	if latest == "" {
		return nil, nil
	}

	newer, err := semver.IsNewer(plugin.Version, latest)
	if err != nil || !newer {
		return nil, nil
	}

	scope, _ := semver.CompareVersions(plugin.Version, latest)

	info := &UpdateInfo{
		Name:      plugin.Name,
		Installed: plugin.Version,
		Available: latest,
		Scope:     scope,
		Changelog: changelogMap[latest],
		Pinned:    plugin.Pinned,
	}

	// Check for security advisories that affect the installed version.
	// An advisory only matters when the installed version falls within its
	// AffectedVersions constraint; otherwise (e.g. the user is already on a
	// fixed version) it must not be reported, or the warning becomes
	// permanent noise for every future version.
	advisories, err := c.marketplace.ListAdvisories(ctx, plugin.Publisher, plugin.Name, "")
	if err == nil {
		for _, adv := range advisories.Advisories {
			if advisoryAffects(plugin.Version, adv.AffectedVersions) {
				info.HasAdvisory = true
				break
			}
		}
	}

	return info, nil
}

// advisoryAffects reports whether the installed version falls within an
// advisory's AffectedVersions constraint. An empty or unparsable
// constraint is treated conservatively as affecting all versions, so a
// malformed advisory still surfaces rather than being silently ignored.
func advisoryAffects(installedVersion, affectedVersions string) bool {
	if affectedVersions == "" {
		return true
	}
	matched, err := semver.Matches(installedVersion, affectedVersions)
	if err != nil {
		return true
	}
	return matched
}
