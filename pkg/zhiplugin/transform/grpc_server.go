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

func (s *GRPCServer) BeforeDisplay(ctx context.Context, req *pb.TransformRequest) (*pb.TransformResponse, error) {
	return s.transform(ctx, req, s.Impl.BeforeDisplay)
}

func (s *GRPCServer) AfterSave(ctx context.Context, req *pb.TransformRequest) (*pb.TransformResponse, error) {
	return s.transform(ctx, req, s.Impl.AfterSave)
}

// transform is shared logic for both BeforeDisplay and AfterSave: rebuild
// the tree from proto, call the plugin, serialise the result back.
func (s *GRPCServer) transform(
	ctx context.Context,
	req *pb.TransformRequest,
	fn func(context.Context, *config.Tree) error,
) (*pb.TransformResponse, error) {
	tree := config.TreeFromProto(req.GetTree())
	if err := fn(ctx, tree); err != nil {
		return nil, err
	}
	return &pb.TransformResponse{
		Tree: config.TreeToProto(tree),
	}, nil
}

func (s *GRPCServer) ValidatePolicy(ctx context.Context, _ *pb.ValidatePolicyRequest) (*pb.ValidatePolicyResponse, error) {
	policy, err := s.Impl.ValidatePolicy(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ValidatePolicyResponse{Policy: int32(policy)}, nil
}
