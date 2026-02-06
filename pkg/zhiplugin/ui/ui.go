// Package ui defines the UI plugin interface for zhi. UI plugins provide
// the interactive frontend (TUI, Web-UI, AI chat, etc.) through which users
// browse and manage configuration trees.
//
// Unlike other plugin types where the core calls the plugin, UI plugins also
// need to call back into the core for operations like saving, validating, and
// exporting. This is achieved through the [Controller] interface, which the
// core provides to the plugin when [Plugin.Run] is called. For external gRPC
// plugins, the Controller is served via hashicorp/go-plugin's GRPCBroker,
// enabling bidirectional communication.
//
// UI plugins that require direct terminal (TTY) access (such as TUI
// implementations) must report RequiresTTY: true in their [Capabilities].
// These plugins must be registered as builtin plugins since external gRPC
// processes do not have access to the host's terminal.
package ui

import (
	"time"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// ExportRequest describes a single export operation for the controller.
type ExportRequest struct {
	TemplatePath string
	Format       string
	OutputPath   string
	Prefix       string
	DryRun       bool
}

// ExportResult holds the output of a single export operation.
type ExportResult struct {
	Name       string
	Content    string
	OutputPath string
}

// ExportTemplate describes a configured export template from the workspace.
type ExportTemplate struct {
	Name     string
	Template string
	Format   string
	Output   string
	Prefix   string
}

// ApplyEvent is a single line of output from an apply command.
type ApplyEvent struct {
	Line   string
	Stream string // "stdout" or "stderr"
}

// ApplyResult is the final outcome of an apply command.
type ApplyResult struct {
	ExitCode int
	Error    string
}

// ComponentInfo describes a component and its current state.
type ComponentInfo struct {
	Name         string
	Description  string
	Enabled      bool
	Mandatory    bool
	Paths        []string
	Dependencies []string
}

// Capabilities describes what a UI plugin supports.
type Capabilities struct {
	// RequiresTTY indicates this UI needs direct terminal access and must
	// run as a builtin plugin (not over gRPC).
	RequiresTTY bool
	// SupportsMarketplace indicates this UI provides marketplace browsing
	// and plugin management features.
	SupportsMarketplace bool
}

// MarketplaceQuery represents a search request.
type MarketplaceQuery struct {
	Query    string // free-text search
	Type     string // filter: "config", "transform", "store", "ui", "workspace"
	Sort     string // "relevance", "downloads", "rating", "updated"
	Verified bool   // only verified publishers
	Page     int
	PerPage  int
}

// MarketplaceResults holds search results.
type MarketplaceResults struct {
	Total   int
	Results []MarketplaceEntry
}

// MarketplaceEntry is a single search result.
type MarketplaceEntry struct {
	Name          string
	Publisher     string
	Type          string
	Description   string
	LatestVersion string
	Rating        float64
	RatingCount   int
	Downloads     int
	Verified      bool
	Installed     bool   // true if already installed locally
	InstalledVer  string // installed version (empty if not installed)
	UpdateAvail   bool   // true if newer version exists
	Platforms     []string
}

// MarketplaceDetail holds full information for a single artifact.
type MarketplaceDetail struct {
	MarketplaceEntry
	LongDescription string
	License         string
	Homepage        string
	Repository      string
	Versions        []VersionEntry
	Ratings         []RatingEntry
	Dependencies    []DependencyEntry
	Keywords        []string
}

// VersionEntry describes a single version of a marketplace artifact.
type VersionEntry struct {
	Version   string
	CreatedAt time.Time
	Digest    string
	Platforms []string
}

// RatingEntry describes a single rating submission.
type RatingEntry struct {
	Score     int
	Comment   string
	Author    string
	CreatedAt time.Time
}

// DependencyEntry describes a dependency of a workspace.
type DependencyEntry struct {
	Name      string
	Type      string
	Publisher string
	Required  bool
}

// InstalledPlugin describes a locally installed plugin.
type InstalledPlugin struct {
	Name        string
	Type        string
	Version     string
	Source      string // "built-in", OCI ref, or local path
	InstalledAt time.Time
	Digest      string
	Verified    bool
	UpdateAvail string // latest available version, empty if up to date
}

// InstallResult describes the outcome of an install/update operation.
type InstallResult struct {
	Name        string
	Type        string
	Version     string
	PrevVersion string // empty for fresh installs
	Digest      string
	Verified    bool
	RuntimeDeps []string
}

// PluginUpdate describes an available update for an installed plugin.
type PluginUpdate struct {
	Name           string
	Type           string
	CurrentVersion string
	LatestVersion  string
	Verified       bool
}

// Rating is a user-submitted review.
type Rating struct {
	Score   int // 1-5
	Comment string
}

// TreeFromEntries reconstructs a [config.Tree] from a list of path/value
// pairs. This is used by gRPC serialisation helpers.
func TreeFromEntries(entries []TreeEntry) *config.Tree {
	t := config.NewTree()
	for _, e := range entries {
		v := e.Value
		// Paths were validated at Set time; re-validation via Set is
		// acceptable here.
		_ = t.Set(e.Path, &v)
	}
	return t
}

// TreeEntry is a single path+value pair used for tree serialisation.
type TreeEntry struct {
	Path  string
	Value config.Value
}

// TreeToEntries converts a [config.TreeReader] into a flat list of
// [TreeEntry] values suitable for serialisation.
func TreeToEntries(tree config.TreeReader) []TreeEntry {
	paths := tree.List()
	entries := make([]TreeEntry, 0, len(paths))
	for _, p := range paths {
		v, ok := tree.Get(p)
		if !ok {
			continue
		}
		entries = append(entries, TreeEntry{Path: p, Value: v})
	}
	return entries
}
