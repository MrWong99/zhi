package transform

import (
	"context"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/transform/proto"
)

// GRPCServer implements the proto TransformServiceServer by delegating to
// a Plugin implementation.
type GRPCServer struct {
	pb.UnimplementedTransformServiceServer
	Impl Plugin
}

func (s *GRPCServer) BeforeDisplay(ctx context.Context, req *pb.BeforeDisplayRequest) (*pb.BeforeDisplayResponse, error) {
	tree := config.TreeFromProto(req.GetTree())
	if err := s.Impl.BeforeDisplay(ctx, tree); err != nil {
		return nil, err
	}
	entries, err := config.TreeToProto(tree)
	if err != nil {
		return nil, err
	}
	return &pb.BeforeDisplayResponse{
		Tree: entries,
	}, nil
}

func (s *GRPCServer) AfterSave(ctx context.Context, req *pb.AfterSaveRequest) (*pb.AfterSaveResponse, error) {
	tree := config.TreeFromProto(req.GetTree())
	if err := s.Impl.AfterSave(ctx, tree); err != nil {
		return nil, err
	}
	entries, err := config.TreeToProto(tree)
	if err != nil {
		return nil, err
	}
	return &pb.AfterSaveResponse{
		Tree: entries,
	}, nil
}

func (s *GRPCServer) ValidatePolicy(ctx context.Context, _ *pb.ValidatePolicyRequest) (*pb.ValidatePolicyResponse, error) {
	policy, err := s.Impl.ValidatePolicy(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ValidatePolicyResponse{Policy: int32(policy)}, nil
}
