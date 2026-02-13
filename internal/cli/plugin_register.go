package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/manifest"
	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
)

var pluginRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a plugin with the marketplace",
	Long: `Register a plugin with the central marketplace after publishing to an OCI registry.

This command reads the plugin manifest (zhi-plugin.yaml) from the current directory
and registers it with the configured marketplace server. An API key is required.

Subsequent version notifications can be sent with --version-only.`,
	Example: `  # Register a new plugin (reads zhi-plugin.yaml)
  zhi plugin register

  # Notify marketplace of a new version
  zhi plugin register --version-only --digest sha256:abc123`,
	RunE: runPluginRegister,
}

var (
	pluginRegisterVersionOnly bool
	pluginRegisterDigest      string
	pluginRegisterChangelog   string
)

func init() {
	pluginRegisterCmd.Flags().BoolVar(&pluginRegisterVersionOnly, "version-only", false, "only notify of a new version (plugin already registered)")
	pluginRegisterCmd.Flags().StringVar(&pluginRegisterDigest, "digest", "", "OCI manifest digest for version notification")
	pluginRegisterCmd.Flags().StringVar(&pluginRegisterChangelog, "changelog", "", "changelog for version notification")

	pluginCmd.AddCommand(pluginRegisterCmd)
}

func runPluginRegister(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	// Load the plugin manifest.
	m, err := manifest.LoadFile(filepath.Join(".", "zhi-plugin.yaml"))
	if err != nil {
		return fmt.Errorf("loading manifest: %w (is there a zhi-plugin.yaml in the current directory?)", err)
	}

	mc, err := newMarketplaceClient()
	if err != nil {
		return err
	}

	// Determine OCI reference from manifest.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determining home directory: %w", err)
	}
	_ = home // available for future use

	if pluginRegisterVersionOnly {
		// Notify of a new version.
		publisher, name := splitPluginPublisher(m.Author, m.Name)

		platforms := make([]string, 0, len(m.Binaries))
		for platform := range m.Binaries {
			platforms = append(platforms, platform)
		}

		req := marketplace.RegisterVersionRequest{
			Version:            m.Version,
			Digest:             pluginRegisterDigest,
			Platforms:          platforms,
			ZhiProtocolVersion: m.ZhiProtocolVersion,
			Changelog:          pluginRegisterChangelog,
		}

		fmt.Fprintf(w, "Notifying marketplace of %s v%s...\n", m.Name, m.Version)
		if err := mc.RegisterVersion(cmd.Context(), m.Type, publisher, name, req); err != nil {
			return fmt.Errorf("registering version: %w", err)
		}
		fmt.Fprintln(w, "Version registered successfully.")
		return nil
	}

	// Full plugin registration.
	ociRef := buildOCIRef(m)
	req := marketplace.RegisterPluginRequest{
		Name:        m.Name,
		Type:        m.Type,
		OCIRef:      ociRef,
		Description: m.Description,
		Homepage:    m.Homepage,
		Keywords:    m.Keywords,
	}

	fmt.Fprintf(w, "Registering %s with marketplace...\n", m.Name)
	if err := mc.RegisterPlugin(cmd.Context(), req); err != nil {
		return fmt.Errorf("registering plugin: %w", err)
	}
	fmt.Fprintf(w, "Plugin %s registered successfully.\n", m.Name)
	fmt.Fprintln(w, "Users can now find it with: zhi plugin search "+m.Name)

	return nil
}

// splitPluginPublisher extracts the publisher and name from a manifest.
// If the author contains a slash, we treat it as publisher/name already.
// Otherwise the author field is used as the publisher.
func splitPluginPublisher(author, name string) (string, string) {
	if strings.Contains(author, "/") {
		parts := strings.SplitN(author, "/", 2)
		return parts[0], parts[1]
	}
	return author, name
}

// buildOCIRef constructs an OCI reference from the plugin manifest.
func buildOCIRef(m *manifest.PluginManifest) string {
	// Use the manifest's Author as the namespace in the reference.
	return fmt.Sprintf("oci://ghcr.io/%s/zhi-%s-%s", m.Author, m.Type, m.Name)
}
