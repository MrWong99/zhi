# Metadata Labels API Design

**Status**: Draft
**Author**: Claude
**Date**: 2026-02-04

## Overview

This document proposes a **Metadata Label Registry** system for zhi that enables:

1. Plugins to interpret configuration values differently based on semantic metadata
2. Easy discovery of available metadata labels for end users and developers
3. Plugin developers to define and register custom labels
4. Runtime introspection via CLI and UI

## Problem Statement

Currently, metadata is stored as `map[string]any` with no schema or discovery mechanism. Plugin developers must:

- Know which keys other plugins recognize by reading documentation
- Hope they don't collide with other plugin's custom keys
- Implement their own validation for metadata values

End users have no way to discover what metadata options are available without reading plugin-specific documentation.

## Design Goals

1. **Backward compatible**: Existing `map[string]any` metadata continues to work
2. **Type-safe where possible**: Well-known labels have defined schemas
3. **Discoverable**: Labels can be queried at runtime via CLI/UI
4. **Extensible**: Plugins can register custom labels in their namespace
5. **Cross-plugin communication**: Standard labels enable plugin interop
6. **Self-documenting**: Labels carry descriptions and usage examples

## Design Options Considered

### Option A: Documentation-Only Convention

Keep `map[string]any` and establish conventions via documentation.

**Pros:**
- Zero code changes
- Maximum flexibility

**Cons:**
- No runtime discovery
- No validation
- Easy to make typos in label names
- No tooling support

### Option B: Type-Safe Separate Fields

Add typed fields to `Value` struct for each category:

```go
type Value struct {
    Val        any
    Metadata   map[string]any  // Legacy
    UIHints    *UIHints        // New typed struct
    StoreHints *StoreHints
    // etc.
}
```

**Pros:**
- Compile-time type safety
- IDE autocompletion

**Cons:**
- Breaking change to Value struct
- Must modify proto definitions
- Hard to extend (adding new hint types requires code changes)
- Plugins can't define custom categories

### Option C: Metadata Label Registry (Recommended)

Create a registry of label definitions that describe valid metadata keys, their schemas, and semantics.

**Pros:**
- Runtime discovery and validation
- Plugins can register custom labels
- Backward compatible (metadata stays `map[string]any`)
- Self-documenting with examples
- Enables tooling (CLI completion, UI rendering)

**Cons:**
- Additional complexity
- Labels validated at runtime not compile-time
- Schema must be flexible enough for varied value types

## Recommended Design: Metadata Label Registry

### Core Data Model

```go
// pkg/zhiplugin/labels/label.go

// Label defines a metadata label that plugins can interpret.
type Label struct {
    // Name is the fully qualified label name (e.g., "ui.readonly", "store.writeonly").
    // Convention: <namespace>.<name> where namespace indicates the plugin type
    // or custom namespace for plugin-specific labels.
    Name string `json:"name" yaml:"name"`

    // Namespace is the plugin type or custom namespace (e.g., "ui", "store", "transform", "mycompany").
    Namespace string `json:"namespace" yaml:"namespace"`

    // Description explains what this label does.
    Description string `json:"description" yaml:"description"`

    // ValueType describes the expected type: "bool", "string", "int", "float",
    // "string[]", "object", or "any".
    ValueType string `json:"value_type" yaml:"value_type"`

    // DefaultValue is the value assumed when the label is not present.
    DefaultValue any `json:"default_value,omitempty" yaml:"default_value,omitempty"`

    // Examples shows usage examples.
    Examples []LabelExample `json:"examples,omitempty" yaml:"examples,omitempty"`

    // Constraints defines validation rules (optional).
    Constraints *LabelConstraints `json:"constraints,omitempty" yaml:"constraints,omitempty"`

    // AppliesTo indicates which plugin types interpret this label.
    // Empty means informational only.
    AppliesTo []string `json:"applies_to,omitempty" yaml:"applies_to,omitempty"`

    // Since indicates the version when this label was introduced.
    Since string `json:"since,omitempty" yaml:"since,omitempty"`

    // Deprecated marks the label as deprecated with migration guidance.
    Deprecated string `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
}

// LabelExample shows how to use a label.
type LabelExample struct {
    Value       any    `json:"value" yaml:"value"`
    Description string `json:"description" yaml:"description"`
}

