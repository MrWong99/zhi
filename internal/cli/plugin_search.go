package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
	"github.com/MrWong99/zhi/pkg/sharing/registry"
)

var pluginSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the marketplace for plugins",
	Long: `Search the zhi marketplace for plugins matching a query string.

Results are displayed in a table with name, type, version, rating,
downloads, and verified badge. Use flags to filter and sort results.

The marketplace URL is read from ~/.zhi/config.yaml under
marketplace.url or defaults to the central marketplace.`,
	Example: `  zhi plugin search ansible
  zhi plugin search "vault store" --type store
  zhi plugin search --type ui --sort rating --verified
  zhi plugin search ansible --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginSearch,
}

var (
	pluginSearchType     string
	pluginSearchSort     string
	pluginSearchVerified bool
	pluginSearchJSON     bool
	pluginSearchLimit    int
)

func init() {
	pluginSearchCmd.Flags().StringVar(&pluginSearchType, "type", "", "filter by plugin type (config, transform, store, ui)")
	pluginSearchCmd.Flags().StringVar(&pluginSearchSort, "sort", "", "sort by: relevance, downloads, rating, updated (default: relevance)")
	pluginSearchCmd.Flags().BoolVar(&pluginSearchVerified, "verified", false, "show only verified plugins")
	pluginSearchCmd.Flags().BoolVar(&pluginSearchJSON, "json", false, "output as JSON")
	pluginSearchCmd.Flags().IntVar(&pluginSearchLimit, "limit", 20, "maximum results")

	pluginCmd.AddCommand(pluginSearchCmd)
}

func runPluginSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	mc, err := newMarketplaceClient()
	if err != nil {
		return err
	}

	opts := marketplace.SearchOptions{
		Type:     pluginSearchType,
		Sort:     pluginSearchSort,
		Verified: pluginSearchVerified,
		Limit:    pluginSearchLimit,
	}

	resp, err := mc.Search(cmd.Context(), query, opts)
	if err != nil {
		return fmt.Errorf("searching marketplace: %w", err)
	}

	w := cmd.OutOrStdout()

	if pluginSearchJSON {
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(resp.Results) == 0 {
		fmt.Fprintln(w, "No plugins found.")
		return nil
	}

	headers := []string{"NAME", "TYPE", "VERSION", "RATING", "DOWNLOADS", "VERIFIED", "DESCRIPTION"}
	var rows [][]string
	for _, r := range resp.Results {
		ratingStr := "-"
		if r.RatingCount > 0 {
			ratingStr = fmt.Sprintf("%.1f (%d)", r.Rating, r.RatingCount)
		}
		dlStr := formatDownloads(r.Downloads)
		verifiedStr := ""
		if r.Verified {
			verifiedStr = "\u2713"
		}
		desc := r.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		rows = append(rows, []string{r.Name, r.Type, r.LatestVersion, ratingStr, dlStr, verifiedStr, desc})
	}
	fprintTable(w, headers, rows)
	return nil
}

// formatDownloads formats a download count for display (e.g. 12450 -> "12.4k").
func formatDownloads(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// newMarketplaceClient creates a marketplace client using config from ~/.zhi/config.yaml.
func newMarketplaceClient() (*marketplace.Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determining home directory: %w", err)
	}
	configPath := filepath.Join(home, ".zhi", "config.yaml")

	regStore, err := registry.NewStore(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	baseURL := regStore.MarketplaceURL()
	if baseURL == "" {
		baseURL = marketplace.DefaultURL
	}

	var opts []marketplace.ClientOption
	if key := regStore.MarketplaceAPIKey(); key != "" {
		opts = append(opts, marketplace.WithAPIKey(key))
	}

	return marketplace.NewClient(baseURL, opts...), nil
}
