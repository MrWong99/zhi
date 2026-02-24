package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"crypto/sha256"
	"runtime"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/internal/tlsutil"
	"github.com/MrWong99/zhi/pkg/sharing/client"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/registry"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage shared plugins",
	Long: `Install, uninstall, publish, search, rate, and manage shared plugins from OCI
registries and the zhi marketplace.

Subcommands:
  install      Install a plugin from an OCI reference or short name
  uninstall    Remove an installed plugin
  list         List installed shared plugins
  search       Search the marketplace for plugins
  info         Show detailed information about a plugin
  rate         Rate a plugin on the marketplace (1-5 stars)
  verify       Verify a plugin artifact's signature without installing
  init         Generate a zhi-plugin.yaml manifest for a new plugin
  new          Scaffold a complete plugin project from a template
  publish      Publish a plugin to an OCI registry
  register     Register a plugin with the marketplace`,
	Example: `  zhi plugin install ansible-config
  zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
  zhi plugin search ansible
  zhi plugin info ansible-config
  zhi plugin rate zhi-project/ansible-config 5
  zhi plugin uninstall ansible-config
  zhi plugin new --name my-config --type config --lang go
  zhi plugin register`,
}

// --- plugin install ---

var pluginInstallCmd = &cobra.Command{
	Use:   "install <reference>",
	Short: "Install a plugin from an OCI reference or short name",
	Long: `Download and install a plugin from an OCI registry.

The reference can be a full OCI reference or a marketplace short name:
  oci://ghcr.io/org/plugin:v1.0     Full OCI reference
  ghcr.io/org/plugin:v1.0           OCI reference without scheme
  ansible-config                    Short name (resolved via marketplace)
  ansible-config@1.2.0              Short name with version

When a short name is given, the marketplace is queried to resolve it
to a full OCI reference before pulling.

Signature verification is performed by default. Use --skip-verify to disable.`,
	Example: `  zhi plugin install ansible-config
  zhi plugin install ansible-config@1.2.0
  zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
  zhi plugin install ghcr.io/org/zhi-config-custom:v1.0.0 --force
  zhi plugin install --path ./my-plugin
  zhi plugin install local://./my-plugin`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginInstall,
}

var (
	pluginInstallPlatform   string
	pluginInstallForce      bool
	pluginInstallSkipVerify bool
	pluginInstallPath       string
)

