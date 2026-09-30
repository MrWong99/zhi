package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MrWong99/zhi/internal/core"
)

// Workspace integrity checks. Each check is an empty struct whose
// behaviour lives entirely in its Run method — the types exist so they
// can be listed in AllChecks.

type workspaceExistsCheck struct{}

func (workspaceExistsCheck) ID() string         { return "workspace.exists" }
func (workspaceExistsCheck) Category() Category { return CategoryWorkspace }
func (workspaceExistsCheck) Description() string {
	return "Workspace config file (zhi.yaml / zhi.json) is present and readable."
}

func (c workspaceExistsCheck) Run(ctx context.Context, env *Environment) []Result {
	if env.WorkspaceErr != nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    "workspace present",
			Status:  StatusError,
			Message: "could not locate workspace",
			Detail:  env.WorkspaceErr.Error(),
			Hint:    "run this command inside a directory containing zhi.yaml, or pass --workspace",
		}}
	}
	if env.Workspace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    "workspace present",
			Status:  StatusError,
			Message: "workspace not loaded",
			Hint:    "run this command inside a directory containing zhi.yaml, or pass --workspace",
		}}
	}
	return []Result{{
		CheckID: c.ID(),
		Name:    "workspace present",
		Status:  StatusOK,
		Message: "loaded from " + env.Workspace.Dir,
	}}
}

type workspaceParsesCheck struct{}

func (workspaceParsesCheck) ID() string         { return "workspace.parses" }
func (workspaceParsesCheck) Category() Category { return CategoryWorkspace }
func (workspaceParsesCheck) Description() string {
	return "Workspace YAML / JSON parses without errors."
}

func (c workspaceParsesCheck) Run(ctx context.Context, env *Environment) []Result {
	// LoadWorkspace wraps parse errors with "parsing <path>: ..."; anything
	// else (missing file, unreadable file) is reported by workspace.exists
	// and leaves this check as skipped to avoid double-reporting.
	if env.WorkspaceErr != nil {
		if strings.Contains(env.WorkspaceErr.Error(), "parsing") {
			return []Result{{
				CheckID: c.ID(),
				Name:    "workspace parses",
				Status:  StatusError,
				Message: "workspace could not be parsed",
				Detail:  env.WorkspaceErr.Error(),
				Hint:    "fix the YAML/JSON syntax reported above",
			}}
		}
		return []Result{{
			CheckID: c.ID(),
			Name:    "workspace parses",
			Status:  StatusSkipped,
			Message: "workspace not loaded",
		}}
	}
	if env.Workspace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    "workspace parses",
			Status:  StatusSkipped,
			Message: "workspace not loaded",
		}}
	}
	return []Result{{
		CheckID: c.ID(),
		Name:    "workspace parses",
		Status:  StatusOK,
		Message: "workspace.yaml parsed successfully",
	}}
}

type workspaceValidatesCheck struct{}

func (workspaceValidatesCheck) ID() string         { return "workspace.validates" }
func (workspaceValidatesCheck) Category() Category { return CategoryWorkspace }
func (workspaceValidatesCheck) Description() string {
	return "Workspace passes core.ValidateWorkspace (references, components, templates)."
}

func (c workspaceValidatesCheck) Run(ctx context.Context, env *Environment) []Result {
	if env.Workspace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    "workspace validates",
			Status:  StatusSkipped,
			Message: "workspace not loaded",
		}}
	}
	if err := core.ValidateWorkspace(env.Workspace, env.Registry); err != nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    "workspace validates",
			Status:  StatusError,
			Message: "workspace validation failed",
			Detail:  err.Error(),
			Hint:    "run `zhi validate` for per-path detail",
		}}
	}
	return []Result{{
		CheckID: c.ID(),
		Name:    "workspace validates",
		Status:  StatusOK,
		Message: "workspace validates cleanly",
	}}
}

type workspacePluginDirsCheck struct{}

func (workspacePluginDirsCheck) ID() string         { return "workspace.plugin_dirs_accessible" }
func (workspacePluginDirsCheck) Category() Category { return CategoryWorkspace }
func (workspacePluginDirsCheck) Description() string {
	return "Every configured plugin directory exists and is readable."
}

