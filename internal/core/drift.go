package core

import (
	"context"
	"fmt"
)

// DriftEntry holds the drift status of a single exported file.
type DriftEntry struct {
	// Name is a descriptive name for the export (template name or format).
	Name string
	// OutputPath is the file being compared.
	OutputPath string
	// IsNew indicates the file does not exist on disk yet.
	IsNew bool
	// Diff is the unified diff output (empty for in-sync entries).
	Diff string
}

// DriftError records a per-template error encountered during drift checking.
type DriftError struct {
	// Name is the template/format name that failed.
	Name string
	// OutputPath is the file that was being checked.
	OutputPath string
	// Err is the underlying error.
	Err error
}

// DriftCheckResult holds the outcome of a full drift check across all
// workspace export templates.
type DriftCheckResult struct {
	Drifted []DriftEntry
	InSync  []DriftEntry
	Errors  []DriftError
	// Warnings collects non-fatal messages (e.g. skipped stdout-only templates).
	Warnings []string
}

// HasDrift returns true if any exported files have drifted.
func (r *DriftCheckResult) HasDrift() bool {
	return len(r.Drifted) > 0
}

// HasErrors returns true if any templates failed during the check.
func (r *DriftCheckResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// CheckDrift runs the full drift detection pipeline: re-renders all workspace
// export templates (dry-run) and compares the rendered content against the
// files currently on disk.
func CheckDrift(ctx context.Context, eng *Engine) (*DriftCheckResult, error) {
	ws := eng.Workspace()
	if ws == nil || len(ws.Export.Templates) == 0 {
		return &DriftCheckResult{
			Warnings: []string{"no export templates configured; nothing to check"},
		}, nil
	}

	td, err := PrepareTreeData(ctx, eng, false, "")
	if err != nil {
		return nil, fmt.Errorf("preparing tree data: %w", err)
	}

	configs := ExpandTemplates(ws.Export.Templates, ws.Dir, true)

	result := &DriftCheckResult{}

	for _, cfg := range configs {
		// Iterate templates use OutputPattern per child, not OutputPath.
		if cfg.Iterate != "" {
			checkIterateDrift(ctx, td, cfg, result)
			continue
		}

		// Skip stdout-only templates (no file to compare against).
		if cfg.OutputPath == "" || cfg.OutputPath == "-" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("template %q has no output path; skipping drift check", templateName(cfg)))
			continue
		}

		checkSingleDrift(ctx, td, cfg, result)
	}

	return result, nil
}

// checkSingleDrift checks drift for a non-iterate export config.
func checkSingleDrift(ctx context.Context, td *TreeData, cfg ExportRunConfig, result *DriftCheckResult) {
	exported, err := Export(ctx, td, cfg)
	if err != nil {
		result.Errors = append(result.Errors, DriftError{
			Name:       templateName(cfg),
			OutputPath: cfg.OutputPath,
			Err:        err,
		})
		return
	}

	diff, err := DiffExport(exported.OutputPath, exported.Content)
	if err != nil {
		result.Errors = append(result.Errors, DriftError{
			Name:       exported.Name,
			OutputPath: exported.OutputPath,
			Err:        err,
		})
		return
	}

	if diff.HasChanges {
		result.Drifted = append(result.Drifted, DriftEntry{
			Name:       exported.Name,
			OutputPath: diff.OutputPath,
			IsNew:      diff.IsNew,
			Diff:       diff.Diff,
		})
	} else {
		result.InSync = append(result.InSync, DriftEntry{
			Name:       exported.Name,
			OutputPath: diff.OutputPath,
		})
	}
}

// checkIterateDrift checks drift for an iterate export config (one file per child).
func checkIterateDrift(ctx context.Context, td *TreeData, cfg ExportRunConfig, result *DriftCheckResult) {
	exported, err := ExportIterate(ctx, td, cfg)
	if err != nil {
		result.Errors = append(result.Errors, DriftError{
			Name:       templateName(cfg),
			OutputPath: cfg.OutputPath,
			Err:        err,
		})
		return
	}

	for _, r := range exported {
		if r.OutputPath == "" || r.OutputPath == "-" {
			continue
		}

		diff, err := DiffExport(r.OutputPath, r.Content)
		if err != nil {
			result.Errors = append(result.Errors, DriftError{
				Name:       r.Name,
				OutputPath: r.OutputPath,
				Err:        err,
			})
			continue
		}

		if diff.HasChanges {
			result.Drifted = append(result.Drifted, DriftEntry{
				Name:       r.Name,
				OutputPath: diff.OutputPath,
				IsNew:      diff.IsNew,
				Diff:       diff.Diff,
			})
		} else {
			result.InSync = append(result.InSync, DriftEntry{
				Name:       r.Name,
				OutputPath: diff.OutputPath,
			})
		}
	}
}

// templateName returns a human-readable name for an export config.
func templateName(cfg ExportRunConfig) string {
	if cfg.Format != "" {
		return cfg.Format
	}
	if cfg.TemplatePath != "" {
		return cfg.TemplatePath
	}
	return "unknown"
}
