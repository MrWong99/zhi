package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export configuration using templates or built-in formats",
	Long: `Export configuration from the current tree using Go templates or built-in
format shortcuts (json, yaml, toml, dotenv).

Examples:
  zhi export --format json                       # export as JSON to stdout
  zhi export --template ./tmpl/app.tmpl -o out   # render template to file
  zhi export                                     # export all templates from zhi.yaml`,
	PersistentPreRunE: withEngine,
	RunE:              runExport,
}

var (
	exportTemplate      string
	exportFormat        string
	exportOutput        string
	exportPrefix        string
	exportAllComponents bool
	exportDryRun        bool
)

func init() {
	exportCmd.Flags().StringVar(&exportTemplate, "template", "", "path to a Go template file")
	exportCmd.Flags().StringVar(&exportFormat, "format", "", "built-in format (json, yaml, toml, dotenv)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file path (default: stdout)")
	exportCmd.Flags().StringVar(&exportPrefix, "prefix", "", "only export paths under this prefix")
	exportCmd.Flags().BoolVar(&exportAllComponents, "all-components", false, "include all components regardless of enabled/disabled state")
	exportCmd.Flags().BoolVar(&exportDryRun, "dry-run", false, "print rendered output without writing to disk")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	w := cmd.OutOrStdout()

	td, err := core.PrepareTreeData(ctx, eng, exportAllComponents, exportPrefix)
	if err != nil {
		return err
	}

	// Case 1: --template specified
	if exportTemplate != "" {
		tmplPath := exportTemplate
		if !filepath.IsAbs(tmplPath) {
			tmplPath = filepath.Join(eng.WorkspaceDir(), tmplPath)
		}
		outputPath := exportOutput
		if outputPath != "" && outputPath != "-" && !filepath.IsAbs(outputPath) {
			outputPath = filepath.Join(eng.WorkspaceDir(), outputPath)
		}
		cfg := core.ExportRunConfig{
			TemplatePath: tmplPath,
			OutputPath:   outputPath,
			Prefix:       exportPrefix,
			DryRun:       exportDryRun,
		}
		result, err := core.Export(ctx, td, cfg)
		if err != nil {
			return err
		}
		return printExportResult(cmd, result)
	}

	// Case 2: --format specified (without template)
	if exportFormat != "" {
		outputPath := exportOutput
		if outputPath != "" && outputPath != "-" && !filepath.IsAbs(outputPath) {
			outputPath = filepath.Join(eng.WorkspaceDir(), outputPath)
		}
		cfg := core.ExportRunConfig{
			Format:     exportFormat,
			OutputPath: outputPath,
			Prefix:     exportPrefix,
			DryRun:     exportDryRun,
		}
		result, err := core.Export(ctx, td, cfg)
		if err != nil {
			return err
		}
		return printExportResult(cmd, result)
	}

	// Case 3: Neither specified — export all templates from workspace config.
	ws := eng.Workspace()
	if ws == nil || len(ws.Export.Templates) == 0 {
		return fmt.Errorf("no export templates defined in workspace config and no --template or --format specified")
	}

	var configs []core.ExportRunConfig
	for _, tmpl := range ws.Export.Templates {
		cfg := core.ExportRunConfig{
			DryRun: exportDryRun,
			Prefix: tmpl.Prefix,
		}
		if tmpl.Template != "" {
			p := tmpl.Template
			if !filepath.IsAbs(p) {
				p = filepath.Join(ws.Dir, p)
			}
			cfg.TemplatePath = p
		}
		if tmpl.Format != "" {
			cfg.Format = tmpl.Format
		}
		if tmpl.Output != "" {
			p := tmpl.Output
			if !filepath.IsAbs(p) {
				p = filepath.Join(ws.Dir, p)
			}
			cfg.OutputPath = p
		}
		configs = append(configs, cfg)
	}

	results, err := core.ExportAll(ctx, td, configs)
	if err != nil {
		return err
	}

	for _, r := range results {
		if exportDryRun || r.OutputPath == "" || r.OutputPath == "-" {
			fmt.Fprintf(w, "--- %s ---\n", r.Name)
			fmt.Fprint(w, r.Content)
			if len(r.Content) > 0 && r.Content[len(r.Content)-1] != '\n' {
				fmt.Fprintln(w)
			}
		} else {
			fmt.Fprintf(w, "Exported: %s -> %s\n", r.Name, r.OutputPath)
		}
	}
	return nil
}

func printExportResult(cmd *cobra.Command, result *core.ExportResult) error {
	w := cmd.OutOrStdout()
	if exportDryRun || result.OutputPath == "" || result.OutputPath == "-" {
		fmt.Fprint(w, result.Content)
		if len(result.Content) > 0 && result.Content[len(result.Content)-1] != '\n' {
			fmt.Fprintln(w)
		}
	} else {
		fmt.Fprintf(w, "Exported: %s -> %s\n", result.Name, result.OutputPath)
	}
	return nil
}
