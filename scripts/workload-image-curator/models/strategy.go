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
