package main

import (
	"context"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/store/jsonfile"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// dispense starts an in-process gRPC plugin server and returns a connected
// store.Plugin client. No subprocess is spawned.
func dispense(t *testing.T) store.Plugin {
	t.Helper()
	s, err := jsonfile.New(jsonfile.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("jsonfile.New: %v", err)
	}
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"store": &store.GRPCPlugin{Impl: s},
	})
	raw, err := client.Dispense("store")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(store.Plugin)
}

// TestGRPCRoundTrip verifies the jsonfile provider works through the full
// gRPC plugin layer.
func TestGRPCRoundTrip(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	values := map[string]config.Value{
		"db/host": {Val: "localhost"},
		"db/port": {Val: float64(5432)},
	}

	if err := p.PutValues(ctx, "prod", values, nil); err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	got, err := p.GetValues(ctx, "prod", []string{"db/host", "db/port"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}

	if got["db/host"].Val != "localhost" {
		t.Errorf("db/host = %v, want %q", got["db/host"].Val, "localhost")
	}
	if got["db/port"].Val != float64(5432) {
		t.Errorf("db/port = %v, want %v", got["db/port"].Val, float64(5432))
	}
}

func TestGRPCCapabilities(t *testing.T) {
	p := dispense(t)

	caps, err := p.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.Versioning != store.VersioningNone {
		t.Errorf("Versioning = %v, want VersioningNone", caps.Versioning)
	}
}
