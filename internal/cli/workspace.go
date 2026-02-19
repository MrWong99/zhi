package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/client"
	"github.com/MrWong99/zhi/pkg/sharing/lockfile"
	"github.com/MrWong99/zhi/pkg/sharing/manifest"
	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspace sharing",
	Long: `Install, publish, and lock workspaces for reproducible sharing.

Subcommands:
  install    Install a workspace and its plugin dependencies from an OCI reference
  publish    Publish the current workspace as an OCI artifact
  lock       Generate or update zhi-plugins.lock from workspace plugin declarations`,
	Example: `  zhi workspace install oci://ghcr.io/org/zhi-workspace-k8s:v1.0 ./my-cluster
  zhi workspace publish --registry ghcr.io/myorg
  zhi workspace lock`,
}

// --- workspace install ---

var workspaceInstallCmd = &cobra.Command{
	Use:   "install <reference> [target-dir]",
	Short: "Install a workspace and its plugin dependencies",
	Long: `Download a workspace artifact from an OCI registry, extract its files,
and install any required plugin dependencies.`,
	Example: `  zhi workspace install oci://ghcr.io/org/zhi-workspace-k8s:v1.0
  zhi workspace install oci://ghcr.io/org/zhi-workspace-k8s:v1.0 ./my-cluster
  zhi workspace install oci://ghcr.io/org/zhi-workspace-k8s:v1.0 --skip-plugins`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runWorkspaceInstall,
}

var (
	workspaceInstallSkipPlugins    bool
	workspaceInstallSkipToolsCheck bool
)

func init() {
	workspaceInstallCmd.Flags().BoolVar(&workspaceInstallSkipPlugins, "skip-plugins", false, "don't install plugin dependencies")
	workspaceInstallCmd.Flags().BoolVar(&workspaceInstallSkipToolsCheck, "skip-tools-check", false, "don't check for required external tools")

	workspaceCmd.AddCommand(workspaceInstallCmd)
	workspaceCmd.AddCommand(workspacePublishCmd)
	workspaceCmd.AddCommand(workspaceLockCmd)
	workspaceCmd.AddCommand(workspaceSetupCmd)
	rootCmd.AddCommand(workspaceCmd)
}

func runWorkspaceInstall(cmd *cobra.Command, args []string) error {
	ref := args[0]
	w := cmd.OutOrStdout()

	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Pulling workspace %s...\n", ref)

	// Determine target directory.
	targetDir := ""
	if len(args) > 1 {
		targetDir = args[1]
	}
	if targetDir == "" {
		targetDir = "."
	}

	opts := client.PullOptions{
		SkipDependencies: workspaceInstallSkipPlugins,
	}
	wm, err := ociClient.PullWorkspace(cmd.Context(), ref, targetDir, opts)
	if err != nil {
		return fmt.Errorf("installing workspace: %w", err)
	}

	// Report dependency installation results.
	if !workspaceInstallSkipPlugins && len(wm.Dependencies) > 0 {
		fmt.Fprintln(w, "\nDependencies:")
		for _, dep := range wm.Dependencies {
			if dep.Ref == "" {
				continue
			}
			optLabel := ""
			if dep.Optional {
				optLabel = " (optional)"
			}
			fmt.Fprintf(w, "  + %s%s\n", dep.Ref, optLabel)
		}
	}

	// Check external tools if not skipped.
	if !workspaceInstallSkipToolsCheck && len(wm.Tools) > 0 {
		fmt.Fprintln(w, "\nChecking tools:")
		for _, tool := range wm.Tools {
			checkExternalTool(w, tool)
		}
	}

	fmt.Fprintf(w, "\nWorkspace installed to %s\n", targetDir)
	return nil
}

// checkExternalTool checks if an external tool is available on PATH.
func checkExternalTool(w interface{ Write([]byte) (int, error) }, tool manifest.ToolRequirement) {
	_, err := exec.LookPath(tool.Name)
	symbol := "+"
	status := "found"
	if err != nil {
		symbol = "!"
		status = "not found"
	}
	msg := fmt.Sprintf("  %s %s %s", symbol, tool.Name, status)
	if tool.Version != "" {
		msg += fmt.Sprintf(" (%s required)", tool.Version)
	}
	fmt.Fprintln(w, msg)
}

// --- workspace publish ---

var workspacePublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the current workspace as an OCI artifact",
	Long: `Package the current workspace (zhi.yaml, templates, apply scripts) and
push it to an OCI registry. Plugin dependencies declared in zhi.yaml are
recorded in the workspace artifact metadata.

Signing:
  --sign             Sign the artifact with cosign after pushing
  --key <path>       Path to a cosign private key (default: keyless via Fulcio/OIDC)

When --sign is used without --key, keyless signing via Sigstore Fulcio is used,
which requires an OIDC identity (e.g. GitHub Actions).`,
	Example: `  zhi workspace publish --registry ghcr.io/myorg
  zhi workspace publish --registry ghcr.io/myorg --tag v1.0.0
  zhi workspace publish --registry ghcr.io/myorg --sign
  zhi workspace publish --registry ghcr.io/myorg --sign --key cosign.key`,
	RunE: runWorkspacePublish,
}

var (
	workspacePublishRegistry string
	workspacePublishTag      string
	workspacePublishSign     bool
	workspacePublishKey      string
)

func init() {
	workspacePublishCmd.Flags().StringVar(&workspacePublishRegistry, "registry", "", "target OCI registry (required)")
	workspacePublishCmd.Flags().StringVar(&workspacePublishTag, "tag", "", "OCI tag (default: v{version} from manifest)")
	workspacePublishCmd.Flags().BoolVar(&workspacePublishSign, "sign", false, "sign the artifact with cosign after pushing")
	workspacePublishCmd.Flags().StringVar(&workspacePublishKey, "key", "", "path to cosign private key (default: keyless via Fulcio/OIDC)")
}

func runWorkspacePublish(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	if workspacePublishRegistry == "" {
		return fmt.Errorf("--registry is required")
	}

	// Validate --key requires --sign.
	if workspacePublishKey != "" && !workspacePublishSign {
		return fmt.Errorf("--key requires --sign")
	}

	// Look for workspace manifest.
	wm, err := loadWorkspaceMetadata()
	if err != nil {
		return err
	}

	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	opts := client.PushOptions{
		Tag: workspacePublishTag,
	}

	fmt.Fprintf(w, "Publishing workspace %s v%s to %s...\n", wm.Name, wm.Version, workspacePublishRegistry)

	result, err := ociClient.PushWorkspace(cmd.Context(), wm, ".", workspacePublishRegistry, opts)
	if err != nil {
		return fmt.Errorf("publishing workspace: %w", err)
	}

	fmt.Fprintf(w, "\nPublished successfully!\n")
	fmt.Fprintf(w, "  Reference: %s\n", result.Reference)
	fmt.Fprintf(w, "  Tag:       %s\n", result.Tag)
	fmt.Fprintf(w, "  Digest:    %s\n", result.Digest)

	// Sign the artifact if requested.
	if workspacePublishSign {
		fmt.Fprintln(w, "\nSigning artifact...")
		if workspacePublishKey != "" {
			fmt.Fprintf(w, "  Method: key-based (%s)\n", workspacePublishKey)
		} else {
			fmt.Fprintln(w, "  Method: keyless (Sigstore Fulcio/OIDC)")
		}

		signer, err := verify.NewSigner(cmd.Context())
		if err != nil {
			return fmt.Errorf("creating signer: %w", err)
		}

		signOpts := verify.SignOptions{
			KeyPath:      workspacePublishKey,
			UseRekor:     true,
			UseTimestamp: true,
		}

		signResult, err := signer.SignArtifact(cmd.Context(), result.Digest, signOpts)
		if err != nil {
			return fmt.Errorf("signing artifact: %w", err)
		}

		bundlePath := result.Digest[7:15] + ".bundle.json"
		if err := verify.SaveBundleToFile(signResult.BundleJSON, bundlePath); err != nil {
			return fmt.Errorf("saving signature bundle: %w", err)
		}

		fmt.Fprintf(w, "  Signed successfully\n")
		fmt.Fprintf(w, "  Identity: %s\n", signResult.SigningIdentity)
		fmt.Fprintf(w, "  Bundle saved to: %s\n", bundlePath)
	}

	fmt.Fprintf(w, "\nInstall with: zhi workspace install %s\n", result.Reference)
	return nil
}

// loadWorkspaceMetadata loads a workspace manifest from zhi-workspace.yaml
// or falls back to extracting metadata from zhi.yaml.
func loadWorkspaceMetadata() (*manifest.WorkspaceManifest, error) {
	// Try zhi-workspace.yaml first.
	if _, err := os.Stat("zhi-workspace.yaml"); err == nil {
		return manifest.LoadWorkspaceFile("zhi-workspace.yaml")
	}

	// Fall back to zhi.yaml for basic metadata.
	if _, err := os.Stat("zhi.yaml"); err != nil {
		return nil, fmt.Errorf("no zhi-workspace.yaml or zhi.yaml found in current directory")
	}

	return nil, fmt.Errorf("zhi-workspace.yaml not found; create one with workspace metadata (name, version, description)")
}

// --- workspace lock ---

var workspaceLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Generate or update zhi-plugins.lock",
	Long: `Resolve all plugin references in zhi.yaml to exact OCI digests and
write a zhi-plugins.lock file for reproducible installations.`,
	Example: `  zhi workspace lock`,
	RunE:    runWorkspaceLock,
}

func runWorkspaceLock(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	metaDir := metadata.DefaultMetadataDir()
	metaStore := metadata.NewStore(metaDir)

	plugins, err := metaStore.List()
	if err != nil {
		return fmt.Errorf("listing installed plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Fprintln(w, "No installed plugins to lock.")
		fmt.Fprintln(w, "Install plugins first with: zhi plugin install <reference>")
		return nil
	}

	lf := &lockfile.LockFile{
		Version:     lockfile.Version,
		GeneratedAt: time.Now().UTC(),
		ZhiVersion:  "0.8.0",
	}

	for _, p := range plugins {
		entry := lockfile.LockedPlugin{
			Name:            p.Name,
			Type:            p.Type,
			Ref:             p.Ref,
			Digest:          p.Digest,
			Signed:          p.Signed,
			SigningIdentity: p.SigningIdentity,
		}
		if p.Platform != "" {
			entry.Platforms = map[string]string{
				p.Platform: p.Digest,
			}
		}
		lf.Plugins = append(lf.Plugins, entry)
	}

	lockPath := "zhi-plugins.lock"
	if err := lf.SaveFile(lockPath); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}

	fmt.Fprintf(w, "Wrote %s with %d plugin(s)\n", lockPath, len(lf.Plugins))
	for _, p := range lf.Plugins {
		digestShort := p.Digest
		if len(digestShort) > 19 {
			digestShort = digestShort[:19] + "..."
		}
		fmt.Fprintf(w, "  %s %s (%s)\n", p.Name, p.Ref, digestShort)
	}
	return nil
}

// --- workspace setup --from-lock ---

var workspaceSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install plugins from lock file",
	Long: `Install all plugins at the exact versions pinned in zhi-plugins.lock.
This command is designed for CI/CD where reproducibility matters.`,
	Example: `  zhi workspace setup --from-lock`,
	RunE:    runWorkspaceSetup,
}

var workspaceSetupFromLock bool

func init() {
	workspaceSetupCmd.Flags().BoolVar(&workspaceSetupFromLock, "from-lock", false, "install from zhi-plugins.lock (required)")
}

func runWorkspaceSetup(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	if !workspaceSetupFromLock {
		return fmt.Errorf("--from-lock is required")
	}

	lockPath := "zhi-plugins.lock"
	lf, err := lockfile.LoadFile(lockPath)
	if err != nil {
		return fmt.Errorf("loading lock file: %w", err)
	}

	if len(lf.Plugins) == 0 {
		fmt.Fprintln(w, "Lock file has no plugins.")
		return nil
	}

	ociClient, err := newSharingClient()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Installing %d plugin(s) from %s...\n", len(lf.Plugins), lockPath)

	for _, lp := range lf.Plugins {
		ref := lp.Ref
		// Use digest-pinned reference for exact version.
		if lp.Digest != "" && !strings.Contains(ref, "@") {
			// Append digest to reference for exact pull.
			ref = stripTag(ref) + "@" + lp.Digest
		}

		fmt.Fprintf(w, "  Installing %s...\n", lp.Name)
		result, err := ociClient.PullPlugin(cmd.Context(), ref, client.PullOptions{Force: true})
		if err != nil {
			return fmt.Errorf("installing %s: %w", lp.Name, err)
		}

		// Verify digest matches lock file.
		if lp.Digest != "" {
			if err := lp.VerifyDigest(result.Digest); err != nil {
				// Remove the binary we just installed since it doesn't match.
				_ = os.Remove(result.BinaryPath)
				return fmt.Errorf("tamper detection: %w", err)
			}
		}

		fmt.Fprintf(w, "    + %s v%s (%s)\n", result.Manifest.Name, result.Manifest.Version, result.Platform)
	}

	fmt.Fprintf(w, "\nAll plugins installed from lock file.\n")
	return nil
}

// stripTag removes the tag from an OCI reference, keeping only host/repo.
func stripTag(ref string) string {
	ref = strings.TrimPrefix(ref, "oci://")
	// Find the last colon after the first slash (tag separator).
	slashIdx := strings.Index(ref, "/")
	if slashIdx < 0 {
		return ref
	}
	colonIdx := strings.LastIndex(ref[slashIdx:], ":")
	if colonIdx >= 0 {
		ref = ref[:slashIdx+colonIdx]
	}
	return ref
}

// workspaceLockFilePath returns the path to zhi-plugins.lock relative to a directory.
func workspaceLockFilePath(dir string) string {
	return filepath.Join(dir, "zhi-plugins.lock")
}