// LabelConstraints defines validation rules for label values.
type LabelConstraints struct {
    // For string types
    Pattern   string   `json:"pattern,omitempty" yaml:"pattern,omitempty"`     // Regex
    MinLength int      `json:"min_length,omitempty" yaml:"min_length,omitempty"`
    MaxLength int      `json:"max_length,omitempty" yaml:"max_length,omitempty"`
    Enum      []string `json:"enum,omitempty" yaml:"enum,omitempty"`           // Allowed values

    // For numeric types
    Min *float64 `json:"min,omitempty" yaml:"min,omitempty"`
    Max *float64 `json:"max,omitempty" yaml:"max,omitempty"`

    // For arrays
    MinItems int `json:"min_items,omitempty" yaml:"min_items,omitempty"`
    MaxItems int `json:"max_items,omitempty" yaml:"max_items,omitempty"`
}
```

### Registry Interface

```go
// pkg/zhiplugin/labels/registry.go

// Registry manages metadata label definitions.
type Registry struct {
    labels    map[string]*Label      // name -> label
    byNS      map[string][]*Label    // namespace -> labels
    mu        sync.RWMutex
}

// NewRegistry creates a registry with built-in labels pre-registered.
func NewRegistry() *Registry

// Register adds a label to the registry. Returns error if name already exists.
func (r *Registry) Register(label *Label) error

// MustRegister panics if registration fails. For use in init().
func (r *Registry) MustRegister(label *Label)

// Get returns a label by name, or nil if not found.
func (r *Registry) Get(name string) *Label

// List returns all registered labels.
func (r *Registry) List() []*Label

// ListByNamespace returns labels in a specific namespace.
func (r *Registry) ListByNamespace(namespace string) []*Label

// Namespaces returns all known namespaces.
func (r *Registry) Namespaces() []string

// Validate checks if a metadata value is valid for the given label.
func (r *Registry) Validate(name string, value any) error

// ValidateMetadata validates all labels in a metadata map.
func (r *Registry) ValidateMetadata(metadata map[string]any) []ValidationError
```

### Built-in Labels

```go
// pkg/zhiplugin/labels/builtin.go

// UI namespace - interpreted by UI plugins
var (
    // UIReadonly prevents modification in the UI.
    UIReadonly = &Label{
        Name:         "ui.readonly",
        Namespace:    "ui",
        Description:  "Prevents users from modifying this value in the UI. The value is displayed but not editable.",
        ValueType:    "bool",
        DefaultValue: false,
        AppliesTo:    []string{"ui"},
        Examples: []LabelExample{
            {Value: true, Description: "Make field read-only"},
        },
    }

    // UIPassword masks input characters.
    UIPassword = &Label{
        Name:        "ui.password",
        Namespace:   "ui",
        Description: "Masks input characters during entry (displays as dots or asterisks). The stored value is unaffected.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"ui"},
        Examples: []LabelExample{
            {Value: true, Description: "Mask password input"},
        },
    }

    // UIHidden hides the value from display entirely.
    UIHidden = &Label{
        Name:        "ui.hidden",
        Namespace:   "ui",
        Description: "Completely hides this value from the UI. Users cannot see or edit it.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"ui"},
    }

    // UIPattern provides a regex for input validation.
    UIPattern = &Label{
        Name:        "ui.pattern",
        Namespace:   "ui",
        Description: "Regular expression pattern that input must match. UI should validate and show errors.",
        ValueType:   "string",
        AppliesTo:   []string{"ui"},
        Examples: []LabelExample{
            {Value: "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$", Description: "Email pattern"},
            {Value: "^\\d{3}-\\d{3}-\\d{4}$", Description: "US phone number"},
        },
    }

    // UIMultiline indicates text should use multiline input.
    UIMultiline = &Label{
        Name:        "ui.multiline",
        Namespace:   "ui",
        Description: "Indicates the value is multiline text and should use a textarea or similar input.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"ui"},
    }

    // UIOrder specifies display order (lower numbers first).
    UIOrder = &Label{
        Name:        "ui.order",
        Namespace:   "ui",
        Description: "Display order hint for UI rendering. Lower numbers appear first.",
        ValueType:   "int",
        DefaultValue: 0,
        AppliesTo:   []string{"ui"},
    }

    // UIGroup groups related fields together.
    UIGroup = &Label{
        Name:        "ui.group",
        Namespace:   "ui",
        Description: "Groups related configuration values together in the UI.",
        ValueType:   "string",
        AppliesTo:   []string{"ui"},
        Examples: []LabelExample{
            {Value: "Database", Description: "Group database-related settings"},
            {Value: "Security", Description: "Group security-related settings"},
        },
    }
)

