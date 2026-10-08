package models

// SelectionStrategy defines how images should be selected
type SelectionStrategy struct {
	RandomMode bool   // If true, use -c count; if false, use SizeDistribution
	Count      int    // Total images for random mode
	SmallCount int    // For size-based mode
	MediumCount int
	LargeCount int
}

// Total returns total count needed to satisfy strategy
func (s *SelectionStrategy) Total() int {
	if s.RandomMode {
		return s.Count
	}
	return s.SmallCount + s.MediumCount + s.LargeCount
}

// Small returns small image target count (for size-based mode)
func (s *SelectionStrategy) Small() int {
	return s.SmallCount
}

// Medium returns medium image target count (for size-based mode)
func (s *SelectionStrategy) Medium() int {
	return s.MediumCount
}

// Large returns large image target count (for size-based mode)
func (s *SelectionStrategy) Large() int {
	return s.LargeCount
}