func (c workspacePluginDirsCheck) Run(ctx context.Context, env *Environment) []Result {
	if env.Workspace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    "plugin directories",
			Status:  StatusSkipped,
			Message: "workspace not loaded",
		}}
	}
	dirs := env.Workspace.PluginDirectories()
	if len(dirs) == 0 {
		return []Result{{
			CheckID: c.ID(),
			Name:    "plugin directories",
			Status:  StatusOK,
			Message: "no plugin directories configured",
		}}
	}

	out := make([]Result, 0, len(dirs))
	for _, raw := range dirs {
		dir := expandTilde(raw)
		info, err := os.Stat(dir)
		switch {
		case err == nil && info.IsDir():
			out = append(out, Result{
				CheckID: c.ID(),
				Name:    "plugin directory",
				Status:  StatusOK,
				Message: dir,
			})
		case err == nil:
			out = append(out, Result{
				CheckID: c.ID(),
				Name:    "plugin directory",
				Status:  StatusError,
				Message: dir,
				Detail:  "path exists but is not a directory",
			})
		case os.IsNotExist(err):
			out = append(out, Result{
				CheckID: c.ID(),
				Name:    "plugin directory",
				Status:  StatusWarning,
				Message: dir,
				Hint:    "plugin directory does not exist; zhi will behave as if no external plugins are installed",
			})
		default:
			out = append(out, Result{
				CheckID: c.ID(),
				Name:    "plugin directory",
				Status:  StatusError,
				Message: dir,
				Detail:  err.Error(),
			})
		}
	}
	return out
}

type workspaceFileStoreCheck struct{}

func (workspaceFileStoreCheck) ID() string         { return "workspace.file_store_path" }
func (workspaceFileStoreCheck) Category() Category { return CategoryWorkspace }
func (workspaceFileStoreCheck) Description() string {
	return "File-backed store providers point at an accessible directory."
}

func (c workspaceFileStoreCheck) Run(ctx context.Context, env *Environment) []Result {
	if env.Workspace == nil {
		return []Result{{
			CheckID: c.ID(),
			Name:    "file store path",
			Status:  StatusSkipped,
			Message: "workspace not loaded",
		}}
	}

	provider := env.Workspace.Store.Provider
	dir, ok := fileStoreDir(env.Workspace)
	if !ok {
		return []Result{{
			CheckID: c.ID(),
			Name:    "file store path",
			Status:  StatusSkipped,
			Message: "store provider is not file-backed",
		}}
	}

	info, err := os.Stat(dir)
	switch {
	case err == nil && info.IsDir():
		return []Result{{
			CheckID: c.ID(),
			Name:    "file store path",
			Status:  StatusOK,
			Message: fmt.Sprintf("%s store directory %s", provider, dir),
		}}
	case err == nil:
		return []Result{{
			CheckID: c.ID(),
			Name:    "file store path",
			Status:  StatusError,
			Message: dir,
			Detail:  "path exists but is not a directory",
		}}
	case os.IsNotExist(err):
		return []Result{{
			CheckID: c.ID(),
			Name:    "file store path",
			Status:  StatusWarning,
			Message: dir,
			Hint:    "directory will be created on first write",
		}}
	default:
		return []Result{{
			CheckID: c.ID(),
			Name:    "file store path",
			Status:  StatusError,
			Message: dir,
			Detail:  err.Error(),
		}}
	}
}

// fileStoreDir resolves the on-disk directory for the workspace's store
// provider when it is one of the file-backed built-ins. The second
// return value is false for providers that do not persist to a
// workspace-relative directory (vault, memory, external plugins, etc.).
//
// The built-in providers read the "directory" option (see
// internal/core/builtin.go); defaults mirror those in
// internal/core/builtin.go and
// pkg/providers/config/structuredfile/structuredfile.go.
func fileStoreDir(ws *core.WorkspaceConfig) (string, bool) {
	var dir string
	switch ws.Store.Provider {
	case "jsonfile":
		dir = core.DefaultStoreDir
	case "structuredfile":
		// The structuredfile default lives at
		// pkg/providers/config/structuredfile/structuredfile.go as
		// DefaultDir = "./structuredfile". We hardcode the literal here
		// to avoid pulling that package into the doctor import graph.
		dir = "./structuredfile"
	default:
		return "", false
	}
	if ws.Store.Options != nil {
		if raw, ok := ws.Store.Options["directory"]; ok {
			if s, ok := raw.(string); ok && s != "" {
				dir = s
			}
		}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(ws.Dir, dir)
	}
	return dir, true
}

// expandTilde replaces a leading "~/" with the user's home directory.
// Anything else is returned unchanged. Mirrors the (unexported) helper
// used by core's plugin discovery.
func expandTilde(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[2:])
}
