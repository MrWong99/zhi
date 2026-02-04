package labels

import (
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.labels == nil {
		t.Error("labels map is nil")
	}
	if r.byNS == nil {
		t.Error("byNS map is nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name    string
		label   *Label
		wantErr bool
	}{
		{
			name: "valid label",
			label: &Label{
				Name:        "test.valid",
				Namespace:   "test",
				Description: "A valid label",
				ValueType:   "bool",
			},
			wantErr: false,
		},
		{
			name: "auto-extract namespace",
			label: &Label{
				Name:        "auto.namespace",
				Description: "Namespace from name",
				ValueType:   "string",
			},
			wantErr: false,
		},
		{
			name:    "nil label",
			label:   nil,
			wantErr: true,
		},
		{
			name: "empty name",
			label: &Label{
				Name:      "",
				Namespace: "test",
			},
			wantErr: true,
		},
		{
			name: "no namespace in name",
			label: &Label{
				Name: "nonamespace",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			err := r.Register(tt.label)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewRegistry()

	label := &Label{
		Name:      "test.dup",
		Namespace: "test",
		ValueType: "bool",
	}

	if err := r.Register(label); err != nil {
		t.Fatalf("First register failed: %v", err)
	}

	if err := r.Register(label); err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	label := &Label{
		Name:      "test.get",
		Namespace: "test",
		ValueType: "string",
	}
	r.MustRegister(label)

	tests := []struct {
		name     string
		labelKey string
		want     bool
	}{
		{"existing", "test.get", true},
		{"non-existing", "test.nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Get(tt.labelKey)
			if (got != nil) != tt.want {
				t.Errorf("Get() found = %v, want %v", got != nil, tt.want)
			}
		})
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{Name: "test.exists", ValueType: "bool"})

	if !r.Has("test.exists") {
		t.Error("Has() should return true for existing label")
	}
	if r.Has("test.notexists") {
		t.Error("Has() should return false for non-existing label")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{Name: "b.label", ValueType: "bool"})
	r.MustRegister(&Label{Name: "a.label", ValueType: "bool"})
	r.MustRegister(&Label{Name: "c.label", ValueType: "bool"})

	labels := r.List()
	if len(labels) != 3 {
		t.Errorf("List() returned %d labels, want 3", len(labels))
	}

	// Check sorted order
	if labels[0].Name != "a.label" || labels[1].Name != "b.label" || labels[2].Name != "c.label" {
		t.Error("List() should return labels sorted by name")
	}
}

func TestRegistry_ListByNamespace(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{Name: "ui.a", Namespace: "ui", ValueType: "bool"})
	r.MustRegister(&Label{Name: "ui.b", Namespace: "ui", ValueType: "bool"})
	r.MustRegister(&Label{Name: "store.a", Namespace: "store", ValueType: "bool"})

	uiLabels := r.ListByNamespace("ui")
	if len(uiLabels) != 2 {
		t.Errorf("ListByNamespace(ui) returned %d labels, want 2", len(uiLabels))
	}

	storeLabels := r.ListByNamespace("store")
	if len(storeLabels) != 1 {
		t.Errorf("ListByNamespace(store) returned %d labels, want 1", len(storeLabels))
	}

	emptyLabels := r.ListByNamespace("nonexistent")
	if len(emptyLabels) != 0 {
		t.Errorf("ListByNamespace(nonexistent) returned %d labels, want 0", len(emptyLabels))
	}
}

func TestRegistry_Namespaces(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{Name: "ui.a", Namespace: "ui", ValueType: "bool"})
	r.MustRegister(&Label{Name: "store.a", Namespace: "store", ValueType: "bool"})
	r.MustRegister(&Label{Name: "config.a", Namespace: "config", ValueType: "bool"})

	ns := r.Namespaces()
	if len(ns) != 3 {
		t.Errorf("Namespaces() returned %d namespaces, want 3", len(ns))
	}

	// Check sorted order
	if ns[0] != "config" || ns[1] != "store" || ns[2] != "ui" {
		t.Errorf("Namespaces() = %v, want [config store ui]", ns)
	}
}

func TestRegistry_Validate(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{
		Name:      "test.bool",
		Namespace: "test",
		ValueType: "bool",
	})

	// Valid value
	if err := r.Validate("test.bool", true); err != nil {
		t.Errorf("Validate() for valid bool should not error: %v", err)
	}

	// Invalid value
	if err := r.Validate("test.bool", "not a bool"); err == nil {
		t.Error("Validate() for invalid bool should error")
	}

	// Unknown label should not error
	if err := r.Validate("unknown.label", "anything"); err != nil {
		t.Errorf("Validate() for unknown label should not error: %v", err)
	}
}

