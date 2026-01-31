package ui

import (
	"context"
	"encoding/json"
	"io"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// controllerGRPCClient implements Controller by calling the
// UIControllerServiceClient. It runs on the plugin side, connected to
// the host via the GRPCBroker.
type controllerGRPCClient struct {
	client pb.UIControllerServiceClient
}

func (c *controllerGRPCClient) LoadTree(ctx context.Context) (*config.Tree, error) {
	resp, err := c.client.LoadTree(ctx, &pb.CtrlLoadTreeRequest{})
	if err != nil {
		return nil, err
	}
	return treeFromProto(resp.GetTree()), nil
}

func (c *controllerGRPCClient) SetValue(ctx context.Context, path string, value config.Value) error {
	valJSON, err := json.Marshal(value.Val)
	if err != nil {
		return err
	}
	var metaJSON []byte
	if value.Metadata != nil {
		metaJSON, err = json.Marshal(value.Metadata)
		if err != nil {
			return err
		}
	}
	_, err = c.client.SetValue(ctx, &pb.CtrlSetValueRequest{
		Path:         path,
		ValueJson:    valJSON,
		MetadataJson: metaJSON,
	})
	return err
}

func (c *controllerGRPCClient) Validate(ctx context.Context) ([]config.ValidationResult, error) {
	resp, err := c.client.Validate(ctx, &pb.CtrlValidateRequest{})
	if err != nil {
		return nil, err
	}
	results := make([]config.ValidationResult, 0, len(resp.GetResults()))
	for _, m := range resp.GetResults() {
		r := config.ValidationResult{
			Severity: config.Severity(m.GetSeverity()),
			Message:  m.GetMessage(),
		}
		if len(m.GetMetadataJson()) > 0 {
			if err := json.Unmarshal(m.GetMetadataJson(), &r.Metadata); err != nil {
				return nil, err
			}
		}
		results = append(results, r)
	}
	return results, nil
}

func (c *controllerGRPCClient) SaveTree(ctx context.Context, id string) error {
	_, err := c.client.SaveTree(ctx, &pb.CtrlSaveTreeRequest{Id: id})
	return err
}

func (c *controllerGRPCClient) ExportTemplates(ctx context.Context) ([]ExportTemplate, error) {
	resp, err := c.client.ExportTemplates(ctx, &pb.CtrlExportTemplatesRequest{})
	if err != nil {
		return nil, err
	}
	templates := make([]ExportTemplate, 0, len(resp.GetTemplates()))
	for _, m := range resp.GetTemplates() {
		templates = append(templates, ExportTemplate{
			Name:     m.GetName(),
			Template: m.GetTemplate(),
			Format:   m.GetFormat(),
			Output:   m.GetOutput(),
			Prefix:   m.GetPrefix(),
		})
	}
	return templates, nil
}

func (c *controllerGRPCClient) Export(ctx context.Context, req ExportRequest) (*ExportResult, error) {
	resp, err := c.client.Export(ctx, &pb.CtrlExportRequest{
		TemplatePath: req.TemplatePath,
		Format:       req.Format,
		OutputPath:   req.OutputPath,
		Prefix:       req.Prefix,
		DryRun:       req.DryRun,
	})
	if err != nil {
		return nil, err
	}
	return &ExportResult{
		Name:       resp.GetName(),
		Content:    resp.GetContent(),
		OutputPath: resp.GetOutputPath(),
	}, nil
}

func (c *controllerGRPCClient) Apply(ctx context.Context, target string, handler func(ApplyEvent)) (*ApplyResult, error) {
	stream, err := c.client.Apply(ctx, &pb.CtrlApplyRequest{Target: target})
	if err != nil {
		return nil, err
	}

	var result *ApplyResult
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if resp.GetDone() {
			result = &ApplyResult{
				ExitCode: int(resp.GetExitCode()),
				Error:    resp.GetError(),
			}
			break
		}
		if handler != nil {
			handler(ApplyEvent{
				Line:   resp.GetLine(),
				Stream: resp.GetStream(),
			})
		}
	}
	if result == nil {
		result = &ApplyResult{}
	}
	return result, nil
}

func (c *controllerGRPCClient) ListComponents(ctx context.Context) ([]ComponentInfo, error) {
	resp, err := c.client.ListComponents(ctx, &pb.CtrlListComponentsRequest{})
	if err != nil {
		return nil, err
	}
	components := make([]ComponentInfo, 0, len(resp.GetComponents()))
	for _, m := range resp.GetComponents() {
		components = append(components, ComponentInfo{
			Name:         m.GetName(),
			Description:  m.GetDescription(),
			Enabled:      m.GetEnabled(),
			Mandatory:    m.GetMandatory(),
			Paths:        m.GetPaths(),
			Dependencies: m.GetDependencies(),
		})
	}
	return components, nil
}

func (c *controllerGRPCClient) EnableComponent(ctx context.Context, name string) ([]string, error) {
	resp, err := c.client.EnableComponent(ctx, &pb.CtrlEnableComponentRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return resp.GetEnabled(), nil
}

func (c *controllerGRPCClient) DisableComponent(ctx context.Context, name string) error {
	_, err := c.client.DisableComponent(ctx, &pb.CtrlDisableComponentRequest{Name: name})
	return err
}

func (c *controllerGRPCClient) WorkspaceName(ctx context.Context) (string, error) {
	resp, err := c.client.WorkspaceName(ctx, &pb.CtrlWorkspaceNameRequest{})
	if err != nil {
		return "", err
	}
	return resp.GetName(), nil
}
