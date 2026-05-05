package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/client"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
)

var pluginPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a plugin to an OCI registry",
	Long: `Build an OCI artifact from a zhi-plugin.yaml manifest and pre-built binaries,
then push to an OCI registry.

Signing:
  --sign             Sign the artifact with cosign after pushing
  --key <path>       Path to a cosign private key (default: keyless via Fulcio/OIDC)

When --sign is used without --key, keyless signing via Sigstore Fulcio is used,
which requires an OIDC identity (e.g. GitHub Actions). The signing event is
recorded in the Rekor transparency log.

Prerequisites:
  - zhi-plugin.yaml must exist in the current directory
  - Built binaries for target platforms listed in the manifest's binaries section`,
	Example: `  zhi plugin publish --registry ghcr.io/myorg
  zhi plugin publish --registry ghcr.io/myorg --tag v1.0.0
  zhi plugin publish --registry ghcr.io/myorg --tag v1.0.0 --latest
  zhi plugin publish --registry ghcr.io/myorg --sign
  zhi plugin publish --registry ghcr.io/myorg --sign --key cosign.key`,
	RunE: runPluginPublish,
}

var (
	pluginPublishRegistry string
	pluginPublishTag      string
	pluginPublishExtraTag []string
	pluginPublishLatest   bool
	pluginPublishSign     bool
	pluginPublishKey      string
)

func init() {
	pluginPublishCmd.Flags().StringVar(&pluginPublishRegistry, "registry", "", "target OCI registry (required, e.g. ghcr.io/myorg)")
	pluginPublishCmd.Flags().StringVar(&pluginPublishTag, "tag", "", "OCI tag (default: v{version} from manifest)")
	pluginPublishCmd.Flags().StringSliceVar(&pluginPublishExtraTag, "also-tag", nil, "additional OCI tags to apply (repeatable)")
	pluginPublishCmd.Flags().BoolVar(&pluginPublishLatest, "latest", false, "also push the artifact under the 'latest' tag")
	pluginPublishCmd.Flags().BoolVar(&pluginPublishSign, "sign", false, "sign the artifact with cosign after pushing")
	pluginPublishCmd.Flags().StringVar(&pluginPublishKey, "key", "", "path to cosign private key (default: keyless via Fulcio/OIDC)")
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

	// Validate --key requires --sign.
	if pluginPublishKey != "" && !pluginPublishSign {
		return fmt.Errorf("--key requires --sign")
	}

	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	additionalTags := append([]string{}, pluginPublishExtraTag...)
	if pluginPublishLatest {
		additionalTags = append(additionalTags, "latest")
	}

	opts := client.PushOptions{
		Tag:            pluginPublishTag,
		AdditionalTags: additionalTags,
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
	for _, t := range additionalTags {
		fmt.Fprintf(w, "  Also tagged: %s\n", t)
	}
	fmt.Fprintf(w, "  Digest:    %s\n", result.Digest)

	// Sign the artifact if requested.
	if pluginPublishSign {
		fmt.Fprintln(w, "\nSigning artifact...")
		if pluginPublishKey != "" {
			fmt.Fprintf(w, "  Method: key-based (%s)\n", pluginPublishKey)
		} else {
			fmt.Fprintln(w, "  Method: keyless (Sigstore Fulcio/OIDC)")
		}

		// Create a signer.
		signer, err := verify.NewSigner(cmd.Context())
		if err != nil {
			return fmt.Errorf("creating signer: %w", err)
		}

		// Configure signing options.
		signOpts := verify.SignOptions{
			KeyPath:      pluginPublishKey,
			UseRekor:     true, // Always use Rekor for transparency
			UseTimestamp: true, // Always use TSA for tamper-evidence
		}

		// Sign the artifact using its digest.
		signResult, err := signer.SignArtifact(cmd.Context(), result.Digest, signOpts)
		if err != nil {
			return fmt.Errorf("signing artifact: %w", err)
		}

		// Save the signature bundle alongside the artifact metadata.
		// The bundle file is named <digest>.bundle.json and should be
		// uploaded to the registry as an OCI attachment or stored separately.
		bundlePath := result.Digest[7:15] + ".bundle.json" // Use first 8 chars of digest
		if err := verify.SaveBundleToFile(signResult.BundleJSON, bundlePath); err != nil {
			return fmt.Errorf("saving signature bundle: %w", err)
		}

		fmt.Fprintf(w, "  ✓ Signed successfully\n")
		fmt.Fprintf(w, "  Identity: %s\n", signResult.SigningIdentity)
		fmt.Fprintf(w, "  Bundle saved to: %s\n", bundlePath)
	}

	fmt.Fprintf(w, "\nInstall with: zhi plugin install %s\n", result.Reference)
	return nil
}
