package config

import (
	"reflect"
	"testing"
)

func TestTryConvertUintRejectsNegative(t *testing.T) {
	cases := []struct {
		name string
		val  any
	}{
		{"negative int", -1},
		{"negative int64", int64(-42)},
		{"negative float64", float64(-1)},
		{"negative string", "-1"},
		{"fractional float64", 3.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &Value{Val: tc.val}
			if err := v.TryConvert(reflect.TypeFor[uint]()); err == nil {
				t.Fatalf("TryConvert(%v -> uint) = nil error, got Val %v (want error)", tc.val, v.Val)
			}
		})
	}
}

func TestTryConvertUintValid(t *testing.T) {
	v := &Value{Val: "42"}
	if err := v.TryConvert(reflect.TypeFor[uint]()); err != nil {
		t.Fatalf("TryConvert(\"42\" -> uint): %v", err)
	}
	if v.Val != uint(42) {
		t.Errorf("Val = %v (%T), want uint(42)", v.Val, v.Val)
	}

	v = &Value{Val: float64(7)}
	if err := v.TryConvert(reflect.TypeFor[uint]()); err != nil {
		t.Fatalf("TryConvert(7.0 -> uint): %v", err)
	}
	if v.Val != uint(7) {
		t.Errorf("Val = %v, want uint(7)", v.Val)
	}
}

func TestTryConvertIntRejectsFractionalFloat(t *testing.T) {
	v := &Value{Val: 3.5}
	if err := v.TryConvert(reflect.TypeFor[int]()); err == nil {
		t.Fatalf("TryConvert(3.5 -> int) = nil error, got Val %v (want error)", v.Val)
	}

	// Whole-valued floats still convert.
	v = &Value{Val: 3.0}
	if err := v.TryConvert(reflect.TypeFor[int]()); err != nil {
		t.Fatalf("TryConvert(3.0 -> int): %v", err)
	}
	if v.Val != 3 {
		t.Errorf("Val = %v, want 3", v.Val)
	}
}
