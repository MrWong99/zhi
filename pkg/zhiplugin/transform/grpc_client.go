package transform

import (
	"context"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/transform/proto"
)

// GRPCClient implements Plugin by talking to the gRPC
// TransformServiceClient generated from the proto definition.
type GRPCClient struct {
	client pb.TransformServiceClient
}

func (c *GRPCClient) BeforeDisplay(ctx context.Context, tree *config.Tree) error {
	entries := config.TreeToProto(tree)
	resp, err := c.client.BeforeDisplay(ctx, &pb.BeforeDisplayRequest{Tree: entries})
	if err != nil {
		return err
	}
	applyTree(config.TreeFromProto(resp.GetTree()), tree)
	return nil
}

func (c *GRPCClient) AfterSave(ctx context.Context, tree *config.Tree) error {
	entries := config.TreeToProto(tree)
	resp, err := c.client.AfterSave(ctx, &pb.AfterSaveRequest{Tree: entries})
	if err != nil {
		return err
	}
	applyTree(config.TreeFromProto(resp.GetTree()), tree)
	return nil
}

func (c *GRPCClient) ValidatePolicy(ctx context.Context) (ValidatePolicy, error) {
	resp, err := c.client.ValidatePolicy(ctx, &pb.ValidatePolicyRequest{})
	if err != nil {
		return 0, err
	}
	return ValidatePolicy(resp.GetPolicy()), nil
}

// applyTree replaces all values in dst with those from src. Paths that
// exist in dst but not in src are removed; paths in src but not dst are
// added.
func applyTree(src, dst *config.Tree) {
	// Remove paths that no longer exist in the transformed result.
	for _, p := range dst.List() {
		if _, ok := src.Get(p); !ok {
			dst.Delete(p)
		}
	}
	// Set/overwrite all paths from the transformed result.
	for _, p := range src.List() {
		v, ok := src.GetPtr(p)
		if !ok {
			continue
		}
		// Paths were already validated when originally Set; skip re-validation.
		_ = dst.Set(p, v)
	}
}
