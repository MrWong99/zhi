package ui

import (
	"context"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"

	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// GRPCServer implements the proto UIServiceServer by delegating to a
// Plugin implementation. It runs on the plugin (server) side.
//
// When Run is called, it dials the UIControllerService via the broker ID
// provided in the request, wraps it as a Controller, and passes it to the
// Plugin's Run method.
type GRPCServer struct {
	pb.UnimplementedUIServiceServer
	Impl   Plugin
	broker *goplugin.GRPCBroker
}

func (s *GRPCServer) Run(ctx context.Context, req *pb.RunRequest) (*pb.RunResponse, error) {
	// Dial the UIControllerService that the host started via the broker.
	conn, err := s.broker.Dial(req.GetControllerBrokerId())
	if err != nil {
		return nil, fmt.Errorf("connecting to UI controller service: %w", err)
	}
	defer conn.Close()

	controller := &controllerGRPCClient{
		client: pb.NewUIControllerServiceClient(conn),
	}

	if err := s.Impl.Run(ctx, controller); err != nil {
		return nil, err
	}
	return &pb.RunResponse{}, nil
}

func (s *GRPCServer) Capabilities(ctx context.Context, _ *pb.CapabilitiesRequest) (*pb.CapabilitiesResponse, error) {
	caps, err := s.Impl.Capabilities(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.CapabilitiesResponse{
		RequiresTty:         caps.RequiresTTY,
		SupportsMarketplace: caps.SupportsMarketplace,
	}, nil
}
