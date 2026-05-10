package isrequest

import (
	"testing"
)

func TestIsFirstThreeZero(t *testing.T) {
	tests := []struct {
		name     string
		message  []byte
		expected bool
	}{
		{
			name:     "empty message",
			message:  []byte{},
			expected: false,
		},
		{
			name:     "one byte",
			message:  []byte{0},
			expected: false,
		},
		{
			name:     "two bytes all zero",
			message:  []byte{0, 0},
			expected: false,
		},
		{
			name:     "three bytes all zero",
			message:  []byte{0, 0, 0},
			expected: true,
		},
		{
			name:     "three bytes with last non-zero",
			message:  []byte{0, 0, 1},
			expected: false,
		},
		{
			name:     "three bytes with first non-zero",
			message:  []byte{1, 0, 0},
			expected: false,
		},
		{
			name:     "four bytes first three zero",
			message:  []byte{0, 0, 0, 1},
			expected: true,
		},
		{
			name:     "long message first three zero",
			message:  []byte{0, 0, 0, 1, 2, 3, 4, 5},
			expected: true,
		},
		{
			name:     "long message first three not zero",
			message:  []byte{1, 2, 3, 0, 0, 0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFirstThreeZero(tt.message)
			if result != tt.expected {
				t.Errorf("IsFirstThreeZero(%v) = %v, want %v", tt.message, result, tt.expected)
			}
		})
	}
}