// Store namespace - interpreted by store plugins
var (
    // StoreWriteonly prevents reading the value back.
    StoreWriteonly = &Label{
        Name:        "store.writeonly",
        Namespace:   "store",
        Description: "Value can be written but not read back from the store. Useful for passwords and secrets that should never be retrieved.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"store"},
        Examples: []LabelExample{
            {Value: true, Description: "Mark password as write-only"},
        },
    }

    // StoreEncrypt forces encryption for this specific value.
    StoreEncrypt = &Label{
        Name:        "store.encrypt",
        Namespace:   "store",
        Description: "Forces encryption for this value even if store-wide encryption is not enabled. Requires store encryption capability.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"store"},
    }

    // StoreNoVersion excludes value from versioning.
    StoreNoVersion = &Label{
        Name:        "store.noversion",
        Namespace:   "store",
        Description: "Excludes this value from version history. Useful for frequently-changing values that don't need history.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"store"},
    }

    // StoreTTL sets time-to-live in seconds.
    StoreTTL = &Label{
        Name:        "store.ttl",
        Namespace:   "store",
        Description: "Time-to-live in seconds. Store may automatically delete the value after this duration.",
        ValueType:   "int",
        AppliesTo:   []string{"store"},
        Constraints: &LabelConstraints{
            Min: ptr(0.0),
        },
    }
)

// Transform namespace - interpreted by transform plugins
var (
    // TransformHidden prevents transform access.
    TransformHidden = &Label{
        Name:        "transform.hidden",
        Namespace:   "transform",
        Description: "Transform plugins cannot access this value. Useful for sensitive data that should bypass transformations.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"transform"},
    }

    // TransformSkip skips specific transforms.
    TransformSkip = &Label{
        Name:        "transform.skip",
        Namespace:   "transform",
        Description: "List of transform plugin names to skip for this value.",
        ValueType:   "string[]",
        AppliesTo:   []string{"transform"},
        Examples: []LabelExample{
            {Value: []string{"encryption", "compression"}, Description: "Skip encryption and compression transforms"},
        },
    }
)

// Config namespace - interpreted by config plugins
var (
    // ConfigRequired marks value as required.
    ConfigRequired = &Label{
        Name:        "config.required",
        Namespace:   "config",
        Description: "Value must be present and non-empty. Config plugin validation will fail if missing.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"config"},
    }

    // ConfigImmutable prevents changes after initial set.
    ConfigImmutable = &Label{
        Name:        "config.immutable",
        Namespace:   "config",
        Description: "Value cannot be changed after initial creation. Config plugin will reject updates.",
        ValueType:   "bool",
        DefaultValue: false,
        AppliesTo:   []string{"config"},
    }
)

// Core namespace - interpreted by the engine itself
var (
    // CoreDescription provides human-readable description.
    CoreDescription = &Label{
        Name:        "core.description",
        Namespace:   "core",
        Description: "Human-readable description of what this configuration value does.",
        ValueType:   "string",
        AppliesTo:   []string{"ui", "config"},
    }

    // CoreType hints at the semantic type.
    CoreType = &Label{
        Name:        "core.type",
        Namespace:   "core",
        Description: "Semantic type hint for the value (e.g., 'email', 'url', 'hostname', 'port', 'filepath').",
        ValueType:   "string",
        Constraints: &LabelConstraints{
            Enum: []string{"string", "int", "float", "bool", "email", "url", "hostname", "port", "filepath", "duration", "bytes"},
        },
    }

    // CoreDeprecated marks value as deprecated.
    CoreDeprecated = &Label{
        Name:        "core.deprecated",
        Namespace:   "core",
        Description: "Marks this configuration value as deprecated with migration guidance.",
        ValueType:   "string",
        Examples: []LabelExample{
            {Value: "Use database/connection_string instead", Description: "Deprecation with migration path"},
        },
    }

    // CoreDoc links to documentation.
    CoreDoc = &Label{
        Name:        "core.doc",
        Namespace:   "core",
        Description: "URL to documentation for this configuration value.",
        ValueType:   "string",
    }
)
```

### Plugin Label Declaration

Plugins declare which labels they interpret and can register custom labels:

```go
// Extended Plugin interface (optional method pattern)
type LabelProvider interface {
    // Labels returns metadata labels this plugin interprets.
    // This is introspection - plugins don't have to implement this.
    Labels(ctx context.Context) ([]*labels.Label, error)
}
```

For plugins that don't implement `LabelProvider`, the engine can still query the global registry for labels in that plugin's namespace.

### Custom Plugin Labels

Plugin developers register labels with their namespace:

```go
// In a custom store plugin
func init() {
    labels.DefaultRegistry.MustRegister(&labels.Label{
        Name:        "mystore.replicas",
        Namespace:   "mystore",
        Description: "Number of replicas to maintain for this value.",
        ValueType:   "int",
        DefaultValue: 1,
        AppliesTo:   []string{"store"},
        Constraints: &labels.LabelConstraints{
            Min: ptr(1.0),
            Max: ptr(10.0),
        },
    })
}
```

### gRPC Protocol Extension

```protobuf
// api/proto/zhiplugin/v1/labels.proto

