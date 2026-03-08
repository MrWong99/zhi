package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ValidateYAML returns a Warning-severity result if the value is not valid YAML.
// Empty strings and nil values are considered valid (no-op).
// This is a convenience validator that config plugins can attach to yaml-typed values.
func ValidateYAML(value any, _ TreeReader) []ValidationResult {
	s, ok := value.(string)
	if !ok || s == "" {
		return nil
	}
	var target any
	if err := yaml.Unmarshal([]byte(s), &target); err != nil {
		return []ValidationResult{{
			Severity: Warning,
			Message:  fmt.Sprintf("value is not valid YAML: %v", err),
		}}
	}
	return nil
}
