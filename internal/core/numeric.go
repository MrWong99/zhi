package core

import (
	"math"
	"reflect"
)

// isNumericType reports whether t is a numeric kind that appears in
// config values: int, int64, float64, or uint.
func isNumericType(t reflect.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Float64:
		return true
	default:
		return false
	}
}

// tryNumericConvert attempts a lossless conversion of val to targetType.
// It returns the converted value and true on success, or (nil, false) if
// conversion is not possible without data loss. Only numeric types
// (int, int64, uint, float64) are handled.
func tryNumericConvert(val any, targetType reflect.Type) (any, bool) {
	if val == nil || targetType == nil {
		return nil, false
	}
	srcType := reflect.TypeOf(val)
	if !isNumericType(srcType) || !isNumericType(targetType) {
		return nil, false
	}

	switch targetType.Kind() {
	case reflect.Int:
		return toInt(val)
	case reflect.Int64:
		return toInt64(val)
	case reflect.Float64:
		return toFloat64(val)
	case reflect.Uint:
		return toUint(val)
	default:
		return nil, false
	}
}

func toInt(val any) (any, bool) {
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		if v >= math.MinInt && v <= math.MaxInt {
			return int(v), true
		}
		return nil, false
	case float64:
		if v != math.Trunc(v) || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, false
		}
		if v < math.MinInt || v > math.MaxInt {
			return nil, false
		}
		return int(v), true
	case uint:
		if v <= math.MaxInt {
			return int(v), true
		}
		return nil, false
	default:
		return nil, false
	}
}

func toInt64(val any) (any, bool) {
	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		if v != math.Trunc(v) || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, false
		}
		// float64 can only represent integers exactly up to 2^53.
		if v < -1<<53 || v > 1<<53 {
			return nil, false
		}
		return int64(v), true
	case uint:
		return int64(v), true
	default:
		return nil, false
	}
}

func toFloat64(val any) (any, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	default:
		return nil, false
	}
}

func toUint(val any) (any, bool) {
	switch v := val.(type) {
	case uint:
		return v, true
	case int:
		if v >= 0 {
			return uint(v), true
		}
		return nil, false
	case int64:
		if v >= 0 {
			return uint(v), true
		}
		return nil, false
	case float64:
		if v != math.Trunc(v) || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, false
		}
		if v < 0 || v > math.MaxUint {
			return nil, false
		}
		return uint(v), true
	default:
		return nil, false
	}
}
