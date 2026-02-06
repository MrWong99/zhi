package core

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/sharing/metadata"
	"github.com/MrWong99/zhi/pkg/sharing/verify"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// auditPluginBinary logs the SHA-256 hash of the plugin binary and warns
// if the file is world-writable. It also verifies the binary digest
// against the stored digest from installation time, catching post-install
// tampering.
func auditPluginBinary(path string) {
	log := Logger()

	info, err := os.Stat(path)
	if err != nil {
		log.Warn("cannot stat plugin binary", "path", path, "error", err)
		return
	}

	// Warn if world-writable.
	if info.Mode()&0o002 != 0 {
		log.Warn("plugin binary is world-writable", "path", path, "permissions", info.Mode().String())
	}

	// Compute SHA-256 hash.
	f, err := os.Open(path)
	if err != nil {
		log.Warn("cannot open plugin binary for hashing", "path", path, "error", err)
		return
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Warn("cannot hash plugin binary", "path", path, "error", err)
		return
	}

	sha := fmt.Sprintf("%x", h.Sum(nil))
	log.Info("launching plugin", "path", path, "sha256", sha)

	// Verify binary integrity against stored digest from install time.
	verifyBinaryIntegrity(log, path, sha)
}

// verifyBinaryIntegrity checks the binary's SHA-256 against the digest
// stored in metadata during installation. This detects post-install
// tampering.
func verifyBinaryIntegrity(log *slog.Logger, path, computedHex string) {
	name := pluginNameFromPath(path)
	if name == "" {
		return // Not an installed shared plugin.
	}

	metaDir := metadata.DefaultMetadataDir()
	if metaDir == "" {
		return
	}

	metaStore := metadata.NewStore(metaDir)
	meta, err := metaStore.Load(name)
	if err != nil || meta == nil {
		return // No metadata — not a shared plugin or never installed via sharing.
	}

	if meta.BinaryDigest == "" {
		return // No stored digest to compare (legacy metadata).
	}

	actualDigest := "sha256:" + computedHex
	if err := verify.VerifyBinaryDigest(path, meta.BinaryDigest); err != nil {
		// Log as error — this is a security-relevant event.
		log.Error("binary integrity check failed",
			"plugin", name,
			"expected", meta.BinaryDigest,
			"actual", actualDigest,
			"remediation", "reinstall with: zhi plugin install "+meta.Ref+" --force",
		)
	} else {
		log.Info("binary integrity verified", "plugin", name, "digest", actualDigest)
	}
}

// pluginNameFromPath extracts the plugin short name from a binary path.
// For example, "/home/user/.zhi/plugins/zhi-config-ansible" returns "ansible".
// Returns empty string if the path doesn't follow the naming convention.
func pluginNameFromPath(path string) string {
	base := filepath.Base(path)
	// Binary names follow: zhi-{type}-{name}
	for _, prefix := range []string{"zhi-config-", "zhi-transform-", "zhi-store-", "zhi-ui-"} {
		if strings.HasPrefix(base, prefix) {
			return base[len(prefix):]
		}
	}
	return ""
}

// LaunchConfig launches an external config plugin binary and returns the
// config.Plugin interface along with a cleanup function that kills the
// plugin process. The caller must call cleanup when done.
func LaunchConfig(path string) (config.Plugin, func(), error) {
	auditPluginBinary(path)

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          config.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to config plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense("config")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing config plugin %s: %w", path, err)
	}

	p, ok := raw.(config.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("config plugin %s: dispensed value does not implement config.Plugin", path)
	}

	return p, client.Kill, nil
}

// LaunchTransform launches an external transform plugin binary and returns
// the transform.Plugin interface along with a cleanup function.
func LaunchTransform(path string) (transform.Plugin, func(), error) {
	auditPluginBinary(path)

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          transform.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to transform plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense("transform")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing transform plugin %s: %w", path, err)
	}

	p, ok := raw.(transform.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("transform plugin %s: dispensed value does not implement transform.Plugin", path)
	}

	return p, client.Kill, nil
}

// LaunchStore launches an external store plugin binary and returns the
// store.Plugin interface along with a cleanup function.
func LaunchStore(path string) (store.Plugin, func(), error) {
	auditPluginBinary(path)

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          store.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to store plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense("store")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing store plugin %s: %w", path, err)
	}

	p, ok := raw.(store.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("store plugin %s: dispensed value does not implement store.Plugin", path)
	}

	return p, client.Kill, nil
}

// LaunchUI launches an external UI plugin binary and returns the
// ui.Plugin interface along with a cleanup function.
func LaunchUI(path string) (zhiui.Plugin, func(), error) {
	auditPluginBinary(path)

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          zhiui.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to UI plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense("ui")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing UI plugin %s: %w", path, err)
	}

	p, ok := raw.(zhiui.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("UI plugin %s: dispensed value does not implement ui.Plugin", path)
	}

	return p, client.Kill, nil
}
