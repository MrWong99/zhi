package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

const vaultAppLabelPrefix = "vault.app."

// scanApps extracts all unique app names from vault.app.* labels in the values map.
func scanApps(values map[string]config.Value) map[string][]string {
	apps := make(map[string][]string)
	for path, val := range values {
		if val.Metadata == nil {
			continue
		}
		for key := range val.Metadata {
			if strings.HasPrefix(key, vaultAppLabelPrefix) {
				appName := key[len(vaultAppLabelPrefix):]
				if appName != "" {
					apps[appName] = append(apps[appName], path)
				}
			}
		}
	}
	return apps
}

// parseCapabilities converts a comma-separated capability string to Vault capabilities.
func parseCapabilities(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]bool)
	var caps []string

	for _, p := range parts {
		switch strings.TrimSpace(p) {
		case "read":
			if !seen["read"] {
				caps = append(caps, "read")
				seen["read"] = true
			}
		case "write":
			if !seen["create"] {
				caps = append(caps, "create")
				seen["create"] = true
			}
			if !seen["update"] {
				caps = append(caps, "update")
				seen["update"] = true
			}
		case "delete":
			if !seen["delete"] {
				caps = append(caps, "delete")
				seen["delete"] = true
			}
		}
	}
	return caps
}

// metadataCapabilities returns the Vault capabilities needed for the metadata path.
func metadataCapabilities(dataCaps []string) []string {
	if slices.Contains(dataCaps, "read") {
		return []string{"read", "list"}
	}
	return nil
}

// buildPolicyHCL generates Vault HCL policy for an app based on vault.app.{name} labels.
func buildPolicyHCL(appName string, values map[string]config.Value, mount, prefix, treeID string) string {
	labelKey := vaultAppLabelPrefix + appName

	type pathRule struct {
		path string
		caps []string
	}
	var rules []pathRule

	for configPath, val := range values {
		if val.Metadata == nil {
			continue
		}
		raw, ok := val.Metadata[labelKey]
		if !ok {
			continue
		}
		capStr, ok := raw.(string)
		if !ok {
			continue
		}

		dataCaps := parseCapabilities(capStr)
		if len(dataCaps) == 0 {
			continue
		}

		dataPath := fmt.Sprintf("%s/data/%s/%s/%s", mount, prefix, treeID, configPath)
		rules = append(rules, pathRule{path: dataPath, caps: dataCaps})

		metaCaps := metadataCapabilities(dataCaps)
		if len(metaCaps) > 0 {
			metaPath := fmt.Sprintf("%s/metadata/%s/%s/%s", mount, prefix, treeID, configPath)
			rules = append(rules, pathRule{path: metaPath, caps: metaCaps})
		}
	}

	if len(rules) == 0 {
		return ""
	}

	sort.Slice(rules, func(i, j int) bool { return rules[i].path < rules[j].path })

	var b strings.Builder
	for i, r := range rules {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "path %q {\n", r.path)
		fmt.Fprintf(&b, "  capabilities = [%s]\n", quoteList(r.caps))
		b.WriteString("}\n")
	}

	return b.String()
}

// policyName returns the Vault policy name for an app in a workspace.
func policyName(workspace, appName string) string {
	return fmt.Sprintf("zhi-%s-%s", workspace, appName)
}

func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

