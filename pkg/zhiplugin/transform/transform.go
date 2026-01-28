// Package transform defines the transform plugin API for zhi.
//
// Transform plugins mutate configuration values before they are displayed
// in the UI (BeforeDisplay) and/or after the UI stores updates (AfterSave).
// Each plugin receives the full, mutable configuration tree and can control
// whether config-plugin validations run before or after the transformation.
package transform

// ValidatePolicy controls when config-plugin validations are executed
// relative to a transformation.
type ValidatePolicy int

const (
	// ValidateBeforeTransform runs config validations before the
	// transformation is applied.
	ValidateBeforeTransform ValidatePolicy = iota
	// ValidateAfterTransform runs config validations after the
	// transformation is applied.
	ValidateAfterTransform
	// ValidateBoth runs config validations both before and after the
	// transformation.
	ValidateBoth
)

func (p ValidatePolicy) String() string {
	switch p {
	case ValidateBeforeTransform:
		return "before"
	case ValidateAfterTransform:
		return "after"
	case ValidateBoth:
		return "both"
	default:
		return "unknown"
	}
}
