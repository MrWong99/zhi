# UI Plugin

The UI plugin API lets a zhi plugin provide an interactive frontend for
browsing and managing configuration trees. Unlike other plugin types
where the host calls the plugin in one direction, UI plugins use
**bidirectional gRPC communication**: the host calls the plugin to start
the UI, and the plugin calls back into the host for core operations
(loading trees, setting values, validating, exporting, etc.).

This bidirectional pattern is implemented via
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin)'s
GRPCBroker, which dynamically allocates a reverse channel so the plugin
process can invoke host operations without the host needing to poll.

## Package layout

```
pkg/zhiplugin/
  plugin.go              shared Handshake (all plugin types)
  ui/
    ui.go                types: ExportRequest, ApplyEvent, ComponentInfo, etc.
    plugin.go            Plugin and Controller interfaces, PluginMap, GRPCPlugin
    grpc_client.go       host-side gRPC client (starts controller server via broker)
    grpc_server.go       plugin-side gRPC server (dials controller via broker)
    controller_client.go plugin-side Controller gRPC client
    controller_server.go host-side Controller gRPC server
    proto_helpers.go     tree ↔ proto serialisation helpers
    proto/               generated protobuf / gRPC stubs
```

## Core types

### Capabilities

Reports what a UI plugin supports.

```go
type Capabilities struct {
    RequiresTTY bool
}
```

| Field         | Meaning                                                       |
|---------------|---------------------------------------------------------------|
| `RequiresTTY` | UI needs direct terminal access (must run as a builtin plugin)|

External gRPC plugins do not have access to the host's terminal, so any
UI that needs a TTY (e.g. a TUI built with Bubbletea) must be registered
as a builtin. HTTP-based UIs, AI chat interfaces, and similar
non-terminal frontends should report `RequiresTTY: false`.

### ExportRequest and ExportResult

```go
type ExportRequest struct {
    TemplatePath string
    Format       string
    OutputPath   string
    Prefix       string
    DryRun       bool
}

type ExportResult struct {
    Name       string
    Content    string
    OutputPath string
}
```

### ExportTemplate

```go
type ExportTemplate struct {
    Name     string
    Template string
    Format   string
    Output   string
    Prefix   string
}
```

### ApplyEvent and ApplyResult

```go
type ApplyEvent struct {
    Line   string  // single line of output
    Stream string  // "stdout" or "stderr"
}

type ApplyResult struct {
    ExitCode int
    Error    string
}
```

`Apply` streams output line-by-line via a callback. On the wire this
uses gRPC server-side streaming.

### ComponentInfo

```go
type ComponentInfo struct {
    Name         string
    Description  string
    Enabled      bool
    Mandatory    bool
    Paths        []string
    Dependencies []string
}
```

### TreeEntry helpers

```go
func TreeToEntries(tree config.TreeReader) []TreeEntry
func TreeFromEntries(entries []TreeEntry) *config.Tree
```

Convert between `config.Tree` and a flat `[]TreeEntry` slice. Used
internally by the gRPC serialisation layer.

## Controller interface

The `Controller` is provided to the plugin when `Run` is called. It
exposes all operations the UI can perform on the zhi core engine.

```go
type Controller interface {
    LoadTree(ctx context.Context) (*config.Tree, error)
    SetValue(ctx context.Context, path string, value config.Value) error
    Validate(ctx context.Context) ([]config.ValidationResult, error)
    SaveTree(ctx context.Context) error
    ExportTemplates(ctx context.Context) ([]ExportTemplate, error)
    Export(ctx context.Context, req ExportRequest) (*ExportResult, error)
    Apply(ctx context.Context, target string, handler func(ApplyEvent)) (*ApplyResult, error)
    ListComponents(ctx context.Context) ([]ComponentInfo, error)
    EnableComponent(ctx context.Context, name string) ([]string, error)
    DisableComponent(ctx context.Context, name string) error
    WorkspaceName(ctx context.Context) (string, error)
}
```

