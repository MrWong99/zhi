package labels

import (
	"strconv"
	"strings"
)

// GetBool returns a boolean label value with default fallback.
func GetBool(metadata map[string]any, name string, defaultVal bool) bool {
	if metadata == nil {
		return defaultVal
	}
	v, ok := metadata[name]
	if !ok {
		return defaultVal
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultVal
}

// GetString returns a string label value with default fallback.
func GetString(metadata map[string]any, name string, defaultVal string) string {
	if metadata == nil {
		return defaultVal
	}
	v, ok := metadata[name]
	if !ok {
		return defaultVal
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}

// GetInt returns an integer label value with default fallback.
func GetInt(metadata map[string]any, name string, defaultVal int) int {
	if metadata == nil {
		return defaultVal
	}
	v, ok := metadata[name]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return defaultVal
}

// GetFloat returns a float label value with default fallback.
func GetFloat(metadata map[string]any, name string, defaultVal float64) float64 {
	if metadata == nil {
		return defaultVal
	}
	v, ok := metadata[name]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// GetStringSlice returns a string slice label value.
func GetStringSlice(metadata map[string]any, name string) []string {
	if metadata == nil {
		return nil
	}
	v, ok := metadata[name]
	if !ok {
		return nil
	}

	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// GetAny returns a label value of any type.
func GetAny(metadata map[string]any, name string) (any, bool) {
	if metadata == nil {
		return nil, false
	}
	v, ok := metadata[name]
	return v, ok
}

// --- Convenience functions for well-known labels ---

// IsReadonly checks if ui.readonly is set to true.
func IsReadonly(metadata map[string]any) bool {
	return GetBool(metadata, LabelUIReadonly, false)
}

// IsPassword checks if ui.password is set to true.
func IsPassword(metadata map[string]any) bool {
	return GetBool(metadata, LabelUIPassword, false)
}

// IsHidden checks if ui.hidden is set to true.
func IsHidden(metadata map[string]any) bool {
	return GetBool(metadata, LabelUIHidden, false)
}

// IsMultiline checks if ui.multiline is set to true.
func IsMultiline(metadata map[string]any) bool {
	return GetBool(metadata, LabelUIMultiline, false)
}

// IsWriteonly checks if store.writeonly is set to true.
func IsWriteonly(metadata map[string]any) bool {
	return GetBool(metadata, LabelStoreWriteonly, false)
}

// ShouldEncrypt checks if store.encrypt is set to true.
func ShouldEncrypt(metadata map[string]any) bool {
	return GetBool(metadata, LabelStoreEncrypt, false)
}

// IsNoVersion checks if store.noversion is set to true.
func IsNoVersion(metadata map[string]any) bool {
	return GetBool(metadata, LabelStoreNoVersion, false)
}

// GetCAS returns the store.cas label value (expected version for optimistic locking).
// Returns 0 if not set.
func GetCAS(metadata map[string]any) int {
	return GetInt(metadata, LabelStoreCAS, 0)
}

// GetMaxVersions returns the store.maxversions label value.
// Returns 0 if not set (meaning no limit).
func GetMaxVersions(metadata map[string]any) int {
	return GetInt(metadata, LabelStoreMaxVersions, 0)
}

// IsTransformHidden checks if transform.hidden is set to true.
func IsTransformHidden(metadata map[string]any) bool {
	return GetBool(metadata, LabelTransformHidden, false)
}

// IsRequired checks if config.required is set to true.
func IsRequired(metadata map[string]any) bool {
	return GetBool(metadata, LabelConfigRequired, false)
}

// IsImmutable checks if config.immutable is set to true.
func IsImmutable(metadata map[string]any) bool {
	return GetBool(metadata, LabelConfigImmutable, false)
}

// IsDeprecatedValue checks if core.deprecated is set.
func IsDeprecatedValue(metadata map[string]any) bool {
	return GetString(metadata, LabelCoreDeprecated, "") != ""
}

// GetDescription returns the core.description label value.
func GetDescription(metadata map[string]any) string {
	return GetString(metadata, LabelCoreDescription, "")
}

// GetPattern returns the ui.pattern label value.
func GetPattern(metadata map[string]any) string {
	return GetString(metadata, LabelUIPattern, "")
}

// GetGroup returns the ui.group label value.
func GetGroup(metadata map[string]any) string {
	return GetString(metadata, LabelUIGroup, "")
}

// GetOrder returns the ui.order label value.
func GetOrder(metadata map[string]any) int {
	return GetInt(metadata, LabelUIOrder, 0)
}

// GetTTL returns the store.ttl label value in seconds.
func GetTTL(metadata map[string]any) int {
	return GetInt(metadata, LabelStoreTTL, 0)
}

// GetSemanticType returns the core.type label value.
func GetSemanticType(metadata map[string]any) string {
	return GetString(metadata, LabelCoreType, "")
}

// GetDocURL returns the core.doc label value.
func GetDocURL(metadata map[string]any) string {
	return GetString(metadata, LabelCoreDoc, "")
}

// GetEnvOverride returns the config.env label value.
func GetEnvOverride(metadata map[string]any) string {
	return GetString(metadata, LabelConfigEnv, "")
}

// GetDefaultValue returns the config.default label value.
func GetDefaultValue(metadata map[string]any) (any, bool) {
	return GetAny(metadata, LabelConfigDefault)
}

// GetTransformSkip returns the transform.skip label value.
func GetTransformSkip(metadata map[string]any) []string {
	return GetStringSlice(metadata, LabelTransformSkip)
}

// ShouldConfirm checks if ui.confirm is set to true.
func ShouldConfirm(metadata map[string]any) bool {
	return GetBool(metadata, LabelUIConfirm, false)
}

// GetShowIf returns the ui.showIf label value (format: "path=value").
func GetShowIf(metadata map[string]any) string {
	return GetString(metadata, LabelUIShowIf, "")
}

// ParseShowIf parses a ui.showIf value into path and expected value.
// Returns empty strings if the label is not set or malformed.
func ParseShowIf(metadata map[string]any) (path, expected string) {
	raw := GetShowIf(metadata)
	if raw == "" {
		return "", ""
	}
	idx := strings.Index(raw, "=")
	if idx < 1 || idx >= len(raw)-1 {
		return "", ""
	}
	return raw[:idx], raw[idx+1:]
}

// GetDisplayName returns the ui.displayName label value.
func GetDisplayName(metadata map[string]any) string {
	return GetString(metadata, LabelUIDisplayName, "")
}

// GetEnum returns the ui.enum label value.
func GetEnum(metadata map[string]any) []string {
	return GetStringSlice(metadata, LabelUIEnum)
}

// GetSection returns the ui.section label value.
func GetSection(metadata map[string]any) string {
	return GetString(metadata, LabelUISection, "")
}

// GetPlaceholder returns the ui.placeholder label value.
func GetPlaceholder(metadata map[string]any) string {
	return GetString(metadata, LabelUIPlaceholder, "")
}

// --- Builder helpers for creating metadata ---

// Builder helps construct metadata maps with labels.
type Builder struct {
	metadata map[string]any
}

// NewBuilder creates a new metadata builder.
func NewBuilder() *Builder {
	return &Builder{
		metadata: make(map[string]any),
	}
}

// FromMetadata creates a builder starting from existing metadata.
func FromMetadata(m map[string]any) *Builder {
	b := NewBuilder()
	for k, v := range m {
		b.metadata[k] = v
	}
	return b
}

// Set sets a label value.
func (b *Builder) Set(name string, value any) *Builder {
	b.metadata[name] = value
	return b
}

// Readonly sets ui.readonly to true.
func (b *Builder) Readonly() *Builder {
	return b.Set(LabelUIReadonly, true)
}

// Password sets ui.password to true.
func (b *Builder) Password() *Builder {
	return b.Set(LabelUIPassword, true)
}

// Hidden sets ui.hidden to true.
func (b *Builder) Hidden() *Builder {
	return b.Set(LabelUIHidden, true)
}

// Writeonly sets store.writeonly to true.
func (b *Builder) Writeonly() *Builder {
	return b.Set(LabelStoreWriteonly, true)
}

// Encrypt sets store.encrypt to true.
func (b *Builder) Encrypt() *Builder {
	return b.Set(LabelStoreEncrypt, true)
}

// Required sets config.required to true.
func (b *Builder) Required() *Builder {
	return b.Set(LabelConfigRequired, true)
}

// Immutable sets config.immutable to true.
func (b *Builder) Immutable() *Builder {
	return b.Set(LabelConfigImmutable, true)
}

// TransformHidden sets transform.hidden to true.
func (b *Builder) TransformHidden() *Builder {
	return b.Set(LabelTransformHidden, true)
}

// Description sets core.description.
func (b *Builder) Description(desc string) *Builder {
	return b.Set(LabelCoreDescription, desc)
}

// Pattern sets ui.pattern.
func (b *Builder) Pattern(pattern string) *Builder {
	return b.Set(LabelUIPattern, pattern)
}

// Group sets ui.group.
func (b *Builder) Group(group string) *Builder {
	return b.Set(LabelUIGroup, group)
}

// Order sets ui.order.
func (b *Builder) Order(order int) *Builder {
	return b.Set(LabelUIOrder, order)
}

// SemanticType sets core.type.
func (b *Builder) SemanticType(t string) *Builder {
	return b.Set(LabelCoreType, t)
}

// Doc sets core.doc.
func (b *Builder) Doc(url string) *Builder {
	return b.Set(LabelCoreDoc, url)
}

// EnvOverride sets config.env.
func (b *Builder) EnvOverride(envVar string) *Builder {
	return b.Set(LabelConfigEnv, envVar)
}

// DefaultValue sets config.default.
func (b *Builder) DefaultValue(val any) *Builder {
	return b.Set(LabelConfigDefault, val)
}

// TTL sets store.ttl in seconds.
func (b *Builder) TTL(seconds int) *Builder {
	return b.Set(LabelStoreTTL, seconds)
}

// NoVersion sets store.noversion to true.
func (b *Builder) NoVersion() *Builder {
	return b.Set(LabelStoreNoVersion, true)
}

// CAS sets store.cas to the expected version.
func (b *Builder) CAS(version int) *Builder {
	return b.Set(LabelStoreCAS, version)
}

// MaxVersions sets store.maxversions.
func (b *Builder) MaxVersions(n int) *Builder {
	return b.Set(LabelStoreMaxVersions, n)
}

// Multiline sets ui.multiline to true.
func (b *Builder) Multiline() *Builder {
	return b.Set(LabelUIMultiline, true)
}

// Confirm sets ui.confirm to true.
func (b *Builder) Confirm() *Builder {
	return b.Set(LabelUIConfirm, true)
}

// Deprecated sets core.deprecated.
func (b *Builder) Deprecated(message string) *Builder {
	return b.Set(LabelCoreDeprecated, message)
}

// ShowIf sets ui.showIf with the format "path=value".
func (b *Builder) ShowIf(path, value string) *Builder {
	return b.Set(LabelUIShowIf, path+"="+value)
}

// DisplayName sets ui.displayName.
func (b *Builder) DisplayName(name string) *Builder {
	return b.Set(LabelUIDisplayName, name)
}

// Enum sets ui.enum.
func (b *Builder) Enum(values []string) *Builder {
	return b.Set(LabelUIEnum, values)
}

// Section sets ui.section.
func (b *Builder) Section(section string) *Builder {
	return b.Set(LabelUISection, section)
}

// Placeholder sets ui.placeholder.
func (b *Builder) Placeholder(text string) *Builder {
	return b.Set(LabelUIPlaceholder, text)
}

// Build returns the constructed metadata map.
func (b *Builder) Build() map[string]any {
	result := make(map[string]any, len(b.metadata))
	for k, v := range b.metadata {
		result[k] = v
	}
	return result
}
