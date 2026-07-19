package config

import "testing"

// TestMergedPluginRejectsPrefixWithoutSlash is a regression for prefixes that
// do not end in "/", which would misroute and corrupt paths.
func TestMergedPluginRejectsPrefixWithoutSlash(t *testing.T) {
	_, err := MergedPlugin(MountedPlugin{
		Impl:   &mergedPlugin{}, // any non-nil Plugin
		Prefix: "app-a",         // missing trailing slash
	})
	if err == nil {
		t.Fatal("MergedPlugin should reject a prefix without a trailing slash")
	}
}

func TestMergedPluginAcceptsSlashPrefix(t *testing.T) {
	_, err := MergedPlugin(MountedPlugin{
		Impl:   &mergedPlugin{},
		Prefix: "app-a/",
	})
	if err != nil {
		t.Fatalf("MergedPlugin with slash-terminated prefix: %v", err)
	}
}
