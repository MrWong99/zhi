package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

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
	cfg, err := parseManagerConfig(opts)
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

	childOpts := childVaultOptions(cfg)
	child, cleanup, err := launch.LaunchStore(vaultBin,
		launch.WithLogger(logger),
		launch.WithIsolatedEnv(map[string]string{
			"VAULT_ADDR": cfg.Addr,
		}),
		launch.WithPluginOptions(childOpts),
	)
	if err != nil {
		logger.Error("failed to launch child vault plugin", "binary", vaultBin, "error", err)
		os.Exit(1)
	}
	defer cleanup()

	admin := newAdminClient(cfg.Addr, "", cfg.Namespace)
	cm := newCredManager(admin, cfg.Mount, cfg.Prefix, cfg.Workspace, logger)

	managerStore := &vaultManagerStore{
		DelegatingPlugin: store.NewDelegatingPlugin(child),
		admin:            admin,
		credManager:      cm,
		apps:             cfg.Apps,
		cfg:              cfg,
		log:              logger,
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
