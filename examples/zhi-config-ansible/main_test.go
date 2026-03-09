package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// testPlugin creates an ansiblePlugin loaded from the testdata directory.
func testPlugin(t *testing.T) *ansiblePlugin {
	t.Helper()
	testdata := filepath.Join("testdata")
	absTestdata, err := filepath.Abs(testdata)
	if err != nil {
		t.Fatal(err)
	}

	opts := map[string]any{
		"inventory":      filepath.Join(absTestdata, "inventory", "hosts.yml"),
		"group_vars_dir": filepath.Join(absTestdata, "group_vars"),
		"host_vars_dir":  filepath.Join(absTestdata, "host_vars"),
	}

	p, err := newAnsiblePlugin(opts, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("newAnsiblePlugin: %v", err)
	}
	return p
}

// dispense starts an in-process gRPC plugin server and returns a connected
// config.Plugin client.
func dispense(t *testing.T) config.Plugin {
	t.Helper()
	p := testPlugin(t)
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"config": &config.GRPCPlugin{Impl: p},
	})
	raw, err := client.Dispense("config")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(config.Plugin)
}

func TestList(t *testing.T) {
	p := dispense(t)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Should have hosts and groups entries.
	hasHosts := false
	hasGroups := false
	for _, path := range got {
		if strings.HasPrefix(path, "hosts/") {
			hasHosts = true
		}
		if strings.HasPrefix(path, "groups/") {
			hasGroups = true
		}
	}
	if !hasHosts {
		t.Error("List() missing hosts/ paths")
	}
	if !hasGroups {
		t.Error("List() missing groups/ paths")
	}
}

func TestGetHost(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	v, ok, err := p.Get(ctx, "hosts/web1/ansible_host")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if v.Val != "10.0.1.10" {
		t.Errorf("Val = %v, want 10.0.1.10", v.Val)
	}
}

func TestGetMetadata(t *testing.T) {
	p := dispense(t)
	v, ok, err := p.Get(context.Background(), "hosts/web1/ansible_host")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	sf, exists := v.Metadata["ansible.source_file"]
	if !exists {
		t.Fatal("metadata missing ansible.source_file")
	}
	if sf == "" {
		t.Error("ansible.source_file is empty")
	}
}

func TestGetGroupMembers(t *testing.T) {
	p := dispense(t)
	v, ok, err := p.Get(context.Background(), "groups/webservers/members")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	members := strings.Split(v.Val.(string), ",")
	if !slices.Contains(members, "web1") {
		t.Errorf("webservers members = %v, want to contain web1", members)
	}
}

func TestGetGroupVars(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	// From group_vars/webservers.yml.
	v, ok, err := p.Get(ctx, "groups/webservers/vars/nginx_workers")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if v.Val != "4" {
		t.Errorf("Val = %v, want 4", v.Val)
	}
}

func TestGetHostVars(t *testing.T) {
	p := dispense(t)
	ctx := context.Background()

	// From host_vars/web1/main.yml.
	v, ok, err := p.Get(ctx, "hosts/web1/vars/nginx_workers")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if v.Val != "8" {
		t.Errorf("Val = %v, want 8", v.Val)
	}
}

func TestGetMissing(t *testing.T) {
	p := dispense(t)
	_, ok, err := p.Get(context.Background(), "hosts/nonexistent/ansible_host")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("Get returned ok=true for missing path")
	}
}

func TestSetAndGet(t *testing.T) {
	dir := t.TempDir()
	copyTestdata(t, dir)

	opts := map[string]any{
		"inventory":      filepath.Join(dir, "inventory", "hosts.yml"),
		"group_vars_dir": filepath.Join(dir, "group_vars"),
		"host_vars_dir":  filepath.Join(dir, "host_vars"),
	}

	p, err := newAnsiblePlugin(opts, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("newAnsiblePlugin: %v", err)
	}

	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"config": &config.GRPCPlugin{Impl: p},
	})
	raw, err := client.Dispense("config")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	plugin := raw.(config.Plugin)

	ctx := context.Background()
	err = plugin.Set(ctx, "groups/webservers/vars/nginx_workers", config.Value{Val: "16"})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, ok, err := plugin.Get(ctx, "groups/webservers/vars/nginx_workers")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found after Set")
	}
	if v.Val != "16" {
		t.Errorf("Val = %v, want 16", v.Val)
	}
}

