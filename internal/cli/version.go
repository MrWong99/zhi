package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information set via SetVersionInfo before Execute.
var (
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

// SetVersionInfo sets the version metadata displayed by the version command.
// It should be called from main before Execute, with values injected via
// ldflags at build time.
func SetVersionInfo(version, commit, date string) {
	if version != "" {
		appVersion = version
	}
	if commit != "" {
		appCommit = commit
	}
	if date != "" {
		appDate = date
	}
}

// VersionString returns the formatted version string.
func VersionString() string {
	return fmt.Sprintf("zhi %s (commit: %s, built: %s)", appVersion, appCommit, appDate)
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  "Display the version, commit hash, and build date of the zhi binary.",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println(VersionString())
	},
}
