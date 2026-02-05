// Package config defines the configuration plugin API for zhi.
package config

import (
	"errors"
	"regexp"
	"strings"
)

// pathSegment matches a single path segment: starts with a lowercase letter,
// may contain lowercase letters, digits, dots, hyphens and underscores, and
// ends with a lowercase letter or digit. Single-character segments are valid.
var pathSegment = regexp.MustCompile(`^[a-z](?:[a-z0-9._-]*[a-z0-9])?$`)

// ValidatePath checks whether path is a valid slash-delimited configuration
// path. Each segment must match [a-z][a-z0-9._-]*[a-z0-9].
func ValidatePath(path string) error {
	if path == "" {
		return errors.New("path must not be empty")
	}
	for seg := range strings.SplitSeq(path, "/") {
		if seg == "" {
			return errors.New("path must not contain empty segments")
		}
		if !pathSegment.MatchString(seg) {
			return errors.New("invalid path segment: " + seg)
		}
	}
	return nil
}

// Severity indicates how critical a validation finding is.
type Severity int

const (
	// Info is a non-blocking informational finding.
	Info Severity = iota
	// Warning signals a potential problem but does not block.
	Warning
	// Blocking prevents the configuration from being accepted.
	Blocking
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Blocking:
		return "blocking"
	default:
		return "unknown"
	}
}

// ValidationResult is the outcome of a single validation rule.
type ValidationResult struct {
	Severity Severity       `json:"severity" yaml:"severity"`
	Message  string         `json:"message" yaml:"message"`
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ValidateFunc validates a single configuration value.
// The tree parameter gives read access to the full configuration tree so
// that cross-value validation is possible. Implementations that only need
// the local value can ignore it.
// The result may be empty if the value is valid.
type ValidateFunc func(value any, tree TreeReader) []ValidationResult

// TreeReader provides read-only access to the configuration tree.
// Returned Values are copies; modifying them does not affect the tree.
type TreeReader interface {
	// Get returns a copy of the Value at the given slash-delimited path.
	// The bool reports whether the path exists. For example "database/host".
	Get(path string) (Value, bool)
	// List returns every path present in the tree.
	List() []string
}

// Value is a single node in the configuration tree.
type Value struct {
	// Val holds the configuration data. It must be a type that can be
	// marshalled to both JSON and YAML (primitive types, slices, maps, or
	// structs with the appropriate tags).
	Val any `json:"value" yaml:"value"`

	// Metadata carries optional, arbitrary key-value information about this
	// configuration entry (descriptions, display hints, etc.).
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Version is an opaque version identifier set by store plugins when
	// reading values. Store plugins use it for automatic optimistic locking
	// (check-and-set): if Version is non-empty when writing, the store
	// rejects the write if the current version differs.
	// This field is transient and not serialised to JSON/YAML.
	Version string `json:"-" yaml:"-"`

	// Validators is an optional set of validation functions executed in
	// order. The field is not serialised.
	Validators []ValidateFunc `json:"-" yaml:"-"`
}

// Validate runs every registered validator and returns the aggregated
// results.
func (v *Value) Validate(tree TreeReader) []ValidationResult {
	var results []ValidationResult
	for _, fn := range v.Validators {
		results = append(results, fn(v.Val, tree)...)
	}
	return results
}

// Tree is a configuration tree. Paths are slash-delimited
// (e.g. "database/host" or "app/tls/cert.pem").
type Tree struct {
	values map[string]*Value
}

// NewTree returns an empty configuration tree.
func NewTree() *Tree {
	return &Tree{values: make(map[string]*Value)}
}

// Set stores a value at the given path, replacing any previous entry.
// It returns an error if the path is invalid.
func (t *Tree) Set(path string, v *Value) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	t.values[path] = v
	return nil
}

// GetPtr returns a pointer to the Value stored at path. The bool reports
// whether the path exists. Because a pointer is returned the caller can
// mutate the value in place. Use Get for a safe, read-only copy instead.
func (t *Tree) GetPtr(path string) (*Value, bool) {
	v, ok := t.values[path]
	return v, ok
}

// Get returns a copy of the Value at path. The bool reports whether the
// path exists.
func (t *Tree) Get(path string) (Value, bool) {
	v, ok := t.GetPtr(path)
	if !ok {
		return Value{}, false
	}
	return *v, true
}

// Delete removes the value at path from the tree. It is a no-op if the
// path does not exist.
func (t *Tree) Delete(path string) {
	delete(t.values, path)
}

// List returns all paths present in the tree in no guaranteed order.
func (t *Tree) List() []string {
	paths := make([]string, 0, len(t.values))
	for p := range t.values {
		paths = append(paths, p)
	}
	return paths
}

// Validate runs every value's validators and returns all results keyed by
// path.
func (t *Tree) Validate() map[string][]ValidationResult {
	out := make(map[string][]ValidationResult)
	for path, v := range t.values {
		if results := v.Validate(t); len(results) > 0 {
			out[path] = results
		}
	}
	return out
}
