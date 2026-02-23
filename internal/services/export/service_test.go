package export

import (
	"testing"
)

func TestDerivePrimaryItem(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		fallback string
		expected string
	}{
		{
			name:     "Single item",
			desc:     "iPhone 15 Pro",
			fallback: "Apple",
			expected: "iPhone 15 Pro",
		},
		{
			name:     "Multiple items",
			desc:     "iPhone 15 Pro, MacBook Pro, iPad",
			fallback: "Apple",
			expected: "iPhone 15 Pro",
		},
		{
			name:     "Multiple items with newlines",
			desc:     "iPhone 15 Pro\nMacBook Pro\niPad",
			fallback: "Apple",
			expected: "iPhone 15 Pro",
		},
		{
			name:     "Item with ellipsis",
			desc:     "iPhone 15 Pro…",
			fallback: "Apple",
			expected: "iPhone 15 Pro",
		},
		{
			name:     "Multiple items with ellipsis",
			desc:     "iPhone 15 Pro, MacBook Pro, iPad…",
			fallback: "Apple",
			expected: "iPhone 15 Pro",
		},
		{
			name:     "Short item falls back",
			desc:     "AB, iPhone 15 Pro",
			fallback: "Apple",
			expected: "iPhone 15 Pro",
		},
		{
			name:     "Empty description uses fallback",
			desc:     "",
			fallback: "Apple",
			expected: "Apple",
		},
		{
			name:     "Only short items use fallback",
			desc:     "AB, CD",
			fallback: "Apple",
			expected: "Apple",
		},
		{
			name:     "Trimmed fallback",
			desc:     "",
			fallback: "  Apple  ",
			expected: "Apple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := derivePrimaryItem(tt.desc, tt.fallback)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
