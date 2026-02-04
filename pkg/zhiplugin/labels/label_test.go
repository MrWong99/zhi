package labels

import (
	"testing"
)

func TestLabel_Validate_Bool(t *testing.T) {
	label := &Label{
		Name:      "test.bool",
		Namespace: "test",
		ValueType: "bool",
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"nil", nil, false},
		{"string", "true", true},
		{"int", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := label.Validate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLabel_Validate_String(t *testing.T) {
	label := &Label{
		Name:      "test.string",
		Namespace: "test",
		ValueType: "string",
		Constraints: &Constraints{
			MinLength: 2,
			MaxLength: 10,
		},
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid", "hello", false},
		{"nil", nil, false},
		{"too short", "a", true},
		{"too long", "this is way too long", true},
		{"int", 123, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := label.Validate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLabel_Validate_StringEnum(t *testing.T) {
	label := &Label{
		Name:      "test.enum",
		Namespace: "test",
		ValueType: "string",
		Constraints: &Constraints{
			Enum: []string{"red", "green", "blue"},
		},
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"red", "red", false},
		{"green", "green", false},
		{"blue", "blue", false},
		{"yellow", "yellow", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := label.Validate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLabel_Validate_StringPattern(t *testing.T) {
	label := &Label{
		Name:      "test.pattern",
		Namespace: "test",
		ValueType: "string",
		Constraints: &Constraints{
			Pattern: `^[a-z]+$`,
		},
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid lowercase", "hello", false},
		{"uppercase", "Hello", true},
		{"numbers", "123", true},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := label.Validate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLabel_Validate_Int(t *testing.T) {
	min := 0.0
	max := 100.0
	label := &Label{
		Name:      "test.int",
		Namespace: "test",
		ValueType: "int",
		Constraints: &Constraints{
			Min: &min,
			Max: &max,
		},
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid int", 50, false},
		{"valid float", 50.0, false},
		{"zero", 0, false},
		{"max", 100, false},
		{"below min", -1, true},
		{"above max", 101, true},
		{"nil", nil, false},
		{"string", "50", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := label.Validate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLabel_Validate_StringSlice(t *testing.T) {
	label := &Label{
		Name:      "test.strings",
		Namespace: "test",
		ValueType: "string[]",
		Constraints: &Constraints{
			MinItems: 1,
			MaxItems: 3,
		},
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"valid string slice", []string{"a", "b"}, false},
		{"valid any slice", []any{"a", "b"}, false},
		{"one item", []string{"a"}, false},
		{"three items", []string{"a", "b", "c"}, false},
		{"empty", []string{}, true},
		{"too many", []string{"a", "b", "c", "d"}, true},
		{"nil", nil, false},
		{"not slice", "a", true},
		{"mixed types", []any{"a", 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := label.Validate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLabel_AppliesToPlugin(t *testing.T) {
	tests := []struct {
		name       string
		appliesTo  []string
		pluginType string
		want       bool
	}{
		{"empty applies to all", nil, "ui", true},
		{"empty applies to all 2", nil, "store", true},
		{"matches ui", []string{"ui"}, "ui", true},
		{"matches store", []string{"store"}, "store", true},
		{"multiple match", []string{"ui", "store"}, "ui", true},
		{"no match", []string{"ui"}, "store", false},
		{"case insensitive", []string{"UI"}, "ui", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := &Label{
				Name:      "test.label",
				AppliesTo: tt.appliesTo,
			}
			if got := label.AppliesToPlugin(tt.pluginType); got != tt.want {
				t.Errorf("AppliesToPlugin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabel_IsDeprecated(t *testing.T) {
	tests := []struct {
		name       string
		deprecated string
		want       bool
	}{
		{"not deprecated", "", false},
		{"deprecated", "use X instead", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := &Label{
				Name:       "test.label",
				Deprecated: tt.deprecated,
			}
			if got := label.IsDeprecated(); got != tt.want {
				t.Errorf("IsDeprecated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Label:   "test.label",
		Value:   "bad",
		Message: "invalid value",
	}

	want := `label "test.label": invalid value`
	if got := err.Error(); got != want {
		t.Errorf("Error() = %v, want %v", got, want)
	}
}
