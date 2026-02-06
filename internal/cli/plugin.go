package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/client"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/registry"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage shared plugins",
	Long: `Install, uninstall, publish, and manage shared plugins from OCI registries.

Subcommands:
  install      Install a plugin from an OCI reference
  uninstall    Remove an installed plugin
  list         List installed shared plugins
  info         Show detailed information about an installed plugin
  verify       Verify a plugin artifact's signature without installing
  init         Generate a zhi-plugin.yaml manifest for a new plugin
  publish      Publish a plugin to an OCI registry`,
	Example: `  zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
  zhi plugin uninstall ansible-config
  zhi plugin list
  zhi plugin info ansible-config
  zhi plugin verify oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
  zhi plugin init --name my-config --type config --version 1.0.0
  zhi plugin publish --registry ghcr.io/myorg`,
}

// --- plugin install ---

var pluginInstallCmd = &cobra.Command{
	Use:   "install <reference>",
	Short: "Install a plugin from an OCI reference",
	Long: `Download and install a plugin from an OCI registry.

The reference can be a full OCI reference or a short name:
  oci://ghcr.io/org/plugin:v1.0
  ghcr.io/org/plugin:v1.0

Signature verification is performed by default. Use --skip-verify to disable.`,
	Example: `  zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
  zhi plugin install ghcr.io/org/zhi-config-custom:v1.0.0 --force
  zhi plugin install ghcr.io/org/zhi-config-custom:v1.0.0 --skip-verify`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInstall,
}

var (
	pluginInstallPlatform   string
	pluginInstallForce      bool
	pluginInstallSkipVerify bool
)

func init() {
	pluginInstallCmd.Flags().StringVar(&pluginInstallPlatform, "platform", "", "override platform detection (e.g. \"linux/amd64\")")
	pluginInstallCmd.Flags().BoolVar(&pluginInstallForce, "force", false, "overwrite existing plugin even if same version")
	pluginInstallCmd.Flags().BoolVar(&pluginInstallSkipVerify, "skip-verify", false, "skip signature verification (not recommended)")

	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginUninstallCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInfoCmd)
	pluginCmd.AddCommand(pluginVerifyCmd)
	pluginCmd.AddCommand(pluginInitCmd)
	pluginCmd.AddCommand(pluginPublishCmd)
	rootCmd.AddCommand(pluginCmd)
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	ref := args[0]
	w := cmd.OutOrStdout()

	// Load verification policy.
	policy, err := verify.LoadPolicyFile(verify.DefaultPolicyPath())
	if err != nil {
		return fmt.Errorf("loading security policy: %w", err)
	}
	verifier := verify.NewVerifier(policy)

	// Verify artifact before installing.
	fmt.Fprintln(w, "Verifying signature...")
	vResult := verifier.VerifyArtifact(ref, pluginInstallSkipVerify)

	if !vResult.OK() {
		return fmt.Errorf("verification failed: %w", vResult.Error)
	}

	if pluginInstallSkipVerify {
		fmt.Fprintln(w, "  Warning: signature verification skipped (--skip-verify)")
	} else if vResult.Signed {
		fmt.Fprintf(w, "  Signed by %s (%s)\n", vResult.SigningIdentity, vResult.SigningMethod)
		if vResult.TrustedPublisher {
			fmt.Fprintln(w, "  Verified publisher: trusted")
		}
	} else {
		fmt.Fprintln(w, "  No signature found (artifact is unsigned)")
	}

	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	opts := client.PullOptions{
		Force:      pluginInstallForce,
		SkipVerify: pluginInstallSkipVerify,
	}

	if pluginInstallPlatform != "" {
		p, err := parsePlatformFlag(pluginInstallPlatform)
		if err != nil {
			return err
		}
		opts.Platform = &p
	}

	fmt.Fprintf(w, "Pulling %s...\n", ref)
	result, err := ociClient.PullPlugin(cmd.Context(), ref, opts)
	if err != nil {
		return fmt.Errorf("installing plugin: %w", err)
	}

	fmt.Fprintf(w, "Installed %s v%s (%s)\n", result.Manifest.Name, result.Manifest.Version, result.Platform)
	fmt.Fprintf(w, "Binary: %s\n", result.BinaryPath)
	if result.Manifest.Type != "" {
		fmt.Fprintf(w, "\nPlugin is ready to use. Add to your workspace:\n")
		fmt.Fprintf(w, "  %s:\n", result.Manifest.Type)
		fmt.Fprintf(w, "    provider: %s\n", result.Manifest.Name)
	}
	return nil
}

