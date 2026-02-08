package ui

import (
	"context"
	"sync"
	"testing"
	"time"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// --- testController: records all calls from the UI plugin -----------------

type testController struct {
	mu sync.Mutex

	loadTreeCalled     bool
	setValueCalls      []setValueCall
	validateCalled     bool
	saveCalls          []string // IDs
	exportTemplates    []ExportTemplate
	exportCalls        []ExportRequest
	applyCalls         []string // targets
	listComponentsCall bool
	enableCalls        []string
	disableCalls       []string
	workspaceNameCall  bool

	// Marketplace calls.
	searchCalls    []MarketplaceQuery
	detailCalls    []string // "publisher/name"
	installCalls   []string // refs
	uninstallCalls []string // "name/type"
	listInstalled  bool
	checkUpdates   bool
	updateCalls    []string // "name/version"
	rateCalls      []string // "publisher/name"

	// Responses
	tree       *config.Tree
	validation []config.ValidationResult
	components []ComponentInfo
}

type setValueCall struct {
	path  string
	value config.Value
}

func newTestController() *testController {
	tree := config.NewTree()
	_ = tree.Set("app/host", &config.Value{Val: "localhost"})
	_ = tree.Set("app/port", &config.Value{Val: 8080, Metadata: map[string]any{"desc": "server port"}})
	return &testController{
		tree: tree,
		validation: []config.ValidationResult{
			{Severity: config.Warning, Message: "port is high"},
		},
		exportTemplates: []ExportTemplate{
			{Name: "env", Format: "dotenv", Output: ".env"},
		},
		components: []ComponentInfo{
			{Name: "app", Enabled: true, Mandatory: true, Paths: []string{"app/"}},
			{Name: "db", Enabled: false, Dependencies: []string{"app"}},
		},
	}
}

func (c *testController) LoadTree(_ context.Context) (*config.Tree, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadTreeCalled = true
	return c.tree, nil
}

func (c *testController) SetValue(_ context.Context, path string, value config.Value) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setValueCalls = append(c.setValueCalls, setValueCall{path: path, value: value})
	return nil
}

func (c *testController) Validate(_ context.Context) ([]config.ValidationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.validateCalled = true
	return c.validation, nil
}

func (c *testController) SaveTree(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.saveCalls = append(c.saveCalls, "saved")
	return nil
}

func (c *testController) ExportTemplates(_ context.Context) ([]ExportTemplate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.exportTemplates, nil
}

func (c *testController) Export(_ context.Context, req ExportRequest) (*ExportResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.exportCalls = append(c.exportCalls, req)
	return &ExportResult{Name: "env", Content: "HOST=localhost", OutputPath: ".env"}, nil
}

func (c *testController) Apply(_ context.Context, target string, handler func(ApplyEvent)) (*ApplyResult, error) {
	c.mu.Lock()
	c.applyCalls = append(c.applyCalls, target)
	c.mu.Unlock()

	if handler != nil {
		handler(ApplyEvent{Line: "starting...", Stream: "stdout"})
		handler(ApplyEvent{Line: "done!", Stream: "stdout"})
	}
	return &ApplyResult{ExitCode: 0}, nil
}

func (c *testController) ListComponents(_ context.Context) ([]ComponentInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listComponentsCall = true
	return c.components, nil
}

func (c *testController) EnableComponent(_ context.Context, name string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enableCalls = append(c.enableCalls, name)
	return []string{name, "dep1"}, nil
}

func (c *testController) DisableComponent(_ context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disableCalls = append(c.disableCalls, name)
	return nil
}

func (c *testController) WorkspaceName(_ context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workspaceNameCall = true
	return "test-workspace", nil
}

func (c *testController) SearchMarketplace(_ context.Context, query MarketplaceQuery) (*MarketplaceResults, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.searchCalls = append(c.searchCalls, query)
	return &MarketplaceResults{
		Total: 1,
		Results: []MarketplaceEntry{
			{
				Name:          "test-plugin",
				Publisher:     "acme",
				Type:          "config",
				Description:   "A test plugin",
				LatestVersion: "1.0.0",
				Rating:        4.5,
				RatingCount:   10,
				Downloads:     100,
				Verified:      true,
				Platforms:     []string{"linux/amd64"},
			},
		},
	}, nil
}

