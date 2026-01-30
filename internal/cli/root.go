// Package cli implements the zhi command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"

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
It uses an extensible plugin system with config, transform, and store providers.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", ".", "workspace directory")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
}

// Execute runs the root command. It is called from main().
func Execute() error {
	return rootCmd.Execute()
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

	eng, err := core.NewEngine(reg, ws)
	if err != nil {
		return fmt.Errorf("initializing engine: %w", err)
	}

	// Load component state if available.
	statePath := stateFilePath(ws.Dir)
	if data, err := os.ReadFile(statePath); err == nil {
		state, err := parseComponentState(data)
		if err == nil {
			eng.Components().LoadState(state)
		}
	}

	ctx := context.WithValue(cmd.Context(), engineKey, eng)
	ctx = context.WithValue(ctx, registryKey, reg)
	cmd.SetContext(ctx)
	return nil
}

// withRegistry is a PersistentPreRunE that only initializes the registry
// (no workspace needed).
func withRegistry(cmd *cobra.Command, _ []string) error {
	reg := core.DefaultRegistry()
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
