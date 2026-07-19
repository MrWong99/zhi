package transform

import (
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	cfgpb "github.com/MrWong99/zhi/pkg/zhiplugin/config/proto"
)

// TestApplyTreeInvalidPathError verifies that a transform-added entry with an
// invalid path name surfaces an error instead of silently vanishing. The
// invalid path is smuggled in via TreeFromProto, which (like a real plugin
// response) bypasses host-side path validation.
func TestApplyTreeInvalidPathError(t *testing.T) {
	src, err := config.TreeFromProto([]*cfgpb.TreeEntry{
		{Path: "TLS/cert", ValueJson: []byte(`"pem"`)}, // uppercase segment is invalid
	})
	if err != nil {
		t.Fatalf("TreeFromProto: %v", err)
	}

	dst := config.NewTree()
	if err := applyTree(src, dst); err == nil {
		t.Fatal("applyTree should return an error for an invalid transformed path")
	}
}

// TestApplyTreeValidPaths verifies the happy path still applies additions and
// removals correctly.
func TestApplyTreeValidPaths(t *testing.T) {
	dst := config.NewTree()
	if err := dst.Set("old/key", &config.Value{Val: "gone"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	src := config.NewTree()
	if err := src.Set("new/key", &config.Value{Val: "added"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := applyTree(src, dst); err != nil {
		t.Fatalf("applyTree: %v", err)
	}
	if _, ok := dst.Get("old/key"); ok {
		t.Error("old/key should have been removed")
	}
	if v, ok := dst.Get("new/key"); !ok || v.Val != "added" {
		t.Error("new/key should have been added")
	}
}
