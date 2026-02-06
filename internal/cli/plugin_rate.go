package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
)

var pluginRateCmd = &cobra.Command{
	Use:   "rate <publisher/name> <score>",
	Short: "Rate a plugin on the marketplace",
	Long: `Submit a rating (1-5 stars) for a plugin on the zhi marketplace.

The plugin must be specified as publisher/name (e.g. "zhi-project/ansible-config").
The score must be an integer from 1 to 5. An optional comment can be provided
with --comment.

Requires a marketplace API key configured in ~/.zhi/config.yaml.`,
	Example: `  zhi plugin rate zhi-project/ansible-config 5
  zhi plugin rate zhi-project/ansible-config 4 --comment "Works well but missing docs"`,
	Args: cobra.ExactArgs(2),
	RunE: runPluginRate,
}

var pluginRateComment string

func init() {
	pluginRateCmd.Flags().StringVar(&pluginRateComment, "comment", "", "optional review text (max 2000 characters)")
	pluginCmd.AddCommand(pluginRateCmd)
}

func runPluginRate(cmd *cobra.Command, args []string) error {
	ref := args[0]
	scoreStr := args[1]
	w := cmd.OutOrStdout()

	publisher, name, err := parsePluginRef(ref)
	if err != nil {
		return err
	}

	score, err := strconv.Atoi(scoreStr)
	if err != nil || score < 1 || score > 5 {
		return fmt.Errorf("score must be an integer between 1 and 5")
	}

	if len(pluginRateComment) > 2000 {
		return fmt.Errorf("comment must be at most 2000 characters")
	}

	mc, err := newMarketplaceClient()
	if err != nil {
		return err
	}

	req := marketplace.SubmitRatingRequest{
		Score:   score,
		Comment: pluginRateComment,
	}

	if err := mc.SubmitRating(cmd.Context(), publisher, name, req); err != nil {
		return fmt.Errorf("submitting rating: %w", err)
	}

	stars := strings.Repeat("*", score) + strings.Repeat(" ", 5-score)
	fmt.Fprintf(w, "Rating submitted: [%s] %s/%s\n", stars, publisher, name)
	if pluginRateComment != "" {
		fmt.Fprintf(w, "Comment: %s\n", pluginRateComment)
	}

	return nil
}

// parsePluginRef parses "publisher/name" into its components.
func parsePluginRef(ref string) (publisher, name string, err error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("plugin reference must be in publisher/name format (e.g. \"zhi-project/ansible-config\")")
	}
	return parts[0], parts[1], nil
}
