package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new zhi workspace",
	Long:  "Scaffold a new zhi workspace in the current (or specified) directory.",
	RunE:  runInit,
}

var (
	initConfigProvider string
	initStoreProvider  string
	initForce          bool
)

func init() {
	initCmd.Flags().StringVar(&initConfigProvider, "config-provider", "structuredfile", "config provider to use")
	initCmd.Flags().StringVar(&initStoreProvider, "store-provider", "json-store", "store provider to use")
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

	// 1. Create zhi.yaml
	yamlContent := fmt.Sprintf(`version: "1"

config:
  provider: %s
  options:
    directory: ./config

# transform: []

# store:
#   provider: %s

components:
  - name: app
    description: Core application configuration
    paths:
      - app/
    mandatory: true
  - name: database
    description: Database connection settings
    paths:
      - database/
    mandatory: true
  - name: monitoring
    description: Monitoring and observability
    paths:
      - monitoring/
    mandatory: false
    dependencies: []
`, initConfigProvider, initStoreProvider)

	if err := os.WriteFile(zhiYaml, []byte(yamlContent), 0o644); err != nil {
		return fmt.Errorf("writing zhi.yaml: %w", err)
	}
	created = append(created, "zhi.yaml")

	// 2. Create ./config/ directory with starter configuration file
	configDir := filepath.Join(absDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	appYaml := `app:
  name:
    val: my-app
    metadata:
      description: Application name
  log-level:
    val: info
    metadata:
      description: Logging level (debug, info, warn, error)

database:
  host:
    val: localhost
    metadata:
      description: Database hostname
  port:
    val: 5432
    metadata:
      description: Database port number

monitoring:
  enabled:
    val: false
    metadata:
      description: Enable monitoring
`
	if err := os.WriteFile(filepath.Join(configDir, "app.yaml"), []byte(appYaml), 0o644); err != nil {
		return fmt.Errorf("writing config/app.yaml: %w", err)
	}
	created = append(created, "config/app.yaml")

	// 3. Create ./templates/ directory with sample export template
	templatesDir := filepath.Join(absDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return fmt.Errorf("creating templates directory: %w", err)
	}

	sampleTemplate := `# Sample export template
# This template demonstrates component-aware rendering.
#
# Usage: reference configuration values using Go template syntax.
# Components can be checked with: {{ "{{ if .ComponentEnabled \"monitoring\" }}" }}
#
# Example:
# app_name={{ "{{ .Get \"app/name\" }}" }}
# db_host={{ "{{ .Get \"database/host\" }}" }}
# {{ "{{ if .ComponentEnabled \"monitoring\" }}" }}
# monitoring_enabled=true
# {{ "{{ end }}" }}
`
	if err := os.WriteFile(filepath.Join(templatesDir, "sample.tmpl"), []byte(sampleTemplate), 0o644); err != nil {
		return fmt.Errorf("writing templates/sample.tmpl: %w", err)
	}
	created = append(created, "templates/sample.tmpl")

	// 4. Create ./.zhi/store/ directory
	storeDir := filepath.Join(absDir, ".zhi", "store")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("creating .zhi/store directory: %w", err)
	}
	created = append(created, ".zhi/store/")

	// 5. Create ./.zhi/components.json with default component state
	componentState := `{
  "app": true,
  "database": true,
  "monitoring": false
}
`
	componentsFile := filepath.Join(absDir, ".zhi", "components.json")
	if err := os.WriteFile(componentsFile, []byte(componentState), 0o644); err != nil {
		return fmt.Errorf("writing .zhi/components.json: %w", err)
	}
	created = append(created, ".zhi/components.json")

	// 6. Print summary
	fmt.Fprintln(w, "Initialized zhi workspace:")
	for _, f := range created {
		fmt.Fprintf(w, "  %s\n", f)
	}

	return nil
}
