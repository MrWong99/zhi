package core

import (
	"math"
	"reflect"
	"testing"
)

func TestTryNumericConvert(t *testing.T) {
	tests := []struct {
		name   string
		val    any
		target reflect.Type
		want   any
		wantOK bool
	}{
		// float64 -> int (whole numbers)
		{
			name:   "float64 whole number to int",
			val:    float64(5432),
			target: reflect.TypeFor[int](),
			want:   5432,
			wantOK: true,
		},
		{
			name:   "float64 zero to int",
			val:    float64(0),
			target: reflect.TypeFor[int](),
			want:   0,
			wantOK: true,
		},
		{
			name:   "float64 negative whole to int",
			val:    float64(-42),
			target: reflect.TypeFor[int](),
			want:   -42,
			wantOK: true,
		},

		// float64 -> int (fractional, should fail)
		{
			name:   "float64 fractional to int fails",
			val:    float64(3.14),
			target: reflect.TypeFor[int](),
			want:   nil,
			wantOK: false,
		},
		{
			name:   "float64 NaN to int fails",
			val:    math.NaN(),
			target: reflect.TypeFor[int](),
			want:   nil,
			wantOK: false,
		},
		{
			name:   "float64 Inf to int fails",
			val:    math.Inf(1),
			target: reflect.TypeFor[int](),
			want:   nil,
			wantOK: false,
		},

		// int64 -> int
		{
			name:   "int64 to int",
			val:    int64(42),
			target: reflect.TypeFor[int](),
			want:   42,
			wantOK: true,
		},

		// int -> int64
		{
			name:   "int to int64",
			val:    42,
			target: reflect.TypeFor[int64](),
			want:   int64(42),
			wantOK: true,
		},

		// int -> float64
		{
			name:   "int to float64",
			val:    5432,
			target: reflect.TypeFor[float64](),
			want:   float64(5432),
			wantOK: true,
		},

		// float64 -> int64 (whole number)
		{
			name:   "float64 whole to int64",
			val:    float64(8080),
			target: reflect.TypeFor[int64](),
			want:   int64(8080),
			wantOK: true,
		},

		// float64 -> int64 (fractional, should fail)
		{
			name:   "float64 fractional to int64 fails",
			val:    float64(0.75),
			target: reflect.TypeFor[int64](),
			want:   nil,
			wantOK: false,
		},

		// uint conversions
		{
			name:   "int to uint",
			val:    42,
			target: reflect.TypeFor[uint](),
			want:   uint(42),
			wantOK: true,
		},
		{
			name:   "negative int to uint fails",
			val:    -1,
			target: reflect.TypeFor[uint](),
			want:   nil,
			wantOK: false,
		},
		{
			name:   "float64 whole to uint",
			val:    float64(100),
			target: reflect.TypeFor[uint](),
			want:   uint(100),
			wantOK: true,
		},
		{
			name:   "negative float64 to uint fails",
			val:    float64(-1),
			target: reflect.TypeFor[uint](),
			want:   nil,
			wantOK: false,
		},

		// Non-numeric types
		{
			name:   "string to int fails",
			val:    "hello",
			target: reflect.TypeFor[int](),
			want:   nil,
			wantOK: false,
		},
		{
			name:   "int to string fails",
			val:    42,
			target: reflect.TypeFor[string](),
			want:   nil,
			wantOK: false,
		},
		{
			name:   "bool to int fails",
			val:    true,
			target: reflect.TypeFor[int](),
			want:   nil,
			wantOK: false,
		},

		// Same type (identity)
		{
			name:   "int to int",
			val:    42,
			target: reflect.TypeFor[int](),
			want:   42,
			wantOK: true,
		},
		{
			name:   "float64 to float64",
			val:    3.14,
			target: reflect.TypeFor[float64](),
			want:   3.14,
			wantOK: true,
		},

		// nil
		{
			name:   "nil value",
			val:    nil,
			target: reflect.TypeFor[int](),
			want:   nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tryNumericConvert(tt.val, tt.target)
			if ok != tt.wantOK {
				t.Errorf("tryNumericConvert(%v, %v) ok = %v, want %v", tt.val, tt.target, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("tryNumericConvert(%v, %v) = %v (%T), want %v (%T)", tt.val, tt.target, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestIsNumericType(t *testing.T) {
	tests := []struct {
		t    reflect.Type
		want bool
	}{
		{reflect.TypeFor[int](), true},
		{reflect.TypeFor[int64](), true},
		{reflect.TypeFor[float64](), true},
		{reflect.TypeFor[uint](), true},
		{reflect.TypeFor[string](), false},
		{reflect.TypeFor[bool](), false},
		{nil, false},
	}

	for _, tt := range tests {
		name := "nil"
		if tt.t != nil {
			name = tt.t.String()
		}
		t.Run(name, func(t *testing.T) {
			if got := isNumericType(tt.t); got != tt.want {
				t.Errorf("isNumericType(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}