syntax = "proto3";
package zhiplugin.v1;

option go_package = "github.com/zhi-io/zhi/pkg/zhiplugin/labels/proto";

message Label {
  string name = 1;
  string namespace = 2;
  string description = 3;
  string value_type = 4;
  bytes default_value_json = 5;
  repeated LabelExample examples = 6;
  LabelConstraints constraints = 7;
  repeated string applies_to = 8;
  string since = 9;
  string deprecated = 10;
}

message LabelExample {
  bytes value_json = 1;
  string description = 2;
}

message LabelConstraints {
  string pattern = 1;
  int32 min_length = 2;
  int32 max_length = 3;
  repeated string enum = 4;
  optional double min = 5;
  optional double max = 6;
  int32 min_items = 7;
  int32 max_items = 8;
}

message ListLabelsRequest {
  string namespace = 1;  // Optional: filter by namespace
}

message ListLabelsResponse {
  repeated Label labels = 1;
}

message GetLabelRequest {
  string name = 1;
}

message GetLabelResponse {
  Label label = 1;
  bool found = 2;
}

// LabelService can be implemented by plugins to declare their labels
service LabelService {
  rpc ListLabels(ListLabelsRequest) returns (ListLabelsResponse);
  rpc GetLabel(GetLabelRequest) returns (GetLabelResponse);
}
```

### CLI Integration

```bash
# List all available labels
$ zhi labels list
NAMESPACE   NAME              TYPE      DESCRIPTION
core        core.description  string    Human-readable description of what this configuration value does.
core        core.type         string    Semantic type hint for the value.
ui          ui.readonly       bool      Prevents users from modifying this value in the UI.
ui          ui.password       bool      Masks input characters during entry.
ui          ui.pattern        string    Regular expression pattern that input must match.
store       store.writeonly   bool      Value can be written but not read back from the store.
store       store.encrypt     bool      Forces encryption for this value.
transform   transform.hidden  bool      Transform plugins cannot access this value.
...

# List labels for a specific namespace
$ zhi labels list --namespace ui
NAME           TYPE    DEFAULT   DESCRIPTION
ui.readonly    bool    false     Prevents users from modifying this value in the UI.
ui.password    bool    false     Masks input characters during entry.
ui.hidden      bool    false     Completely hides this value from the UI.
ui.pattern     string  -         Regular expression pattern that input must match.
ui.multiline   bool    false     Indicates the value is multiline text.
ui.order       int     0         Display order hint for UI rendering.
ui.group       string  -         Groups related configuration values together.

# Show detailed info about a label
$ zhi labels info ui.pattern
Name:        ui.pattern
Namespace:   ui
Type:        string
Description: Regular expression pattern that input must match. UI should validate and show errors.
Applies to:  ui

Examples:
  - "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$"  (Email pattern)
  - "^\\d{3}-\\d{3}-\\d{4}$"                     (US phone number)

# Validate metadata
$ zhi labels validate --path database/password
Validating metadata for 'database/password':
  ui.password: true ✓
  store.writeonly: true ✓
  unknown.label: "foo" ⚠ Unknown label (not in registry)
```

### UI Integration

The UI plugin receives the label registry and can:

1. **Render fields appropriately** based on labels:
   - `ui.password` → mask input
   - `ui.readonly` → disable input
   - `ui.multiline` → use textarea
   - `ui.pattern` → validate on input

2. **Show label browser**: A help screen listing available labels with descriptions

3. **Provide autocompletion**: When editing metadata, suggest known labels

### Helper Functions for Plugin Developers

```go
// pkg/zhiplugin/labels/helpers.go

