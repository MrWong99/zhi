package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newPluginNewCmd creates a fresh plugin new command for testing to avoid
// global flag state issues between test runs.
func newPluginNewCmd() *cobra.Command {
	var (
		name        string
		pluginType  string
		lang        string
		module      string
		author      string
		license     string
		description string
		registry    string
		outputDir   string
	)

	cmd := &cobra.Command{
		Use:           "new",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pluginNewName = name
			pluginNewType = pluginType
			pluginNewLang = lang
			pluginNewModule = module
			pluginNewAuthor = author
			pluginNewLicense = license
			pluginNewDescription = description
			pluginNewRegistry = registry
			pluginNewOutputDir = outputDir
			return runPluginNew(cmd, nil)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "plugin name")
	cmd.Flags().StringVar(&pluginType, "type", "", "plugin type")
	cmd.Flags().StringVar(&lang, "lang", "go", "project language")
	cmd.Flags().StringVar(&module, "module", "", "module path")
	cmd.Flags().StringVar(&author, "author", "", "author")
	cmd.Flags().StringVar(&license, "license", "", "license")
	cmd.Flags().StringVar(&description, "description", "", "description")
	cmd.Flags().StringVar(&registry, "registry", "", "OCI registry")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "output directory")

	return cmd
}

func TestPluginNewMissingName(t *testing.T) {
	cmd := newPluginNewCmd()
	_, err := executeCommand(cmd, "--type", "config", "--output-dir", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing --name")
	}
	if !strings.Contains(err.Error(), "--name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPluginNewMissingType(t *testing.T) {
	cmd := newPluginNewCmd()
	_, err := executeCommand(cmd, "--name", "test", "--output-dir", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing --type")
	}
	if !strings.Contains(err.Error(), "--type is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPluginNewInvalidName(t *testing.T) {
	cmd := newPluginNewCmd()
	_, err := executeCommand(cmd, "--name", "Invalid_Name", "--type", "config", "--output-dir", t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
	if !strings.Contains(err.Error(), "must match") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPluginNewInvalidType(t *testing.T) {
	cmd := newPluginNewCmd()
	_, err := executeCommand(cmd, "--name", "test", "--type", "badtype", "--output-dir", t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "must be one of") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPluginNewSuccess(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "zhi-config-test")

	cmd := newPluginNewCmd()
	output, err := executeCommand(cmd,
		"--name", "test",
		"--type", "config",
		"--author", "testorg",
		"--output-dir", outputDir,
	)
	if err != nil {
		t.Fatalf("plugin new: %v", err)
	}
	if !strings.Contains(output, "Created config plugin project") {
		t.Errorf("expected success message, got: %s", output)
	}

	expectedFiles := []string{
		"go.mod",
		"Makefile",
		"zhi-plugin.yaml",
		"cmd/zhi-config-test/main.go",
		"pkg/plugin/plugin.go",
		"pkg/plugin/plugin_test.go",
		"workspace/zhi.yaml",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestPluginNewAllTypes(t *testing.T) {
	for _, pluginType := range []string{"config", "store", "transform", "ui"} {
		t.Run(pluginType, func(t *testing.T) {
			outputDir := filepath.Join(t.TempDir(), "zhi-"+pluginType+"-test")

			cmd := newPluginNewCmd()
			_, err := executeCommand(cmd,
				"--name", "test",
				"--type", pluginType,
				"--output-dir", outputDir,
			)
			if err != nil {
				t.Fatalf("plugin new --%s: %v", pluginType, err)
			}

			mainGo := filepath.Join(outputDir, "cmd/zhi-"+pluginType+"-test/main.go")
			if _, err := os.Stat(mainGo); os.IsNotExist(err) {
				t.Errorf("main.go not found for %s", pluginType)
			}
		})
	}
}

func TestPluginNewWithModule(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "project")

	cmd := newPluginNewCmd()
	_, err := executeCommand(cmd,
		"--name", "test",
		"--type", "config",
		"--module", "github.com/custom/my-plugin",
		"--output-dir", outputDir,
	)
	if err != nil {
		t.Fatalf("plugin new: %v", err)
	}

	goMod, err := os.ReadFile(filepath.Join(outputDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "github.com/custom/my-plugin") {
		t.Error("go.mod should contain custom module path")
	}
}

func TestPluginNewWithRegistry(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "project")

	cmd := newPluginNewCmd()
	output, err := executeCommand(cmd,
		"--name", "test",
		"--type", "store",
		"--registry", "ghcr.io/myorg",
		"--output-dir", outputDir,
	)
	if err != nil {
		t.Fatalf("plugin new: %v", err)
	}
	if !strings.Contains(output, "ghcr.io/myorg") {
		t.Error("output should mention the registry")
	}
}
