# UI Integration

Marketplace browsing and plugin management from any UI plugin — TUI, Web, HTTP API, or future frontends.

## Controller Interface Extension

The `Controller` interface (`pkg/zhiplugin/ui/plugin.go`) is extended with marketplace operations. These are **additive** — existing UI plugins continue to work without implementing marketplace features.

### New Controller Methods

```go
// Sharing-related Controller methods (added to existing interface)

// SearchMarketplace queries the marketplace for plugins or workspaces.
SearchMarketplace(ctx context.Context, query MarketplaceQuery) (*MarketplaceResults, error)

// GetMarketplaceDetail returns detailed information about a marketplace artifact.
GetMarketplaceDetail(ctx context.Context, publisher, name string) (*MarketplaceDetail, error)

// InstallPlugin downloads and installs a plugin from an OCI reference.
InstallPlugin(ctx context.Context, ref string) (*InstallResult, error)

// UninstallPlugin removes an installed plugin.
UninstallPlugin(ctx context.Context, name string, pluginType string) error

// ListInstalledPlugins returns all installed plugins with their metadata.
ListInstalledPlugins(ctx context.Context) ([]InstalledPlugin, error)

// CheckUpdates returns available updates for installed plugins.
CheckUpdates(ctx context.Context) ([]PluginUpdate, error)

// UpdatePlugin updates a specific plugin to the latest (or specified) version.
UpdatePlugin(ctx context.Context, name string, version string) (*InstallResult, error)

// RatePlugin submits a rating for a marketplace plugin.
RatePlugin(ctx context.Context, publisher, name string, rating Rating) error
```

### Data Types

```go
// MarketplaceQuery represents a search request.
type MarketplaceQuery struct {
    Query       string   // free-text search
    Type        string   // filter: "config", "transform", "store", "ui", "workspace"
    Sort        string   // "relevance", "downloads", "rating", "updated"
    Verified    bool     // only verified publishers
    Page        int
    PerPage     int
}

// MarketplaceResults holds search results.
type MarketplaceResults struct {
    Total   int
    Results []MarketplaceEntry
}

// MarketplaceEntry is a single search result.
type MarketplaceEntry struct {
    Name          string
    Publisher     string
    Type          string
    Description   string
    LatestVersion string
    Rating        float64
    RatingCount   int
    Downloads     int
    Verified      bool
    Installed     bool     // true if already installed locally
    InstalledVer  string   // installed version (empty if not installed)
    UpdateAvail   bool     // true if newer version exists
    Platforms     []string // supported platforms
}

// MarketplaceDetail holds full information for a single artifact.
type MarketplaceDetail struct {
    MarketplaceEntry
    LongDescription string
    License         string
    Homepage        string
    Repository      string
    Versions        []VersionEntry
    Ratings         []RatingEntry
    Dependencies    []DependencyEntry // for workspaces
    Keywords        []string
}

// InstalledPlugin describes a locally installed plugin.
type InstalledPlugin struct {
    Name         string
    Type         string
    Version      string
    Source       string // "built-in", OCI ref, or local path
    InstalledAt  time.Time
    Digest       string
    Verified     bool
    UpdateAvail  string // latest available version, empty if up to date
}

// InstallResult describes the outcome of an install/update operation.
type InstallResult struct {
    Name         string
    Type         string
    Version      string
    PrevVersion  string // empty for fresh installs
    Digest       string
    Verified     bool
    RuntimeDeps  []string // any runtime dependencies that were installed
}

// Rating is a user-submitted review.
type Rating struct {
    Score   int    // 1-5
    Comment string
}
```

## gRPC Protocol Extension

The UI plugin gRPC protocol (`api/proto/zhiplugin/v1/ui.proto`) is extended with marketplace service methods. These are added to the existing `UIControllerService` that UI plugins call back into.

```protobuf
// Added to UIControllerService in ui.proto

rpc SearchMarketplace(SearchMarketplaceRequest) returns (SearchMarketplaceResponse);
rpc GetMarketplaceDetail(GetMarketplaceDetailRequest) returns (GetMarketplaceDetailResponse);
rpc InstallPlugin(InstallPluginRequest) returns (InstallPluginResponse);
rpc UninstallPlugin(UninstallPluginRequest) returns (UninstallPluginResponse);
rpc ListInstalledPlugins(ListInstalledPluginsRequest) returns (ListInstalledPluginsResponse);
rpc CheckUpdates(CheckUpdatesRequest) returns (CheckUpdatesResponse);
rpc UpdatePlugin(UpdatePluginRequest) returns (UpdatePluginResponse);
rpc RatePlugin(RatePluginRequest) returns (RatePluginResponse);
```

## UI Capabilities Extension

The `Capabilities` struct for UI plugins is extended so UIs can declare whether they support marketplace features:

```go
type Capabilities struct {
    RequiresTTY     bool
    SupportsStreaming bool
    // New:
    SupportsMarketplace bool // UI provides marketplace browsing
}
```

This allows the engine to know if the active UI supports marketplace interaction, falling back to CLI commands otherwise.

## TUI Marketplace View

The built-in TUI (`internal/ui/tui/`) gains a new marketplace tab/view using Bubbletea:

### Navigation

