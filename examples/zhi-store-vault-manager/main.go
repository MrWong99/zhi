// zhi-store-vault-manager is a meta-plugin that wraps zhi-store-vault with
// automatic credential management for deployed applications. It uses the
// vaultmanager package for all business logic and serves as a thin wrapper.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/internal/tlsutil"
	"github.com/MrWong99/zhi/pkg/providers/store/vaultmanager"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/launch"
	"github.com/MrWong99/zhi/pkg/zhiplugin/pluginopts"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-store-vault-manager",
		Level:  level,
		Output: os.Stderr,
	})
	logger.Info("starting vault manager meta-plugin")

	opts := pluginopts.Options()
	cfg, err := vaultmanager.ParseConfig(opts)
	if err != nil {
		logger.Error("failed to parse configuration", "error", err)
		os.Exit(1)
	}

	selfPath, err := os.Executable()
	if err != nil {
		logger.Error("cannot determine own executable path", "error", err)
		os.Exit(1)
	}
	dir := filepath.Dir(selfPath)

	vaultBin := findBinary(dir, "zhi-store-vault")
	if vaultBin == "" {
		logger.Error("child plugin zhi-store-vault not found", "dir", dir)
		os.Exit(1)
	}

	childOpts := vaultmanager.ChildVaultOptions(cfg)
	child, cleanup, err := launch.LaunchStore(vaultBin,
		launch.WithLogger(logger),
		launch.WithIsolatedEnv(vaultmanager.ChildVaultEnv(cfg)),
		launch.WithPluginOptions(childOpts),
	)
	if err != nil {
		logger.Error("failed to launch child vault plugin", "binary", vaultBin, "error", err)
		os.Exit(1)
	}
	defer cleanup()

	adminHTTPClient, err := buildAdminHTTPClient(cfg)
	if err != nil {
		logger.Error("failed to configure TLS for admin client", "error", err)
		os.Exit(1)
	}
	admin := vaultmanager.NewAdminClient(cfg.Addr, "", cfg.Namespace, adminHTTPClient)
	cm := vaultmanager.NewCredManager(admin, cfg.Mount, cfg.Prefix, cfg.Workspace, logger)

	managerStore := &vaultmanager.Store{
		DelegatingPlugin: store.NewDelegatingPlugin(child),
		Admin:            admin,
		CredManager:      cm,
		Apps:             cfg.Apps,
		Cfg:              cfg,
		Log:              logger,
	}

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"store": &store.GRPCPlugin{Impl: managerStore},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}

func buildAdminHTTPClient(cfg *vaultmanager.Config) (*http.Client, error) {
	tlsCfg := tlsutil.ClientConfig{
		CAFile:             cfg.CACert,
		CertFile:           cfg.ClientCert,
		KeyFile:            cfg.ClientKey,
		InsecureSkipVerify: cfg.SkipVerify,
	}
	if tlsCfg.Enabled() {
		if cfg.SkipVerify {
			slog.Warn("VAULT_SKIP_VERIFY is enabled, TLS certificate verification disabled")
		}
		return tlsCfg.HTTPClient(30 * time.Second)
	}
	return &http.Client{Timeout: 30 * time.Second}, nil
}

func findBinary(dir, name string) string {
	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	candidate = filepath.Join(dir, "examples", name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	path, err := exec.LookPath(name)
	if err == nil {
		return path
	}
	fmt.Fprintf(os.Stderr, "warning: plugin binary %q not found in %s or PATH\n", name, dir)
	return ""
}
