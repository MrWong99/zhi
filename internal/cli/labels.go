package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "Manage and discover metadata labels",
	Long: `Metadata labels are semantic annotations on configuration values that control
how plugins interpret and handle those values.

Labels follow a namespace convention: <namespace>.<name> where namespace
indicates the plugin type (ui, store, transform, config, core) or a custom
namespace for plugin-specific labels.

Examples:
  ui.readonly       Prevents users from modifying this value in the UI
  ui.password       Masks input characters during entry
  store.writeonly   Value can be written but not read back (for secrets)
  transform.hidden  Transform plugins cannot access this value

Subcommands:
  list    List all available labels
  info    Show detailed info about a specific label`,
	Example: `  zhi labels list
  zhi labels list --namespace ui
  zhi labels info ui.password
  zhi labels list --json`,
}

var (
	labelsJSON      bool
	labelsNamespace string
)

func init() {
	labelsCmd.PersistentFlags().BoolVar(&labelsJSON, "json", false, "output as JSON")

	labelsCmd.AddCommand(labelsListCmd)
	labelsCmd.AddCommand(labelsInfoCmd)
	rootCmd.AddCommand(labelsCmd)
}

// --- labels list ---

var labelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available metadata labels",
	Long: `List all registered metadata labels.

Use --namespace to filter by namespace (ui, store, transform, config, core).
Use --json for machine-readable output.`,
	Example: `  zhi labels list
  zhi labels list --namespace ui
  zhi labels list --namespace store --json`,
	RunE: runLabelsList,
}

func init() {
	labelsListCmd.Flags().StringVar(&labelsNamespace, "namespace", "", "filter by namespace (ui, store, transform, config, core)")
}

type labelJSONEntry struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Description  string   `json:"description"`
	ValueType    string   `json:"value_type"`
	DefaultValue any      `json:"default_value,omitempty"`
	AppliesTo    []string `json:"applies_to,omitempty"`
	Deprecated   string   `json:"deprecated,omitempty"`
}