// --- plugin uninstall ---

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Remove an installed plugin",
	Long: `Remove an installed shared plugin binary and its metadata.

The name is the plugin's short name (e.g. "ansible-config").`,
	Example: `  zhi plugin uninstall ansible-config`,
	Args:    cobra.ExactArgs(1),
	RunE:    runPluginUninstall,
}

func runPluginUninstall(cmd *cobra.Command, args []string) error {
	name := args[0]
	w := cmd.OutOrStdout()

	metaDir := metadata.DefaultMetadataDir()
	metaStore := metadata.NewStore(metaDir)

	// Load metadata to find the binary name.
	meta, err := metaStore.Load(name)
	if err != nil {
		return fmt.Errorf("loading plugin metadata: %w", err)
	}
	if meta == nil {
		return fmt.Errorf("plugin %q is not installed (no metadata found)", name)
	}

	// Remove the binary.
	pluginDir := core.DefaultPluginDir()
	binaryName := fmt.Sprintf("zhi-%s-%s", meta.Type, meta.Name)
	binaryPath := filepath.Join(pluginDir, binaryName)

	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing binary: %w", err)
	}

	// Remove metadata.
	if err := metaStore.Delete(name); err != nil {
		return fmt.Errorf("removing metadata: %w", err)
	}

	fmt.Fprintf(w, "Uninstalled %s\n", name)
	return nil
}

// --- plugin list ---

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed shared plugins",
	Long:  `List all plugins installed from OCI registries with version, source, and signing information.`,
	RunE:  runPluginList,
}

var pluginListJSON bool

func init() {
	pluginListCmd.Flags().BoolVar(&pluginListJSON, "json", false, "output as JSON")
}

type pluginListEntry struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Version         string `json:"version"`
	Ref             string `json:"ref"`
	Platform        string `json:"platform"`
	Installed       string `json:"installed"`
	Signed          bool   `json:"signed"`
	SigningIdentity string `json:"signingIdentity,omitempty"`
}

