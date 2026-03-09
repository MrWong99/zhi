---
title: "feat: Map and List Value Editors for UI Plugins"
type: feat
status: active
date: 2026-03-08
---

# Map and List Value Editors for UI Plugins

## Overview

Add first-class UI support for editing map (key-value) and list (array) values across all UI plugins (TUI, Web, MCP). Currently, maps and lists can be stored as `config.Value` (the `Val any` field accepts any JSON-serializable type), but all UIs render them as opaque stringified text (`map[key:value]`), making them unusable for operators.

## Problem Statement / Motivation

Config plugins for Kubernetes deployments need dynamic key-value maps (e.g., `nodeSelector`, `annotations`, `tolerations`) and lists (e.g., `imagePullSecrets`, `tolerations`). Today, plugin authors work around this by storing these as JSON strings with `core.type: json`, requiring operators to hand-edit JSON — error-prone and hostile UX.

zhi already transports maps and lists correctly over gRPC (JSON-encoded `value_json`). The gap is purely in the UI layer: no plugin knows how to render an interactive editor for these types.

## Proposed Solution

### New `core.type` Values

Add two new semantic type hints:

| Type | Go representation | Description |
|---|---|---|
| `map` | `map[string]string` | Flat key-value pairs. Keys and values are strings. |
| `list` | `[]string` | Ordered list of string values. |

These cover the common Kubernetes use cases (labels, annotations, nodeSelector, imagePullSecrets). Complex nested structures (like tolerations with multiple fields per entry) are out of scope — those should use `core.type: yaml` (see companion plan).

### New Metadata Labels

| Label | Type | Description |
|---|---|---|
| `ui.mapKeyPlaceholder` | string | Placeholder text for map key input (e.g., "label key") |
| `ui.mapValuePlaceholder` | string | Placeholder text for map value input (e.g., "label value") |
| `ui.listItemPlaceholder` | string | Placeholder text for list item input (e.g., "secret name") |

### Config Plugin Usage

```go
// Map example: nodeSelector
{
    Path: "gateway/node-selector", Default: map[string]string{},
    Section: "Gateway", DisplayName: "Node Selector",
    Description: "Kubernetes nodeSelector labels for pod scheduling",
    Type: "map",
},

// List example: imagePullSecrets
{
    Path: "core/image-pull-secrets", Default: []string{},
    Section: "Core", DisplayName: "Image Pull Secrets",
    Description: "List of Kubernetes secret names for pulling container images",
    Type: "list",
},
```

### Template Usage

Templates access these values directly — no `fromJson` roundtrip needed:

```
{{- $nodeSelector := .GetOr "gateway/node-selector" (dict) -}}
{{ $nodeSelector | toYaml | nindent 8 }}

{{- $pullSecrets := .GetOr "core/image-pull-secrets" (list) -}}
{{ range $pullSecrets }}
- name: {{ . | quote }}
{{ end }}
```

## Technical Approach

### Phase 1: Core Type Registration

**File:** `pkg/zhiplugin/labels/builtin.go`

Add `"map"` and `"list"` to the recognized `core.type` values alongside the existing string, int, float, bool, etc.

**File:** `pkg/zhiplugin/config/config.go`

No changes needed — `Val any` already supports maps and slices. JSON marshaling handles both types natively.

### Phase 2: Web UI Editor

**File:** `pkg/providers/ui/webui/editor.go`

Extend `valueType()` to detect maps and slices, returning `"map"` or `"list"` respectively.

**File:** `pkg/providers/ui/webui/templates/fragments/value_form.html`

Add two new conditional blocks:

**Map editor:**
```html
<!-- Renders as a table of key-value rows with add/remove buttons -->
<div class="map-editor" data-path="{{ .Path }}">
  {{ range $k, $v := .MapValue }}
  <div class="map-row">
    <input type="text" name="map_key[]" value="{{ $k }}" placeholder="{{ .KeyPlaceholder }}">
    <input type="text" name="map_value[]" value="{{ $v }}" placeholder="{{ .ValuePlaceholder }}">
    <button type="button" class="remove-row">-</button>
  </div>
  {{ end }}
  <button type="button" class="add-row">+ Add Entry</button>
</div>
```

**List editor:**
```html
<!-- Renders as a list of text inputs with add/remove buttons -->
<div class="list-editor" data-path="{{ .Path }}">
  {{ range .ListValue }}
  <div class="list-row">
    <input type="text" name="list_item[]" value="{{ . }}" placeholder="{{ .ItemPlaceholder }}">
    <button type="button" class="remove-row">-</button>
  </div>
  {{ end }}
  <button type="button" class="add-row">+ Add Item</button>
</div>
```

**File:** `pkg/providers/ui/webui/editor.go`

Update form submission handling to reconstruct `map[string]string` or `[]string` from the form arrays before calling `SetValue`.

### Phase 3: TUI Editor

**File:** `internal/ui/tui/value_editor.go`

Add two new editor modes triggered by `core.type`:

**Map mode:**
- Display as a vertical list of `key = value` rows
- `a` to add a new entry (prompts for key, then value)
- `d` to delete the selected entry
- `Enter` to edit the selected entry's value
- `e` to edit the selected entry's key
- Renders inline: `{ key1: val1, key2: val2 }` in tree view

**List mode:**
- Display as a numbered vertical list
- `a` to add a new item
- `d` to delete the selected item
- `Enter` to edit the selected item
- Renders inline: `[item1, item2, item3]` in tree view

### Phase 4: MCP Plugin

**File:** `pkg/mcpbridge/tools.go`

No changes needed for `SetValueInput` — it already accepts `any` JSON type. Maps and lists serialize naturally. The MCP tool descriptions should document that map and list typed values accept JSON objects/arrays respectively.

### Phase 5: gRPC / Proto

No proto changes needed. `value_json` bytes already handle maps and lists through standard JSON marshaling. The type metadata (`core.type: map/list`) travels in `metadata_json`.

## Acceptance Criteria

- [x] `core.type: map` values render as interactive key-value editors in Web UI and TUI
- [x] `core.type: list` values render as interactive list editors in Web UI and TUI
- [x] Adding, editing, and removing entries persists correctly via `SetValue`
- [x] Empty maps (`{}`) and empty lists (`[]`) are handled gracefully
- [ ] Map/list values round-trip correctly through gRPC (JSON encoding)
- [ ] Default values of type `map[string]string` and `[]string` work in config plugins
- [x] Tree view shows compact inline representation of maps and lists
- [x] Existing `string`, `int`, `bool` types are unaffected
- [x] `make check` passes
- [x] All new code has tests with `t.Parallel()`

## Scope Limitations

- **Only `map[string]string` and `[]string`** — not nested maps, not lists of objects. Complex structures like Kubernetes tolerations (list of objects with key/effect/value fields) should use the YAML multiline editor (companion plan).
- **No drag-and-drop reordering** for lists — items can be added/removed, order is insertion order.
- **No key validation** in the generic editor — config plugins handle validation via their `Validate()` method.

## Dependencies

None. This is a self-contained UI enhancement.
