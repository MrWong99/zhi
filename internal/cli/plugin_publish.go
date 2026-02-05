package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/client"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
)

var pluginPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a plugin to an OCI registry",
	Long: `Build an OCI artifact from a zhi-plugin.yaml manifest and pre-built binaries,
then push to an OCI registry.

Prerequisites:
  - zhi-plugin.yaml must exist in the current directory
  - Built binaries for target platforms listed in the manifest's binaries section`,
	Example: `  zhi plugin publish --registry ghcr.io/myorg
  zhi plugin publish --registry ghcr.io/myorg --tag v1.0.0
  zhi plugin publish --registry ghcr.io/myorg --sign`,
	RunE: runPluginPublish,
}

var (
	pluginPublishRegistry string
	pluginPublishTag      string
	pluginPublishSign     bool
)

func init() {
	pluginPublishCmd.Flags().StringVar(&pluginPublishRegistry, "registry", "", "target OCI registry (required, e.g. ghcr.io/myorg)")
	pluginPublishCmd.Flags().StringVar(&pluginPublishTag, "tag", "", "OCI tag (default: v{version} from manifest)")
	pluginPublishCmd.Flags().BoolVar(&pluginPublishSign, "sign", false, "sign the artifact with cosign (not yet implemented)")
}

func runPluginPublish(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	if pluginPublishRegistry == "" {
		return fmt.Errorf("--registry is required")
	}

	// Load plugin manifest.
	m, err := manifest.LoadFile("zhi-plugin.yaml")
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if len(m.Binaries) == 0 {
		return fmt.Errorf("no binaries listed in zhi-plugin.yaml; add a 'binaries' section with platform/binary path mappings")
	}

	// Verify all binaries exist.
	for platform, path := range m.Binaries {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("binary for %s not found at %s: %w", platform, path, err)
		}
	}

	if pluginPublishSign {
		fmt.Fprintln(w, "Warning: --sign is accepted but signing is not yet configured (see Phase 3)")
	}

	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	opts := client.PushOptions{
		Tag: pluginPublishTag,
	}

	fmt.Fprintf(w, "Publishing %s v%s to %s...\n", m.Name, m.Version, pluginPublishRegistry)
	fmt.Fprintf(w, "  Platforms: %d\n", len(m.Binaries))
	for platform := range m.Binaries {
		fmt.Fprintf(w, "    - %s\n", platform)
	}

	result, err := ociClient.PushPlugin(cmd.Context(), m, m.Binaries, pluginPublishRegistry, opts)
	if err != nil {
		return fmt.Errorf("publishing plugin: %w", err)
	}

	fmt.Fprintf(w, "\nPublished successfully!\n")
	fmt.Fprintf(w, "  Reference: %s\n", result.Reference)
	fmt.Fprintf(w, "  Tag:       %s\n", result.Tag)
	fmt.Fprintf(w, "  Digest:    %s\n", result.Digest)
	fmt.Fprintf(w, "\nInstall with: zhi plugin install %s\n", result.Reference)
	return nil
}
