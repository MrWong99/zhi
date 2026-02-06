package main

import (
	"context"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/ui/webui"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// TestGRPCRoundTrip verifies the webui plugin works over gRPC via go-plugin.
func TestGRPCRoundTrip(t *testing.T) {
	cfg := webui.DefaultConfig()
	cfg.Addr = "127.0.0.1:0"
	cfg.AutoOpen = false
	w := webui.New(cfg)

	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"ui": &ui.GRPCPlugin{Impl: w},
	})
	raw, err := client.Dispense("ui")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	p := raw.(ui.Plugin)

	caps, err := p.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.RequiresTTY {
		t.Error("RequiresTTY should be false for Web UI")
	}
}
