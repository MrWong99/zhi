package labels

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Registry manages metadata label definitions.
type Registry struct {
	labels map[string]*Label   // name -> label
	byNS   map[string][]*Label // namespace -> labels
	mu     sync.RWMutex
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		labels: make(map[string]*Label),
		byNS:   make(map[string][]*Label),
	}
}

// DefaultRegistry is the global registry with built-in labels pre-registered.
var DefaultRegistry = NewRegistry()

func init() {
	// Register all built-in labels
	registerBuiltinLabels(DefaultRegistry)
}

// Register adds a label to the registry.
// Returns error if a label with the same name already exists.
func (r *Registry) Register(label *Label) error {
	if label == nil {
		return fmt.Errorf("label cannot be nil")
	}
	if label.Name == "" {
		return fmt.Errorf("label name cannot be empty")
	}
	if label.Namespace == "" {
		// Extract namespace from name
		parts := strings.SplitN(label.Name, ".", 2)
		if len(parts) < 2 {
			return fmt.Errorf("label name %q must contain a namespace (e.g., 'ui.readonly')", label.Name)
		}
		label.Namespace = parts[0]
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.labels[label.Name]; exists {
		return fmt.Errorf("label %q already registered", label.Name)
	}

	r.labels[label.Name] = label
	r.byNS[label.Namespace] = append(r.byNS[label.Namespace], label)

	return nil
}

// MustRegister panics if registration fails. For use in init().
func (r *Registry) MustRegister(label *Label) {
	if err := r.Register(label); err != nil {
		panic(fmt.Sprintf("failed to register label: %v", err))
	}
}

// Get returns a label by name, or nil if not found.
func (r *Registry) Get(name string) *Label {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.labels[name]
}

// Has returns true if a label with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.labels[name]
	return exists
}

// List returns all registered labels, sorted by name.
func (r *Registry) List() []*Label {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Label, 0, len(r.labels))
	for _, label := range r.labels {
		result = append(result, label)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// ListByNamespace returns labels in a specific namespace, sorted by name.
func (r *Registry) ListByNamespace(namespace string) []*Label {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := slices.Clone(r.byNS[namespace])

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// Namespaces returns all known namespaces, sorted alphabetically.
func (r *Registry) Namespaces() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.byNS))
}

// Validate checks if a metadata value is valid for the given label.
// Returns nil if the label is not registered (unknown labels are allowed).
func (r *Registry) Validate(name string, value any) error {
	r.mu.RLock()
	label, exists := r.labels[name]
	r.mu.RUnlock()

	if !exists {
		return nil // Unknown labels are allowed
	}

	return label.Validate(value)
}

// ValidateMetadata validates all labels in a metadata map.
// Returns a slice of validation errors (empty if all valid).
// Unknown labels are not treated as errors but are returned as warnings.
func (r *Registry) ValidateMetadata(metadata map[string]any) []ValidationError {
	if metadata == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var errors []ValidationError

	for name, value := range metadata {
		label, exists := r.labels[name]
		if !exists {
			// Unknown label - could be a typo or custom label
			// We don't treat this as an error, but callers might want to warn
			continue
		}

		if err := label.Validate(value); err != nil {
			if ve, ok := err.(*ValidationError); ok {
				errors = append(errors, *ve)
			}
		}
	}

	return errors
}

// UnknownLabels returns label names in metadata that are not in the registry.
func (r *Registry) UnknownLabels(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var unknown []string
	for name := range metadata {
		// Skip non-label keys (ones without a namespace separator)
		if !strings.Contains(name, ".") {
			continue
		}
		if _, exists := r.labels[name]; !exists {
			unknown = append(unknown, name)
		}
	}

	sort.Strings(unknown)
	return unknown
}

// Clone creates a copy of the registry.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clone := NewRegistry()
	for name, label := range r.labels {
		labelCopy := *label
		clone.labels[name] = &labelCopy
		clone.byNS[label.Namespace] = append(clone.byNS[label.Namespace], &labelCopy)
	}

	return clone
}

// Merge adds all labels from another registry.
// Returns error if there are name conflicts.
func (r *Registry) Merge(other *Registry) error {
	if other == nil {
		return nil
	}

	other.mu.RLock()
	labels := make([]*Label, 0, len(other.labels))
	for _, label := range other.labels {
		labels = append(labels, label)
	}
	other.mu.RUnlock()

	for _, label := range labels {
		if err := r.Register(label); err != nil {
			return err
		}
	}

	return nil
}