func (c *testController) GetMarketplaceDetail(_ context.Context, publisher, name string) (*MarketplaceDetail, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.detailCalls = append(c.detailCalls, publisher+"/"+name)
	return &MarketplaceDetail{
		MarketplaceEntry: MarketplaceEntry{
			Name:          name,
			Publisher:     publisher,
			Type:          "config",
			Description:   "A test plugin",
			LatestVersion: "1.0.0",
			Rating:        4.5,
			RatingCount:   10,
			Downloads:     100,
			Verified:      true,
		},
		LongDescription: "Long description",
		License:         "MIT",
		Homepage:        "https://example.com",
		Repository:      "https://github.com/acme/test",
		Keywords:        []string{"config", "test"},
		Versions: []VersionEntry{
			{Version: "1.0.0", CreatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), Digest: "sha256:abc", Platforms: []string{"linux/amd64"}},
		},
		Ratings: []RatingEntry{
			{Score: 5, Comment: "Great!", Author: "user1", CreatedAt: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)},
		},
		Dependencies: []DependencyEntry{
			{Name: "dep-plugin", Type: "config", Publisher: "acme", Required: true},
		},
	}, nil
}

func (c *testController) InstallPlugin(_ context.Context, ref string) (*InstallResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.installCalls = append(c.installCalls, ref)
	return &InstallResult{
		Name:        "test-plugin",
		Type:        "config",
		Version:     "1.0.0",
		Digest:      "sha256:abc",
		Verified:    true,
		RuntimeDeps: []string{"dep-plugin"},
	}, nil
}

func (c *testController) UninstallPlugin(_ context.Context, name string, pluginType string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.uninstallCalls = append(c.uninstallCalls, name+"/"+pluginType)
	return nil
}

func (c *testController) ListInstalledPlugins(_ context.Context) ([]InstalledPlugin, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listInstalled = true
	return []InstalledPlugin{
		{
			Name:        "test-plugin",
			Type:        "config",
			Version:     "1.0.0",
			Source:      "registry.example.com/acme/test-plugin:1.0.0",
			InstalledAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			Digest:      "sha256:abc",
			Verified:    true,
			UpdateAvail: "1.1.0",
		},
	}, nil
}

func (c *testController) CheckUpdates(_ context.Context) ([]PluginUpdate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkUpdates = true
	return []PluginUpdate{
		{
			Name:           "test-plugin",
			Type:           "config",
			CurrentVersion: "1.0.0",
			LatestVersion:  "1.1.0",
			Verified:       true,
		},
	}, nil
}

func (c *testController) UpdatePlugin(_ context.Context, name string, version string) (*InstallResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateCalls = append(c.updateCalls, name+"/"+version)
	return &InstallResult{
		Name:        name,
		Type:        "config",
		Version:     "1.1.0",
		PrevVersion: "1.0.0",
		Digest:      "sha256:def",
		Verified:    true,
	}, nil
}

func (c *testController) RatePlugin(_ context.Context, publisher, name string, _ Rating) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rateCalls = append(c.rateCalls, publisher+"/"+name)
	return nil
}

// --- testUIPlugin: exercises the Controller during Run --------------------

type testUIPlugin struct {
	runCalled bool
	runErr    error
	actions   func(ctx context.Context, ctrl Controller) error
}

func (p *testUIPlugin) Run(ctx context.Context, ctrl Controller) error {
	p.runCalled = true
	if p.actions != nil {
		return p.actions(ctx, ctrl)
	}
	return p.runErr
}

func (p *testUIPlugin) Capabilities(_ context.Context) (Capabilities, error) {
	return Capabilities{RequiresTTY: false}, nil
}

// --- dispense helper for gRPC round-trip ----------------------------------

func dispense(t *testing.T, impl Plugin) Plugin {
	t.Helper()
	client, _ := goplugin.TestPluginGRPCConn(t, false, map[string]goplugin.Plugin{
		"ui": &GRPCPlugin{Impl: impl},
	})
	raw, err := client.Dispense("ui")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	return raw.(Plugin)
}

// --- tests ----------------------------------------------------------------

func TestCapabilities(t *testing.T) {
	impl := &testUIPlugin{}
	p := dispense(t, impl)

	caps, err := p.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if caps.RequiresTTY {
		t.Errorf("expected RequiresTTY=false, got true")
	}
}

