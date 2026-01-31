package ui

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"

	cfgpb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// controllerGRPCServer implements pb.UIControllerServiceServer. It runs
// on the host side, started via the GRPCBroker, and delegates all calls
// to a Controller implementation backed by the engine.
type controllerGRPCServer struct {
	pb.UnimplementedUIControllerServiceServer
	impl Controller
}

func (s *controllerGRPCServer) LoadTree(ctx context.Context, _ *pb.CtrlLoadTreeRequest) (*pb.CtrlLoadTreeResponse, error) {
	tree, err := s.impl.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	entries := treeToProto(tree)
	return &pb.CtrlLoadTreeResponse{Tree: entries}, nil
}

func (s *controllerGRPCServer) SetValue(ctx context.Context, req *pb.CtrlSetValueRequest) (*pb.CtrlSetValueResponse, error) {
	v, err := valueFromProto(req.GetValueJson(), req.GetMetadataJson())
	if err != nil {
		return nil, err
	}
	if err := s.impl.SetValue(ctx, req.GetPath(), v); err != nil {
		return nil, err
	}
	return &pb.CtrlSetValueResponse{}, nil
}

func (s *controllerGRPCServer) Validate(ctx context.Context, _ *pb.CtrlValidateRequest) (*pb.CtrlValidateResponse, error) {
	results, err := s.impl.Validate(ctx)
	if err != nil {
		return nil, err
	}
	msgs := make([]*cfgpb.ValidationResultMsg, 0, len(results))
	for _, r := range results {
		m := &cfgpb.ValidationResultMsg{
			Severity: int32(r.Severity),
			Message:  r.Message,
		}
		if r.Metadata != nil {
			m.MetadataJson, _ = json.Marshal(r.Metadata)
		}
		msgs = append(msgs, m)
	}
	return &pb.CtrlValidateResponse{Results: msgs}, nil
}

func (s *controllerGRPCServer) SaveTree(ctx context.Context, req *pb.CtrlSaveTreeRequest) (*pb.CtrlSaveTreeResponse, error) {
	if err := s.impl.SaveTree(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.CtrlSaveTreeResponse{}, nil
}

func (s *controllerGRPCServer) ExportTemplates(ctx context.Context, _ *pb.CtrlExportTemplatesRequest) (*pb.CtrlExportTemplatesResponse, error) {
	templates, err := s.impl.ExportTemplates(ctx)
	if err != nil {
		return nil, err
	}
	msgs := make([]*pb.CtrlExportTemplateMsg, 0, len(templates))
	for _, t := range templates {
		msgs = append(msgs, &pb.CtrlExportTemplateMsg{
			Name:     t.Name,
			Template: t.Template,
			Format:   t.Format,
			Output:   t.Output,
			Prefix:   t.Prefix,
		})
	}
	return &pb.CtrlExportTemplatesResponse{Templates: msgs}, nil
}

func (s *controllerGRPCServer) Export(ctx context.Context, req *pb.CtrlExportRequest) (*pb.CtrlExportResponse, error) {
	result, err := s.impl.Export(ctx, ExportRequest{
		TemplatePath: req.GetTemplatePath(),
		Format:       req.GetFormat(),
		OutputPath:   req.GetOutputPath(),
		Prefix:       req.GetPrefix(),
		DryRun:       req.GetDryRun(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.CtrlExportResponse{
		Name:       result.Name,
		Content:    result.Content,
		OutputPath: result.OutputPath,
	}, nil
}

func (s *controllerGRPCServer) Apply(req *pb.CtrlApplyRequest, stream grpc.ServerStreamingServer[pb.CtrlApplyResponse]) error {
	ctx := stream.Context()
	handler := func(event ApplyEvent) {
		_ = stream.Send(&pb.CtrlApplyResponse{
			Done:   false,
			Line:   event.Line,
			Stream: event.Stream,
		})
	}

	result, err := s.impl.Apply(ctx, req.GetTarget(), handler)

	// Send the final result message.
	finalMsg := &pb.CtrlApplyResponse{Done: true}
	if result != nil {
		finalMsg.ExitCode = int32(result.ExitCode)
		finalMsg.Error = result.Error
	}
	if err != nil {
		finalMsg.Error = err.Error()
	}
	_ = stream.Send(finalMsg)

	return nil
}

func (s *controllerGRPCServer) ListComponents(ctx context.Context, _ *pb.CtrlListComponentsRequest) (*pb.CtrlListComponentsResponse, error) {
	components, err := s.impl.ListComponents(ctx)
	if err != nil {
		return nil, err
	}
	msgs := make([]*pb.CtrlComponentInfoMsg, 0, len(components))
	for _, c := range components {
		msgs = append(msgs, &pb.CtrlComponentInfoMsg{
			Name:         c.Name,
			Description:  c.Description,
			Enabled:      c.Enabled,
			Mandatory:    c.Mandatory,
			Paths:        c.Paths,
			Dependencies: c.Dependencies,
		})
	}
	return &pb.CtrlListComponentsResponse{Components: msgs}, nil
}

func (s *controllerGRPCServer) EnableComponent(ctx context.Context, req *pb.CtrlEnableComponentRequest) (*pb.CtrlEnableComponentResponse, error) {
	enabled, err := s.impl.EnableComponent(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return &pb.CtrlEnableComponentResponse{Enabled: enabled}, nil
}

func (s *controllerGRPCServer) DisableComponent(ctx context.Context, req *pb.CtrlDisableComponentRequest) (*pb.CtrlDisableComponentResponse, error) {
	if err := s.impl.DisableComponent(ctx, req.GetName()); err != nil {
		return nil, err
	}
	return &pb.CtrlDisableComponentResponse{}, nil
}

func (s *controllerGRPCServer) WorkspaceName(ctx context.Context, _ *pb.CtrlWorkspaceNameRequest) (*pb.CtrlWorkspaceNameResponse, error) {
	name, err := s.impl.WorkspaceName(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.CtrlWorkspaceNameResponse{Name: name}, nil
}
