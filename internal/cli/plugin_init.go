package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/manifest"
)

var pluginInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a zhi-plugin.yaml manifest for a new plugin",
	Long: `Initialize a new plugin project by generating a zhi-plugin.yaml manifest.

The command accepts flags for all fields. If required fields are missing,
an error is returned.`,
	Example: `  zhi plugin init --name my-config --type config --version 1.0.0
  zhi plugin init --name vault-store --type store --version 0.1.0 --author myorg --license Apache-2.0`,
	RunE: runPluginInit,
}

var (
	pluginInitName        string
	pluginInitType        string
	pluginInitVersion     string
	pluginInitDescription string
	pluginInitAuthor      string
	pluginInitLicense     string
)

func init() {
	pluginInitCmd.Flags().StringVar(&pluginInitName, "name", "", "plugin name (required, must match [a-z][a-z0-9-]*)")
	pluginInitCmd.Flags().StringVar(&pluginInitType, "type", "", "plugin type: config, transform, store, ui (required)")
	pluginInitCmd.Flags().StringVar(&pluginInitVersion, "version", "0.1.0", "initial version (semver)")
	pluginInitCmd.Flags().StringVar(&pluginInitDescription, "description", "", "short description of the plugin")
	pluginInitCmd.Flags().StringVar(&pluginInitAuthor, "author", "", "author or organisation name")
	pluginInitCmd.Flags().StringVar(&pluginInitLicense, "license", "", "SPDX license identifier (e.g. Apache-2.0, MIT)")
}

func runPluginInit(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	if pluginInitName == "" {
		return fmt.Errorf("--name is required")
	}
	if !manifest.IsValidName(pluginInitName) {
		return fmt.Errorf("name %q must match [a-z][a-z0-9-]*", pluginInitName)
	}
	if pluginInitType == "" {
		return fmt.Errorf("--type is required")
	}
	if !manifest.IsValidPluginType(pluginInitType) {
		return fmt.Errorf("type %q must be one of: config, transform, store, ui", pluginInitType)
	}
	if !manifest.IsValidSemver(pluginInitVersion) {
		return fmt.Errorf("version %q is not valid semver", pluginInitVersion)
	}

	m := &manifest.PluginManifest{
		SchemaVersion:      manifest.SchemaVersion,
		Name:               pluginInitName,
		Type:               pluginInitType,
		Version:            pluginInitVersion,
		ZhiProtocolVersion: "1",
		Description:        pluginInitDescription,
		Author:             pluginInitAuthor,
		License:            pluginInitLicense,
	}

	// Auto-detect binaries in dist/ directory.
	binaries := detectBinaries(pluginInitType, pluginInitName)
	if len(binaries) > 0 {
		m.Binaries = binaries
	}

	data, err := m.Marshal()
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}

	outPath := "zhi-plugin.yaml"
	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("zhi-plugin.yaml already exists; remove it first or edit it directly")
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Fprintf(w, "Created %s\n", outPath)
	fmt.Fprintf(w, "  Name:    %s\n", m.Name)
	fmt.Fprintf(w, "  Type:    %s\n", m.Type)
	fmt.Fprintf(w, "  Version: %s\n", m.Version)
	if len(binaries) > 0 {
		fmt.Fprintf(w, "  Binaries: %d platform(s) detected in dist/\n", len(binaries))
	} else {
		fmt.Fprintf(w, "\nBuild your plugin binaries into dist/ and add them to the binaries section.\n")
	}
	fmt.Fprintf(w, "\nNext: zhi plugin publish --registry <registry-host>\n")
	return nil
}

// detectBinaries scans the dist/ directory for platform-named binaries.
func detectBinaries(pluginType, pluginName string) map[string]string {
	distDir := "dist"
	binaryPrefix := fmt.Sprintf("zhi-%s-%s_", pluginType, pluginName)

	entries, err := os.ReadDir(distDir)
	if err != nil {
		return nil
	}

	result := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, binaryPrefix) {
			continue
		}
		// Extract platform from suffix: e.g. "zhi-config-myplugin_linux_amd64" → "linux/amd64"
		suffix := strings.TrimPrefix(name, binaryPrefix)
		parts := strings.SplitN(suffix, "_", 2)
		if len(parts) == 2 {
			platform := parts[0] + "/" + parts[1]
			result[platform] = filepath.Join(distDir, name)
		}
	}
	return result
}