func TestValidateMissingHost(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tree := config.NewTree()
	paths, _ := p.List(ctx)
	for _, path := range paths {
		v, ok, _ := p.Get(ctx, path)
		if ok {
			_ = tree.Set(path, &config.Value{Val: v.Val, Metadata: v.Metadata})
		}
	}

	// Add a group referencing a nonexistent host.
	_ = tree.Set("groups/broken/members", &config.Value{Val: "ghost_host"})

	results, err := p.Validate(ctx, "groups/broken/members", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Severity != config.Blocking {
		t.Errorf("Severity = %v, want Blocking", results[0].Severity)
	}
	if !strings.Contains(results[0].Message, "ghost_host") {
		t.Errorf("Message = %q, expected to mention ghost_host", results[0].Message)
	}
}

func TestValidatePort(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tests := []struct {
		name  string
		val   any
		wantN int
	}{
		{"valid float64", float64(22), 0},
		{"valid string", "8080", 0},
		{"too high", float64(70000), 1},
		{"zero", float64(0), 1},
		{"negative", float64(-1), 1},
		{"non-numeric", "abc", 1},
		{"fractional", float64(22.5), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			_ = tree.Set("hosts/web1/ansible_port", &config.Value{Val: tt.val})

			results, err := p.Validate(ctx, "hosts/web1/ansible_port", tree)
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if len(results) != tt.wantN {
				t.Errorf("got %d results, want %d: %v", len(results), tt.wantN, results)
			}
		})
	}
}

func TestValidateGroupName(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("groups/webservers/vars/port", &config.Value{Val: "80"})

	results, err := p.Validate(ctx, "groups/webservers/vars/port", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results for valid group, want 0", len(results))
	}
}

func TestPathConfinement(t *testing.T) {
	dir := t.TempDir()
	copyTestdata(t, dir)

	opts := map[string]any{
		"inventory":      filepath.Join(dir, "inventory", "hosts.yml"),
		"group_vars_dir": filepath.Join(dir, "group_vars"),
		"host_vars_dir":  filepath.Join(dir, "host_vars"),
	}

	p, err := newAnsiblePlugin(opts, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("newAnsiblePlugin: %v", err)
	}

	err = p.Set(context.Background(), "groups/webservers/vars/hack", config.Value{
		Val:      "evil",
		Metadata: map[string]any{"ansible.source_file": "/tmp/outside.yml"},
	})
	if err == nil {
		t.Error("expected error for source file outside project root")
	}
	if err != nil && !strings.Contains(err.Error(), "outside project root") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMissingPath(t *testing.T) {
	p := testPlugin(t)
	tree := config.NewTree()

	results, err := p.Validate(context.Background(), "hosts/web1/ansible_host", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results for missing path, want 0", len(results))
	}
}

// testPluginINI creates an ansiblePlugin loaded from the INI testdata.
func testPluginINI(t *testing.T) *ansiblePlugin {
	t.Helper()
	absTestdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}

	opts := map[string]any{
		"inventory": filepath.Join(absTestdata, "inventory", "hosts.ini"),
	}

	p, err := newAnsiblePlugin(opts, hclog.NewNullLogger())
	if err != nil {
		t.Fatalf("newAnsiblePlugin (INI): %v", err)
	}
	return p
}

func TestINIInventoryList(t *testing.T) {
	p := testPluginINI(t)
	paths, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	hasHosts := false
	hasGroups := false
	for _, path := range paths {
		if strings.HasPrefix(path, "hosts/") {
			hasHosts = true
		}
		if strings.HasPrefix(path, "groups/") {
			hasGroups = true
		}
	}
	if !hasHosts {
		t.Error("INI List() missing hosts/ paths")
	}
	if !hasGroups {
		t.Error("INI List() missing groups/ paths")
	}
}

func TestINIInventoryGetHost(t *testing.T) {
	p := testPluginINI(t)

	v, ok, err := p.Get(context.Background(), "hosts/web1/ansible_host")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if v.Val != "10.0.1.10" {
		t.Errorf("Val = %v, want 10.0.1.10", v.Val)
	}
}

