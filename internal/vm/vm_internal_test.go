package vm

import (
	"math"
	"testing"
)

// TestV11FloatFormatting implements the ADR-0004 item 12 table.
func TestV11FloatFormatting(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1.0, "1.0"},
		{1.25, "1.25"},
		{math.Copysign(0, -1), "-0.0"},
		{0, "0.0"},
		{100.0, "100.0"},
		{-7.0, "-7.0"},
		{0.1, "0.1"},
		{1e6, "1e+06"},
		{1e21, "1e+21"},
		{1e-5, "1e-05"},
		{5e-324, "5e-324"},
		{123456.0, "123456.0"},
		{math.NaN(), "NaN"},
		{math.Inf(1), "+Inf"},
		{math.Inf(-1), "-Inf"},
	}
	for _, tt := range tests {
		if got := FormatFloat(tt.in); got != tt.want {
			t.Errorf("FormatFloat(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