func TestRunCallsPlugin(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			// Verify we can call WorkspaceName through the gRPC round-trip.
			name, err := c.WorkspaceName(ctx)
			if err != nil {
				return err
			}
			if name != "test-workspace" {
				t.Errorf("WorkspaceName = %q, want %q", name, "test-workspace")
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !impl.runCalled {
		t.Error("expected Run to be called on impl")
	}
	if !ctrl.workspaceNameCall {
		t.Error("expected WorkspaceName to be called on controller")
	}
}

func TestRunLoadTree(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			tree, err := c.LoadTree(ctx)
			if err != nil {
				return err
			}
			paths := tree.List()
			if len(paths) != 2 {
				t.Errorf("LoadTree returned %d paths, want 2", len(paths))
			}
			v, ok := tree.Get("app/host")
			if !ok {
				t.Error("app/host not found in loaded tree")
			} else if v.Val != "localhost" {
				t.Errorf("app/host = %v, want localhost", v.Val)
			}
			v, ok = tree.Get("app/port")
			if !ok {
				t.Error("app/port not found in loaded tree")
			}
			if v.Metadata == nil || v.Metadata["desc"] != "server port" {
				t.Errorf("app/port metadata = %v, want {desc: server port}", v.Metadata)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunSetValue(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			return c.SetValue(ctx, "app/host", config.Value{
				Val:      "example.com",
				Metadata: map[string]any{"changed": true},
			})
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.setValueCalls) != 1 {
		t.Fatalf("expected 1 SetValue call, got %d", len(ctrl.setValueCalls))
	}
	call := ctrl.setValueCalls[0]
	if call.path != "app/host" {
		t.Errorf("SetValue path = %q, want app/host", call.path)
	}
	if call.value.Val != "example.com" {
		t.Errorf("SetValue val = %v, want example.com", call.value.Val)
	}
}

func TestRunValidate(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			results, err := c.Validate(ctx)
			if err != nil {
				return err
			}
			if len(results) != 1 {
				t.Errorf("Validate returned %d results, want 1", len(results))
				return nil
			}
			if results[0].Severity != config.Warning {
				t.Errorf("severity = %v, want Warning", results[0].Severity)
			}
			if results[0].Message != "port is high" {
				t.Errorf("message = %q, want %q", results[0].Message, "port is high")
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ctrl.validateCalled {
		t.Error("expected Validate to be called on controller")
	}
}

func TestRunSaveTree(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			return c.SaveTree(ctx)
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.saveCalls) != 1 {
		t.Errorf("SaveTree calls = %v, want 1 call", ctrl.saveCalls)
	}
}

func TestRunExportTemplates(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			templates, err := c.ExportTemplates(ctx)
			if err != nil {
				return err
			}
			if len(templates) != 1 {
				t.Errorf("ExportTemplates returned %d, want 1", len(templates))
				return nil
			}
			if templates[0].Name != "env" || templates[0].Format != "dotenv" {
				t.Errorf("template = %+v, want name=env format=dotenv", templates[0])
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunExport(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			result, err := c.Export(ctx, ExportRequest{
				Format: "dotenv",
				DryRun: true,
			})
			if err != nil {
				return err
			}
			if result.Content != "HOST=localhost" {
				t.Errorf("export content = %q, want HOST=localhost", result.Content)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunApplyStreaming(t *testing.T) {
	ctrl := newTestController()

	var receivedEvents []ApplyEvent
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			result, err := c.Apply(ctx, "default", func(event ApplyEvent) {
				receivedEvents = append(receivedEvents, event)
			})
			if err != nil {
				return err
			}
			if result.ExitCode != 0 {
				t.Errorf("exit code = %d, want 0", result.ExitCode)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.applyCalls) != 1 || ctrl.applyCalls[0] != "default" {
		t.Errorf("Apply calls = %v, want [default]", ctrl.applyCalls)
	}
	if len(receivedEvents) != 2 {
		t.Fatalf("received %d events, want 2", len(receivedEvents))
	}
	if receivedEvents[0].Line != "starting..." {
		t.Errorf("event[0].Line = %q, want starting...", receivedEvents[0].Line)
	}
	if receivedEvents[1].Line != "done!" {
		t.Errorf("event[1].Line = %q, want done!", receivedEvents[1].Line)
	}
}

func TestRunListComponents(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			components, err := c.ListComponents(ctx)
			if err != nil {
				return err
			}
			if len(components) != 2 {
				t.Errorf("ListComponents returned %d, want 2", len(components))
				return nil
			}
			// Find the "app" component.
			found := false
			for _, comp := range components {
				if comp.Name == "app" {
					found = true
					if !comp.Enabled || !comp.Mandatory {
						t.Errorf("app component: enabled=%v mandatory=%v, want both true", comp.Enabled, comp.Mandatory)
					}
				}
			}
			if !found {
				t.Error("component 'app' not found")
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunEnableDisableComponent(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			enabled, err := c.EnableComponent(ctx, "db")
			if err != nil {
				return err
			}
			if len(enabled) != 2 {
				t.Errorf("EnableComponent returned %d names, want 2", len(enabled))
			}

			err = c.DisableComponent(ctx, "db")
			if err != nil {
				return err
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.enableCalls) != 1 || ctrl.enableCalls[0] != "db" {
		t.Errorf("Enable calls = %v, want [db]", ctrl.enableCalls)
	}
	if len(ctrl.disableCalls) != 1 || ctrl.disableCalls[0] != "db" {
		t.Errorf("Disable calls = %v, want [db]", ctrl.disableCalls)
	}
}

func TestRunSearchMarketplace(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			results, err := c.SearchMarketplace(ctx, MarketplaceQuery{
				Query:   "test",
				Type:    "config",
				Sort:    "downloads",
				Page:    1,
				PerPage: 10,
			})
			if err != nil {
				return err
			}
			if results.Total != 1 {
				t.Errorf("SearchMarketplace total = %d, want 1", results.Total)
			}
			if len(results.Results) != 1 {
				t.Fatalf("SearchMarketplace returned %d results, want 1", len(results.Results))
			}
			r := results.Results[0]
			if r.Name != "test-plugin" {
				t.Errorf("result name = %q, want test-plugin", r.Name)
			}
			if r.Publisher != "acme" {
				t.Errorf("result publisher = %q, want acme", r.Publisher)
			}
			if r.Rating != 4.5 {
				t.Errorf("result rating = %v, want 4.5", r.Rating)
			}
			if !r.Verified {
				t.Error("expected result to be verified")
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.searchCalls) != 1 {
		t.Fatalf("expected 1 search call, got %d", len(ctrl.searchCalls))
	}
	if ctrl.searchCalls[0].Query != "test" {
		t.Errorf("search query = %q, want test", ctrl.searchCalls[0].Query)
	}
}

func TestRunGetMarketplaceDetail(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			detail, err := c.GetMarketplaceDetail(ctx, "acme", "test-plugin")
			if err != nil {
				return err
			}
			if detail.Name != "test-plugin" {
				t.Errorf("detail name = %q, want test-plugin", detail.Name)
			}
			if detail.LongDescription != "Long description" {
				t.Errorf("long desc = %q, want 'Long description'", detail.LongDescription)
			}
			if detail.License != "MIT" {
				t.Errorf("license = %q, want MIT", detail.License)
			}
			if len(detail.Versions) != 1 {
				t.Fatalf("versions = %d, want 1", len(detail.Versions))
			}
			if detail.Versions[0].Version != "1.0.0" {
				t.Errorf("version = %q, want 1.0.0", detail.Versions[0].Version)
			}
			if len(detail.Ratings) != 1 || detail.Ratings[0].Score != 5 {
				t.Errorf("ratings = %+v, want [score=5]", detail.Ratings)
			}
			if len(detail.Dependencies) != 1 || detail.Dependencies[0].Name != "dep-plugin" {
				t.Errorf("deps = %+v, want [dep-plugin]", detail.Dependencies)
			}
			if len(detail.Keywords) != 2 {
				t.Errorf("keywords = %v, want [config test]", detail.Keywords)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunInstallPlugin(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			result, err := c.InstallPlugin(ctx, "acme/test-plugin")
			if err != nil {
				return err
			}
			if result.Name != "test-plugin" {
				t.Errorf("install name = %q, want test-plugin", result.Name)
			}
			if result.Version != "1.0.0" {
				t.Errorf("install version = %q, want 1.0.0", result.Version)
			}
			if !result.Verified {
				t.Error("expected install result to be verified")
			}
			if len(result.RuntimeDeps) != 1 || result.RuntimeDeps[0] != "dep-plugin" {
				t.Errorf("runtime deps = %v, want [dep-plugin]", result.RuntimeDeps)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.installCalls) != 1 || ctrl.installCalls[0] != "acme/test-plugin" {
		t.Errorf("install calls = %v, want [acme/test-plugin]", ctrl.installCalls)
	}
}

func TestRunUninstallPlugin(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			return c.UninstallPlugin(ctx, "test-plugin", "config")
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.uninstallCalls) != 1 || ctrl.uninstallCalls[0] != "test-plugin/config" {
		t.Errorf("uninstall calls = %v, want [test-plugin/config]", ctrl.uninstallCalls)
	}
}

func TestRunListInstalledPlugins(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			plugins, err := c.ListInstalledPlugins(ctx)
			if err != nil {
				return err
			}
			if len(plugins) != 1 {
				t.Fatalf("listed %d plugins, want 1", len(plugins))
			}
			p := plugins[0]
			if p.Name != "test-plugin" {
				t.Errorf("plugin name = %q, want test-plugin", p.Name)
			}
			if p.Version != "1.0.0" {
				t.Errorf("plugin version = %q, want 1.0.0", p.Version)
			}
			if p.UpdateAvail != "1.1.0" {
				t.Errorf("update avail = %q, want 1.1.0", p.UpdateAvail)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ctrl.listInstalled {
		t.Error("expected ListInstalledPlugins to be called")
	}
}

func TestRunCheckUpdates(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			updates, err := c.CheckUpdates(ctx)
			if err != nil {
				return err
			}
			if len(updates) != 1 {
				t.Fatalf("got %d updates, want 1", len(updates))
			}
			if updates[0].Name != "test-plugin" {
				t.Errorf("update name = %q, want test-plugin", updates[0].Name)
			}
			if updates[0].LatestVersion != "1.1.0" {
				t.Errorf("latest version = %q, want 1.1.0", updates[0].LatestVersion)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ctrl.checkUpdates {
		t.Error("expected CheckUpdates to be called")
	}
}

func TestRunUpdatePlugin(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			result, err := c.UpdatePlugin(ctx, "test-plugin", "1.1.0")
			if err != nil {
				return err
			}
			if result.Version != "1.1.0" {
				t.Errorf("update version = %q, want 1.1.0", result.Version)
			}
			if result.PrevVersion != "1.0.0" {
				t.Errorf("prev version = %q, want 1.0.0", result.PrevVersion)
			}
			return nil
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.updateCalls) != 1 || ctrl.updateCalls[0] != "test-plugin/1.1.0" {
		t.Errorf("update calls = %v, want [test-plugin/1.1.0]", ctrl.updateCalls)
	}
}

func TestRunRatePlugin(t *testing.T) {
	ctrl := newTestController()
	impl := &testUIPlugin{
		actions: func(ctx context.Context, c Controller) error {
			return c.RatePlugin(ctx, "acme", "test-plugin", Rating{Score: 5, Comment: "Great!"})
		},
	}

	p := dispense(t, impl)
	if err := p.Run(context.Background(), ctrl); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ctrl.rateCalls) != 1 || ctrl.rateCalls[0] != "acme/test-plugin" {
		t.Errorf("rate calls = %v, want [acme/test-plugin]", ctrl.rateCalls)
	}
}

func TestTreeEntryRoundTrip(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("a/b", &config.Value{Val: "hello", Metadata: map[string]any{"key": "val"}})
	_ = tree.Set("c/d", &config.Value{Val: 42.0})

	entries := TreeToEntries(tree)
	if len(entries) != 2 {
		t.Fatalf("TreeToEntries returned %d entries, want 2", len(entries))
	}

	rebuilt := TreeFromEntries(entries)
	paths := rebuilt.List()
	if len(paths) != 2 {
		t.Fatalf("rebuilt tree has %d paths, want 2", len(paths))
	}

	v, ok := rebuilt.Get("a/b")
	if !ok {
		t.Fatal("a/b not found in rebuilt tree")
	}
	if v.Val != "hello" {
		t.Errorf("a/b val = %v, want hello", v.Val)
	}
	if v.Metadata["key"] != "val" {
		t.Errorf("a/b metadata = %v, want {key: val}", v.Metadata)
	}
}
