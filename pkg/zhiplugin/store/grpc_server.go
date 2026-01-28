package store

import (
	"context"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/store/proto"
)

// GRPCServer implements the proto StoreServiceServer by delegating to a
// Plugin implementation.
type GRPCServer struct {
	pb.UnimplementedStoreServiceServer
	Impl Plugin
}

func (s *GRPCServer) Save(ctx context.Context, req *pb.SaveRequest) (*pb.SaveResponse, error) {
	tree := config.TreeFromProto(req.GetTree())
	if err := s.Impl.Save(ctx, req.GetId(), tree); err != nil {
		return nil, err
	}
	return &pb.SaveResponse{}, nil
}

func (s *GRPCServer) Load(ctx context.Context, req *pb.LoadRequest) (*pb.LoadResponse, error) {
	tree, found, err := s.Impl.Load(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &pb.LoadResponse{Found: false}, nil
	}
	return &pb.LoadResponse{
		Found: true,
		Tree:  config.TreeToProto(tree),
	}, nil
}

func (s *GRPCServer) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if err := s.Impl.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteResponse{}, nil
}

func (s *GRPCServer) ListTrees(ctx context.Context, _ *pb.ListTreesRequest) (*pb.ListTreesResponse, error) {
	ids, err := s.Impl.ListTrees(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListTreesResponse{Ids: ids}, nil
}

func (s *GRPCServer) SupportsVersioning(ctx context.Context, _ *pb.SupportsVersioningRequest) (*pb.SupportsVersioningResponse, error) {
	supported, err := s.Impl.SupportsVersioning(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.SupportsVersioningResponse{Supported: supported}, nil
}

func (s *GRPCServer) ListVersions(ctx context.Context, req *pb.ListVersionsRequest) (*pb.ListVersionsResponse, error) {
	versions, err := s.Impl.ListVersions(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.ListVersionsResponse{Versions: versions}, nil
}

func (s *GRPCServer) LoadVersion(ctx context.Context, req *pb.LoadVersionRequest) (*pb.LoadVersionResponse, error) {
	tree, found, err := s.Impl.LoadVersion(ctx, req.GetId(), req.GetVersion())
	if err != nil {
		return nil, err
	}
	if !found {
		return &pb.LoadVersionResponse{Found: false}, nil
	}
	return &pb.LoadVersionResponse{
		Found: true,
		Tree:  config.TreeToProto(tree),
	}, nil
}

func (s *GRPCServer) DeleteVersion(ctx context.Context, req *pb.DeleteVersionRequest) (*pb.DeleteVersionResponse, error) {
	if err := s.Impl.DeleteVersion(ctx, req.GetId(), req.GetVersion()); err != nil {
		return nil, err
	}
	return &pb.DeleteVersionResponse{}, nil
}

func (s *GRPCServer) EncryptionStatus(ctx context.Context, _ *pb.EncryptionStatusRequest) (*pb.EncryptionStatusResponse, error) {
	status, err := s.Impl.EncryptionStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.EncryptionStatusResponse{Status: int32(status)}, nil
}

func (s *GRPCServer) InitEncryption(ctx context.Context, req *pb.InitEncryptionRequest) (*pb.InitEncryptionResponse, error) {
	if err := s.Impl.InitEncryption(ctx, req.GetPassphrase()); err != nil {
		return nil, err
	}
	return &pb.InitEncryptionResponse{}, nil
}

func (s *GRPCServer) RotateEncryption(ctx context.Context, req *pb.RotateEncryptionRequest) (*pb.RotateEncryptionResponse, error) {
	if err := s.Impl.RotateEncryption(ctx, req.GetOldPassphrase(), req.GetNewPassphrase()); err != nil {
		return nil, err
	}
	return &pb.RotateEncryptionResponse{}, nil
}
