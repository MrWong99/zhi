// Package labels provides a metadata label registry for zhi plugins.
//
// Labels are semantic annotations on configuration values that control
// how plugins interpret and handle those values. For example, "ui.readonly"
// tells UI plugins to prevent modification, while "store.writeonly" tells
// store plugins to accept writes but never return the value on read.
//
// Labels follow a namespace convention: <namespace>.<name> where namespace
// indicates the plugin type (ui, store, transform, config) or a custom
// namespace for plugin-specific labels.
package labels

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Label defines a metadata label that plugins can interpret.
type Label struct {
	// Name is the fully qualified label name (e.g., "ui.readonly", "store.writeonly").
	// Convention: <namespace>.<name> where namespace indicates the plugin type
	// or custom namespace for plugin-specific labels.
	Name string `json:"name" yaml:"name"`

	// Namespace is the plugin type or custom namespace (e.g., "ui", "store", "transform").
	Namespace string `json:"namespace" yaml:"namespace"`

	// Description explains what this label does.
	Description string `json:"description" yaml:"description"`

	// ValueType describes the expected type: "bool", "string", "int", "float",
	// "string[]", "object", or "any".
	ValueType string `json:"value_type" yaml:"value_type"`

	// DefaultValue is the value assumed when the label is not present.
	DefaultValue any `json:"default_value,omitempty" yaml:"default_value,omitempty"`

	// Examples shows usage examples.
	Examples []Example `json:"examples,omitempty" yaml:"examples,omitempty"`

	// Constraints defines validation rules (optional).
	Constraints *Constraints `json:"constraints,omitempty" yaml:"constraints,omitempty"`

	// AppliesTo indicates which plugin types interpret this label.
	// Empty means informational only.
	AppliesTo []string `json:"applies_to,omitempty" yaml:"applies_to,omitempty"`

	// Since indicates the version when this label was introduced.
	Since string `json:"since,omitempty" yaml:"since,omitempty"`

	// Deprecated marks the label as deprecated with migration guidance.
	Deprecated string `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
}

// Example shows how to use a label.
type Example struct {
	Value       any    `json:"value" yaml:"value"`
	Description string `json:"description" yaml:"description"`
}

// Constraints defines validation rules for label values.
type Constraints struct {
	// For string types
	Pattern   string   `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	MinLength int      `json:"min_length,omitempty" yaml:"min_length,omitempty"`
	MaxLength int      `json:"max_length,omitempty" yaml:"max_length,omitempty"`
	Enum      []string `json:"enum,omitempty" yaml:"enum,omitempty"`

	// For numeric types
	Min *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	Max *float64 `json:"max,omitempty" yaml:"max,omitempty"`

	// For arrays
	MinItems int `json:"min_items,omitempty" yaml:"min_items,omitempty"`
	MaxItems int `json:"max_items,omitempty" yaml:"max_items,omitempty"`
}

