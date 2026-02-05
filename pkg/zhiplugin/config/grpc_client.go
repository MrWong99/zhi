package config

import (
	"context"
	"encoding/json"

	pb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
)

// GRPCClient implements ConfigPlugin by talking to the gRPC
// ConfigServiceClient generated from the proto definition.
type GRPCClient struct {
	client pb.ConfigServiceClient
}

func (c *GRPCClient) List(ctx context.Context) ([]string, error) {
	resp, err := c.client.List(ctx, &pb.ListRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetPaths(), nil
}

func (c *GRPCClient) Get(ctx context.Context, path string) (Value, bool, error) {
	resp, err := c.client.Get(ctx, &pb.GetRequest{Path: path})
	if err != nil {
		return Value{}, false, err
	}
	if !resp.GetFound() {
		return Value{}, false, nil
	}
	v, err := ValueFromProto(resp.GetValueJson(), resp.GetMetadataJson())
	if err != nil {
		return Value{}, false, err
	}
	return v, true, nil
}

func (c *GRPCClient) Set(ctx context.Context, path string, v Value) error {
	valJSON, err := json.Marshal(v.Val)
	if err != nil {
		return err
	}
	var metaJSON []byte
	if v.Metadata != nil {
		metaJSON, err = json.Marshal(v.Metadata)
		if err != nil {
			return err
		}
	}
	_, err = c.client.Set(ctx, &pb.SetRequest{
		Path:         path,
		ValueJson:    valJSON,
		MetadataJson: metaJSON,
	})
	return err
}

func (c *GRPCClient) Validate(ctx context.Context, path string, tree TreeReader) ([]ValidationResult, error) {
	entries := TreeToProto(tree)
	resp, err := c.client.Validate(ctx, &pb.ValidateRequest{
		Path: path,
		Tree: entries,
	})
	if err != nil {
		return nil, err
	}
	return resultsFromProto(resp.GetResults())
}

// TreeToProto serialises a TreeReader into a slice of proto TreeEntry
// messages for transmission over the wire.
func TreeToProto(tree TreeReader) []*pb.TreeEntry {
	paths := tree.List()
	entries := make([]*pb.TreeEntry, 0, len(paths))
	for _, p := range paths {
		v, ok := tree.Get(p)
		if !ok {
			continue
		}
		valJSON, err := json.Marshal(v.Val)
		if err != nil {
			continue
		}
		var metaJSON []byte
		if v.Metadata != nil {
			metaJSON, _ = json.Marshal(v.Metadata)
		}
		entries = append(entries, &pb.TreeEntry{
			Path:         p,
			ValueJson:    valJSON,
			MetadataJson: metaJSON,
		})
	}
	return entries
}

// resultsFromProto converts proto ValidationResultMsg messages back into
// ValidationResult values.
func resultsFromProto(msgs []*pb.ValidationResultMsg) ([]ValidationResult, error) {
	results := make([]ValidationResult, 0, len(msgs))
	for _, m := range msgs {
		r := ValidationResult{
			Severity: Severity(m.GetSeverity()),
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

// ValueFromProto reconstructs a Value from JSON-encoded bytes received over
// the wire.
func ValueFromProto(valJSON, metaJSON []byte) (Value, error) {
	return ValueFromProtoVersioned(valJSON, metaJSON, "")
}

// ValueFromProtoVersioned reconstructs a Value from JSON-encoded bytes and
// an optional version identifier received over the wire.
func ValueFromProtoVersioned(valJSON, metaJSON []byte, version string) (Value, error) {
	var v Value
	if len(valJSON) > 0 {
		if err := json.Unmarshal(valJSON, &v.Val); err != nil {
			return Value{}, err
		}
	}
	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &v.Metadata); err != nil {
			return Value{}, err
		}
	}
	v.Version = version
	return v, nil
}