func TestINIInventoryGetGroupMembers(t *testing.T) {
	p := testPluginINI(t)

	v, ok, err := p.Get(context.Background(), "groups/webservers/members")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if !strings.Contains(v.Val.(string), "web1") {
		t.Errorf("webservers members = %v, want to contain web1", v.Val)
	}
}

func TestINIInventoryGetGroupVars(t *testing.T) {
	p := testPluginINI(t)

	v, ok, err := p.Get(context.Background(), "groups/webservers/vars/http_port")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if v.Val != "8080" {
		t.Errorf("Val = %v, want 8080", v.Val)
	}
}

func TestINIInventoryHostPort(t *testing.T) {
	p := testPluginINI(t)

	v, ok, err := p.Get(context.Background(), "hosts/web1/ansible_port")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if v.Val != "22" {
		t.Errorf("Val = %v, want 22", v.Val)
	}
}

func TestValidateAnsibleHost(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tests := []struct {
		name  string
		val   string
		wantN int
	}{
		{"valid IP", "10.0.1.10", 0},
		{"valid IPv6", "::1", 0},
		{"valid hostname", "web1.example.com", 0},
		{"valid short hostname", "web1", 0},
		{"empty", "", 1},
		{"invalid chars", "web@server!", 1},
		{"spaces", "web server", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := config.NewTree()
			_ = tree.Set("hosts/web1/ansible_host", &config.Value{Val: tt.val})
			// Add group membership so the "ungrouped host" rule doesn't interfere.
			_ = tree.Set("groups/webservers/members", &config.Value{Val: "web1"})

			results, err := p.Validate(ctx, "hosts/web1/ansible_host", tree)
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if len(results) != tt.wantN {
				t.Errorf("got %d results, want %d: %v", len(results), tt.wantN, results)
			}
		})
	}
}

func TestValidateAnsibleUserEmpty(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("hosts/web1/ansible_user", &config.Value{Val: ""})

	results, err := p.Validate(ctx, "hosts/web1/ansible_user", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Message, "ansible_user") {
		t.Errorf("Message = %q, expected to mention ansible_user", results[0].Message)
	}
}

func TestValidateAnsibleUserNonEmpty(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("hosts/web1/ansible_user", &config.Value{Val: "deploy"})

	results, err := p.Validate(ctx, "hosts/web1/ansible_user", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results for valid user, want 0", len(results))
	}
}

func TestValidateHostNoGroup(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tree := config.NewTree()
	// Add a host that belongs to no group.
	_ = tree.Set("hosts/orphan/ansible_host", &config.Value{Val: "10.0.1.99"})

	results, err := p.Validate(ctx, "hosts/orphan/ansible_host", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Should get a warning about ansible_host being valid (0) + info about no group (1).
	found := false
	for _, r := range results {
		if strings.Contains(r.Message, "does not belong to any group") {
			found = true
			if r.Severity != config.Info {
				t.Errorf("Severity = %v, want Info", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected 'does not belong to any group' result")
	}
}

func TestValidateHostInGroup(t *testing.T) {
	p := testPlugin(t)
	ctx := context.Background()

	tree := config.NewTree()
	_ = tree.Set("hosts/web1/ansible_host", &config.Value{Val: "10.0.1.10"})
	_ = tree.Set("groups/webservers/members", &config.Value{Val: "web1"})

	results, err := p.Validate(ctx, "hosts/web1/ansible_host", tree)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, r := range results {
		if strings.Contains(r.Message, "does not belong to any group") {
			t.Error("host in a group should not trigger ungrouped warning")
		}
	}
}

// copyTestdata copies the testdata directory to dst.
func copyTestdata(t *testing.T, dst string) {
	t.Helper()
	entries := []struct {
		src string
		dst string
	}{
		{"testdata/inventory/hosts.yml", "inventory/hosts.yml"},
		{"testdata/group_vars/webservers.yml", "group_vars/webservers.yml"},
		{"testdata/host_vars/web1/main.yml", "host_vars/web1/main.yml"},
	}
	for _, e := range entries {
		data, err := os.ReadFile(e.src)
		if err != nil {
			t.Fatalf("reading %s: %v", e.src, err)
		}
		dstPath := filepath.Join(dst, e.dst)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