func init() {
	pluginInstallCmd.Flags().StringVar(&pluginInstallPlatform, "platform", "", "override platform detection (e.g. \"linux/amd64\")")
	pluginInstallCmd.Flags().BoolVar(&pluginInstallForce, "force", false, "overwrite existing plugin even if same version")
	pluginInstallCmd.Flags().BoolVar(&pluginInstallSkipVerify, "skip-verify", false, "skip signature verification (not recommended)")
	pluginInstallCmd.Flags().StringVar(&pluginInstallPath, "path", "", "install a plugin from a local binary (requires adjacent zhi-plugin.yaml)")

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
	w := cmd.OutOrStdout()

	// Determine if this is a local install via --path or local:// scheme.
	localPath := pluginInstallPath
	if len(args) > 0 && strings.HasPrefix(args[0], "local://") {
		localPath = strings.TrimPrefix(args[0], "local://")
	}
	if localPath != "" {
		return runPluginInstallLocal(cmd, localPath)
	}

	if len(args) == 0 {
		return fmt.Errorf("requires a plugin reference argument or --path flag")
	}

	ref := args[0]

	// Resolve short name via marketplace if necessary.
	if client.IsShortName(ref) {
		fmt.Fprintf(w, "Resolving %s via marketplace...\n", ref)
		mc, err := newMarketplaceClient()
		if err != nil {
			return fmt.Errorf("creating marketplace client: %w", err)
		}
		resolved, err := mc.ResolveShortName(cmd.Context(), ref)
		if err != nil {
			return fmt.Errorf("resolving short name %q: %w", ref, err)
		}
		fmt.Fprintf(w, "  -> %s\n", resolved)
		ref = resolved
	}

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

// runPluginInstallLocal handles installing a plugin from a local binary path.
// It expects an adjacent zhi-plugin.yaml manifest next to the binary.
func runPluginInstallLocal(cmd *cobra.Command, localPath string) error {
	w := cmd.OutOrStdout()

	// Resolve to absolute path.
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Validate the binary exists.
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary not found: %s", absPath)
		}
		return fmt.Errorf("accessing binary: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a binary: %s", absPath)
	}

	// Check executable permission.
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("binary is not executable: %s", absPath)
	}

	// Find and load the manifest from an adjacent zhi-plugin.yaml.
	manifestPath := filepath.Join(filepath.Dir(absPath), "zhi-plugin.yaml")
	m, err := manifest.LoadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("loading manifest (a zhi-plugin.yaml file must exist next to the binary): %w", err)
	}

	pluginDir := core.DefaultPluginDir()
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}

	binaryName := m.BinaryName()
	destPath := filepath.Join(pluginDir, binaryName)

	// Check if already installed (unless --force).
	if !pluginInstallForce {
		if _, err := os.Stat(destPath); err == nil {
			return fmt.Errorf("plugin %q already exists at %s (use --force to overwrite)", m.Name, destPath)
		}
	}

	// Copy the binary to the plugins directory.
	fmt.Fprintf(w, "Installing %s v%s from local path...\n", m.Name, m.Version)

	srcFile, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("opening binary: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("creating destination binary: %w", err)
	}

	// Compute SHA-256 digest while copying.
	h := sha256.New()
	tee := io.TeeReader(srcFile, h)

	if _, err := io.Copy(destFile, tee); err != nil {
		destFile.Close()
		os.Remove(destPath)
		return fmt.Errorf("copying binary: %w", err)
	}
	if err := destFile.Close(); err != nil {
		return fmt.Errorf("closing binary: %w", err)
	}

	binaryDigest := fmt.Sprintf("sha256:%x", h.Sum(nil))

	// Save metadata.
	metaDir := metadata.DefaultMetadataDir()
	metaStore := metadata.NewStore(metaDir)

	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	meta := &metadata.InstalledPlugin{
		Name:         m.Name,
		Type:         m.Type,
		Version:      m.Version,
		Ref:          "local://" + absPath,
		Platform:     platform,
		InstalledAt:  time.Now(),
		Publisher:    m.Author,
		BinaryDigest: binaryDigest,
	}
	if err := metaStore.Save(meta); err != nil {
		return fmt.Errorf("saving metadata: %w", err)
	}

	fmt.Fprintf(w, "Installed %s v%s (%s)\n", m.Name, m.Version, platform)
	fmt.Fprintf(w, "Binary: %s\n", destPath)
	if m.Type != "" {
		fmt.Fprintf(w, "\nPlugin is ready to use. Add to your workspace:\n")
		fmt.Fprintf(w, "  %s:\n", m.Type)
		fmt.Fprintf(w, "    provider: %s\n", m.Name)
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
	Short: "Show detailed information about a plugin",
	Long: `Display metadata for a plugin. If the plugin is installed locally, local
metadata is shown. Otherwise, the marketplace is queried for plugin details.

When marketplace data is available, additional fields like rating, downloads,
and available versions are displayed.`,
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

	// If not installed locally, try the marketplace.
	if meta == nil {
		return runPluginInfoFromMarketplace(cmd, name)
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

// runPluginInfoFromMarketplace queries the marketplace for plugin details
// when the plugin is not installed locally.
func runPluginInfoFromMarketplace(cmd *cobra.Command, name string) error {
	w := cmd.OutOrStdout()

	mc, err := newMarketplaceClient()
	if err != nil {
		return fmt.Errorf("plugin %q is not installed and marketplace is not available: %w", name, err)
	}

	// Search for the plugin by name to find publisher/name.
	resp, err := mc.Search(cmd.Context(), name, marketplace.SearchOptions{Limit: 10})
	if err != nil {
		return fmt.Errorf("plugin %q is not installed and marketplace search failed: %w", name, err)
	}

	// Find exact match.
	var match *marketplace.SearchResult
	for i := range resp.Results {
		if resp.Results[i].Name == name {
			match = &resp.Results[i]
			break
		}
	}
	if match == nil {
		return fmt.Errorf("plugin %q is not installed and not found in marketplace", name)
	}

	// Try to get full plugin detail from marketplace for richer output.
	detail, detailErr := mc.GetPlugin(cmd.Context(), match.Type, match.Author, match.Name)

	if pluginInfoJSON {
		if detailErr == nil && detail != nil {
			data, err := json.MarshalIndent(detail, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(w, string(data))
			return nil
		}
		data, err := json.MarshalIndent(match, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	fmt.Fprintf(w, "Name:             %s\n", match.Name)
	fmt.Fprintf(w, "Type:             %s\n", match.Type)
	fmt.Fprintf(w, "Version:          %s (latest)\n", match.LatestVersion)
	author := match.Author
	if match.Verified {
		author += " \u2713 (verified)"
	}
	fmt.Fprintf(w, "Author:           %s\n", author)

	// Show rating info from detail if available.
	if detailErr == nil && detail != nil && detail.Rating.Count > 0 {
		fmt.Fprintf(w, "Rating:           %.1f (%d ratings)\n", detail.Rating.Average, detail.Rating.Count)
		printRatingDistribution(w, detail.Rating)
		fmt.Fprintf(w, "Downloads:        %s total", formatDownloads(detail.Statistics.TotalDownloads))
		if detail.Statistics.MonthlyDownloads > 0 {
			fmt.Fprintf(w, "  |  %s this month", formatDownloads(detail.Statistics.MonthlyDownloads))
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "Rating:           %.1f (%d ratings)\n", match.Rating, match.RatingCount)
		fmt.Fprintf(w, "Downloads:        %s\n", formatDownloads(match.Downloads))
	}

	fmt.Fprintf(w, "OCI Reference:    %s\n", match.OCIRef)
	if len(match.Platforms) > 0 {
		fmt.Fprintf(w, "Platforms:        %s\n", joinPlatforms(match.Platforms))
	}
	fmt.Fprintf(w, "Installed:        no\n")
	if match.Description != "" {
		fmt.Fprintf(w, "\nDescription:\n  %s\n", match.Description)
	}
	fmt.Fprintf(w, "\nInstall with: zhi plugin install %s\n", match.Name)

	return nil
}

// printRatingDistribution renders a star distribution histogram.
func printRatingDistribution(w io.Writer, rating marketplace.PluginRating) {
	dist := []int{
		rating.Distribution.Five,
		rating.Distribution.Four,
		rating.Distribution.Three,
		rating.Distribution.Two,
		rating.Distribution.One,
	}
	labels := []string{"5", "4", "3", "2", "1"}

	maxCount := 0
	for _, c := range dist {
		if c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		return
	}

	barWidth := 20
	for i, c := range dist {
		var bar strings.Builder
		if maxCount > 0 {
			width := c * barWidth / maxCount
			for range width {
				bar.WriteString("#")
			}
		}
		pct := 0
		if rating.Count > 0 {
			pct = c * 100 / rating.Count
		}
		fmt.Fprintf(w, "  %s star  %-20s  %d (%d%%)\n", labels[i], bar.String(), c, pct)
	}
}

// joinPlatforms joins a slice of platform strings with ", ".
func joinPlatforms(platforms []string) string {
	var result strings.Builder
	for i, p := range platforms {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(p)
	}
	return result.String()
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

	var opts []client.ClientOption
	tlsCfg := regStore.GlobalClientTLS()
	clientTLS := &tlsutil.ClientConfig{
		CertFile: tlsCfg.CertFile,
		KeyFile:  tlsCfg.KeyFile,
		CAFile:   tlsCfg.CAFile,
	}
	if clientTLS.Enabled() {
		hc, err := clientTLS.HTTPClient(60 * time.Second)
		if err != nil {
			return nil, fmt.Errorf("configuring registry TLS: %w", err)
		}
		opts = append(opts, client.WithHTTPClient(hc))
	}

	return client.NewClient(pluginDir, cacheDir, regStore, metaStore, opts...), nil
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
