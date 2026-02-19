package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/scaffold"
	_ "github.com/MrWong99/zhi/internal/scaffold/golang" // register Go scaffolder
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
)

var pluginNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Generate a new plugin project from a template",
	Long: `Scaffold a complete plugin project with implementation stubs, tests,
build automation, CI/CD workflows, and a sample workspace.

The generated project is a standalone repository ready to be pushed
to a Git remote. It includes:

  - Plugin implementation with correct interface stubs
  - Tests using in-process gRPC (no subprocess needed)
  - Makefile with build, test, lint, and cross-compile targets
  - GitHub Actions workflow for automated build and publish
  - zhi-plugin.yaml manifest for OCI distribution
  - Sample workspace with zhi.yaml for local testing
  - go.mod with zhi dependencies

The --lang flag selects the project language. Currently only "go" is
supported; additional languages can be added in the future.`,
	Example: `  # Generate a Go config plugin
  zhi plugin new --name my-config --type config

  # Generate a store plugin with full metadata
  zhi plugin new --name my-store --type store --author myorg --license MIT

  # Specify Go module path and OCI registry
  zhi plugin new --name my-transform --type transform \
    --module github.com/myorg/zhi-transform-my-transform \
    --registry ghcr.io/myorg

  # Generate into a custom directory
  zhi plugin new --name my-ui --type ui --output-dir ./plugins/my-ui`,
	RunE: runPluginNew,
}

var (
	pluginNewName        string
	pluginNewType        string
	pluginNewLang        string
	pluginNewModule      string
	pluginNewAuthor      string
	pluginNewLicense     string
	pluginNewDescription string
	pluginNewRegistry    string
	pluginNewOutputDir   string
)

func init() {
	pluginNewCmd.Flags().StringVar(&pluginNewName, "name", "", "plugin name (required, must match [a-z][a-z0-9-]*)")
	pluginNewCmd.Flags().StringVar(&pluginNewType, "type", "", "plugin type: config, transform, store, ui (required)")
	pluginNewCmd.Flags().StringVar(&pluginNewLang, "lang", "go", "project language (available: "+scaffold.Languages()+")")
	pluginNewCmd.Flags().StringVar(&pluginNewModule, "module", "", "Go module path (default: github.com/<author>/zhi-<type>-<name>)")
	pluginNewCmd.Flags().StringVar(&pluginNewAuthor, "author", "", "author or organisation name")
	pluginNewCmd.Flags().StringVar(&pluginNewLicense, "license", "", "SPDX license identifier (e.g. MIT, Apache-2.0)")
	pluginNewCmd.Flags().StringVar(&pluginNewDescription, "description", "", "short description of the plugin")
	pluginNewCmd.Flags().StringVar(&pluginNewRegistry, "registry", "", "target OCI registry for publishing (e.g. ghcr.io/myorg)")
	pluginNewCmd.Flags().StringVarP(&pluginNewOutputDir, "output-dir", "o", "", "output directory (default: ./zhi-<type>-<name>)")

	pluginCmd.AddCommand(pluginNewCmd)
}

func runPluginNew(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	// --- validate inputs ---

	if pluginNewName == "" {
		return fmt.Errorf("--name is required")
	}
	if !manifest.IsValidName(pluginNewName) {
		return fmt.Errorf("name %q must match [a-z][a-z0-9-]*", pluginNewName)
	}
	if pluginNewType == "" {
		return fmt.Errorf("--type is required")
	}
	if !manifest.IsValidPluginType(pluginNewType) {
		return fmt.Errorf("type %q must be one of: config, transform, store, ui", pluginNewType)
	}

	scaffolder, err := scaffold.Get(pluginNewLang)
	if err != nil {
		return err
	}

	binaryName := fmt.Sprintf("zhi-%s-%s", pluginNewType, pluginNewName)

	// Derive module path if not set.
	modulePath := pluginNewModule
	if modulePath == "" {
		owner := pluginNewAuthor
		if owner == "" {
			owner = "your-org"
		}
		modulePath = fmt.Sprintf("github.com/%s/%s", owner, binaryName)
	}

	// Derive output directory.
	outputDir := pluginNewOutputDir
	if outputDir == "" {
		outputDir = binaryName
	}
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}

	// Default description.
	description := pluginNewDescription
	if description == "" {
		description = fmt.Sprintf("A zhi %s plugin", pluginNewType)
	}

	data := scaffold.Data{
		PluginName:  pluginNewName,
		PluginType:  pluginNewType,
		BinaryName:  binaryName,
		ModulePath:  modulePath,
		PackageName: scaffold.ToPackageName(pluginNewName),
		Author:      pluginNewAuthor,
		License:     pluginNewLicense,
		Version:     "0.1.0",
		Description: description,
		Registry:    pluginNewRegistry,
		GoVersion:   "1.26",
		ZhiModule:   "github.com/MrWong99/zhi",
		Year:        strconv.Itoa(time.Now().Year()),
	}

	fmt.Fprintf(w, "Scaffolding %s plugin %q (%s)...\n", data.PluginType, data.PluginName, scaffolder.Language())

	if err := scaffolder.Scaffold(data, absDir); err != nil {
		return fmt.Errorf("scaffolding failed: %w", err)
	}

	// --- print summary ---

	fmt.Fprintf(w, "\nCreated %s plugin project in %s/\n", data.PluginType, outputDir)
	fmt.Fprintf(w, "  Name:    %s\n", data.PluginName)
	fmt.Fprintf(w, "  Type:    %s\n", data.PluginType)
	fmt.Fprintf(w, "  Binary:  %s\n", data.BinaryName)
	fmt.Fprintf(w, "  Module:  %s\n", data.ModulePath)
	if data.Registry != "" {
		fmt.Fprintf(w, "  Registry: %s\n", data.Registry)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintf(w, "  1. cd %s\n", outputDir)
	fmt.Fprintln(w, "  2. Update go.mod with the correct zhi version")
	fmt.Fprintln(w, "  3. Run: go mod tidy")
	fmt.Fprintln(w, "  4. Build: make build")
	fmt.Fprintln(w, "  5. Test:  make test")
	fmt.Fprintln(w, "  6. Try the sample workspace: cd workspace && zhi list")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Publish when ready:")
	fmt.Fprintln(w, "  make cross-compile")
	if data.Registry != "" {
		fmt.Fprintf(w, "  zhi plugin publish --registry %s\n", data.Registry)
	} else {
		fmt.Fprintln(w, "  zhi plugin publish --registry <your-registry>")
	}

	return nil
}
