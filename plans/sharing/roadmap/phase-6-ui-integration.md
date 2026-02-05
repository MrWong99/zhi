# Phase 6: UI Integration — Marketplace in TUI/Web

**Goal**: Marketplace is browsable from any UI plugin.

**Prerequisites**: [Phase 4 (Discovery)](phase-4-discovery.md) — marketplace API must exist. [Phase 5 (Community)](phase-5-community.md) is recommended but not required (ratings and verification badges enhance the UI but the marketplace tab works without them).

## Overview

This phase brings the marketplace into every UI plugin — TUI, Web, HTTP API, and future frontends. Rather than requiring users to switch to the CLI for plugin management, they can search, browse, install, and update directly from whatever UI they are using.

The integration is through the existing bidirectional `Controller` interface in `pkg/zhiplugin/ui/plugin.go`. New methods are added for marketplace operations, and the gRPC protocol is extended accordingly. Full specifications are in the [UI Integration](../ui-integration.md) plan.

## Deliverables

### 1. Controller Interface Extension

Add marketplace methods to the `Controller` interface that UI plugins call back into (see [UI Integration](../ui-integration.md) for full type definitions):

```go
// New methods on Controller interface
SearchMarketplace(ctx, query) (*MarketplaceResults, error)
GetMarketplaceDetail(ctx, publisher, name) (*MarketplaceDetail, error)
InstallPlugin(ctx, ref) (*InstallResult, error)
UninstallPlugin(ctx, name, pluginType) error
ListInstalledPlugins(ctx) ([]InstalledPlugin, error)
CheckUpdates(ctx) ([]PluginUpdate, error)
UpdatePlugin(ctx, name, version) (*InstallResult, error)
RatePlugin(ctx, publisher, name, rating) error
```

**Implementation**: These methods are implemented in `UIController` (`internal/ui/controller.go`), which wraps the sharing client from [Phase 1](phase-1-foundation.md) and marketplace client from [Phase 4](phase-4-discovery.md).

### 2. gRPC Protocol Extension

Add corresponding RPCs to the `UIControllerService` in `api/proto/zhiplugin/v1/ui.proto`:

```protobuf
rpc SearchMarketplace(SearchMarketplaceRequest) returns (SearchMarketplaceResponse);
rpc GetMarketplaceDetail(GetMarketplaceDetailRequest) returns (GetMarketplaceDetailResponse);
rpc InstallPlugin(InstallPluginRequest) returns (InstallPluginResponse);
rpc UninstallPlugin(UninstallPluginRequest) returns (UninstallPluginResponse);
rpc ListInstalledPlugins(ListInstalledPluginsRequest) returns (ListInstalledPluginsResponse);
rpc CheckUpdates(CheckUpdatesRequest) returns (CheckUpdatesResponse);
rpc UpdatePlugin(UpdatePluginRequest) returns (UpdatePluginResponse);
rpc RatePlugin(RatePluginRequest) returns (RatePluginResponse);
```

After editing the proto file, run `make proto` to regenerate stubs, then implement `grpc_client.go` and `grpc_server.go` translations in `pkg/zhiplugin/ui/`.

### 3. UI Capabilities Extension

Extend the `Capabilities` struct so UIs can declare marketplace support:

```go
type Capabilities struct {
    RequiresTTY         bool
    SupportsStreaming   bool
    SupportsMarketplace bool  // new
}
```

The engine uses this to determine whether the active UI can handle marketplace interactions or if the user should be directed to CLI commands.

### 4. TUI Marketplace View

Implement two new tabs in the built-in TUI (`internal/ui/tui/`) using Bubbletea:

**Marketplace tab**: Search, filter, and browse plugins from the marketplace. See [UI Integration](../ui-integration.md) for detailed mockups including:
- Search bar with type/sort filters
- Result list with ratings, download counts, and verified badges
- Keyboard shortcuts: `[i]nfo`, `[Enter] install`, `[/] search`

**Installed tab**: Manage locally installed plugins:
- Table of installed plugins with version, source, and update availability
- Keyboard shortcuts: `[u]pdate selected`, `[U]pdate all`, `[d]elete`

**Plugin detail view**: Accessible from both tabs:
- Full description, version history, rating distribution
- Install/update/rate actions
- Platform and runtime information

### 5. HTTP API Endpoints

Marketplace proxy endpoints for the web UI and HTTP API plugins:

```
GET  /api/marketplace/search?q=...&type=...
GET  /api/marketplace/plugins/{publisher}/{name}
POST /api/marketplace/plugins/{publisher}/{name}/install
POST /api/marketplace/plugins/{publisher}/{name}/rate
GET  /api/plugins/installed
GET  /api/plugins/updates
POST /api/plugins/{name}/update
DELETE /api/plugins/{name}
```

The `zhi-ui-httpapi` example (`examples/zhi-ui-httpapi/`) is updated as a reference implementation.

### 6. Update Notifications

UI plugins check for updates on startup and display a non-intrusive notification:

- Call `CheckUpdates()` on the Controller during `Run()` initialization
- Display notification if updates are available (see [UI Integration](../ui-integration.md) for mockup)
- Non-blocking: the notification dismisses after a timeout or user interaction

### 7. Progressive Enhancement

The marketplace integration degrades gracefully (see [UI Integration](../ui-integration.md) under "Progressive Enhancement"):

| State | Behavior |
|---|---|
| No marketplace configured | Marketplace tab shows configuration instructions |
| Marketplace configured, no auth | Search and browse work; install works for public plugins; rating requires auth |
| Full auth | All features available |
| Offline | Installed tab works; marketplace shows "offline" with cached metadata |

## Key Files to Modify

| File | Change |
|---|---|
| `pkg/zhiplugin/ui/plugin.go` | Extend `Controller` interface and `Capabilities` struct |
| `api/proto/zhiplugin/v1/ui.proto` | Add marketplace RPCs to `UIControllerService` |
| `pkg/zhiplugin/ui/grpc_client.go` | Implement client-side gRPC translation for new methods |
| `pkg/zhiplugin/ui/grpc_server.go` | Implement server-side gRPC translation for new methods |
| `internal/ui/controller.go` | Implement marketplace methods in `UIController` |
| `internal/ui/tui/` | Add Marketplace and Installed tabs |
| `examples/zhi-ui-httpapi/main.go` | Add marketplace API endpoints |

## New Files

```
internal/ui/tui/marketplace.go   # Marketplace tab model and view
internal/ui/tui/installed.go     # Installed plugins tab model and view
internal/ui/tui/plugindetail.go  # Plugin detail view model and view
internal/ui/tui/notification.go  # Update notification component
```

## Exit Criteria

- The TUI has a working Marketplace tab with search, filter, and install
- The TUI has an Installed tab showing all plugins with update indicators
- Plugin detail view shows ratings, versions, and allows install/update/rate
- The HTTP API example exposes marketplace endpoints
- Update notifications appear in UIs on startup when updates are available
- UI plugins without marketplace support continue to work unchanged (backward compatible)

## Design References

- [UI Integration](../ui-integration.md) — Full Controller interface extension, gRPC protocol additions, TUI mockups, HTTP API endpoints, progressive enhancement
- [Architecture Overview](../architecture.md) — UI Controller extension section
- [Marketplace Server](../marketplace-server.md) — API that the UI proxies through the Controller
- [Ratings & Verification](../ratings-and-verification.md) — Rating submission from UI, badge display
