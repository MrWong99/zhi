package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// PluginType identifies the kind of external plugin.
type PluginType string

const (
	PluginTypeConfig    PluginType = "config"
	PluginTypeTransform PluginType = "transform"
	PluginTypeStore     PluginType = "store"
	PluginTypeUI        PluginType = "ui"
)

// PluginInfo describes a discovered external plugin binary.
type PluginInfo struct {
	Name string     // provider name (e.g. "pokedex")
	Type PluginType // plugin type: config, transform, or store
	Path string     // absolute path to the binary
}

// DiscoveryConfig controls where to scan for external plugin binaries.
type DiscoveryConfig struct {
	// Directories to scan for plugin binaries. If empty, defaults to
	// ~/.zhi/plugins/.
	Directories []string
}

// DefaultPluginDir returns the default plugin directory (~/.zhi/plugins/).
func DefaultPluginDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zhi", "plugins")
}

// Discover scans the configured directories for external plugin binaries
// and returns information about each found plugin. Non-executable files
// are skipped. Both flat naming (zhi-<type>-<name>) and subdirectory
// layouts (<type>/<name>) are supported. Flat naming takes precedence.
func Discover(cfg DiscoveryConfig) ([]PluginInfo, error) {
	dirs := cfg.Directories
	if len(dirs) == 0 {
		if d := DefaultPluginDir(); d != "" {
			dirs = []string{d}
		}
	}

	// Track discovered plugins by type+name to avoid duplicates.
	seen := make(map[string]bool)
	var plugins []PluginInfo

	log := Logger()

	for _, dir := range dirs {
		// Reject directories containing path traversal.
		if containsPathTraversal(dir) {
			log.Warn("skipping plugin directory with path traversal", "dir", dir)
			continue
		}

		expanded := expandHome(dir)
		flat, err := discoverFlat(expanded)
		if err != nil {
			log.Debug("skipping plugin directory", "dir", expanded, "error", err)
			continue // skip directories that don't exist or can't be read
		}
		for _, p := range flat {
			key := string(p.Type) + "/" + p.Name
			if !seen[key] {
				seen[key] = true
				plugins = append(plugins, p)
				log.Debug("discovered plugin", "type", p.Type, "name", p.Name, "path", p.Path)
			}
		}

		sub, err := discoverSubdirectory(expanded)
		if err != nil {
			continue
		}
		for _, p := range sub {
			key := string(p.Type) + "/" + p.Name
			if !seen[key] {
				seen[key] = true
				plugins = append(plugins, p)
				log.Debug("discovered plugin", "type", p.Type, "name", p.Name, "path", p.Path)
			}
		}
	}

	log.Debug("plugin discovery complete", "count", len(plugins))
	return plugins, nil
}

// discoverFlat scans a directory for binaries named zhi-<type>-<name>.
func discoverFlat(dir string) ([]PluginInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var plugins []PluginInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		pluginType, pluginName, ok := parseFlatName(name)
		if !ok {
			continue
		}
		path := filepath.Join(dir, name)
		if !isExecutable(entry) {
			continue
		}
		plugins = append(plugins, PluginInfo{
			Name: pluginName,
			Type: pluginType,
			Path: path,
		})
	}
	return plugins, nil
}

// discoverSubdirectory scans for type-based subdirectory layouts:
// <dir>/config/<name>, <dir>/transform/<name>, <dir>/store/<name>,
// <dir>/ui/<name>.
func discoverSubdirectory(dir string) ([]PluginInfo, error) {
	typeNames := []PluginType{PluginTypeConfig, PluginTypeTransform, PluginTypeStore, PluginTypeUI}
	var plugins []PluginInfo

	for _, pt := range typeNames {
		subdir := filepath.Join(dir, string(pt))
		entries, err := os.ReadDir(subdir)
		if err != nil {
			continue // subdirectory doesn't exist
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !isExecutable(entry) {
				continue
			}
			plugins = append(plugins, PluginInfo{
				Name: entry.Name(),
				Type: pt,
				Path: filepath.Join(subdir, entry.Name()),
			})
		}
	}
	return plugins, nil
}

// parseFlatName parses a binary name like "zhi-config-pokedex" into
// (PluginTypeConfig, "pokedex", true). Returns false if the name doesn't
// match the expected pattern.
func parseFlatName(name string) (PluginType, string, bool) {
	if !strings.HasPrefix(name, "zhi-") {
		return "", "", false
	}
	rest := name[len("zhi-"):]

	// Try each known type prefix.
	for _, pt := range []PluginType{PluginTypeConfig, PluginTypeTransform, PluginTypeStore, PluginTypeUI} {
		prefix := string(pt) + "-"
		if strings.HasPrefix(rest, prefix) {
			pluginName := rest[len(prefix):]
			if pluginName == "" {
				return "", "", false
			}
			return pt, pluginName, true
		}
	}
	return "", "", false
}

// isExecutable checks if a directory entry has executable permission.
func isExecutable(entry fs.DirEntry) bool {
	info, err := entry.Info()
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// String returns a human-readable representation of PluginInfo.
func (p PluginInfo) String() string {
	return fmt.Sprintf("%s/%s (%s)", p.Type, p.Name, p.Path)
}
