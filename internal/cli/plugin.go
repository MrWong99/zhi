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
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage shared plugins",
	Long: `Install, uninstall, and list shared plugins from OCI registries.

Subcommands:
  install      Install a plugin from an OCI reference
  uninstall    Remove an installed plugin
  list         List installed shared plugins`,
	Example: `  zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
  zhi plugin uninstall ansible-config
  zhi plugin list`,
}

// --- plugin install ---

var pluginInstallCmd = &cobra.Command{
	Use:   "install <reference>",
	Short: "Install a plugin from an OCI reference",
	Long: `Download and install a plugin from an OCI registry.

The reference can be a full OCI reference or a short name:
  oci://ghcr.io/org/plugin:v1.0
  ghcr.io/org/plugin:v1.0`,
	Example: `  zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
  zhi plugin install ghcr.io/org/zhi-config-custom:v1.0.0 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInstall,
}

var (
	pluginInstallPlatform string
	pluginInstallForce    bool
)

func init() {
	pluginInstallCmd.Flags().StringVar(&pluginInstallPlatform, "platform", "", "override platform detection (e.g. \"linux/amd64\")")
	pluginInstallCmd.Flags().BoolVar(&pluginInstallForce, "force", false, "overwrite existing plugin even if same version")

	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginUninstallCmd)
	pluginCmd.AddCommand(pluginListCmd)
	rootCmd.AddCommand(pluginCmd)
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	ref := args[0]
	w := cmd.OutOrStdout()

	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	opts := client.PullOptions{
		Force: pluginInstallForce,
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
	Long:  `List all plugins installed from OCI registries with version and source information.`,
	RunE:  runPluginList,
}

var pluginListJSON bool

func init() {
	pluginListCmd.Flags().BoolVar(&pluginListJSON, "json", false, "output as JSON")
}

type pluginListEntry struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Ref       string `json:"ref"`
	Platform  string `json:"platform"`
	Installed string `json:"installed"`
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
				Name:      p.Name,
				Type:      p.Type,
				Version:   p.Version,
				Ref:       p.Ref,
				Platform:  p.Platform,
				Installed: p.InstalledAt.Format(time.RFC3339),
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

	headers := []string{"NAME", "TYPE", "VERSION", "PLATFORM", "SOURCE"}
	var rows [][]string
	for _, p := range plugins {
		rows = append(rows, []string{p.Name, p.Type, p.Version, p.Platform, p.Ref})
	}
	fprintTable(w, headers, rows)
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
