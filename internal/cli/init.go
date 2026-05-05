package cli

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed all:init-template
var initTemplateFS embed.FS

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new zhi workspace",
	Long: `Initialize a new zhi workspace by scaffolding a sample Docker Compose
application.

This creates:
  - zhi.yaml:      Workspace configuration to manage the app.
  - config/:       Starter configuration values for the services.
  - templates/:    A template to generate docker-compose.yaml.
  - app-data/:     Data and configuration for the example services.
  - .zhi/:         Internal directory for workspace state.

Run 'zhi export' to generate the final docker-compose.yaml.`,
	Example: `  zhi init
  zhi init --config-provider structuredfile --store-provider jsonfile
  zhi init --force`,
	RunE: runInit,
}

var (
	initConfigProvider string
	initStoreProvider  string
	initForce          bool
	initBare           bool
)

func init() {
	initCmd.Flags().StringVar(&initConfigProvider, "config-provider", "structuredfile", "config provider to use")
	initCmd.Flags().StringVar(&initStoreProvider, "store-provider", "jsonfile", "store provider to use (built-in: jsonfile)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing zhi.yaml if present")
	initCmd.Flags().BoolVar(&initBare, "bare", false, "create a minimal workspace without demo content")
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

	zhiYamlPath := filepath.Join(absDir, "zhi.yaml")
	if !initForce {
		if _, err := os.Stat(zhiYamlPath); err == nil {
			return fmt.Errorf("zhi.yaml already exists (use --force to overwrite)")
		}
	}

	w := cmd.OutOrStdout()
	var created []string

	// Create ./.zhi/ and subdirectories first with specific permissions
	zhiDir := filepath.Join(absDir, ".zhi")
	if err := os.MkdirAll(zhiDir, 0o700); err != nil {
		return fmt.Errorf("creating .zhi directory: %w", err)
	}
	storeDir := filepath.Join(zhiDir, "store")
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		return fmt.Errorf("creating .zhi/store directory: %w", err)
	}
	created = append(created, ".zhi/store/")

	if initBare {
		return runInitBare(w, absDir, zhiYamlPath, created)
	}

	// Walk the embedded FS to create the workspace files
	templateData := map[string]string{
		"ConfigProvider": initConfigProvider,
		"StoreProvider":  initStoreProvider,
	}

	walkErr := fs.WalkDir(initTemplateFS, "init-template", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "init-template" {
			return nil // Skip the root directory itself.
		}

		// The path relative to the root of the embedded templates.
		relativePath := strings.TrimPrefix(path, "init-template/")

		// Determine the destination path on the filesystem.
		destPath := filepath.Join(absDir, relativePath)
		createdPath := relativePath
		perms := fs.FileMode(0o644)

		// Apply special handling for specific files.
		switch relativePath {
		case "zhi.yaml.tmpl":
			// This is a template, render it to zhi.yaml in the workspace root.
			tmpl, err := template.ParseFS(initTemplateFS, path)
			if err != nil {
				return fmt.Errorf("parsing zhi.yaml template: %w", err)
			}
			file, err := os.Create(zhiYamlPath)
			if err != nil {
				return fmt.Errorf("creating zhi.yaml: %w", err)
			}
			defer file.Close()
			if err := tmpl.Execute(file, templateData); err != nil {
				return fmt.Errorf("writing zhi.yaml: %w", err)
			}
			created = append(created, "zhi.yaml")
			return nil // Handled.

		case "config_app.yaml":
			// This file gets a new name and location.
			destPath = filepath.Join(absDir, "config", "app.yaml")
			createdPath = "config/app.yaml"

		case "components.json":
			// This file goes into the .zhi directory with stricter permissions.
			destPath = filepath.Join(zhiDir, "components.json")
			createdPath = ".zhi/components.json"
			perms = 0o600
		}

		if d.IsDir() {
			// For all other directories, just create them.
			// This handles `templates`, `app-data`, etc.
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", destPath, err)
			}
			return nil
		}

		// For all other files, copy them over.
		content, err := initTemplateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}

		// Ensure the parent directory exists.
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("creating parent directory for %s: %w", destPath, err)
		}

		if err := os.WriteFile(destPath, content, perms); err != nil {
			return fmt.Errorf("writing file %s: %w", destPath, err)
		}

		created = append(created, createdPath)
		return nil
	})

	if walkErr != nil {
		return walkErr
	}

	// Print summary
	fmt.Fprintln(w, "Initialized zhi workspace:")
	// Sort created files for consistent output? For now, it's FS order.
	for _, f := range created {
		fmt.Fprintf(w, "  %s\n", f)
	}

	// Print what's next message
	fmt.Fprintln(w)
	fmt.Fprintln(w, "What's next?")
	fmt.Fprintln(w, "  1. Generate the Docker Compose file:  zhi export")
	fmt.Fprintln(w, "  2. Start the Pokedex stack:           docker compose up")
	fmt.Fprintln(w, "  3. View the Pokedex at:               http://localhost:8080")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Explore more commands:")
	fmt.Fprintln(w, "  - Edit configuration: zhi edit")
	fmt.Fprintln(w, "  - See all config:     zhi list")
	fmt.Fprintln(w, "  - Manage components:  zhi component list")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Add .zhi/ to your .gitignore to keep internal state out of version control.")

	return nil
}

// runInitBare creates a minimal workspace without demo content.
func runInitBare(w io.Writer, absDir, zhiYamlPath string, created []string) error {
	// Write minimal zhi.yaml with config + store sections.
	bareYAML := fmt.Sprintf(`version: "1"

config:
  provider: %s
  options:
    directory: ./config

store:
  provider: %s
  options:
    directory: ./.zhi/store
`, initConfigProvider, initStoreProvider)

	if err := os.WriteFile(zhiYamlPath, []byte(bareYAML), 0o644); err != nil {
		return fmt.Errorf("creating zhi.yaml: %w", err)
	}
	created = append(created, "zhi.yaml")

	// Create empty config/ directory.
	configDir := filepath.Join(absDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	created = append(created, "config/")

	// Print summary.
	fmt.Fprintln(w, "Initialized zhi workspace:")
	for _, f := range created {
		fmt.Fprintf(w, "  %s\n", f)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "What's next?")
	fmt.Fprintln(w, "  1. Add config files in:         config/")
	fmt.Fprintln(w, "  2. See your configuration:      zhi list")
	fmt.Fprintln(w, "  3. Edit interactively:          zhi edit")
	fmt.Fprintln(w, "  4. Manage components:           zhi component list")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Add .zhi/ to your .gitignore to keep internal state out of version control.")

	return nil
}
