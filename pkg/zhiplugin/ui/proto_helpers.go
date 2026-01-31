package ui

import (
	"encoding/json"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	cfgpb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
)

// treeToProto serialises a config.TreeReader into proto TreeEntry messages.
func treeToProto(tree config.TreeReader) []*cfgpb.TreeEntry {
	paths := tree.List()
	entries := make([]*cfgpb.TreeEntry, 0, len(paths))
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
		entries = append(entries, &cfgpb.TreeEntry{
			Path:         p,
			ValueJson:    valJSON,
			MetadataJson: metaJSON,
		})
	}
	return entries
}

// treeFromProto reconstructs a config.Tree from proto TreeEntry messages.
func treeFromProto(entries []*cfgpb.TreeEntry) *config.Tree {
	t := config.NewTree()
	for _, e := range entries {
		v, err := valueFromProto(e.GetValueJson(), e.GetMetadataJson())
		if err != nil {
			continue
		}
		// Paths were validated at Set time; re-validation is acceptable.
		_ = t.Set(e.GetPath(), &v)
	}
	return t
}

// valueFromProto reconstructs a config.Value from JSON-encoded bytes.
func valueFromProto(valJSON, metaJSON []byte) (config.Value, error) {
	var v config.Value
	if len(valJSON) > 0 {
		if err := json.Unmarshal(valJSON, &v.Val); err != nil {
			return config.Value{}, err
		}
	}
	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &v.Metadata); err != nil {
			return config.Value{}, err
		}
	}
	return v, nil
}
