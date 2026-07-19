package config

import (
	"context"
	"encoding/json"
	"fmt"

	pb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
)

// GRPCServer implements the proto ConfigServiceServer by delegating to a
// Plugin implementation.
type GRPCServer struct {
	pb.UnimplementedConfigServiceServer
	Impl Plugin
}

func (s *GRPCServer) List(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	paths, err := s.Impl.List(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListResponse{Paths: paths}, nil
}

func (s *GRPCServer) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	v, ok, err := s.Impl.Get(ctx, req.GetPath())
	if err != nil {
		return nil, err
	}
	if !ok {
		return &pb.GetResponse{Found: false}, nil
	}
	valJSON, metaJSON, err := ValueToProto(v)
	if err != nil {
		return nil, err
	}
	return &pb.GetResponse{
		Found:        true,
		ValueJson:    valJSON,
		MetadataJson: metaJSON,
	}, nil
}

func (s *GRPCServer) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	v, err := ValueFromProto(req.GetValueJson(), req.GetMetadataJson())
	if err != nil {
		return nil, err
	}
	if err := s.Impl.Set(ctx, req.GetPath(), v); err != nil {
		return nil, err
	}
	return &pb.SetResponse{}, nil
}

func (s *GRPCServer) Validate(ctx context.Context, req *pb.ValidateRequest) (*pb.ValidateResponse, error) {
	tree, err := TreeFromProto(req.GetTree())
	if err != nil {
		return nil, err
	}
	results, err := s.Impl.Validate(ctx, req.GetPath(), tree)
	if err != nil {
		return nil, err
	}
	msgs := make([]*pb.ValidationResultMsg, 0, len(results))
	for _, r := range results {
		m := &pb.ValidationResultMsg{
			Severity: int32(r.Severity),
			Message:  r.Message,
		}
		if r.Metadata != nil {
			metaJSON, err := json.Marshal(r.Metadata)
			if err != nil {
				return nil, fmt.Errorf("marshaling validation result metadata: %w", err)
			}
			m.MetadataJson = metaJSON
		}
		msgs = append(msgs, m)
	}
	return &pb.ValidateResponse{Results: msgs}, nil
}

// TreeFromProto reconstructs a Tree from the proto TreeEntry slice received
// over the wire. Path validation is skipped because the entries were
// validated when they were originally Set. An error is returned if any entry
// cannot be decoded, so callers do not silently proceed with a partial tree.
func TreeFromProto(entries []*pb.TreeEntry) (*Tree, error) {
	t := NewTree()
	for _, e := range entries {
		v, err := ValueFromProtoVersioned(e.GetValueJson(), e.GetMetadataJson(), e.GetVersion())
		if err != nil {
			return nil, fmt.Errorf("decoding tree entry %q: %w", e.GetPath(), err)
		}
		// Paths were validated at Set time; skip re-validation to avoid
		// rejecting values that are already in the tree.
		t.values[e.GetPath()] = &v
	}
	return t, nil
}