// ValidationError describes a validation failure for a label.
type ValidationError struct {
	Label   string `json:"label"`
	Value   any    `json:"value"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("label %q: %s", e.Label, e.Message)
}

// Validate checks if a value is valid for this label.
func (l *Label) Validate(value any) error {
	if value == nil {
		return nil // nil is always valid (treated as default)
	}

	switch l.ValueType {
	case "bool":
		if _, ok := value.(bool); !ok {
			return &ValidationError{
				Label:   l.Name,
				Value:   value,
				Message: fmt.Sprintf("expected bool, got %T", value),
			}
		}

	case "string":
		s, ok := value.(string)
		if !ok {
			return &ValidationError{
				Label:   l.Name,
				Value:   value,
				Message: fmt.Sprintf("expected string, got %T", value),
			}
		}
		if err := l.validateString(s); err != nil {
			return err
		}

	case "int":
		if err := l.validateNumeric(value); err != nil {
			return err
		}

	case "float":
		if err := l.validateNumeric(value); err != nil {
			return err
		}

	case "string[]":
		if err := l.validateStringSlice(value); err != nil {
			return err
		}

	case "object", "any":
		// Any value is valid
	}

	return nil
}

func (l *Label) validateString(s string) error {
	if l.Constraints == nil {
		return nil
	}

	c := l.Constraints

	if c.MinLength > 0 && len(s) < c.MinLength {
		return &ValidationError{
			Label:   l.Name,
			Value:   s,
			Message: fmt.Sprintf("string length %d is less than minimum %d", len(s), c.MinLength),
		}
	}

	if c.MaxLength > 0 && len(s) > c.MaxLength {
		return &ValidationError{
			Label:   l.Name,
			Value:   s,
			Message: fmt.Sprintf("string length %d exceeds maximum %d", len(s), c.MaxLength),
		}
	}

	if len(c.Enum) > 0 {
		found := slices.Contains(c.Enum, s)
		if !found {
			return &ValidationError{
				Label:   l.Name,
				Value:   s,
				Message: fmt.Sprintf("value %q not in allowed values: %v", s, c.Enum),
			}
		}
	}

	if c.Pattern != "" {
		re, err := regexp.Compile(c.Pattern)
		if err != nil {
			return &ValidationError{
				Label:   l.Name,
				Value:   s,
				Message: fmt.Sprintf("invalid pattern %q: %v", c.Pattern, err),
			}
		}
		if !re.MatchString(s) {
			return &ValidationError{
				Label:   l.Name,
				Value:   s,
				Message: fmt.Sprintf("value %q does not match pattern %q", s, c.Pattern),
			}
		}
	}

	return nil
}

func (l *Label) validateNumeric(value any) error {
	var f float64

	switch v := value.(type) {
	case int:
		f = float64(v)
	case int32:
		f = float64(v)
	case int64:
		f = float64(v)
	case float32:
		f = float64(v)
	case float64:
		f = v
	case json.Number:
		var err error
		f, err = v.Float64()
		if err != nil {
			return &ValidationError{
				Label:   l.Name,
				Value:   value,
				Message: fmt.Sprintf("invalid number: %v", err),
			}
		}
	default:
		return &ValidationError{
			Label:   l.Name,
			Value:   value,
			Message: fmt.Sprintf("expected number, got %T", value),
		}
	}

	if l.Constraints == nil {
		return nil
	}

	c := l.Constraints

	if c.Min != nil && f < *c.Min {
		return &ValidationError{
			Label:   l.Name,
			Value:   value,
			Message: fmt.Sprintf("value %v is less than minimum %v", f, *c.Min),
		}
	}

	if c.Max != nil && f > *c.Max {
		return &ValidationError{
			Label:   l.Name,
			Value:   value,
			Message: fmt.Sprintf("value %v exceeds maximum %v", f, *c.Max),
		}
	}

	return nil
}

func (l *Label) validateStringSlice(value any) error {
	var items []string

	switch v := value.(type) {
	case []string:
		items = v
	case []any:
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return &ValidationError{
					Label:   l.Name,
					Value:   value,
					Message: fmt.Sprintf("expected string at index %d, got %T", i, item),
				}
			}
			items = append(items, s)
		}
	default:
		return &ValidationError{
			Label:   l.Name,
			Value:   value,
			Message: fmt.Sprintf("expected string array, got %T", value),
		}
	}

	if l.Constraints == nil {
		return nil
	}

	c := l.Constraints

	if c.MinItems > 0 && len(items) < c.MinItems {
		return &ValidationError{
			Label:   l.Name,
			Value:   value,
			Message: fmt.Sprintf("array has %d items, minimum is %d", len(items), c.MinItems),
		}
	}

	if c.MaxItems > 0 && len(items) > c.MaxItems {
		return &ValidationError{
			Label:   l.Name,
			Value:   value,
			Message: fmt.Sprintf("array has %d items, maximum is %d", len(items), c.MaxItems),
		}
	}

	return nil
}

// AppliesToPlugin checks if this label applies to a given plugin type.
func (l *Label) AppliesToPlugin(pluginType string) bool {
	if len(l.AppliesTo) == 0 {
		return true // No restriction means applies to all
	}
	for _, pt := range l.AppliesTo {
		if strings.EqualFold(pt, pluginType) {
			return true
		}
	}
	return false
}

// IsDeprecated returns true if this label is deprecated.
func (l *Label) IsDeprecated() bool {
	return l.Deprecated != ""
}
