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