```
┌─ zhi ──────────────────────────────────────────────────────────────┐
│  [Config]  [Components]  [Export]  [Marketplace]  [Installed]      │
│                                                                     │
│  Search: ansible_                                                   │
│  Filter: [All] Config  Transform  Store  UI  Workspace             │
│  Sort:   [Relevance] Downloads  Rating  Updated                    │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  ★4.7  ansible-config                          config  ✓    │  │
│  │        Ansible inventory configuration provider              │  │
│  │        by zhi-project · v1.2.0 · 12.4k downloads            │  │
│  ├──────────────────────────────────────────────────────────────┤  │
│  │  ★4.2  ansible-transform                       transform    │  │
│  │        Ansible playbook transforms                           │  │
│  │        by community-dev · v0.9.1 · 3.1k downloads           │  │
│  ├──────────────────────────────────────────────────────────────┤  │
│  │  ★4.9  vault-store                             store    ✓   │  │
│  │        HashiCorp Vault KV store with encryption              │  │
│  │        by zhi-project · v2.0.0 · 28.7k downloads            │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  [i]nfo  [Enter] install  [/] search  [Tab] next tab  [q] quit    │
└─────────────────────────────────────────────────────────────────────┘
```

### Plugin Detail View

```
┌─ zhi · Plugin Detail ──────────────────────────────────────────────┐
│                                                                     │
│  ansible-config                                        config      │
│  by zhi-project (verified ✓)                                       │
│                                                                     │
│  ★★★★★ 4.7 (89 ratings)    12,450 downloads    Apache-2.0         │
│                                                                     │
│  Configuration provider that reads from Ansible inventory files.   │
│  Supports INI and YAML inventory formats with group variables.     │
│  Automatically discovers host and group variables from the         │
│  standard Ansible directory structure.                             │
│                                                                     │
│  Version:    1.2.0 (latest)                                        │
│  Platforms:  linux/amd64, linux/arm64, darwin/amd64, darwin/arm64  │
│  Runtime:    none (native Go binary)                               │
│  Signed:     Yes (release@zhi.dev via Sigstore)                    │
│  Homepage:   https://github.com/zhi-project/zhi-config-ansible    │
│                                                                     │
│  Status:     Installed (v1.1.0) — update available                 │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Versions:                                                    │  │
│  │    v1.2.0  2026-01-15  (latest)                              │  │
│  │    v1.1.0  2025-11-01  (installed)                           │  │
│  │    v1.0.0  2025-08-20                                        │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  [u]pdate  [Enter] install version  [r]ate  [Esc] back            │
└─────────────────────────────────────────────────────────────────────┘
```

### Installed Plugins Tab

```
┌─ zhi ──────────────────────────────────────────────────────────────┐
│  [Config]  [Components]  [Export]  [Marketplace]  [Installed]      │
│                                                                     │
│  Installed Plugins                                                  │
│                                                                     │
│  NAME                TYPE        VERSION   SOURCE        UPDATE    │
│  ──────────────────────────────────────────────────────────────    │
│  structuredfile      config      -         built-in      -        │
│  ansible-config      config      1.1.0     ghcr.io/...   1.2.0 ↑ │
│  vault               store       -         built-in      -        │
│  json-store          store       0.5.0     ghcr.io/...   (latest) │
│  tui                 ui          -         built-in      -        │
│                                                                     │
│  [u]pdate selected  [U]pdate all  [d]elete  [i]nfo  [Esc] back   │
└─────────────────────────────────────────────────────────────────────┘
```

## Web UI / HTTP API Integration

The HTTP API example plugin (`examples/zhi-ui-httpapi/`) and any future Web UI gain marketplace endpoints through the same Controller interface:

```
GET  /api/marketplace/search?q=ansible&type=config
GET  /api/marketplace/plugins/{publisher}/{name}
POST /api/marketplace/plugins/{publisher}/{name}/install
POST /api/marketplace/plugins/{publisher}/{name}/rate
GET  /api/plugins/installed
GET  /api/plugins/updates
POST /api/plugins/{name}/update
DELETE /api/plugins/{name}
```

A Web UI can render a rich marketplace experience with:
- Full-text search with filters
- Plugin detail pages with README rendering (Markdown → HTML)
- Star ratings with reviews
- One-click install/update buttons
- Update notification badges

## Notification System

UI plugins can check for updates on startup and show a non-intrusive notification:

```
┌─ Notifications ────────────────────────────────────┐
│ 2 plugin updates available:                        │
│   ansible-config: 1.1.0 → 1.2.0                   │
│   json-store: 0.4.0 → 0.5.0                       │
│                                                     │
│ [u]pdate all  [d]ismiss  [s]how details            │
└─────────────────────────────────────────────────────┘
```

This is implemented as part of the `Run()` method — the UI plugin calls `CheckUpdates()` on the Controller during initialization and renders the notification if updates are available.

## Progressive Enhancement

The marketplace integration is designed for progressive enhancement:

1. **No marketplace configured**: UI only shows installed plugins tab. Marketplace tab shows "Configure marketplace in ~/.zhi/config.yaml".
2. **Marketplace configured, no auth**: Search and browse work. Install works for public plugins. Rating requires authentication.
3. **Full auth**: All features available including rating and publishing.
4. **Offline mode**: Installed plugins tab works. Marketplace tab shows "Offline — connect to browse marketplace" with option to browse cached metadata.
