package domain

import (
	"testing"
)

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		ok       bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(42), 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"int32", int32(50), 50.0, true},
		{"string", "hello", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
		{"slice", []int{1, 2}, 0, false},
		{"float64 zero", float64(0), 0.0, true},
		{"negative int", int(-5), -5.0, true},
		{"negative float64", float64(-3.14), -3.14, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToFloat64(tt.input)
			if ok != tt.ok {
				t.Errorf("ToFloat64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.expected {
				t.Errorf("ToFloat64(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
