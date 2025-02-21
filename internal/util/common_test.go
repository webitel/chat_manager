package util

import (
	"reflect"
	"testing"
)

func TestMargeMaps(t *testing.T) {
	tests := []struct {
		name     string
		into     map[string]int
		from     map[string]int
		expected map[string]int
	}{
		{
			name:     "Merge non-empty maps",
			into:     map[string]int{"a": 1, "b": 2},
			from:     map[string]int{"b": 3, "c": 4},
			expected: map[string]int{"a": 1, "b": 3, "c": 4},
		},
		{
			name:     "Merge into nil map",
			into:     nil,
			from:     map[string]int{"a": 5},
			expected: map[string]int{"a": 5},
		},
		{
			name:     "Merge empty from map",
			into:     map[string]int{"x": 10},
			from:     map[string]int{},
			expected: map[string]int{"x": 10},
		},
		{
			name:     "Merge empty into map",
			into:     map[string]int{},
			from:     map[string]int{"y": 20},
			expected: map[string]int{"y": 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MargeMaps(tt.into, tt.from)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}
