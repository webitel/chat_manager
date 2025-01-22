package util

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1023, "1023 B"},             // less than 1 KB
		{1024, "1.00 KB"},            // exactly 1 KB
		{2048, "2.00 KB"},            // exactly 2 KB
		{1048576, "1.00 MB"},         // exactly 1 MB
		{10485760, "10.00 MB"},       // exactly 10 MB
		{1073741824, "1.00 GB"},      // exactly 1 GB
		{10737418240, "10.00 GB"},    // exactly 10 GB
		{1099511627776, "1.00 TB"},   // exactly 1 TB
		{10995116277760, "10.00 TB"}, // exactly 10 TB
		{0, "0 B"},                   // zero bytes
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBytes(tt.input)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}
