package store

import (
	"context"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	pb "github.com/MrWong99/zhi/pkg/zhiplugin/store/proto"
)

// GRPCClient implements Plugin by talking to the gRPC
// StoreServiceClient generated from the proto definition.
type GRPCClient struct {
	client pb.StoreServiceClient
}

func (c *GRPCClient) Save(ctx context.Context, id string, tree config.TreeReader) error {
	entries := config.TreeToProto(tree)
	_, err := c.client.Save(ctx, &pb.SaveRequest{
		Id:   id,
		Tree: entries,
	})
	return err
}

func (c *GRPCClient) Load(ctx context.Context, id string) (*config.Tree, bool, error) {
	resp, err := c.client.Load(ctx, &pb.LoadRequest{Id: id})
	if err != nil {
		return nil, false, err
	}
	if !resp.GetFound() {
		return nil, false, nil
	}
	return config.TreeFromProto(resp.GetTree()), true, nil
}

func (c *GRPCClient) Delete(ctx context.Context, id string) error {
	_, err := c.client.Delete(ctx, &pb.DeleteRequest{Id: id})
	return err
}

func (c *GRPCClient) ListTrees(ctx context.Context) ([]string, error) {
	resp, err := c.client.ListTrees(ctx, &pb.ListTreesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetIds(), nil
}

func (c *GRPCClient) SupportsVersioning(ctx context.Context) (bool, error) {
	resp, err := c.client.SupportsVersioning(ctx, &pb.SupportsVersioningRequest{})
	if err != nil {
		return false, err
	}
	return resp.GetSupported(), nil
}

func (c *GRPCClient) ListVersions(ctx context.Context, id string) ([]string, error) {
	resp, err := c.client.ListVersions(ctx, &pb.ListVersionsRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return resp.GetVersions(), nil
}

func (c *GRPCClient) LoadVersion(ctx context.Context, id string, version string) (*config.Tree, bool, error) {
	resp, err := c.client.LoadVersion(ctx, &pb.LoadVersionRequest{
		Id:      id,
		Version: version,
	})
	if err != nil {
		return nil, false, err
	}
	if !resp.GetFound() {
		return nil, false, nil
	}
	return config.TreeFromProto(resp.GetTree()), true, nil
}

func (c *GRPCClient) DeleteVersion(ctx context.Context, id string, version string) error {
	_, err := c.client.DeleteVersion(ctx, &pb.DeleteVersionRequest{
		Id:      id,
		Version: version,
	})
	return err
}

func (c *GRPCClient) EncryptionStatus(ctx context.Context) (EncryptionStatus, error) {
	resp, err := c.client.EncryptionStatus(ctx, &pb.EncryptionStatusRequest{})
	if err != nil {
		return 0, err
	}
	return EncryptionStatus(resp.GetStatus()), nil
}

func (c *GRPCClient) InitEncryption(ctx context.Context, passphrase []byte) error {
	_, err := c.client.InitEncryption(ctx, &pb.InitEncryptionRequest{
		Passphrase: passphrase,
	})
	return err
}

func (c *GRPCClient) RotateEncryption(ctx context.Context, oldPassphrase, newPassphrase []byte) error {
	_, err := c.client.RotateEncryption(ctx, &pb.RotateEncryptionRequest{
		OldPassphrase: oldPassphrase,
		NewPassphrase: newPassphrase,
	})
	return err
}
