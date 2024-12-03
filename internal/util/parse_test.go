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
