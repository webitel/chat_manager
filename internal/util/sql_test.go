package util

import (
	"database/sql"
	"testing"
)

func TestValidateNullStrings(t *testing.T) {
	// Test cases for the function
	tests := []struct {
		name     string
		input    []*sql.NullString
		expected []bool
	}{
		{
			name: "Test with valid strings",
			input: []*sql.NullString{
				{String: "hello", Valid: false},
				{String: "world", Valid: false},
			},
			expected: []bool{true, true}, // Both strings are non-empty, so Valid should be true
		},
		{
			name: "Test with empty strings",
			input: []*sql.NullString{
				{String: "", Valid: true}, // Empty string, so Valid should be false
			},
			expected: []bool{false},
		},
		{
			name: "Test with all empty strings",
			input: []*sql.NullString{
				{String: "", Valid: true},
				{String: "", Valid: true},
			},
			expected: []bool{false, false}, // Both are empty, Valid should be false
		},
		{
			name: "Test with nil values",
			input: []*sql.NullString{
				nil,
				{String: "test", Valid: false},
			},
			expected: []bool{false, true}, // First element is nil, so Valid doesn't change; second is non-empty
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy the input array before passing to the function to avoid modifying the original data
			stringsCopy := make([]*sql.NullString, len(tt.input))
			copy(stringsCopy, tt.input)

			// Call the function being tested
			ValidateNullStrings(stringsCopy...)

			// Check the results
			for i, str := range stringsCopy {
				if str == nil {
					continue
				}

				if str.Valid != tt.expected[i] {
					t.Errorf("Test case %s failed. Expected Valid = %v, but got %v", tt.name, tt.expected[i], str.Valid)
				}
			}
		})
	}
}
