package ui

import (
	"context"
	"fmt"
	"sync/atomic"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pb "github.com/MrWong99/zhi/pkg/zhiplugin/ui/proto"
)

// GRPCClient implements Plugin by talking to the gRPC UIServiceClient
// generated from the proto definition. It is created on the host side
// when dispensing a "ui" plugin via hashicorp/go-plugin.
//
// When Run is called, GRPCClient starts a UIControllerService server on
// the broker so the plugin can call back into the core.
type GRPCClient struct {
	client pb.UIServiceClient
	broker *goplugin.GRPCBroker
}

func (c *GRPCClient) Run(ctx context.Context, controller Controller) error {
	// Start a UIControllerService gRPC server via the broker so the
	// plugin process can call back into the host.
	controllerServer := &controllerGRPCServer{impl: controller}

	// serverFunc runs on the broker's accept goroutine, so the server
	// pointer must be published atomically for the read after Run returns.
	var s atomic.Pointer[grpc.Server]
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		srv := grpc.NewServer(opts...)
		pb.RegisterUIControllerServiceServer(srv, controllerServer)
		s.Store(srv)
		return srv
	}

	brokerID := c.broker.NextId()
	go c.broker.AcceptAndServe(brokerID, serverFunc)

	_, err := c.client.Run(ctx, &pb.RunRequest{
		ControllerBrokerId: brokerID,
	})

	if srv := s.Load(); srv != nil {
		srv.Stop()
	}

	return err
}

func (c *GRPCClient) Capabilities(ctx context.Context) (Capabilities, error) {
	resp, err := c.client.Capabilities(ctx, &pb.CapabilitiesRequest{})
	if err != nil {
		return Capabilities{}, fmt.Errorf("querying UI capabilities: %w", err)
	}
	return Capabilities{
		RequiresTTY:         resp.GetRequiresTty(),
		SupportsMarketplace: resp.GetSupportsMarketplace(),
	}, nil
}