func runLabelsList(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	registry := labels.DefaultRegistry

	var labelList []*labels.Label
	if labelsNamespace != "" {
		labelList = registry.ListByNamespace(labelsNamespace)
		if len(labelList) == 0 {
			// Check if namespace exists
			found := slices.Contains(registry.Namespaces(), labelsNamespace)
			if !found {
				fmt.Fprintf(w, "Unknown namespace: %s\n", labelsNamespace)
				fmt.Fprintf(w, "Available namespaces: %s\n", strings.Join(registry.Namespaces(), ", "))
				return nil
			}
		}
	} else {
		labelList = registry.List()
	}

	if labelsJSON {
		entries := make([]labelJSONEntry, len(labelList))
		for i, l := range labelList {
			entries[i] = labelJSONEntry{
				Name:         l.Name,
				Namespace:    l.Namespace,
				Description:  l.Description,
				ValueType:    l.ValueType,
				DefaultValue: l.DefaultValue,
				AppliesTo:    l.AppliesTo,
				Deprecated:   l.Deprecated,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	if len(labelList) == 0 {
		fmt.Fprintln(w, "No labels found.")
		return nil
	}

	// Group by namespace for display
	if labelsNamespace == "" {
		// Show all namespaces
		for _, ns := range registry.Namespaces() {
			nsLabels := registry.ListByNamespace(ns)
			if len(nsLabels) == 0 {
				continue
			}
			fmt.Fprintf(w, "\n%s namespace:\n", strings.ToUpper(ns))
			printLabelsTable(w, nsLabels)
		}
	} else {
		printLabelsTable(w, labelList)
	}

	return nil
}

func printLabelsTable(w interface{ Write([]byte) (int, error) }, labelList []*labels.Label) {
	headers := []string{"NAME", "TYPE", "DEFAULT", "DESCRIPTION"}
	var rows [][]string

	for _, l := range labelList {
		defaultStr := "-"
		if l.DefaultValue != nil {
			defaultStr = fmt.Sprintf("%v", l.DefaultValue)
		}

		// Truncate description for table
		desc := l.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}

		deprecated := ""
		if l.Deprecated != "" {
			deprecated = " [DEPRECATED]"
		}

		rows = append(rows, []string{l.Name, l.ValueType, defaultStr, desc + deprecated})
	}

	fprintTable(w, headers, rows)
}

// --- labels info ---

var labelsInfoCmd = &cobra.Command{
	Use:   "info <label-name>",
	Short: "Show detailed info about a metadata label",
	Long: `Show detailed information about a specific metadata label, including:
  - Full description
  - Value type and constraints
  - Default value
  - Which plugin types interpret this label
  - Usage examples`,
	Example: `  zhi labels info ui.readonly
  zhi labels info store.writeonly
  zhi labels info ui.pattern --json`,
	Args: cobra.ExactArgs(1),
	RunE: runLabelsInfo,
}

func runLabelsInfo(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()
	registry := labels.DefaultRegistry

	name := args[0]
	label := registry.Get(name)

	if label == nil {
		fmt.Fprintf(w, "Label not found: %s\n\n", name)

		// Suggest similar labels
		suggestions := findSimilarLabels(registry, name)
		if len(suggestions) > 0 {
			fmt.Fprintln(w, "Did you mean:")
			for _, s := range suggestions {
				fmt.Fprintf(w, "  %s\n", s)
			}
		}
		return nil
	}

	if labelsJSON {
		out := map[string]any{
			"name":        label.Name,
			"namespace":   label.Namespace,
			"description": label.Description,
			"value_type":  label.ValueType,
		}
		if label.DefaultValue != nil {
			out["default_value"] = label.DefaultValue
		}
		if len(label.AppliesTo) > 0 {
			out["applies_to"] = label.AppliesTo
		}
		if label.Constraints != nil {
			out["constraints"] = label.Constraints
		}
		if len(label.Examples) > 0 {
			out["examples"] = label.Examples
		}
		if label.Since != "" {
			out["since"] = label.Since
		}
		if label.Deprecated != "" {
			out["deprecated"] = label.Deprecated
		}

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}

	// Human-readable output
	fmt.Fprintf(w, "Name:        %s\n", label.Name)
	fmt.Fprintf(w, "Namespace:   %s\n", label.Namespace)
	fmt.Fprintf(w, "Type:        %s\n", label.ValueType)
	fmt.Fprintf(w, "Description: %s\n", label.Description)

	if label.DefaultValue != nil {
		fmt.Fprintf(w, "Default:     %v\n", label.DefaultValue)
	}

	if len(label.AppliesTo) > 0 {
		fmt.Fprintf(w, "Applies to:  %s\n", strings.Join(label.AppliesTo, ", "))
	}

	if label.Constraints != nil {
		fmt.Fprintln(w, "\nConstraints:")
		c := label.Constraints
		if c.Pattern != "" {
			fmt.Fprintf(w, "  Pattern:    %s\n", c.Pattern)
		}
		if c.MinLength > 0 {
			fmt.Fprintf(w, "  Min length: %d\n", c.MinLength)
		}
		if c.MaxLength > 0 {
			fmt.Fprintf(w, "  Max length: %d\n", c.MaxLength)
		}
		if len(c.Enum) > 0 {
			fmt.Fprintf(w, "  Allowed:    %s\n", strings.Join(c.Enum, ", "))
		}
		if c.Min != nil {
			fmt.Fprintf(w, "  Min:        %v\n", *c.Min)
		}
		if c.Max != nil {
			fmt.Fprintf(w, "  Max:        %v\n", *c.Max)
		}
		if c.MinItems > 0 {
			fmt.Fprintf(w, "  Min items:  %d\n", c.MinItems)
		}
		if c.MaxItems > 0 {
			fmt.Fprintf(w, "  Max items:  %d\n", c.MaxItems)
		}
	}

	if len(label.Examples) > 0 {
		fmt.Fprintln(w, "\nExamples:")
		for _, ex := range label.Examples {
			valueStr := formatExampleValue(ex.Value)
			fmt.Fprintf(w, "  %s\n", valueStr)
			if ex.Description != "" {
				fmt.Fprintf(w, "    %s\n", ex.Description)
			}
		}
	}

	if label.Since != "" {
		fmt.Fprintf(w, "\nSince: %s\n", label.Since)
	}

	if label.Deprecated != "" {
		fmt.Fprintf(w, "\nDEPRECATED: %s\n", label.Deprecated)
	}

	return nil
}

func formatExampleValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case []string:
		return fmt.Sprintf("%v", val)
	case []any:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func findSimilarLabels(registry *labels.Registry, name string) []string {
	allLabels := registry.List()

	// Prioritized match buckets
	var prefixMatches []string
	var substringMatches []string
	var keyPartMatches []string
	var namespaceMatches []string

	nameLower := strings.ToLower(name)
	parts := strings.SplitN(name, ".", 2)

	for _, l := range allLabels {
		labelLower := strings.ToLower(l.Name)

		// Check for prefix match (highest priority)
		if strings.HasPrefix(labelLower, nameLower) {
			prefixMatches = append(prefixMatches, l.Name)
			continue
		}

		// Check for substring match
		if strings.Contains(labelLower, nameLower) {
			substringMatches = append(substringMatches, l.Name)
			continue
		}

		// Check if searching for just the key part
		if len(parts) == 2 {
			keyPart := strings.ToLower(parts[1])
			if strings.Contains(l.Name, keyPart) {
				keyPartMatches = append(keyPartMatches, l.Name)
				continue
			}
		}

		// Check namespace match (lowest priority)
		if len(parts) >= 1 && l.Namespace == parts[0] {
			namespaceMatches = append(namespaceMatches, l.Name)
		}
	}

	// Combine in priority order
	var suggestions []string
	suggestions = append(suggestions, prefixMatches...)
	suggestions = append(suggestions, substringMatches...)
	suggestions = append(suggestions, keyPartMatches...)
	suggestions = append(suggestions, namespaceMatches...)

	// Limit suggestions
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}
