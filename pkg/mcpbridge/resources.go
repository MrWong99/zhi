package mcpbridge

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// RegisterResources registers all MCP resources against the given controller.
func RegisterResources(server *mcp.Server, ctrl ui.Controller) {
	addStaticResources(server, ctrl)
	addResourceTemplates(server, ctrl)
}

func addStaticResources(server *mcp.Server, ctrl ui.Controller) {
	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://workspace/name",
			Name:        "workspace-name",
			Description: "Current workspace name",
			MIMEType:    "text/plain",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			name, err := ctrl.WorkspaceName(ctx)
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:  req.Params.URI,
					Text: name,
				}},
			}, nil
		},
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://tree",
			Name:        "tree",
			Description: "Full configuration tree with all paths and values",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			tree, err := ctrl.LoadTree(ctx)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(treeToMap(tree), "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://components",
			Name:        "components",
			Description: "All components with their enabled/disabled state, paths, and dependencies",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			components, err := ctrl.ListComponents(ctx)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(components, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://validation",
			Name:        "validation",
			Description: "Current validation results for the configuration tree",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			results, err := ctrl.Validate(ctx)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://export/templates",
			Name:        "export-templates",
			Description: "Available export template definitions from the workspace",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			templates, err := ctrl.ExportTemplates(ctx)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(templates, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://marketplace/installed",
			Name:        "marketplace-installed",
			Description: "All locally installed plugins with metadata",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			plugins, err := ctrl.ListInstalledPlugins(ctx)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(plugins, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://store/auth/status",
			Name:        "store-auth-status",
			Description: "Current store authentication status",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			session, err := ctrl.StoreAuthStatus(ctx)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(session, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	server.AddResource(
		&mcp.Resource{
			URI:         "zhi://store/auth/methods",
			Name:        "store-auth-methods",
			Description: "Available authentication methods for the store",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			methods, err := ctrl.StoreAuthMethods(ctx)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(methods, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)
}

func addResourceTemplates(server *mcp.Server, ctrl ui.Controller) {
	// zhi://tree/{+path} — single config value by path.
	// Uses RFC 6570 reserved expansion {+path} to handle multi-segment
	// paths like "database/host".
	server.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: "zhi://tree/{+path}",
			Name:        "tree-value",
			Description: "A single configuration value at the given path",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			path := extractPathFromTreeURI(req.Params.URI)
			if path == "" {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			tree, err := ctrl.LoadTree(ctx)
			if err != nil {
				return nil, err
			}
			v, ok := tree.Get(path)
			if !ok {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			entry := map[string]any{"path": path, "value": v.Val}
			if len(v.Metadata) > 0 {
				entry["metadata"] = v.Metadata
			}
			data, err := json.MarshalIndent(entry, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	// zhi://export/{name} — preview of an export template (dry-run).
	server.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: "zhi://export/{name}",
			Name:        "export-preview",
			Description: "Rendered export content for the named template (dry-run)",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			name := extractExportName(req.Params.URI)
			if name == "" {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			templates, err := ctrl.ExportTemplates(ctx)
			if err != nil {
				return nil, err
			}
			var tmpl *ui.ExportTemplate
			for i := range templates {
				if templates[i].Name == name {
					tmpl = &templates[i]
					break
				}
			}
			if tmpl == nil {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			result, err := ctrl.Export(ctx, ui.ExportRequest{
				TemplatePath: tmpl.Template,
				Format:       tmpl.Format,
				OutputPath:   tmpl.Output,
				Prefix:       tmpl.Prefix,
				DryRun:       true,
			})
			if err != nil {
				return nil, err
			}
			mime := mimeForFormat(tmpl.Format)
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: mime,
					Text:     result.Content,
				}},
			}, nil
		},
	)

	// zhi://marketplace/{type}/{publisher}/{name} — marketplace plugin detail.
	server.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: "zhi://marketplace/{type}/{publisher}/{name}",
			Name:        "marketplace-detail",
			Description: "Detailed information about a marketplace plugin",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			pType, publisher, pName := extractMarketplaceParts(req.Params.URI)
			if pType == "" || publisher == "" || pName == "" {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			detail, err := ctrl.GetMarketplaceDetail(ctx, pType, publisher, pName)
			if err != nil {
				return nil, err
			}
			data, err := json.MarshalIndent(detail, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)
}

// --- URI parsing helpers ---

// extractPathFromTreeURI extracts the config path from "zhi://tree/some/path".
func extractPathFromTreeURI(uri string) string {
	const prefix = "zhi://tree/"
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	return strings.TrimPrefix(uri, prefix)
}

// extractExportName extracts the template name from "zhi://export/name".
func extractExportName(uri string) string {
	const prefix = "zhi://export/"
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	name := strings.TrimPrefix(uri, prefix)
	// Exclude the static "templates" resource.
	if name == "templates" {
		return ""
	}
	return name
}

// extractMarketplaceParts extracts type/publisher/name from
// "zhi://marketplace/{type}/{publisher}/{name}".
func extractMarketplaceParts(uri string) (pType, publisher, name string) {
	const prefix = "zhi://marketplace/"
	if !strings.HasPrefix(uri, prefix) {
		return "", "", ""
	}
	rest := strings.TrimPrefix(uri, prefix)
	// Skip the static "installed" resource.
	if rest == "installed" {
		return "", "", ""
	}
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

func mimeForFormat(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return "application/json"
	case "yaml", "yml":
		return "application/yaml"
	case "toml":
		return "application/toml"
	case "dotenv", "env":
		return "text/plain"
	default:
		return "text/plain"
	}
}
