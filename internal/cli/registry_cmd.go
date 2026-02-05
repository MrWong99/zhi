package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/registry"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage OCI registry authentication",
	Long: `Authenticate with OCI registries for plugin sharing.

Subcommands:
  login     Authenticate with an OCI registry
  logout    Remove stored credentials
  list      List configured registries`,
	Example: `  zhi registry login ghcr.io --username myuser
  zhi registry logout ghcr.io
  zhi registry list`,
}

// --- registry login ---

var registryLoginCmd = &cobra.Command{
	Use:   "login <host>",
	Short: "Authenticate with an OCI registry",
	Long: `Store credentials for an OCI registry. The password/token is read from
stdin if --password is not provided.`,
	Example: `  zhi registry login ghcr.io --username myuser --password mytoken
  echo $GHCR_TOKEN | zhi registry login ghcr.io --username myuser --password-stdin`,
	Args: cobra.ExactArgs(1),
	RunE: runRegistryLogin,
}

var (
	registryLoginUsername  string
	registryLoginPassword  string
	registryLoginPassStdin bool
)

// --- registry logout ---

var registryLogoutCmd = &cobra.Command{
	Use:     "logout <host>",
	Short:   "Remove stored credentials for an OCI registry",
	Example: `  zhi registry logout ghcr.io`,
	Args:    cobra.ExactArgs(1),
	RunE:    runRegistryLogout,
}

// --- registry list ---

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured registries",
	RunE:  runRegistryList,
}

func init() {
	registryLoginCmd.Flags().StringVar(&registryLoginUsername, "username", "", "registry username")
	registryLoginCmd.Flags().StringVar(&registryLoginPassword, "password", "", "registry password or token")
	registryLoginCmd.Flags().BoolVar(&registryLoginPassStdin, "password-stdin", false, "read password from stdin")

	registryCmd.AddCommand(registryLoginCmd)
	registryCmd.AddCommand(registryLogoutCmd)
	registryCmd.AddCommand(registryListCmd)
	rootCmd.AddCommand(registryCmd)
}

func runRegistryLogin(cmd *cobra.Command, args []string) error {
	host := args[0]
	w := cmd.OutOrStdout()

	username := registryLoginUsername
	password := registryLoginPassword

	if registryLoginPassStdin {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			password = strings.TrimSpace(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading password from stdin: %w", err)
		}
	}

	if username == "" {
		return fmt.Errorf("--username is required")
	}
	if password == "" {
		return fmt.Errorf("password is required (use --password or --password-stdin)")
	}

	store, err := newRegistryStore()
	if err != nil {
		return err
	}

	if err := store.Login(host, username, password); err != nil {
		return fmt.Errorf("storing credentials: %w", err)
	}

	fmt.Fprintf(w, "Login succeeded for %s\n", host)
	return nil
}

func runRegistryLogout(cmd *cobra.Command, args []string) error {
	host := args[0]
	w := cmd.OutOrStdout()

	store, err := newRegistryStore()
	if err != nil {
		return err
	}

	if err := store.Logout(host); err != nil {
		return fmt.Errorf("removing credentials: %w", err)
	}

	fmt.Fprintf(w, "Logged out from %s\n", host)
	return nil
}

func runRegistryList(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	store, err := newRegistryStore()
	if err != nil {
		return err
	}

	hosts := store.ListHosts()
	if len(hosts) == 0 {
		fmt.Fprintln(w, "No registries configured.")
		fmt.Fprintln(w, "Login with: zhi registry login <host> --username <user>")
		return nil
	}

	sort.Strings(hosts)
	fmt.Fprintln(w, "Configured registries:")
	for _, h := range hosts {
		u, _, _ := store.GetCredentials(h)
		fmt.Fprintf(w, "  %-30s (user: %s)\n", h, u)
	}
	return nil
}

func newRegistryStore() (*registry.Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determining home directory: %w", err)
	}
	configPath := filepath.Join(home, ".zhi", "config.yaml")
	store, err := registry.NewStore(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading registry config: %w", err)
	}
	return store, nil
}