| Method            | Purpose                                                      |
|-------------------|--------------------------------------------------------------|
| `LoadTree`        | load or reload the full configuration tree (with transforms) |
| `SetValue`        | store a configuration value at a path                        |
| `Validate`        | run validation on the current tree                           |
| `SaveTree`        | persist the current tree to the store                        |
| `ExportTemplates` | return the workspace's configured export templates           |
| `Export`          | run a single export operation                                |
| `Apply`           | run the apply command; output streams via the callback       |
| `ListComponents`  | list all components with their current state                 |
| `EnableComponent` | enable a component and its dependencies                      |
| `DisableComponent`| disable a component                                          |
| `WorkspaceName`   | return the workspace display name                            |

For builtin plugins the `Controller` is backed directly by the engine.
For external gRPC plugins it is a client that calls back to the host
via the GRPCBroker.

## Plugin interface

To create a UI plugin, implement `ui.Plugin`:

```go
type Plugin interface {
    Run(ctx context.Context, controller Controller) error
    Capabilities(ctx context.Context) (Capabilities, error)
}
```

| Method         | Purpose                                                      |
|----------------|--------------------------------------------------------------|
| `Run`          | start the UI; block until the user exits or ctx is cancelled |
| `Capabilities` | report what this UI supports                                 |

`Run` receives a `Controller` that the UI uses for all core operations.
The method must block for the lifetime of the UI session. When the user
exits (or `ctx` is cancelled) `Run` should clean up and return.

## Wiring a plugin binary

A plugin binary needs a `main` function that calls `plugin.Serve`:

```go
package main

import (
    "github.com/hashicorp/go-plugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin"
    "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: zhiplugin.Handshake,
        Plugins: map[string]plugin.Plugin{
            "ui": &ui.GRPCPlugin{Impl: &myUI{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

See the [zhi-ui-httpapi example](../../examples/zhi-ui-httpapi/) for a
complete, runnable plugin that exposes an HTTP/JSON API with SSE
streaming.

## How bidirectional communication works over gRPC

The UI plugin uses hashicorp/go-plugin's GRPCBroker to establish two
gRPC services — one in each direction:

```
  Plugin Process                         Host Process
  ──────────────                         ────────────
  UIService (gRPC server)                UIControllerService (gRPC server)
  ├─ Run()                               ├─ LoadTree()
  └─ Capabilities()                      ├─ SetValue()
                                         ├─ Validate()
                                         ├─ SaveTree()
                                         ├─ ExportTemplates()
                                         ├─ Export()
                                         ├─ Apply() [server-streaming]
                                         ├─ ListComponents()
                                         ├─ EnableComponent()
                                         ├─ DisableComponent()
                                         └─ WorkspaceName()
```

### Startup sequence

1. The host dispenses the `"ui"` plugin, obtaining a `GRPCClient`.
2. The host calls `GRPCClient.Run(ctx, controller)`.
3. `GRPCClient` starts a `UIControllerService` gRPC server on the
   broker and obtains a `brokerID`.
4. `GRPCClient` sends `RunRequest{controller_broker_id: brokerID}` to
   the plugin's `UIService.Run`.
5. The plugin-side `GRPCServer.Run` dials the broker ID to obtain a
   `UIControllerServiceClient`, wraps it as a `Controller`, and passes
   it to the `Plugin.Run` implementation.
6. The plugin's `Run` method uses the `Controller` to interact with
   the host for the duration of the session.

### Apply streaming

`Apply` uses gRPC server-side streaming. The host's controller server
sends `CtrlApplyResponse` messages as output arrives:

- `done=false`: an output event with `line` and `stream` fields
- `done=true`: the final result with `exit_code` and `error`

The plugin-side controller client reads the stream and delivers events
to the `handler` callback provided by the UI implementation.

## Comparison with other plugin types

| Plugin type | Communication   | Direction                         |
|-------------|-----------------|-----------------------------------|
| Config      | unidirectional  | host → plugin                     |
| Transform   | unidirectional  | host → plugin                     |
| Store       | unidirectional  | host → plugin                     |
| **UI**      | **bidirectional** | host → plugin **and** plugin → host |

This bidirectional pattern is necessary because UI plugins are
interactive: they need to respond to user actions by invoking operations
on the core engine, rather than passively responding to host-initiated
calls.
