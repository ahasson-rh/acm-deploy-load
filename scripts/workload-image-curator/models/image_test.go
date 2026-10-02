package models

import "testing"

func TestImageCategory_String(t *testing.T) {
	tests := []struct {
		category ImageCategory
		expected string
	}{
		{CategorySmall, "small"},
		{CategoryMedium, "medium"},
		{CategoryLarge, "large"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSelectionStrategy_Total(t *testing.T) {
	tests := []struct {
		name     string
		strategy SelectionStrategy
		expected int
	}{
		{
			name:     "random mode",
			strategy: SelectionStrategy{RandomMode: true, Count: 50},
			expected: 50,
		},
		{
			name:     "size-based mode",
			strategy: SelectionStrategy{RandomMode: false, SmallCount: 10, MediumCount: 30, LargeCount: 10},
			expected: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategy.Total(); got != tt.expected {
				t.Errorf("got %d, want %d", got, tt.expected)
			}
		})
	}
}
