package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// RegisterTools registers all MCP tools against the given controller.
// When readOnly is true, mutation tools are omitted.
func RegisterTools(server *mcp.Server, ctrl ui.Controller, readOnly bool) {
	// Read-only tools are always registered.
	addReadTools(server, ctrl)
	if !readOnly {
		addWriteTools(server, ctrl)
	}
}

// --- Input types for tools ---

// SetValueInput is the input for the set_value tool.
type SetValueInput struct {
	Path     string         `json:"path" jsonschema:"Config path (slash-delimited, e.g. database/host)"`
	Value    any            `json:"value" jsonschema:"The configuration value (any JSON type)"`
	Metadata map[string]any `json:"metadata,omitempty" jsonschema:"Optional metadata key-value pairs"`
}

// ApplyInput is the input for the apply tool.
type ApplyInput struct {
	Target string `json:"target,omitempty" jsonschema:"The apply target name (empty for default)"`
}

// ExportInput is the input for the export tool.
type ExportInput struct {
	TemplateName string `json:"template_name" jsonschema:"Name of the export template to use"`
	Format       string `json:"format,omitempty" jsonschema:"Output format override (json, yaml, toml, dotenv)"`
	OutputPath   string `json:"output_path,omitempty" jsonschema:"File path to write output (relative to workspace)"`
	Prefix       string `json:"prefix,omitempty" jsonschema:"Path prefix filter"`
	DryRun       bool   `json:"dry_run,omitempty" jsonschema:"If true return content without writing files"`
}

// ComponentInput is the input for enable/disable component tools.
type ComponentInput struct {
	Name string `json:"name" jsonschema:"Component name"`
}

// SearchInput is the input for marketplace_search.
type SearchInput struct {
	Query  string `json:"query" jsonschema:"Free-text search query"`
	Type   string `json:"type,omitempty" jsonschema:"Filter by plugin type (config, transform, store, ui, workspace)"`
	SortBy string `json:"sort_by,omitempty" jsonschema:"Sort order (relevance, downloads, rating, updated)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max results per page"`
	Page   int    `json:"page,omitempty" jsonschema:"1-based page number (first page is 1)"`
}

// InstallInput is the input for marketplace_install.
type InstallInput struct {
	Ref string `json:"ref" jsonschema:"OCI reference to install (e.g. oci://ghcr.io/user/plugin:v1.0)"`
}

// UninstallInput is the input for marketplace_uninstall.
type UninstallInput struct {
	Name string `json:"name" jsonschema:"Plugin name to uninstall"`
	Type string `json:"type" jsonschema:"Plugin type (config, transform, store, ui)"`
}

// UpdateInput is the input for marketplace_update.
type UpdateInput struct {
	Name    string `json:"name" jsonschema:"Plugin name to update"`
	Version string `json:"version,omitempty" jsonschema:"Target version (empty for latest)"`
}

// RateInput is the input for marketplace_rate.
type RateInput struct {
	Type      string `json:"type" jsonschema:"Plugin type"`
	Publisher string `json:"publisher" jsonschema:"Plugin publisher"`
	Name      string `json:"name" jsonschema:"Plugin name"`
	Score     int    `json:"score" jsonschema:"Rating score 1-5"`
	Comment   string `json:"comment,omitempty" jsonschema:"Optional review comment"`
}

// DetailInput is the input for marketplace_detail.
type DetailInput struct {
	Type      string `json:"type" jsonschema:"Plugin type"`
	Publisher string `json:"publisher" jsonschema:"Plugin publisher"`
	Name      string `json:"name" jsonschema:"Plugin name"`
}

// StoreLoginInput is the input for store_login.
type StoreLoginInput struct {
	Method      string            `json:"method" jsonschema:"Authentication method name"`
	Credentials map[string]string `json:"credentials" jsonschema:"Credential key-value pairs"`
}

// --- Tool registration helpers ---

func addReadTools(server *mcp.Server, ctrl ui.Controller) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "reload_tree",
		Description: "Reload the configuration tree from the config provider",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		tree, err := ctrl.LoadTree(ctx)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(treeToMap(tree))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate",
		Description: "Run validation on the current configuration tree and return results",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		results, err := ctrl.Validate(ctx)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(results)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_updates",
		Description: "Check for available plugin updates",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		updates, err := ctrl.CheckUpdates(ctx)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(updates)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketplace_search",
		Description: "Search the plugin marketplace",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, any, error) {
		results, err := ctrl.SearchMarketplace(ctx, ui.MarketplaceQuery{
			Query:   input.Query,
			Type:    input.Type,
			Sort:    input.SortBy,
			PerPage: input.Limit,
			Page:    input.Page,
		})
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(results)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketplace_detail",
		Description: "Get detailed information about a marketplace plugin",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input DetailInput) (*mcp.CallToolResult, any, error) {
		detail, err := ctrl.GetMarketplaceDetail(ctx, input.Type, input.Publisher, input.Name)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(detail)
	})
}