func runPluginList(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	metaDir := metadata.DefaultMetadataDir()
	metaStore := metadata.NewStore(metaDir)

	plugins, err := metaStore.List()
	if err != nil {
		return fmt.Errorf("listing plugins: %w", err)
	}

	if pluginListJSON {
		entries := make([]pluginListEntry, len(plugins))
		for i, p := range plugins {
			entries[i] = pluginListEntry{
				Name:            p.Name,
				Type:            p.Type,
				Version:         p.Version,
				Ref:             p.Ref,
				Platform:        p.Platform,
				Installed:       p.InstalledAt.Format(time.RFC3339),
				Signed:          p.Signed,
				SigningIdentity: p.SigningIdentity,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(plugins) == 0 {
		fmt.Fprintln(w, "No shared plugins installed.")
		fmt.Fprintln(w, "Install one with: zhi plugin install <oci-reference>")
		return nil
	}

	headers := []string{"NAME", "TYPE", "VERSION", "SIGNED", "PLATFORM", "SOURCE"}
	var rows [][]string
	for _, p := range plugins {
		signedStr := "no"
		if p.Signed {
			signedStr = "yes"
		}
		rows = append(rows, []string{p.Name, p.Type, p.Version, signedStr, p.Platform, p.Ref})
	}
	fprintTable(w, headers, rows)
	return nil
}

// --- plugin info ---

var pluginInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed information about an installed plugin",
	Long: `Display full metadata for an installed plugin including version, source,
platform, signer identity, signing method, trust level, and binary digest.`,
	Example: `  zhi plugin info ansible-config
  zhi plugin info ansible-config --json`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInfo,
}

var pluginInfoJSON bool

func init() {
	pluginInfoCmd.Flags().BoolVar(&pluginInfoJSON, "json", false, "output as JSON")
}

type pluginInfoOutput struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Version         string `json:"version"`
	Ref             string `json:"ref"`
	Digest          string `json:"digest"`
	BinaryDigest    string `json:"binaryDigest,omitempty"`
	Platform        string `json:"platform"`
	Publisher       string `json:"publisher,omitempty"`
	Signed          bool   `json:"signed"`
	SigningIdentity string `json:"signingIdentity,omitempty"`
	InstalledAt     string `json:"installedAt"`
}

func runPluginInfo(cmd *cobra.Command, args []string) error {
	name := args[0]
	w := cmd.OutOrStdout()

	metaDir := metadata.DefaultMetadataDir()
	metaStore := metadata.NewStore(metaDir)

	meta, err := metaStore.Load(name)
	if err != nil {
		return fmt.Errorf("loading plugin metadata: %w", err)
	}
	if meta == nil {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	if pluginInfoJSON {
		out := pluginInfoOutput{
			Name:            meta.Name,
			Type:            meta.Type,
			Version:         meta.Version,
			Ref:             meta.Ref,
			Digest:          meta.Digest,
			BinaryDigest:    meta.BinaryDigest,
			Platform:        meta.Platform,
			Publisher:       meta.Publisher,
			Signed:          meta.Signed,
			SigningIdentity: meta.SigningIdentity,
			InstalledAt:     meta.InstalledAt.Format(time.RFC3339),
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	fmt.Fprintf(w, "Name:             %s\n", meta.Name)
	fmt.Fprintf(w, "Type:             %s\n", meta.Type)
	fmt.Fprintf(w, "Version:          %s\n", meta.Version)
	fmt.Fprintf(w, "Platform:         %s\n", meta.Platform)
	fmt.Fprintf(w, "Reference:        %s\n", meta.Ref)
	fmt.Fprintf(w, "OCI Digest:       %s\n", meta.Digest)
	if meta.BinaryDigest != "" {
		fmt.Fprintf(w, "Binary Digest:    %s\n", meta.BinaryDigest)
	}
	if meta.Publisher != "" {
		fmt.Fprintf(w, "Publisher:        %s\n", meta.Publisher)
	}
	if meta.Signed {
		fmt.Fprintln(w, "Signed:           yes")
		if meta.SigningIdentity != "" {
			fmt.Fprintf(w, "Signer:           %s\n", meta.SigningIdentity)
		}
	} else {
		fmt.Fprintln(w, "Signed:           no")
	}
	if !meta.InstalledAt.IsZero() {
		fmt.Fprintf(w, "Installed At:     %s\n", meta.InstalledAt.Format(time.RFC3339))
	}

	return nil
}

// --- plugin verify ---

var pluginVerifyCmd = &cobra.Command{
	Use:   "verify <reference>",
	Short: "Verify a plugin artifact's signature without installing",
	Long: `Check the signature and trust status of an OCI plugin artifact.
This is useful for auditing and compliance workflows without installing the plugin.`,
	Example: `  zhi plugin verify oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0`,
	Args:    cobra.ExactArgs(1),
	RunE:    runPluginVerify,
}

func runPluginVerify(cmd *cobra.Command, args []string) error {
	ref := args[0]
	w := cmd.OutOrStdout()

	// Load verification policy.
	policy, err := verify.LoadPolicyFile(verify.DefaultPolicyPath())
	if err != nil {
		return fmt.Errorf("loading security policy: %w", err)
	}
	verifier := verify.NewVerifier(policy)

	fmt.Fprintf(w, "Verifying %s...\n", ref)
	result := verifier.VerifyArtifact(ref, false)

	if !result.OK() {
		fmt.Fprintf(w, "  Verification failed: %v\n", result.Error)
		return fmt.Errorf("verification failed: %w", result.Error)
	}

	fmt.Fprintf(w, "  Verification Level: %s\n", result.Level)
	if result.Signed {
		fmt.Fprintf(w, "  Signed:             yes\n")
		fmt.Fprintf(w, "  Signer:             %s\n", result.SigningIdentity)
		fmt.Fprintf(w, "  Signing Method:     %s\n", result.SigningMethod)
		if result.TrustedPublisher {
			fmt.Fprintf(w, "  Trusted Publisher:  yes\n")
		}
	} else {
		fmt.Fprintf(w, "  Signed:             no\n")
	}
	fmt.Fprintf(w, "  Status:             %s\n", result.Summary())

	return nil
}

// --- helpers ---

// newSharingClient creates a client.Client with default directories.
func newSharingClient() (*client.Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determining home directory: %w", err)
	}

	pluginDir := filepath.Join(home, ".zhi", "plugins")
	cacheDir := filepath.Join(home, ".zhi", "cache", "oci")
	configPath := filepath.Join(home, ".zhi", "config.yaml")
	metaDir := filepath.Join(home, ".zhi", "metadata")

	regStore, err := registry.NewStore(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading registry config: %w", err)
	}

	metaStore := metadata.NewStore(metaDir)
	return client.NewClient(pluginDir, cacheDir, regStore, metaStore), nil
}

// parsePlatformFlag parses a "os/arch" string into a Platform.
func parsePlatformFlag(s string) (client.Platform, error) {
	for i, c := range s {
		if c == '/' {
			return client.Platform{OS: s[:i], Arch: s[i+1:]}, nil
		}
	}
	return client.Platform{}, fmt.Errorf("invalid platform %q: expected os/arch format", s)
}
