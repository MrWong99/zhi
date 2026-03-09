---
title: "feat: YAML/Multiline Semantic Type for Structured Text Values"
type: feat
status: active
date: 2026-03-08
---

# YAML/Multiline Semantic Type for Structured Text Values

## Overview

Add `core.type: yaml` as a recognized semantic type that signals "this value is a structured YAML/JSON text blob." Combined with the existing `ui.multiline: true` rendering hint, this gives UI plugins enough information to render appropriate editors (syntax-highlighted textarea, format validation, etc.) for values that hold embedded configuration, complex structures, or provider-specific options maps.

## Problem Statement / Motivation

Config plugins need to store structured text values that don't fit the flat key-value model:

1. **Provider options maps** — provider-specific config like `{"temperature": 0.7, "top_p": 0.9}` for an LLM provider. These are `map[string]any` with provider-defined keys.
2. **Complex Kubernetes structures** — tolerations (`[{"key":"...", "effect":"..."}]`), which are lists of multi-field objects.
3. **Embedded config blobs** — MCP server definitions, NPC arrays, or other variable-length structured data that can't be decomposed into fixed paths.

Today, plugin authors store these as strings with `ui.multiline: true`, but the UI has no semantic understanding — it's just a textarea. There's no syntax validation, no formatting help, and no indication to the operator that this field expects YAML or JSON.

## Proposed Solution

### New `core.type` Value

| Type | Description |
|---|---|
| `yaml` | Value is a string containing YAML (or JSON, since JSON is valid YAML). UIs should render a multiline editor with appropriate affordances. |

This is a **semantic type hint**, not a new value representation. The underlying `Val` remains a `string`. The type signals to UIs:

- Render as multiline text editor (implies `ui.multiline: true`)
- Optionally provide syntax highlighting or formatting
- Validate that the content is parseable YAML on save (warning severity, not blocking — config plugins handle blocking validation)

### New Metadata Labels

| Label | Type | Description |
|---|---|---|
| `ui.yamlSchema` | string | Optional. Human-readable description of expected structure (e.g., "List of {key, effect, value} objects"). Displayed as help text. |

### Config Plugin Usage

```go
// Provider options (map[string]any as YAML string)
{
    Path: "config/providers/llm/options", Default: "",
    Section: "Providers — LLM", DisplayName: "Provider Options",
    Description: "Provider-specific options as YAML (e.g., temperature, top_p)",
    Type: "yaml",
    Placeholder: "temperature: 0.7\ntop_p: 0.9",
},

// Kubernetes tolerations (complex list as YAML string)
{
    Path: "gateway/tolerations", Default: "",
    Section: "Gateway", DisplayName: "Tolerations",
    Description: "Kubernetes tolerations for pod scheduling",
    Type: "yaml",
    Placeholder: "- key: dedicated\n  operator: Equal\n  value: glyphoxa\n  effect: NoSchedule",
},

// MCP servers (array of objects as YAML string)
{
    Path: "config/mcp-servers", Default: "",
    Section: "MCP", DisplayName: "MCP Servers",
    Description: "MCP tool server definitions",
    Type: "yaml",
    Placeholder: "- name: dice-roller\n  transport: stdio\n  command: mcp-dice",
},
```

### Template Usage

Templates parse the YAML string back to structured data:

```
{{- $tolerations := .GetOr "gateway/tolerations" "" -}}
{{ if $tolerations }}
tolerations:
{{ $tolerations | nindent 8 }}
{{ end }}
```

Or for values that need Go-level processing:

```
{{- $options := .GetOr "config/providers/llm/options" "" | fromYaml -}}
{{ range $k, $v := $options }}
{{ $k }}: {{ $v }}
{{ end }}
```

## Technical Approach

### Phase 1: Type Registration

**File:** `pkg/zhiplugin/labels/builtin.go`

Add `"yaml"` to the recognized `core.type` values.

### Phase 2: Web UI

**File:** `pkg/providers/ui/webui/editor.go`

When `core.type` is `"yaml"`, force multiline mode and set a CSS class for optional syntax highlighting.

**File:** `pkg/providers/ui/webui/templates/fragments/value_form.html`

For `yaml` type values:
- Render `<textarea>` with `rows="8"` (larger than default multiline `rows="4"`)
- Add `class="yaml-editor"` for CSS/JS hooks
- Display `ui.yamlSchema` as help text below the editor
- Display `ui.placeholder` as the textarea placeholder
- On form submission, validate YAML parseability client-side (JavaScript `js-yaml` or simple try-parse) and show inline warning if invalid

### Phase 3: TUI

**File:** `internal/ui/tui/value_editor.go`

When `core.type` is `"yaml"`:
- Use the existing multiline editor (same as `ui.multiline: true`)
- Display the `ui.yamlSchema` hint above the editor
- On save, attempt YAML parse and show warning if invalid (non-blocking)

### Phase 4: Validation Helper

**File:** `pkg/zhiplugin/config/validators.go` (new file, or add to existing)

Provide a reusable validation function that config plugins can use:

```go
// ValidateYAML returns a Warning-severity result if the value is not valid YAML.
func ValidateYAML(v Value, _ TreeReader) ([]ValidationResult, error) {
    s, ok := v.Val.(string)
    if !ok || s == "" {
        return nil, nil
    }
    var target any
    if err := yaml.Unmarshal([]byte(s), &target); err != nil {
        return []ValidationResult{{
            Severity: SeverityWarning,
            Message:  fmt.Sprintf("value is not valid YAML: %v", err),
        }}, nil
    }
    return nil, nil
}
```

This is a convenience — config plugins can also write their own validators that parse the YAML and check structure.

## Acceptance Criteria

- [x] `core.type: yaml` is recognized and does not produce unknown-type warnings
- [x] Web UI renders yaml-typed values with a larger textarea and optional schema hint
- [x] TUI renders yaml-typed values in multiline mode with schema hint
- [x] `ui.placeholder` works correctly with yaml type (multiline placeholder)
- [x] `ValidateYAML` helper function is available for config plugins
- [x] Invalid YAML produces a warning (not blocking) in the UI on save
- [x] MCP plugin accepts yaml-typed values as plain strings (no special handling needed)
- [x] Existing `ui.multiline` behavior is unchanged (yaml type implies multiline but doesn't break explicit multiline usage)
- [x] `make check` passes
- [x] All new code has tests with `t.Parallel()`

## Dependencies

None. This is independent of the map/list editor plan (they are complementary — `map`/`list` handle simple flat structures, `yaml` handles everything else).
