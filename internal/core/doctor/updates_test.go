package doctor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MrWong99/zhi/internal/core"
	"github.com/MrWong99/zhi/pkg/sharing/lockfile"
	"github.com/MrWong99/zhi/pkg/sharing/marketplace"
)

func TestParseOCIRef(t *testing.T) {
	cases := []struct {
		in            string
		wantPublisher string
		wantShort     string
		wantOK        bool
	}{
		{"ghcr.io/mrwong99/zhi/zhi-store-memory:v0.0.9", "mrwong99", "memory", true},
		{"ghcr.io/mrwong99/zhi/zhi-config-pokedex:v1.0.0", "mrwong99", "pokedex", true},
		{"oci://ghcr.io/acme/zhi/zhi-transform-encrypt", "acme", "encrypt", true},
		{"ghcr.io/acme/raw-name:v1", "acme", "raw-name", true},
		{"broken", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			pub, short, ok := parseOCIRef(tc.in)
			if ok != tc.wantOK || pub != tc.wantPublisher || short != tc.wantShort {
				t.Fatalf("parseOCIRef(%q) = %q, %q, %v — want %q, %q, %v",
					tc.in, pub, short, ok, tc.wantPublisher, tc.wantShort, tc.wantOK)
			}
		})
	}
}

func TestUpdatesAvailable_NoMarketplace(t *testing.T) {
	env := &Environment{Workspace: &core.WorkspaceConfig{Dir: t.TempDir()}}
	res := (updatesAvailableCheck{}).Run(context.Background(), env)
	if len(res) != 1 || res[0].Status != StatusSkipped {
		t.Fatalf("want single skipped, got %+v", res)
	}
}

func TestUpdatesAvailable_NoLockfile(t *testing.T) {
	dir := t.TempDir()
	env := &Environment{
		Workspace:   &core.WorkspaceConfig{Dir: dir},
		Marketplace: marketplace.NewClient("http://unused.example"),
	}
	res := (updatesAvailableCheck{}).Run(context.Background(), env)
	if len(res) != 1 || res[0].Status != StatusSkipped {
		t.Fatalf("want skipped (no lockfile), got %+v", res)
	}
}

func TestUpdatesAvailable_UpToDateAndStale(t *testing.T) {
	// Simulate a marketplace that returns a different latest per plugin.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path is like /api/v1/plugins/store/mrwong99/memory
		var latest string
		switch r.URL.Path {
		case "/api/v1/plugins/store/mrwong99/memory":
			latest = "0.0.9" // same as installed → up to date
		case "/api/v1/plugins/config/mrwong99/pokedex":
			latest = "2.0.0" // newer than installed 1.0.0 → stale
		default:
			http.NotFound(w, r)
			return
		}
		detail := marketplace.PluginDetail{
			Versions: []marketplace.PluginVersion{{Version: latest}},
		}
		_ = json.NewEncoder(w).Encode(detail)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	lf := &lockfile.LockFile{
		Version:     1,
		GeneratedAt: time.Now(),
		ZhiVersion:  "1.7.0",
		Plugins: []lockfile.LockedPlugin{
			{Name: "memory", Type: "store", Ref: "ghcr.io/mrwong99/zhi/zhi-store-memory:v0.0.9", Digest: "sha256:abc"},
			{Name: "pokedex", Type: "config", Ref: "ghcr.io/mrwong99/zhi/zhi-config-pokedex:v1.0.0", Digest: "sha256:def"},
		},
	}
	data, err := lf.Marshal()
	if err != nil {
		t.Fatalf("marshal lockfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zhi-plugins.lock"), data, 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	env := &Environment{
		Workspace:   &core.WorkspaceConfig{Dir: dir},
		Marketplace: marketplace.NewClient(srv.URL),
	}
	res := (updatesAvailableCheck{}).Run(context.Background(), env)
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d: %+v", len(res), res)
	}

	// Results preserve lockfile order.
	if res[0].Status != StatusOK {
		t.Errorf("memory: want OK, got %s — %s", res[0].Status, res[0].Message)
	}
	if res[1].Status != StatusWarning {
		t.Errorf("pokedex: want Warning, got %s — %s", res[1].Status, res[1].Message)
	}
}
