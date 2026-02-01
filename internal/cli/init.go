package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed init-template/*
var initTemplateFS embed.FS

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new zhi workspace",
	Long: `Initialize a new zhi workspace by scaffolding the directory structure
and configuration files needed to get started.

This creates:
  - zhi.yaml                 Workspace configuration
  - config/app.yaml          Starter configuration values
  - templates/sample.tmpl    Example export template
  - .zhi/                    Internal data directory

Examples:
  zhi init                        # initialize in the current directory
  zhi init --workspace ./myapp    # initialize in ./myapp
  zhi init --force                # overwrite existing zhi.yaml`,
	Example: `  zhi init
  zhi init --config-provider structuredfile --store-provider zhi-store-json
  zhi init --force`,
	RunE: runInit,
}

var (
	initConfigProvider string
	initStoreProvider  string
	initForce          bool
)

func init() {
	initCmd.Flags().StringVar(&initConfigProvider, "config-provider", "structuredfile", "config provider to use")
	initCmd.Flags().StringVar(&initStoreProvider, "store-provider", "zhi-store-json", "store provider to use")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing zhi.yaml if present")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	dir := workspace
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory: %w", err)
	}

	zhiYaml := filepath.Join(absDir, "zhi.yaml")
	if !initForce {
		if _, err := os.Stat(zhiYaml); err == nil {
			return fmt.Errorf("zhi.yaml already exists (use --force to overwrite)")
		}
	}

	w := cmd.OutOrStdout()
	var created []string

	// 1. Create zhi.yaml from template
	tmpl, err := template.ParseFS(initTemplateFS, "init-template/zhi.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("parsing zhi.yaml template: %w", err)
	}

	file, err := os.Create(zhiYaml)
	if err != nil {
		return fmt.Errorf("creating zhi.yaml: %w", err)
	}
	defer file.Close()

	templateData := map[string]string{
		"ConfigProvider": initConfigProvider,
		"StoreProvider":  initStoreProvider,
	}
	if err := tmpl.Execute(file, templateData); err != nil {
		return fmt.Errorf("writing zhi.yaml: %w", err)
	}
	created = append(created, "zhi.yaml")

	// 2. Create ./config/ directory with starter configuration file
	configDir := filepath.Join(absDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	appYamlContent, err := initTemplateFS.ReadFile("init-template/config_app.yaml")
	if err != nil {
		return fmt.Errorf("reading embedded config_app.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.yaml"), appYamlContent, 0o644); err != nil {
		return fmt.Errorf("writing config/app.yaml: %w", err)
	}
	created = append(created, "config/app.yaml")

	// 3. Create ./templates/ directory with sample export template
	templatesDir := filepath.Join(absDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return fmt.Errorf("creating templates directory: %w", err)
	}

	sampleTemplateContent, err := initTemplateFS.ReadFile("init-template/templates_sample.tmpl")
	if err != nil {
		return fmt.Errorf("reading embedded templates_sample.tmpl: %w", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "sample.tmpl"), sampleTemplateContent, 0o644); err != nil {
		return fmt.Errorf("writing templates/sample.tmpl: %w", err)
	}
	created = append(created, "templates/sample.tmpl")

	// 4. Create ./.zhi/ directory with user-only permissions (0700)
	zhiDir := filepath.Join(absDir, ".zhi")
	if err := os.MkdirAll(zhiDir, 0o700); err != nil {
		return fmt.Errorf("creating .zhi directory: %w", err)
	}

	// Create .zhi/store/ subdirectory
	storeDir := filepath.Join(zhiDir, "store")
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		return fmt.Errorf("creating .zhi/store directory: %w", err)
	}
	created = append(created, ".zhi/store/")

	// 5. Create ./.zhi/components.json with default component state (0600 permissions)
	componentStateContent, err := initTemplateFS.ReadFile("init-template/components.json")
	if err != nil {
		return fmt.Errorf("reading embedded components.json: %w", err)
	}
	componentsFile := filepath.Join(zhiDir, "components.json")
	if err := os.WriteFile(componentsFile, componentStateContent, 0o600); err != nil {
		return fmt.Errorf("writing .zhi/components.json: %w", err)
	}
	created = append(created, ".zhi/components.json")

	// 6. Print summary
	fmt.Fprintln(w, "Initialized zhi workspace:")
	for _, f := range created {
		fmt.Fprintf(w, "  %s\n", f)
	}

	// 7. Print what's next message
	fmt.Fprintln(w)
	fmt.Fprintln(w, "What's next?")
	fmt.Fprintln(w, "  Edit configuration values:   zhi edit")
	fmt.Fprintln(w, "  Export configuration:         zhi export --format json")
	fmt.Fprintln(w, "  Validate configuration:       zhi validate")
	fmt.Fprintln(w, "  List config paths:            zhi list paths")
	fmt.Fprintln(w, "  Manage components:            zhi component list")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Add .zhi/ to your .gitignore to keep internal state out of version control.")

	return nil
}
