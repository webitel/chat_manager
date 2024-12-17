package util

import (
	"testing"
)

// TestParseFullName tests the ParseFullName function
func TestParseFullName(t *testing.T) {
	tests := []struct {
		input         string
		expectedFirst string
		expectedLast  string
	}{
		// Test cases
		{"John Doe", "John", "Doe"},                           // Regular two-part name
		{"John", "John", ""},                                  // Single name
		{"  John   ", "John", ""},                             // Name with leading/trailing spaces
		{"John Michael Doe", "John Michael", "Doe"},           // Multi-part last name
		{"", "", ""},                                          // Empty string
		{"   ", "", ""},                                       // String with only spaces
		{"Marie-Claire O'Connor", "Marie-Claire", "O'Connor"}, // Hyphenated and apostrophized name
	}

	for _, test := range tests {
		firstName, lastName := ParseFullName(test.input)
		if firstName != test.expectedFirst || lastName != test.expectedLast {
			t.Errorf("ParseFullName(%q) = (%q, %q); want (%q, %q)",
				test.input, firstName, lastName, test.expectedFirst, test.expectedLast)
		}
	}
}

// TestParseMediaType tests the ParseMediaType function
func TestParseMediaType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid MIME type",
			input:    "image/png",
			expected: "image",
		},
		{
			name:     "MIME type with spaces",
			input:    "  text/html  ",
			expected: "text",
		},
		{
			name:     "MIME type with uppercase letters",
			input:    "APPLICATION/json",
			expected: "application",
		},
		{
			name:     "MIME type without '/'",
			input:    "plaintext",
			expected: "plaintext",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMediaType(tt.input)
			if result != tt.expected {
				t.Errorf("ParseMediaType(%q) = %q; expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
