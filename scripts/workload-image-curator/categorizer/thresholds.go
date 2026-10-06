package categorizer

// SizeThresholds defines byte boundaries for image categorization
type SizeThresholds struct {
	SmallThreshold int64 // Boundary between small and medium
	LargeThreshold int64 // Boundary between medium and large
}

// DefaultThresholds returns the default size thresholds
// Default: Small < 50MB, Medium 50-200MB, Large > 200MB
func DefaultThresholds() *SizeThresholds {
	return &SizeThresholds{
		SmallThreshold: 52428800,  // 50MB
		LargeThreshold: 209715200, // 200MB
	}
}

// NewThresholds creates a new threshold config with validation
func NewThresholds(smallThreshold, largeThreshold int64) (*SizeThresholds, error) {
	// Ensure thresholds are sensible
	if smallThreshold <= 0 {
		smallThreshold = 52428800 // 50MB
	}
	if largeThreshold <= 0 {
		largeThreshold = 209715200 // 200MB
	}
	// Ensure small < large
	if smallThreshold >= largeThreshold {
		smallThreshold = 52428800  // 50MB
		largeThreshold = 209715200 // 200MB
	}

	return &SizeThresholds{
		SmallThreshold: smallThreshold,
		LargeThreshold: largeThreshold,
	}, nil
}

// GetThresholdMB returns thresholds in megabytes (for display)
func (t *SizeThresholds) GetThresholdMB() (smallMB, largeMB float64) {
	return float64(t.SmallThreshold) / 1024.0 / 1024.0,
		float64(t.LargeThreshold) / 1024.0 / 1024.0
}
