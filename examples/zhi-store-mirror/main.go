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
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/launch"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Find sibling plugin binaries in the same directory as this binary.
	selfPath, err := os.Executable()
	if err != nil {
		logger.Error("cannot determine own executable path", "error", err)
		os.Exit(1)
	}
	dir := filepath.Dir(selfPath)

	primaryBin := findBinary(dir, "zhi-store-memory")
	mirrorBin := findBinary(dir, "zhi-store-json")

	if primaryBin == "" || mirrorBin == "" {
		logger.Error("required child plugins not found",
			"dir", dir,
			"primary", primaryBin,
			"mirror", mirrorBin,
		)
		os.Exit(1)
	}

	// Launch child plugins.
	primary, cleanupPrimary, err := launch.LaunchStore(primaryBin, launch.WithLogger(logger))
	if err != nil {
		logger.Error("failed to launch primary store", "binary", primaryBin, "error", err)
		os.Exit(1)
	}
	defer cleanupPrimary()

	mirror, cleanupMirror, err := launch.LaunchStore(mirrorBin, launch.WithLogger(logger))
	if err != nil {
		logger.Error("failed to launch mirror store", "binary", mirrorBin, "error", err)
		os.Exit(1)
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
	})
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
