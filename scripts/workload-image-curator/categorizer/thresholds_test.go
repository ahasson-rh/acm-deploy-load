package categorizer

import (
	"testing"
)

func TestDefaultThresholds_Values(t *testing.T) {
	th := DefaultThresholds()

	expectedSmall := int64(52428800)  // 50MB
	expectedLarge := int64(209715200) // 200MB

	if th.SmallThreshold != expectedSmall {
		t.Errorf("SmallThreshold: expected %d, got %d", expectedSmall, th.SmallThreshold)
	}
	if th.LargeThreshold != expectedLarge {
		t.Errorf("LargeThreshold: expected %d, got %d", expectedLarge, th.LargeThreshold)
	}
}

func TestGetThresholdMB(t *testing.T) {
	th := DefaultThresholds()
	smallMB, largeMB := th.GetThresholdMB()

	if smallMB != 50.0 {
		t.Errorf("SmallThreshold in MB: expected 50.0, got %f", smallMB)
	}
	if largeMB != 200.0 {
		t.Errorf("LargeThreshold in MB: expected 200.0, got %f", largeMB)
	}
}

func TestNewThresholds_Valid(t *testing.T) {
	smallThreshold := int64(100 * 1024 * 1024) // 100MB
	largeThreshold := int64(500 * 1024 * 1024) // 500MB

	th, err := NewThresholds(smallThreshold, largeThreshold)

	if err != nil {
		t.Fatalf("NewThresholds failed: %v", err)
	}
	if th.SmallThreshold != smallThreshold {
		t.Errorf("SmallThreshold: expected %d, got %d", smallThreshold, th.SmallThreshold)
	}
	if th.LargeThreshold != largeThreshold {
		t.Errorf("LargeThreshold: expected %d, got %d", largeThreshold, th.LargeThreshold)
	}
}

func TestNewThresholds_ZeroSmallThreshold(t *testing.T) {
	// Zero or negative thresholds should use defaults
	th, _ := NewThresholds(0, 500*1024*1024)

	if th.SmallThreshold != DefaultThresholds().SmallThreshold {
		t.Errorf("Zero SmallThreshold should use default")
	}
}

func TestNewThresholds_ZeroLargeThreshold(t *testing.T) {
	th, _ := NewThresholds(50*1024*1024, 0)

	if th.LargeThreshold != DefaultThresholds().LargeThreshold {
		t.Errorf("Zero LargeThreshold should use default")
	}
}

func TestNewThresholds_SmallLargerThanLarge(t *testing.T) {
	// When small > large, should reset to defaults
	th, _ := NewThresholds(500*1024*1024, 100*1024*1024)

	if th.SmallThreshold != DefaultThresholds().SmallThreshold ||
		th.LargeThreshold != DefaultThresholds().LargeThreshold {
		t.Errorf("Invalid threshold order should reset to defaults")
	}
}

func TestThresholdOrdering(t *testing.T) {
	th := DefaultThresholds()

	if th.SmallThreshold >= th.LargeThreshold {
		t.Error("SmallThreshold must be less than LargeThreshold")
	}
}
