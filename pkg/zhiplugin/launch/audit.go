package launch

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
)

// validateBinaryPath checks that the binary path is safe to execute.
// It resolves symlinks and rejects paths containing "..".
func validateBinaryPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolving plugin binary path %q: %w", path, err)
	}

	// Reject paths with ".." components after resolution (should not happen
	// after EvalSymlinks, but defend in depth).
	if strings.Contains(resolved, "..") {
		return "", fmt.Errorf("plugin binary path %q contains '..' after symlink resolution", path)
	}

	return resolved, nil
}

// AuditBinary logs the SHA-256 hash of the plugin binary, warns if the file
// is world-writable, and verifies the digest against the stored digest from
// installation time. Returns an error if the integrity check fails.
func AuditBinary(log *slog.Logger, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat plugin binary %q: %w", path, err)
	}

	// Warn if world-writable.
	if info.Mode()&0o002 != 0 {
		log.Warn("plugin binary is world-writable", "path", path, "permissions", info.Mode().String())
	}

	// Compute SHA-256 hash.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open plugin binary %q for hashing: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("cannot hash plugin binary %q: %w", path, err)
	}

	sha := fmt.Sprintf("%x", h.Sum(nil))
	log.Info("launching plugin", "path", path, "sha256", sha)

	// Verify binary integrity against stored digest from install time.
	return verifyBinaryIntegrity(log, path, sha)
}

// verifyBinaryIntegrity checks the binary's SHA-256 against the digest
// stored in metadata during installation. Returns an error if the digests
// do not match (indicating post-install tampering). Returns nil if no
// stored digest is available (e.g., locally-built plugins).
func verifyBinaryIntegrity(log *slog.Logger, path, computedHex string) error {
	name := PluginNameFromPath(path)
	if name == "" {
		return nil // Not an installed shared plugin.
	}

	metaDir := metadata.DefaultMetadataDir()
	if metaDir == "" {
		return nil
	}

	metaStore := metadata.NewStore(metaDir)
	meta, err := metaStore.Load(name)
	if err != nil || meta == nil {
		return nil // No metadata — not a shared plugin or never installed via sharing.
	}

	if meta.BinaryDigest == "" {
		return nil // No stored digest to compare (legacy metadata).
	}

	actualDigest := "sha256:" + computedHex
	if err := verify.VerifyBinaryDigest(path, meta.BinaryDigest); err != nil {
		log.Error("binary integrity check failed",
			"plugin", name,
			"expected", meta.BinaryDigest,
			"actual", actualDigest,
			"remediation", "reinstall with: zhi plugin install "+meta.Ref+" --force",
		)
		return fmt.Errorf("binary integrity check failed for plugin %q: %w", name, err)
	}

	log.Info("binary integrity verified", "plugin", name, "digest", actualDigest)
	return nil
}

// PluginNameFromPath extracts the plugin short name from a binary path.
// For example, "/home/user/.zhi/plugins/zhi-config-ansible" returns "ansible".
// Returns empty string if the path doesn't follow the naming convention.
func PluginNameFromPath(path string) string {
	base := filepath.Base(path)
	// Binary names follow: zhi-{type}-{name}
	for _, prefix := range []string{"zhi-config-", "zhi-transform-", "zhi-store-", "zhi-ui-"} {
		if strings.HasPrefix(base, prefix) {
			return base[len(prefix):]
		}
	}
	return ""
}
