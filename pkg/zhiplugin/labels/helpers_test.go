package labels

import (
	"reflect"
	"testing"
)

func TestGetBool(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]any
		key        string
		defaultVal bool
		want       bool
	}{
		{"true value", map[string]any{"key": true}, "key", false, true},
		{"false value", map[string]any{"key": false}, "key", true, false},
		{"missing key", map[string]any{}, "key", true, true},
		{"nil metadata", nil, "key", true, true},
		{"wrong type", map[string]any{"key": "true"}, "key", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetBool(tt.metadata, tt.key, tt.defaultVal); got != tt.want {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]any
		key        string
		defaultVal string
		want       string
	}{
		{"string value", map[string]any{"key": "hello"}, "key", "default", "hello"},
		{"missing key", map[string]any{}, "key", "default", "default"},
		{"nil metadata", nil, "key", "default", "default"},
		{"wrong type", map[string]any{"key": 123}, "key", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetString(tt.metadata, tt.key, tt.defaultVal); got != tt.want {
				t.Errorf("GetString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]any
		key        string
		defaultVal int
		want       int
	}{
		{"int value", map[string]any{"key": 42}, "key", 0, 42},
		{"int32 value", map[string]any{"key": int32(42)}, "key", 0, 42},
		{"int64 value", map[string]any{"key": int64(42)}, "key", 0, 42},
		{"float64 value", map[string]any{"key": 42.0}, "key", 0, 42},
		{"string value", map[string]any{"key": "42"}, "key", 0, 42},
		{"invalid string", map[string]any{"key": "abc"}, "key", 99, 99},
		{"missing key", map[string]any{}, "key", 99, 99},
		{"nil metadata", nil, "key", 99, 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetInt(tt.metadata, tt.key, tt.defaultVal); got != tt.want {
				t.Errorf("GetInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]any
		key        string
		defaultVal float64
		want       float64
	}{
		{"float64 value", map[string]any{"key": 3.14}, "key", 0, 3.14},
		{"float32 value", map[string]any{"key": float32(3.14)}, "key", 0, float64(float32(3.14))},
		{"int value", map[string]any{"key": 42}, "key", 0, 42.0},
		{"string value", map[string]any{"key": "3.14"}, "key", 0, 3.14},
		{"invalid string", map[string]any{"key": "abc"}, "key", 99.9, 99.9},
		{"missing key", map[string]any{}, "key", 99.9, 99.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetFloat(tt.metadata, tt.key, tt.defaultVal); got != tt.want {
				t.Errorf("GetFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		key      string
		want     []string
	}{
		{"string slice", map[string]any{"key": []string{"a", "b"}}, "key", []string{"a", "b"}},
		{"any slice", map[string]any{"key": []any{"a", "b"}}, "key", []string{"a", "b"}},
		{"mixed types ignored", map[string]any{"key": []any{"a", 1}}, "key", []string{"a"}},
		{"missing key", map[string]any{}, "key", nil},
		{"nil metadata", nil, "key", nil},
		{"wrong type", map[string]any{"key": "not a slice"}, "key", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStringSlice(tt.metadata, tt.key)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetStringSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAny(t *testing.T) {
	metadata := map[string]any{
		"exists":   "value",
		"nilValue": nil,
	}

	t.Run("existing key", func(t *testing.T) {
		val, ok := GetAny(metadata, "exists")
		if !ok || val != "value" {
			t.Errorf("GetAny() = %v, %v, want value, true", val, ok)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		val, ok := GetAny(metadata, "nilValue")
		if !ok || val != nil {
			t.Errorf("GetAny() = %v, %v, want nil, true", val, ok)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		_, ok := GetAny(metadata, "missing")
		if ok {
			t.Error("GetAny() should return false for missing key")
		}
	})

	t.Run("nil metadata", func(t *testing.T) {
		_, ok := GetAny(nil, "key")
		if ok {
			t.Error("GetAny() should return false for nil metadata")
		}
	})
}

func TestConvenienceFunctions(t *testing.T) {
	metadata := map[string]any{
		LabelUIReadonly:      true,
		LabelUIPassword:      true,
		LabelUIHidden:        true,
		LabelUIMultiline:     true,
		LabelStoreWriteonly:  true,
		LabelStoreEncrypt:    true,
		LabelTransformHidden: true,
		LabelConfigRequired:  true,
		LabelConfigImmutable: true,
		LabelCoreDeprecated:  "use X instead",
		LabelCoreDescription: "A description",
		LabelUIPattern:       "[a-z]+",
		LabelUIGroup:         "Security",
		LabelUIOrder:         10,
		LabelStoreTTL:        3600,
		LabelCoreType:        "email",
		LabelCoreDoc:         "https://example.com",
		LabelConfigEnv:       "MY_VAR",
		LabelUIConfirm:       true,
	}

	tests := []struct {
		name string
		fn   func() bool
		want bool
	}{
		{"IsReadonly", func() bool { return IsReadonly(metadata) }, true},
		{"IsPassword", func() bool { return IsPassword(metadata) }, true},
		{"IsHidden", func() bool { return IsHidden(metadata) }, true},
		{"IsMultiline", func() bool { return IsMultiline(metadata) }, true},
		{"IsWriteonly", func() bool { return IsWriteonly(metadata) }, true},
		{"ShouldEncrypt", func() bool { return ShouldEncrypt(metadata) }, true},
		{"IsTransformHidden", func() bool { return IsTransformHidden(metadata) }, true},
		{"IsRequired", func() bool { return IsRequired(metadata) }, true},
		{"IsImmutable", func() bool { return IsImmutable(metadata) }, true},
		{"IsDeprecatedValue", func() bool { return IsDeprecatedValue(metadata) }, true},
		{"ShouldConfirm", func() bool { return ShouldConfirm(metadata) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(); got != tt.want {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}

	// Test string getters
	if got := GetDescription(metadata); got != "A description" {
		t.Errorf("GetDescription() = %v, want %v", got, "A description")
	}
	if got := GetPattern(metadata); got != "[a-z]+" {
		t.Errorf("GetPattern() = %v, want %v", got, "[a-z]+")
	}
	if got := GetGroup(metadata); got != "Security" {
		t.Errorf("GetGroup() = %v, want %v", got, "Security")
	}
	if got := GetSemanticType(metadata); got != "email" {
		t.Errorf("GetSemanticType() = %v, want %v", got, "email")
	}
	if got := GetDocURL(metadata); got != "https://example.com" {
		t.Errorf("GetDocURL() = %v, want %v", got, "https://example.com")
	}
	if got := GetEnvOverride(metadata); got != "MY_VAR" {
		t.Errorf("GetEnvOverride() = %v, want %v", got, "MY_VAR")
	}

	// Test int getters
	if got := GetOrder(metadata); got != 10 {
		t.Errorf("GetOrder() = %v, want %v", got, 10)
	}
	if got := GetTTL(metadata); got != 3600 {
		t.Errorf("GetTTL() = %v, want %v", got, 3600)
	}
}

func TestConvenienceFunctions_Defaults(t *testing.T) {
	metadata := map[string]any{}

	// All bool functions should return false for empty metadata
	if IsReadonly(metadata) {
		t.Error("IsReadonly should return false for empty metadata")
	}
	if IsPassword(metadata) {
		t.Error("IsPassword should return false for empty metadata")
	}
	if IsDeprecatedValue(metadata) {
		t.Error("IsDeprecatedValue should return false for empty metadata")
	}

	// String functions should return empty string
	if GetDescription(metadata) != "" {
		t.Error("GetDescription should return empty string for empty metadata")
	}

	// Int functions should return 0
	if GetOrder(metadata) != 0 {
		t.Error("GetOrder should return 0 for empty metadata")
	}
}

func TestBuilder(t *testing.T) {
	meta := NewBuilder().
		Readonly().
		Password().
		Writeonly().
		Required().
		Description("Test value").
		Pattern("[a-z]+").
		Group("Test").
		Order(5).
		SemanticType("email").
		TTL(3600).
		Build()

	if !IsReadonly(meta) {
		t.Error("Builder.Readonly() should set ui.readonly")
	}
	if !IsPassword(meta) {
		t.Error("Builder.Password() should set ui.password")
	}
	if !IsWriteonly(meta) {
		t.Error("Builder.Writeonly() should set store.writeonly")
	}
	if !IsRequired(meta) {
		t.Error("Builder.Required() should set config.required")
	}
	if GetDescription(meta) != "Test value" {
		t.Errorf("Builder.Description() = %v, want Test value", GetDescription(meta))
	}
	if GetPattern(meta) != "[a-z]+" {
		t.Errorf("Builder.Pattern() = %v, want [a-z]+", GetPattern(meta))
	}
	if GetGroup(meta) != "Test" {
		t.Errorf("Builder.Group() = %v, want Test", GetGroup(meta))
	}
	if GetOrder(meta) != 5 {
		t.Errorf("Builder.Order() = %v, want 5", GetOrder(meta))
	}
	if GetSemanticType(meta) != "email" {
		t.Errorf("Builder.SemanticType() = %v, want email", GetSemanticType(meta))
	}
	if GetTTL(meta) != 3600 {
		t.Errorf("Builder.TTL() = %v, want 3600", GetTTL(meta))
	}
}

func TestBuilder_FromMetadata(t *testing.T) {
	existing := map[string]any{
		"existing.key": "value",
	}

	meta := FromMetadata(existing).
		Readonly().
		Build()

	if meta["existing.key"] != "value" {
		t.Error("FromMetadata should preserve existing keys")
	}
	if !IsReadonly(meta) {
		t.Error("Builder should add new keys")
	}

	// Verify original wasn't modified
	if existing[LabelUIReadonly] == true {
		t.Error("FromMetadata should not modify original")
	}
}

func TestBuilder_Set(t *testing.T) {
	meta := NewBuilder().
		Set("custom.label", "custom value").
		Set("another.label", 42).
		Build()

	if meta["custom.label"] != "custom value" {
		t.Errorf("Set() should set custom.label")
	}
	if meta["another.label"] != 42 {
		t.Errorf("Set() should set another.label")
	}
}

func TestBuilder_AllMethods(t *testing.T) {
	// Test all builder methods compile and work
	meta := NewBuilder().
		Readonly().
		Password().
		Hidden().
		Writeonly().
		Encrypt().
		Required().
		Immutable().
		TransformHidden().
		Description("desc").
		Pattern("pattern").
		Group("group").
		Order(1).
		SemanticType("string").
		Doc("https://example.com").
		EnvOverride("MY_VAR").
		DefaultValue("default").
		TTL(100).
		Multiline().
		Confirm().
		Deprecated("use X").
		Build()

	// Just verify it built something
	if len(meta) == 0 {
		t.Error("Builder should produce non-empty metadata")
	}
}

func TestGetTransformSkip(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		want     []string
	}{
		{
			name: "string slice",
			metadata: map[string]any{
				LabelTransformSkip: []string{"a", "b"},
			},
			want: []string{"a", "b"},
		},
		{
			name: "any slice",
			metadata: map[string]any{
				LabelTransformSkip: []any{"a", "b"},
			},
			want: []string{"a", "b"},
		},
		{
			name:     "missing",
			metadata: map[string]any{},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTransformSkip(tt.metadata)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTransformSkip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDefaultValue(t *testing.T) {
	t.Run("with default", func(t *testing.T) {
		meta := map[string]any{LabelConfigDefault: "mydefault"}
		val, ok := GetDefaultValue(meta)
		if !ok || val != "mydefault" {
			t.Errorf("GetDefaultValue() = %v, %v, want mydefault, true", val, ok)
		}
	})

	t.Run("without default", func(t *testing.T) {
		meta := map[string]any{}
		_, ok := GetDefaultValue(meta)
		if ok {
			t.Error("GetDefaultValue() should return false when not set")
		}
	})
}

func TestParseShowIf(t *testing.T) {
	tests := []struct {
		name         string
		metadata     map[string]any
		wantPath     string
		wantExpected string
	}{
		{
			name:         "valid condition",
			metadata:     map[string]any{LabelUIShowIf: "database/type=postgres"},
			wantPath:     "database/type",
			wantExpected: "postgres",
		},
		{
			name:         "boolean condition",
			metadata:     map[string]any{LabelUIShowIf: "app/tls/enabled=true"},
			wantPath:     "app/tls/enabled",
			wantExpected: "true",
		},
		{
			name:         "missing label",
			metadata:     map[string]any{},
			wantPath:     "",
			wantExpected: "",
		},
		{
			name:         "nil metadata",
			metadata:     nil,
			wantPath:     "",
			wantExpected: "",
		},
		{
			name:         "malformed no equals",
			metadata:     map[string]any{LabelUIShowIf: "noequalssign"},
			wantPath:     "",
			wantExpected: "",
		},
		{
			name:         "malformed empty path",
			metadata:     map[string]any{LabelUIShowIf: "=value"},
			wantPath:     "",
			wantExpected: "",
		},
		{
			name:         "malformed empty value",
			metadata:     map[string]any{LabelUIShowIf: "path="},
			wantPath:     "",
			wantExpected: "",
		},
		{
			name:         "value with equals sign",
			metadata:     map[string]any{LabelUIShowIf: "path=a=b"},
			wantPath:     "path",
			wantExpected: "a=b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotExpected := ParseShowIf(tt.metadata)
			if gotPath != tt.wantPath {
				t.Errorf("ParseShowIf() path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotExpected != tt.wantExpected {
				t.Errorf("ParseShowIf() expected = %q, want %q", gotExpected, tt.wantExpected)
			}
		})
	}
}

func TestGetShowIf(t *testing.T) {
	meta := map[string]any{LabelUIShowIf: "db/type=postgres"}
	if got := GetShowIf(meta); got != "db/type=postgres" {
		t.Errorf("GetShowIf() = %v, want db/type=postgres", got)
	}

	if got := GetShowIf(nil); got != "" {
		t.Errorf("GetShowIf(nil) = %v, want empty", got)
	}
}

func TestGetDisplayName(t *testing.T) {
	meta := map[string]any{LabelUIDisplayName: "Database Host"}
	if got := GetDisplayName(meta); got != "Database Host" {
		t.Errorf("GetDisplayName() = %v, want Database Host", got)
	}

	if got := GetDisplayName(nil); got != "" {
		t.Errorf("GetDisplayName(nil) = %v, want empty", got)
	}
}

func TestGetEnum(t *testing.T) {
	meta := map[string]any{LabelUIEnum: []string{"a", "b", "c"}}
	got := GetEnum(meta)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("GetEnum() = %v, want [a b c]", got)
	}

	if got := GetEnum(nil); got != nil {
		t.Errorf("GetEnum(nil) = %v, want nil", got)
	}
}

func TestGetSection(t *testing.T) {
	meta := map[string]any{LabelUISection: "Advanced"}
	if got := GetSection(meta); got != "Advanced" {
		t.Errorf("GetSection() = %v, want Advanced", got)
	}

	if got := GetSection(nil); got != "" {
		t.Errorf("GetSection(nil) = %v, want empty", got)
	}
}

func TestGetPlaceholder(t *testing.T) {
	meta := map[string]any{LabelUIPlaceholder: "Enter a value..."}
	if got := GetPlaceholder(meta); got != "Enter a value..." {
		t.Errorf("GetPlaceholder() = %v, want 'Enter a value...'", got)
	}

	if got := GetPlaceholder(nil); got != "" {
		t.Errorf("GetPlaceholder(nil) = %v, want empty", got)
	}
}

func TestBuilder_NewLabels(t *testing.T) {
	meta := NewBuilder().
		ShowIf("db/type", "postgres").
		DisplayName("Database Host").
		Enum([]string{"a", "b"}).
		Section("Advanced").
		Placeholder("Enter value").
		Build()

	if GetShowIf(meta) != "db/type=postgres" {
		t.Errorf("Builder.ShowIf() = %v, want db/type=postgres", GetShowIf(meta))
	}
	if GetDisplayName(meta) != "Database Host" {
		t.Errorf("Builder.DisplayName() = %v, want Database Host", GetDisplayName(meta))
	}
	if got := GetEnum(meta); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Builder.Enum() = %v, want [a b]", got)
	}
	if GetSection(meta) != "Advanced" {
		t.Errorf("Builder.Section() = %v, want Advanced", GetSection(meta))
	}
	if GetPlaceholder(meta) != "Enter value" {
		t.Errorf("Builder.Placeholder() = %v, want Enter value", GetPlaceholder(meta))
	}
}
