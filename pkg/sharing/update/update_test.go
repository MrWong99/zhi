package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdvisoryAffects(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		affected  string
		want      bool
	}{
		{"within range", "1.2.0", "<1.5.0", true},
		{"outside range (fixed)", "1.6.2", "<1.5.0", false},
		{"exact boundary excluded", "1.5.0", "<1.5.0", false},
		{"range match", "1.3.0", ">=1.0.0, <2.0.0", true},
		{"range no match", "2.1.0", ">=1.0.0, <2.0.0", false},
		{"empty constraint treated as affected", "1.6.2", "", true},
		{"unparsable constraint treated as affected", "1.6.2", "not-a-constraint", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := advisoryAffects(tt.installed, tt.affected); got != tt.want {
				t.Errorf("advisoryAffects(%q, %q) = %v, want %v", tt.installed, tt.affected, got, tt.want)
			}
		})
	}
}

func TestCacheSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache", "update-check.json")

	cache := NewCache(path, 1*time.Hour)

	result := &CheckResult{
		CheckedAt: time.Now().Truncate(time.Second),
		Updates: []UpdateInfo{
			{
				Name:      "ansible-config",
				Installed: "1.1.0",
				Available: "1.2.0",
				Scope:     "minor",
			},
		},
	}

	if err := cache.Save(result); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := cache.Load()
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if len(loaded.Updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(loaded.Updates))
	}
	if loaded.Updates[0].Name != "ansible-config" {
		t.Errorf("Name = %q, want %q", loaded.Updates[0].Name, "ansible-config")
	}
	if loaded.Updates[0].Installed != "1.1.0" {
		t.Errorf("Installed = %q, want %q", loaded.Updates[0].Installed, "1.1.0")
	}
	if loaded.Updates[0].Available != "1.2.0" {
		t.Errorf("Available = %q, want %q", loaded.Updates[0].Available, "1.2.0")
	}
}

func TestCacheExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")

	// Create a cache with a very short interval.
	cache := NewCache(path, 1*time.Millisecond)

	result := &CheckResult{
		CheckedAt: time.Now().Add(-1 * time.Second),
		Updates:   []UpdateInfo{},
	}

	if err := cache.Save(result); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Should return nil because the cache is expired.
	loaded := cache.Load()
	if loaded != nil {
		t.Error("expected nil for expired cache")
	}
}

func TestCacheLoadMissing(t *testing.T) {
	cache := NewCache(filepath.Join(t.TempDir(), "nonexistent.json"), 1*time.Hour)
	loaded := cache.Load()
	if loaded != nil {
		t.Error("expected nil for missing cache file")
	}
}

func TestCacheLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	cache := NewCache(path, 1*time.Hour)
	loaded := cache.Load()
	if loaded != nil {
		t.Error("expected nil for invalid JSON")
	}
}

func TestUpdateInfoJSON(t *testing.T) {
	info := UpdateInfo{
		Name:        "test-plugin",
		Installed:   "1.0.0",
		Available:   "1.1.0",
		Scope:       "minor",
		HasAdvisory: true,
		Changelog:   "Fixed a bug",
		Pinned:      false,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded UpdateInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Name != info.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, info.Name)
	}
	if decoded.HasAdvisory != info.HasAdvisory {
		t.Errorf("HasAdvisory = %v, want %v", decoded.HasAdvisory, info.HasAdvisory)
	}
}

func TestDefaultCachePath(t *testing.T) {
	path := DefaultCachePath()
	if path == "" {
		t.Skip("cannot determine home directory")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultCachePath() = %q, want absolute path", path)
	}
}
