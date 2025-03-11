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

func TestIsURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://www.example.com", true},              // Valid URL with https
		{"http://localhost:8080", true},                // Valid URL with http
		{"ftp://ftp.example.com", true},                // Valid URL with ftp
		{"invalid-url", false},                         // Invalid URL without a valid scheme
		{"www.missing-scheme.com", false},              // Invalid URL without scheme
		{"https://", false},                            // Invalid URL missing host
		{"", false},                                    // Invalid empty string
		{"https://example.com/path/to/resource", true}, // Valid URL with path
		{" google.com ", false},                        // Invalid URL with spaces
	}

	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			got := IsURL(test.url)
			if got != test.expected {
				t.Errorf("IsURL(%v) = %v; want %v", test.url, got, test.expected)
			}
		})
	}
}