func addWriteTools(server *mcp.Server, ctrl ui.Controller) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_value",
		Description: "Set a configuration value at the given path",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SetValueInput) (*mcp.CallToolResult, any, error) {
		v := config.Value{Val: input.Value, Metadata: input.Metadata}
		if err := ctrl.SetValue(ctx, input.Path, v); err != nil {
			return errResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Value set at %q", input.Path))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "save",
		Description: "Persist the current configuration tree to the store",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if err := ctrl.SaveTree(ctx); err != nil {
			return errResult(err), nil, nil
		}
		return textResult("Tree saved successfully")
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply",
		Description: "Apply the configuration by running the workspace apply command",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(true),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ApplyInput) (*mcp.CallToolResult, any, error) {
		var output strings.Builder
		result, err := ctrl.Apply(ctx, input.Target, func(ev ui.ApplyEvent) {
			fmt.Fprintf(&output, "[%s] %s\n", ev.Stream, ev.Line)
		})
		if err != nil {
			return errResult(err), nil, nil
		}
		msg := output.String()
		if result.Error != "" {
			msg += "\nError: " + result.Error
		}
		msg += fmt.Sprintf("\nExit code: %d", result.ExitCode)
		if result.ExitCode != 0 {
			return errResult(fmt.Errorf("apply failed (exit %d):\n%s", result.ExitCode, msg)), nil, nil
		}
		return textResult(msg)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "export",
		Description: "Export configuration using a named template",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ExportInput) (*mcp.CallToolResult, any, error) {
		templates, err := ctrl.ExportTemplates(ctx)
		if err != nil {
			return errResult(err), nil, nil
		}
		var tmpl *ui.ExportTemplate
		for i := range templates {
			if templates[i].Name == input.TemplateName {
				tmpl = &templates[i]
				break
			}
		}
		if tmpl == nil {
			return errResult(fmt.Errorf("export template %q not found", input.TemplateName)), nil, nil
		}
		req := ui.ExportRequest{
			TemplatePath: tmpl.Template,
			Format:       tmpl.Format,
			OutputPath:   tmpl.Output,
			Prefix:       tmpl.Prefix,
			DryRun:       input.DryRun,
		}
		if input.Format != "" {
			req.Format = input.Format
		}
		if input.OutputPath != "" {
			req.OutputPath = input.OutputPath
		}
		if input.Prefix != "" {
			req.Prefix = input.Prefix
		}
		result, err := ctrl.Export(ctx, req)
		if err != nil {
			return errResult(err), nil, nil
		}
		return textResult(result.Content)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enable_component",
		Description: "Enable a component and its dependencies",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ComponentInput) (*mcp.CallToolResult, any, error) {
		enabled, err := ctrl.EnableComponent(ctx, input.Name)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(map[string]any{
			"enabled": enabled,
			"message": fmt.Sprintf("Enabled %d component(s)", len(enabled)),
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "disable_component",
		Description: "Disable a component",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ComponentInput) (*mcp.CallToolResult, any, error) {
		if err := ctrl.DisableComponent(ctx, input.Name); err != nil {
			return errResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Component %q disabled", input.Name))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketplace_install",
		Description: "Install a plugin from the marketplace via OCI reference",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input InstallInput) (*mcp.CallToolResult, any, error) {
		result, err := ctrl.InstallPlugin(ctx, input.Ref)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(result)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketplace_uninstall",
		Description: "Uninstall a plugin",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input UninstallInput) (*mcp.CallToolResult, any, error) {
		if err := ctrl.UninstallPlugin(ctx, input.Name, input.Type); err != nil {
			return errResult(err), nil, nil
		}
		return textResult(fmt.Sprintf("Plugin %q (%s) uninstalled", input.Name, input.Type))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketplace_update",
		Description: "Update a plugin to the latest or specified version",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateInput) (*mcp.CallToolResult, any, error) {
		result, err := ctrl.UpdatePlugin(ctx, input.Name, input.Version)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(result)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "marketplace_rate",
		Description: "Submit a rating for a marketplace plugin",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RateInput) (*mcp.CallToolResult, any, error) {
		if err := ctrl.RatePlugin(ctx, input.Type, input.Publisher, input.Name, ui.Rating{
			Score:   input.Score,
			Comment: input.Comment,
		}); err != nil {
			return errResult(err), nil, nil
		}
		return textResult("Rating submitted")
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "store_login",
		Description: "Authenticate with the configuration store",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input StoreLoginInput) (*mcp.CallToolResult, any, error) {
		session, err := ctrl.StoreLogin(ctx, input.Method, input.Credentials)
		if err != nil {
			return errResult(err), nil, nil
		}
		return jsonResult(session)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "store_logout",
		Description: "Log out of the configuration store",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if err := ctrl.StoreLogout(ctx); err != nil {
			return errResult(err), nil, nil
		}
		return textResult("Logged out")
	})
}

// --- Result helpers ---

func textResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, nil, nil
}

func errResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		IsError: true,
	}
}

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(err), nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil, nil
}

func boolPtr(b bool) *bool { return &b }

// treeToMap converts a config.Tree to a map suitable for JSON serialization.
func treeToMap(tree *config.Tree) map[string]any {
	result := make(map[string]any)
	for _, path := range tree.List() {
		v, ok := tree.Get(path)
		if !ok {
			continue
		}
		entry := map[string]any{"value": v.Val}
		if len(v.Metadata) > 0 {
			entry["metadata"] = v.Metadata
		}
		result[path] = entry
	}
	return result
}
