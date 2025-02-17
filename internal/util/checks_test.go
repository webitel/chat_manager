package util

import "testing"

func TestIsInteger(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},     // valid integer string
		{"-123", true},    // negative integer string
		{"0", true},       // zero
		{"12.34", false},  // floating-point number (not an integer)
		{"abc", false},    // non-numeric string
		{"123abc", false}, // non-numeric string
		{"", false},       // empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsInteger(tt.input)
			if result != tt.expected {
				t.Errorf("IsInteger(%s) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}
