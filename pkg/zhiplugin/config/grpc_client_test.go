package config

import (
	"testing"

	pb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
)

// TestTreeProtoRoundTripPreservesVersion verifies that the optimistic-locking
// version survives a TreeToProto/TreeFromProto round-trip (regression for the
// dropped TreeEntry.version field).
func TestTreeProtoRoundTripPreservesVersion(t *testing.T) {
	tree := NewTree()
	if err := tree.Set("database/host", &Value{Val: "localhost", Version: "3"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	entries, err := TreeToProto(tree)
	if err != nil {
		t.Fatalf("TreeToProto: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].GetVersion() != "3" {
		t.Errorf("proto entry version = %q, want %q", entries[0].GetVersion(), "3")
	}

	got, err := TreeFromProto(entries)
	if err != nil {
		t.Fatalf("TreeFromProto: %v", err)
	}
	v, ok := got.Get("database/host")
	if !ok {
		t.Fatal("path missing after round-trip")
	}
	if v.Version != "3" {
		t.Errorf("Version = %q, want %q after round-trip", v.Version, "3")
	}
}

// TestTreeFromProtoDecodeErrorPropagates verifies that an undecodable entry
// surfaces an error instead of being silently dropped (which would delete the
// path from the host tree during a transform round-trip).
func TestTreeFromProtoDecodeErrorPropagates(t *testing.T) {
	entries := []*pb.TreeEntry{
		{Path: "app/port", ValueJson: []byte(`8080`)},
		{Path: "app/broken", ValueJson: []byte(`{"port":`)}, // malformed JSON
	}
	if _, err := TreeFromProto(entries); err == nil {
		t.Fatal("TreeFromProto with a malformed entry should return an error")
	}
}
