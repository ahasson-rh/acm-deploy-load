package strategy

import (
	"testing"

	"github.com/acm-deploy-load/workload-image-curator/categorizer"
	"github.com/acm-deploy-load/workload-image-curator/models"
	"github.com/acm-deploy-load/workload-image-curator/registry"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{52428800, "50.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{100, "100 B"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestCalculateRemaining_RandomMode(t *testing.T) {
	strategy := &models.SelectionStrategy{
		RandomMode: true,
		Count:      10,
	}
	existing := make(map[models.ImageCategory]int)
	existing[models.CategorySmall] = 2
	existing[models.CategoryMedium] = 3

	needed := calculateRemaining(strategy, existing)
	total := totalNeeded(needed)

	expectedTotal := 10 - 2 - 3 // = 5
	if total != expectedTotal {
		t.Errorf("calculateRemaining random mode total = %d, want %d", total, expectedTotal)
	}
}

func TestCalculateRemaining_SizeBasedMode(t *testing.T) {
	strategy := &models.SelectionStrategy{
		RandomMode:  false,
		SmallCount:  5,
		MediumCount: 10,
		LargeCount:  3,
	}
	existing := make(map[models.ImageCategory]int)
	existing[models.CategorySmall] = 2
	existing[models.CategoryMedium] = 5

	needed := calculateRemaining(strategy, existing)

	if needed[models.CategorySmall] != 3 {
		t.Errorf("needed small = %d, want 3", needed[models.CategorySmall])
	}
	if needed[models.CategoryMedium] != 5 {
		t.Errorf("needed medium = %d, want 5", needed[models.CategoryMedium])
	}
	if needed[models.CategoryLarge] != 3 {
		t.Errorf("needed large = %d, want 3", needed[models.CategoryLarge])
	}
}

func TestTotalNeeded(t *testing.T) {
	needed := make(map[models.ImageCategory]int)
	needed[models.CategorySmall] = 3
	needed[models.CategoryMedium] = 5
	needed[models.CategoryLarge] = 2

	total := totalNeeded(needed)
	if total != 10 {
		t.Errorf("totalNeeded = %d, want 10", total)
	}
}

func TestNewPlanner(t *testing.T) {
	inspector := registry.NewMirrorInspector("localhost:5000", 8)
	thresholds, _ := categorizer.NewThresholds(52428800, 209715200)
	planner := NewPlanner(inspector, thresholds)

	if planner == nil {
		t.Fatal("NewPlanner returned nil")
	}
	if planner.inspector == nil {
		t.Fatal("planner.inspector is nil")
	}
	if planner.categorizer == nil {
		t.Fatal("planner.categorizer is nil")
	}
}

func TestAssessAndSelect_NoExisting(t *testing.T) {
	inspector := registry.NewMirrorInspector("localhost:5000", 8)
	thresholds, _ := categorizer.NewThresholds(52428800, 209715200)
	planner := NewPlanner(inspector, thresholds)

	// Create test images
	images := []*models.OperatorImage{
		{
			Operator:   "op1",
			QuayImage:  "quay.io/op1@sha256:abc",
			ShaDigest:  "sha256:abc",
			Size:       10 * 1024 * 1024, // 10 MB (small)
			LayerCount: 5,
		},
		{
			Operator:   "op2",
			QuayImage:  "quay.io/op2@sha256:def",
			ShaDigest:  "sha256:def",
			Size:       100 * 1024 * 1024, // 100 MB (medium)
			LayerCount: 10,
		},
	}

	strategy := &models.SelectionStrategy{
		RandomMode: false,
		SmallCount: 1,
		MediumCount: 1,
	}

	existing := make(map[string]*registry.ExistingImage)

	selected, result := planner.AssessAndSelect(images, strategy, existing)

	if result.ExistingCount != 0 {
		t.Errorf("ExistingCount = %d, want 0", result.ExistingCount)
	}
	if result.SelectedCount != 2 {
		t.Errorf("SelectedCount = %d, want 2", result.SelectedCount)
	}
	if len(selected) != 2 {
		t.Errorf("selected length = %d, want 2", len(selected))
	}
	if result.SkipDownload {
		t.Errorf("SkipDownload = true, want false")
	}
}

func TestAssessAndSelect_StrategyAlreadySatisfied(t *testing.T) {
	inspector := registry.NewMirrorInspector("localhost:5000", 8)
	thresholds, _ := categorizer.NewThresholds(52428800, 209715200)
	planner := NewPlanner(inspector, thresholds)

	images := []*models.OperatorImage{
		{
			Operator:   "op1",
			QuayImage:  "quay.io/op1@sha256:abc",
			ShaDigest:  "sha256:abc",
			Size:       10 * 1024 * 1024,
			LayerCount: 5,
		},
	}

	strategy := &models.SelectionStrategy{
		RandomMode: false,
		SmallCount: 1,
	}

	// Mark image as already existing
	existing := make(map[string]*registry.ExistingImage)
	existing["sha256:abc"] = &registry.ExistingImage{
		Image: images[0],
		Size:  images[0].Size,
	}

	selected, result := planner.AssessAndSelect(images, strategy, existing)

	if result.ExistingCount != 1 {
		t.Errorf("ExistingCount = %d, want 1", result.ExistingCount)
	}
	if result.SelectedCount != 0 {
		t.Errorf("SelectedCount = %d, want 0", result.SelectedCount)
	}
	if len(selected) != 0 {
		t.Errorf("selected length = %d, want 0", len(selected))
	}
	if !result.SkipDownload {
		t.Errorf("SkipDownload = false, want true")
	}
}
