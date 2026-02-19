package mcpbridge

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// RegisterPrompts registers all MCP prompts against the given controller.
func RegisterPrompts(server *mcp.Server, _ ui.Controller) {
	server.AddPrompt(
		&mcp.Prompt{
			Name:        "explore-workspace",
			Description: "Overview of the workspace: name, components, templates, and tree structure",
		},
		func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: "Explore the zhi workspace",
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: "Read the following resources to understand this zhi workspace:\n" +
						"- zhi://workspace/name (workspace identity)\n" +
						"- zhi://components (available components and their state)\n" +
						"- zhi://export/templates (available export templates)\n" +
						"- zhi://tree (full configuration tree)\n\n" +
						"Summarize the workspace configuration: what components exist, which are enabled, " +
						"what export templates are available, and highlight any notable configuration values."}},
				},
			}, nil
		},
	)

	server.AddPrompt(
		&mcp.Prompt{
			Name:        "review-config",
			Description: "Review configuration for issues, optionally filtered by path prefix",
			Arguments: []*mcp.PromptArgument{
				{Name: "path", Description: "Optional path prefix to filter the review scope", Required: false},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			path := req.Params.Arguments["path"]
			treeRef := "zhi://tree"
			if path != "" {
				treeRef = fmt.Sprintf("zhi://tree/%s", path)
			}
			return &mcp.GetPromptResult{
				Description: "Review zhi configuration for issues",
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: fmt.Sprintf(
						"Review the zhi configuration for issues:\n\n"+
							"1. Read %s to see the current configuration values\n"+
							"2. Read zhi://validation to see all validation results\n"+
							"3. Analyze the results grouped by severity (blocking, warning, info)\n"+
							"4. For each issue, explain the cause and suggest a fix using the set_value tool\n"+
							"5. Prioritize blocking issues first",
						treeRef,
					)}},
				},
			}, nil
		},
	)

	server.AddPrompt(
		&mcp.Prompt{
			Name:        "apply-changes",
			Description: "Walk through validating, saving, exporting, and applying configuration changes",
			Arguments: []*mcp.PromptArgument{
				{Name: "target", Description: "Optional apply target name", Required: false},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			target := req.Params.Arguments["target"]
			applyStep := "5. Call the `apply` tool to apply the configuration"
			if target != "" {
				applyStep = fmt.Sprintf("5. Call the `apply` tool with target %q to apply the configuration", target)
			}
			return &mcp.GetPromptResult{
				Description: "Apply configuration changes",
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: fmt.Sprintf(
						"Apply configuration changes step by step:\n\n"+
							"1. Read zhi://validation to check for issues\n"+
							"2. If there are blocking issues, stop and report them — do not proceed\n"+
							"3. Call the `save` tool to persist the current tree\n"+
							"4. Read zhi://export/templates to see available exports, then call `export` for each\n"+
							"%s\n"+
							"6. Report the result of each step",
						applyStep,
					)}},
				},
			}, nil
		},
	)

	server.AddPrompt(
		&mcp.Prompt{
			Name:        "setup-component",
			Description: "Guide through enabling and configuring a component",
			Arguments: []*mcp.PromptArgument{
				{Name: "component", Description: "Component name to set up", Required: true},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			component := req.Params.Arguments["component"]
			return &mcp.GetPromptResult{
				Description: fmt.Sprintf("Set up component %q", component),
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: fmt.Sprintf(
						"Set up the %q component:\n\n"+
							"1. Read zhi://components to find the component and check its current state\n"+
							"2. If it is disabled, call `enable_component` with name %q\n"+
							"3. Read zhi://tree to see the paths belonging to this component\n"+
							"4. For each path that has no value set, suggest a reasonable default\n"+
							"5. Ask me for confirmation before setting any values with `set_value`",
						component, component,
					)}},
				},
			}, nil
		},
	)

	server.AddPrompt(
		&mcp.Prompt{
			Name:        "audit-validation",
			Description: "Audit validation results with remediation suggestions",
			Arguments: []*mcp.PromptArgument{
				{Name: "severity", Description: "Filter by severity: info, warning, or blocking", Required: false},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			severity := req.Params.Arguments["severity"]
			filter := ""
			if severity != "" {
				filter = fmt.Sprintf(" Filter results to %q severity only.", severity)
			}
			return &mcp.GetPromptResult{
				Description: "Audit configuration validation",
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: fmt.Sprintf(
						"Audit the zhi configuration validation results:\n\n"+
							"1. Read zhi://validation to get all validation results%s\n"+
							"2. Group results by severity (blocking, warning, info)\n"+
							"3. For each issue: explain the cause, assess the risk, and provide "+
							"a specific fix using the `set_value` tool\n"+
							"4. Prioritize blocking issues — these prevent saving and applying",
						filter,
					)}},
				},
			}, nil
		},
	)
}
