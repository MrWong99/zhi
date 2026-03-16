// Package cli implements the zhi command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

// contextKey is used to store engine-related values in the command context.
type contextKey string

const (
	engineKey   contextKey = "engine"
	registryKey contextKey = "registry"
)

var (
	workspace string
	verbose   bool
)

// rootCmd is the top-level cobra command.
var rootCmd = &cobra.Command{
	Use:   "zhi",
	Short: "Security-first configuration management and provisioning",
	Long: `zhi is a security-first platform for configuration management and provisioning.

It uses an extensible plugin system over gRPC with three plugin types:
  - config: provides and validates configuration values
  - transform: mutates configuration before display or after save
  - store: persists configuration trees with optional encryption

Common workflow:
  zhi init          Initialize a new workspace
  zhi edit          Launch the interactive TUI editor
  zhi export        Export configuration using templates or formats
  zhi apply         Run the configured provisioning command
  zhi validate      Check configuration for errors

Use "zhi <command> --help" for more information about a command.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", ".", "workspace directory")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable debug logging to stderr")
}

// Execute runs the root command. It is called from main().
func Execute() error {
	// Set up signal handling for graceful shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rootCmd.SetContext(ctx)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	return err
}

// applyVerbose configures the log level if --verbose was passed. Subcommand
// PersistentPreRunE hooks must call this because Cobra (without
// EnableTraverseRunHooks) only runs the closest persistent hook, skipping
// the root command's PersistentPreRun.
func applyVerbose() {
	if verbose {
		core.SetLogLevel(hclog.Trace)
		core.Logger().Debug("verbose logging enabled")
	}
}

// engineFromCmd returns the Engine stored in the command's context.
// Commands that need the engine should call this in their RunE.
func engineFromCmd(cmd *cobra.Command) (*core.Engine, error) {
	val := cmd.Context().Value(engineKey)
	if val == nil {
		return nil, fmt.Errorf("engine not initialized")
	}
	eng, ok := val.(*core.Engine)
	if !ok {
		return nil, fmt.Errorf("engine has unexpected type")
	}
	return eng, nil
}

// registryFromCmd returns the Registry stored in the command's context.
func registryFromCmd(cmd *cobra.Command) (*core.Registry, error) {
	val := cmd.Context().Value(registryKey)
	if val == nil {
		return nil, fmt.Errorf("registry not initialized")
	}
	reg, ok := val.(*core.Registry)
	if !ok {
		return nil, fmt.Errorf("registry has unexpected type")
	}
	return reg, nil
}

// withEngine is a PersistentPreRunE that loads the workspace config,
// creates a registry and engine, and stores them in the command context.
func withEngine(cmd *cobra.Command, _ []string) error {
	applyVerbose()
	log := core.Logger()

	dir := workspace
	if dir == "" {
		dir = "."
	}

	absDir, err := resolveDir(dir)
	if err != nil {
		return err
	}

	reg := core.DefaultRegistry()

	ws, err := core.LoadWorkspace(absDir)
	if err != nil {
		return fmt.Errorf("loading workspace: %w", err)
	}
	log.Info("workspace loaded", "dir", ws.Dir, "version", ws.Version)

	// Discover external plugins from workspace-configured directories.
	pluginDirs := ws.PluginDirectories()
	log.Debug("discovering external plugins", "directories", pluginDirs)
	_ = reg.RefreshExternal(core.DiscoveryConfig{
		Directories: pluginDirs,
	})

	log.Debug("creating engine",
		"config_provider", ws.Config.Provider,
		"store_provider", ws.Store.Provider,
		"transform_count", len(ws.Transform))
	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		return fmt.Errorf("initializing engine: %w", err)
	}
	log.Info("engine initialized")

	// Register cleanup to kill plugin processes on shutdown.
	// Capture the current context before SetContext to avoid a data race
	// between this goroutine reading cmd.Context() and the main goroutine
	// writing it via cmd.SetContext() below.
	shutdownCtx := cmd.Context()
	go func() {
		<-shutdownCtx.Done()
		log.Info("shutting down engine, killing plugin processes")
		eng.Close()
	}()

	// Load component state if available.
	statePath := stateFilePath(ws.Dir)
	if data, err := os.ReadFile(statePath); err == nil {
		state, err := parseComponentState(data)
		if err == nil {
			eng.Components().LoadState(state)
			log.Debug("component state loaded", "path", statePath)
		} else {
			log.Warn("failed to parse component state", "path", statePath, "error", err)
		}
	}

	ctx := context.WithValue(cmd.Context(), engineKey, eng)
	ctx = context.WithValue(ctx, registryKey, reg)
	cmd.SetContext(ctx)
	return nil
}

// withRegistry is a PersistentPreRunE that only initializes the registry
// (no workspace needed). It still discovers external plugins from the
// default plugin directory.
func withRegistry(cmd *cobra.Command, _ []string) error {
	applyVerbose()
	reg := core.DefaultRegistry()

	// Discover external plugins from default directories.
	_ = reg.RefreshExternal(core.DiscoveryConfig{})

	ctx := context.WithValue(cmd.Context(), registryKey, reg)
	cmd.SetContext(ctx)
	return nil
}

// resolveDir returns the absolute path for the given directory.
func resolveDir(dir string) (string, error) {
	if dir == "" {
		return os.Getwd()
	}
	abs, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if dir == "." {
		return abs, nil
	}
	if dir[0] == '/' {
		return dir, nil
	}
	return abs + "/" + dir, nil
}
