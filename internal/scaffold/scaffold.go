// Package scaffold provides the plugin scaffolding system for generating
// new plugin projects from templates. It defines the Scaffolder interface
// and a registry for language-specific scaffolders.
//
// To add support for a new language, implement the Scaffolder interface and
// register it with Register.
package scaffold

import (
	"fmt"
	"strings"
	"sync"
)

// Data holds the template variables used to render scaffold templates.
type Data struct {
	// PluginName is the short name of the plugin (e.g. "my-plugin").
	PluginName string

	// PluginType is the type of plugin: config, store, transform, or ui.
	PluginType string

	// BinaryName is the full binary name (e.g. "zhi-config-my-plugin").
	BinaryName string

	// ModulePath is the language-specific module path (e.g. Go module path).
	ModulePath string

	// PackageName is a language-safe identifier derived from PluginName.
	// For Go, hyphens are removed and the result is lowercased.
	PackageName string

	// Author is the author or organisation name.
	Author string

	// License is the SPDX license identifier (e.g. "MIT", "Apache-2.0").
	License string

	// Version is the initial semver version (e.g. "0.1.0").
	Version string

	// Description is a short description of the plugin.
	Description string

	// Registry is the OCI registry path for publishing (e.g. "ghcr.io/user").
	Registry string

	// GoVersion is the Go version to use in go.mod (e.g. "1.26").
	GoVersion string

	// ZhiModule is the zhi Go module path.
	ZhiModule string

	// ZhiVersion is the zhi Go module version to require in go.mod (e.g.
	// "v1.10.2"). When empty, the scaffolder substitutes a known-good
	// default so the generated project builds out of the box.
	ZhiVersion string

	// Year is the current year for license headers.
	Year string
}

// Scaffolder generates a plugin project in a given output directory.
type Scaffolder interface {
	// Scaffold generates the plugin project files in outputDir using the
	// provided template data. The outputDir must not already exist.
	Scaffold(data Data, outputDir string) error

	// Language returns the language identifier (e.g. "go", "python").
	Language() string
}

var (
	mu          sync.RWMutex
	scaffolders = make(map[string]Scaffolder)
)

// Register adds a scaffolder to the global registry. It panics if a
// scaffolder with the same language is already registered.
func Register(s Scaffolder) {
	mu.Lock()
	defer mu.Unlock()
	lang := s.Language()
	if _, exists := scaffolders[lang]; exists {
		panic(fmt.Sprintf("scaffold: scaffolder already registered for language %q", lang))
	}
	scaffolders[lang] = s
}

// Get returns the scaffolder for the given language, or an error if none
// is registered.
func Get(lang string) (Scaffolder, error) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := scaffolders[lang]
	if !ok {
		return nil, fmt.Errorf("no scaffolder registered for language %q (available: %s)", lang, Languages())
	}
	return s, nil
}

// Languages returns a comma-separated list of registered language names.
func Languages() string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(scaffolders))
	for name := range scaffolders {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// ToPackageName converts a plugin name like "my-plugin" to a Go-safe
// package name like "myplugin".
func ToPackageName(name string) string {
	return strings.ReplaceAll(name, "-", "")
}
