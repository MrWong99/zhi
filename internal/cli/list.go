package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/internal/core"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace information",
	Long: `List available information about the workspace.

Subcommands:
  providers    List all registered config, transform, and store providers
  trees        List all stored tree IDs
  paths        List all configuration paths in the current tree
  components   List all defined components with their status`,
	Example: `  zhi list providers
  zhi list paths --json
  zhi list trees
  zhi list components`,
}

var listJSON bool

func init() {
	listCmd.PersistentFlags().BoolVar(&listJSON, "json", false, "output as JSON")
	listCmd.AddCommand(listProvidersCmd)
	listCmd.AddCommand(listTreesCmd)
	listCmd.AddCommand(listPathsCmd)
	listCmd.AddCommand(listComponentsCmd)
	rootCmd.AddCommand(listCmd)
}

// --- list providers ---

var listProvidersCmd = &cobra.Command{
	Use:               "providers",
	Short:             "List all registered providers",
	PersistentPreRunE: withRegistry,
	RunE:              runListProviders,
}

type providerJSONEntry struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

func runListProviders(cmd *cobra.Command, _ []string) error {
	reg, err := registryFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	configInfos := reg.ListConfigProviders()
	transformInfos := reg.ListTransformProviders()
	storeInfos := reg.ListStoreProviders()

	if listJSON {
		out := map[string][]providerJSONEntry{
			"config":    toJSONEntries(configInfos),
			"transform": toJSONEntries(transformInfos),
			"store":     toJSONEntries(storeInfos),
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	fmt.Fprintln(w, "Config providers:")
	printProviderInfos(w, configInfos)
	fmt.Fprintln(w, "Transform providers:")
	if len(transformInfos) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	printProviderInfos(w, transformInfos)
	fmt.Fprintln(w, "Store providers:")
	if len(storeInfos) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	printProviderInfos(w, storeInfos)
	return nil
}

func printProviderInfos(w io.Writer, infos []core.ProviderInfo) {
	for _, info := range infos {
		if info.Source == "built-in" {
			fmt.Fprintf(w, "  %-20s (built-in)\n", info.Name)
		} else {
			fmt.Fprintf(w, "  %-20s (external: %s)\n", info.Name, info.Source)
		}
	}
}

func toJSONEntries(infos []core.ProviderInfo) []providerJSONEntry {
	entries := make([]providerJSONEntry, len(infos))
	for i, info := range infos {
		entries[i] = providerJSONEntry{Name: info.Name, Source: info.Source}
	}
	return entries
}

// --- list trees ---

var listTreesCmd = &cobra.Command{
	Use:               "trees",
	Short:             "List all stored tree IDs",
	PersistentPreRunE: withEngine,
	RunE:              runListTrees,
}

func runListTrees(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	ids, err := eng.ListTrees(cmd.Context())
	if err != nil {
		return err
	}

	if listJSON {
		data, err := json.MarshalIndent(ids, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(ids) == 0 {
		fmt.Fprintln(w, "No stored trees.")
		return nil
	}
	for _, id := range ids {
		fmt.Fprintln(w, id)
	}
	return nil
}

// --- list paths ---

var listPathsCmd = &cobra.Command{
	Use:               "paths",
	Short:             "List all config paths in the current tree",
	PersistentPreRunE: withEngine,
	RunE:              runListPaths,
}

func runListPaths(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	tree, err := eng.LoadTree(cmd.Context())
	if err != nil {
		return err
	}

	paths := tree.List()
	sort.Strings(paths)

	if listJSON {
		data, err := json.MarshalIndent(paths, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(paths) == 0 {
		fmt.Fprintln(w, "No config paths.")
		return nil
	}
	for _, p := range paths {
		fmt.Fprintln(w, p)
	}
	return nil
}

// --- list components ---

var listComponentsCmd = &cobra.Command{
	Use:               "components",
	Short:             "List all defined components with status",
	PersistentPreRunE: withEngine,
	RunE:              runListComponents,
}

type componentListEntry struct {
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Mandatory    bool     `json:"mandatory"`
	Dependencies []string `json:"dependencies"`
	Paths        []string `json:"paths,omitempty"`
}

func runListComponents(cmd *cobra.Command, _ []string) error {
	eng, err := engineFromCmd(cmd)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	components := eng.Components().ListComponents()
	defs := eng.Components().Definitions()

	if listJSON {
		entries := make([]componentListEntry, len(components))
		for i, c := range components {
			status := "disabled"
			if c.Enabled {
				status = "enabled"
			}
			var paths []string
			for _, d := range defs {
				if d.Name == c.Name {
					paths = d.Paths
					break
				}
			}
			entries[i] = componentListEntry{
				Name:         c.Name,
				Status:       status,
				Mandatory:    c.Mandatory,
				Dependencies: c.Dependencies,
				Paths:        paths,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(components) == 0 {
		fmt.Fprintln(w, "No components defined.")
		return nil
	}

	headers := []string{"NAME", "STATUS", "MANDATORY", "DEPENDENCIES", "PATHS"}
	var rows [][]string
	for _, c := range components {
		status := "disabled"
		if c.Enabled {
			status = "enabled"
		}
		mandatory := "no"
		if c.Mandatory {
			mandatory = "yes"
		}
		deps := "-"
		if len(c.Dependencies) > 0 {
			deps = strings.Join(c.Dependencies, ", ")
		}
		var paths string
		for _, d := range defs {
			if d.Name == c.Name {
				paths = strings.Join(d.Paths, ", ")
				break
			}
		}
		rows = append(rows, []string{c.Name, status, mandatory, deps, paths})
	}
	fprintTable(w, headers, rows)
	return nil
}
