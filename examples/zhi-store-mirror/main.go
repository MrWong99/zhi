// zhi-store-mirror is an example meta-plugin that demonstrates plugin
// composition. It launches two child store plugins (memory and JSON file)
// and mirrors all writes to both, reading from the primary (memory).
//
// This example shows how to use the SDK composition helpers to build a
// meta-plugin with minimal code.
//
// Usage in zhi.yaml:
//
//	store:
//	  provider: mirror
//	  options:
//	    primary: zhi-store-memory
//	    mirror: zhi-store-json
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
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-store-mirror",
		Level:  level,
		Output: os.Stderr,
	})

	// Run through a helper that uses deferred cleanup so that no os.Exit call
	// bypasses the child-plugin Kill deferreds. Calling os.Exit inside main
	// after launching a child would orphan the already-running child process.
	if err := run(logger); err != nil {
		logger.Error("mirror store plugin failed", "error", err)
		os.Exit(1)
	}
}

func run(logger hclog.Logger) error {
	logger.Info("starting mirror store plugin")

	// Find sibling plugin binaries in the same directory as this binary.
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine own executable path: %w", err)
	}
	dir := filepath.Dir(selfPath)

	primaryBin := findBinary(dir, "zhi-store-memory")
	mirrorBin := findBinary(dir, "zhi-store-json")

	if primaryBin == "" || mirrorBin == "" {
		return fmt.Errorf("required child plugins not found in %s (primary=%q, mirror=%q)", dir, primaryBin, mirrorBin)
	}

	// Launch child plugins. Deferred cleanups ensure both children are killed
	// on every error path once launched.
	primary, cleanupPrimary, err := launch.LaunchStore(primaryBin, launch.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("failed to launch primary store %q: %w", primaryBin, err)
	}
	defer cleanupPrimary()

	mirror, cleanupMirror, err := launch.LaunchStore(mirrorBin, launch.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("failed to launch mirror store %q: %w", mirrorBin, err)
	}
	defer cleanupMirror()

	// Compose using MirroredPlugin.
	composed := store.MirroredPlugin(logger, primary, mirror)

	// Serve the composed plugin to the engine.
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"store": &store.GRPCPlugin{Impl: composed},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
	return nil
}

// findBinary searches for a plugin binary by name. It checks the given
// directory first, then looks for it on PATH.
func findBinary(dir, name string) string {
	// Check same directory as this binary.
	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	// Check subdirectory (e.g., bin/examples/).
	candidate = filepath.Join(dir, "examples", name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	// Fall back to PATH.
	path, err := exec.LookPath(name)
	if err == nil {
		return path
	}

	fmt.Fprintf(os.Stderr, "warning: plugin binary %q not found in %s or PATH\n", name, dir)
	return ""
}