// GetBool returns a boolean label value with default fallback.
func GetBool(metadata map[string]any, name string, defaultVal bool) bool

// GetString returns a string label value with default fallback.
func GetString(metadata map[string]any, name string, defaultVal string) string

// GetInt returns an integer label value with default fallback.
func GetInt(metadata map[string]any, name string, defaultVal int) int

// GetStringSlice returns a string slice label value.
func GetStringSlice(metadata map[string]any, name string) []string

// IsReadonly checks if ui.readonly is set.
func IsReadonly(metadata map[string]any) bool

// IsPassword checks if ui.password is set.
func IsPassword(metadata map[string]any) bool

// IsWriteonly checks if store.writeonly is set.
func IsWriteonly(metadata map[string]any) bool

// IsTransformHidden checks if transform.hidden is set.
func IsTransformHidden(metadata map[string]any) bool
```

Example usage in a UI plugin:

```go
func (p *TUIPlugin) renderField(path string, value config.Value) {
    if labels.IsReadonly(value.Metadata) {
        p.renderReadonlyField(path, value)
        return
    }

    if labels.IsPassword(value.Metadata) {
        p.renderPasswordField(path, value)
        return
    }

    if pattern := labels.GetString(value.Metadata, "ui.pattern", ""); pattern != "" {
        p.renderPatternField(path, value, pattern)
        return
    }

    p.renderTextField(path, value)
}
```

### Engine Integration

The engine validates metadata labels during tree validation:

```go
// internal/core/engine.go

func (e *Engine) ValidateTree(ctx context.Context, tree *config.Tree) []ValidationResult {
    var results []ValidationResult

    // Existing validation...

    // Validate metadata labels
    for _, path := range tree.List() {
        val, _ := tree.Get(path)
        errs := labels.DefaultRegistry.ValidateMetadata(val.Metadata)
        for _, err := range errs {
            results = append(results, ValidationResult{
                Path:     path,
                Severity: Warning, // Unknown labels are warnings, not errors
                Message:  err.Error(),
            })
        }
    }

    return results
}
```

## Migration Strategy

1. **Phase 1**: Add label registry package with built-in labels
2. **Phase 2**: Add CLI commands for label discovery
3. **Phase 3**: Update TUI to use labels for rendering hints
4. **Phase 4**: Add optional gRPC service for plugin label declaration
5. **Phase 5**: Update documentation and examples

Existing metadata continues to work. Unknown labels produce warnings, not errors.

## Backward Compatibility

- Existing `map[string]any` metadata unchanged
- Old plugins work without modification
- Labels are opt-in: plugins can ignore them
- Unknown labels allowed (with warnings)
- No breaking changes to proto definitions

## Alternative: Metadata Attributes (Rejected)

We considered adding a separate `Attributes` field to `Value`:

```go
type Value struct {
    Val        any
    Metadata   map[string]any      // User-defined arbitrary data
    Attributes map[string]any      // System-defined label values
}
```

This was rejected because:
- Requires proto changes
- Unclear which field to use for what
- Adds confusion without clear benefit
- Labels in metadata is simpler and backward compatible

## Security Considerations

1. **Label injection**: Malicious config could set `store.writeonly` to hide data. Mitigated by:
   - Store plugins validating label semantics
   - Audit logging of label changes

2. **Namespace squatting**: Custom plugins could use `ui.*` labels. Mitigated by:
   - Convention: only use namespaces you own
   - Future: namespace ownership validation

3. **Pattern ReDoS**: `ui.pattern` regex could be malicious. Mitigated by:
   - Regex complexity limits
   - Timeout on pattern matching

## Open Questions

1. **Should labels be immutable once set?** Current design allows changing labels.

2. **Should we support label inheritance?** (e.g., all paths under `secrets/*` get `store.encrypt`)

3. **Should validation be strict or lenient?** Current: lenient (unknown labels = warning)

4. **Should labels affect export?** (e.g., `export.exclude` to skip values in exports)

## Summary

The Metadata Label Registry provides:

- **Discoverability**: CLI and UI can list available labels
- **Documentation**: Labels are self-documenting with descriptions and examples
- **Type safety**: Labels have defined schemas and validation
- **Extensibility**: Plugins can register custom labels
- **Backward compatibility**: Existing code works unchanged
- **Cross-plugin communication**: Standard labels enable interop

This design balances flexibility with structure, allowing the ecosystem to evolve while maintaining compatibility.