func TestRegistry_ValidateMetadata(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{
		Name:      "test.bool",
		Namespace: "test",
		ValueType: "bool",
	})
	r.MustRegister(&Label{
		Name:      "test.string",
		Namespace: "test",
		ValueType: "string",
		Constraints: &Constraints{
			MinLength: 3,
		},
	})

	tests := []struct {
		name     string
		metadata map[string]any
		wantErrs int
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			wantErrs: 0,
		},
		{
			name: "all valid",
			metadata: map[string]any{
				"test.bool":   true,
				"test.string": "hello",
			},
			wantErrs: 0,
		},
		{
			name: "one invalid",
			metadata: map[string]any{
				"test.bool":   "not a bool",
				"test.string": "hello",
			},
			wantErrs: 1,
		},
		{
			name: "two invalid",
			metadata: map[string]any{
				"test.bool":   "not a bool",
				"test.string": "ab",
			},
			wantErrs: 2,
		},
		{
			name: "unknown label ignored",
			metadata: map[string]any{
				"unknown.label": "anything",
			},
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := r.ValidateMetadata(tt.metadata)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateMetadata() returned %d errors, want %d", len(errs), tt.wantErrs)
			}
		})
	}
}

func TestRegistry_UnknownLabels(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{Name: "known.label", ValueType: "bool"})

	tests := []struct {
		name     string
		metadata map[string]any
		want     []string
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			want:     nil,
		},
		{
			name: "all known",
			metadata: map[string]any{
				"known.label": true,
			},
			want: nil,
		},
		{
			name: "one unknown",
			metadata: map[string]any{
				"known.label":   true,
				"unknown.label": "foo",
			},
			want: []string{"unknown.label"},
		},
		{
			name: "non-label keys ignored",
			metadata: map[string]any{
				"description": "no namespace, not a label",
				"type":        "also not a label",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.UnknownLabels(tt.metadata)
			if len(got) != len(tt.want) {
				t.Errorf("UnknownLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistry_Clone(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Label{Name: "test.clone", ValueType: "bool"})

	clone := r.Clone()

	// Clone should have the same labels
	if clone.Get("test.clone") == nil {
		t.Error("Clone should have test.clone label")
	}

	// Modifying clone shouldn't affect original
	clone.MustRegister(&Label{Name: "test.cloneonly", ValueType: "string"})
	if r.Get("test.cloneonly") != nil {
		t.Error("Original should not have test.cloneonly label")
	}
}

func TestRegistry_Merge(t *testing.T) {
	r1 := NewRegistry()
	r1.MustRegister(&Label{Name: "r1.label", ValueType: "bool"})

	r2 := NewRegistry()
	r2.MustRegister(&Label{Name: "r2.label", ValueType: "string"})

	if err := r1.Merge(r2); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	if r1.Get("r1.label") == nil {
		t.Error("Merge should preserve original labels")
	}
	if r1.Get("r2.label") == nil {
		t.Error("Merge should add merged labels")
	}
}

func TestRegistry_Merge_Conflict(t *testing.T) {
	r1 := NewRegistry()
	r1.MustRegister(&Label{Name: "conflict.label", ValueType: "bool"})

	r2 := NewRegistry()
	r2.MustRegister(&Label{Name: "conflict.label", ValueType: "string"})

	if err := r1.Merge(r2); err == nil {
		t.Error("Merge should error on conflicts")
	}
}

func TestDefaultRegistry_HasBuiltinLabels(t *testing.T) {
	// Check that DefaultRegistry has built-in labels
	wellKnown := []string{
		"ui.readonly",
		"ui.password",
		"store.writeonly",
		"transform.hidden",
		"config.required",
		"core.description",
	}

	for _, name := range wellKnown {
		if DefaultRegistry.Get(name) == nil {
			t.Errorf("DefaultRegistry should have built-in label %q", name)
		}
	}
}

func TestDefaultRegistry_Namespaces(t *testing.T) {
	ns := DefaultRegistry.Namespaces()

	expected := []string{"config", "core", "store", "transform", "ui"}
	if len(ns) != len(expected) {
		t.Errorf("Namespaces() = %v, want %v", ns, expected)
	}

	for i, n := range expected {
		if ns[i] != n {
			t.Errorf("Namespaces()[%d] = %q, want %q", i, ns[i], n)
		}
	}
}
