package categorizer

import (
	"testing"

	"github.com/acm-deploy-load/workload-image-curator/models"
)

func TestCategorizeImage(t *testing.T) {
	thresholds := DefaultThresholds()
	c := NewCategorizer(thresholds)

	tests := []struct {
		name     string
		size     int64
		expected models.ImageCategory
	}{
		{"Small image", 1024 * 1024, models.CategorySmall},           // 1MB
		{"Small image at threshold", 52428800 - 1, models.CategorySmall},
		{"Medium image at threshold", 52428800, models.CategoryMedium},
		{"Medium image", 100 * 1024 * 1024, models.CategoryMedium}, // 100MB
		{"Medium image at large threshold", 209715200 - 1, models.CategoryMedium},
		{"Large image at threshold", 209715200, models.CategoryLarge},
		{"Large image", 500 * 1024 * 1024, models.CategoryLarge}, // 500MB
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := &models.OperatorImage{
				Operator:   "test-op",
				Size:       tt.size,
				LayerCount: 5,
			}
			got := c.CategorizeImage(img)
			if got != tt.expected {
				t.Errorf("CategorizeImage(%d bytes) = %v, want %v", tt.size, got, tt.expected)
			}
		})
	}
}

func TestCategorize(t *testing.T) {
	thresholds := DefaultThresholds()
	c := NewCategorizer(thresholds)

	images := []*models.OperatorImage{
		{Operator: "small1", Size: 10 * 1024 * 1024, LayerCount: 3},   // 10MB
		{Operator: "small2", Size: 30 * 1024 * 1024, LayerCount: 5},   // 30MB
		{Operator: "medium1", Size: 100 * 1024 * 1024, LayerCount: 15}, // 100MB
		{Operator: "medium2", Size: 150 * 1024 * 1024, LayerCount: 20}, // 150MB
		{Operator: "large1", Size: 300 * 1024 * 1024, LayerCount: 30}, // 300MB
		{Operator: "large2", Size: 500 * 1024 * 1024, LayerCount: 40}, // 500MB
	}

	categorized, stats := c.Categorize(images)

	if len(categorized.Small) != 2 {
		t.Errorf("Expected 2 small images, got %d", len(categorized.Small))
	}
	if len(categorized.Medium) != 2 {
		t.Errorf("Expected 2 medium images, got %d", len(categorized.Medium))
	}
	if len(categorized.Large) != 2 {
		t.Errorf("Expected 2 large images, got %d", len(categorized.Large))
	}

	// Verify stats
	smallStats := stats[models.CategorySmall]
	if smallStats.Count != 2 {
		t.Errorf("Small category count: expected 2, got %d", smallStats.Count)
	}
	if smallStats.SmallestSize != 10*1024*1024 {
		t.Errorf("Small smallest size: expected 10485760, got %d", smallStats.SmallestSize)
	}
	if smallStats.LargestSize != 30*1024*1024 {
		t.Errorf("Small largest size: expected 31457280, got %d", smallStats.LargestSize)
	}

	// Verify sorting (smallest first)
	if categorized.Small[0].Size > categorized.Small[1].Size {
		t.Error("Small images not sorted by size")
	}
	if categorized.Medium[0].Size > categorized.Medium[1].Size {
		t.Error("Medium images not sorted by size")
	}
}

func TestSelectByDistribution(t *testing.T) {
	thresholds := DefaultThresholds()
	c := NewCategorizer(thresholds)

	images := []*models.OperatorImage{
		{Operator: "small1", Size: 10 * 1024 * 1024, LayerCount: 3},
		{Operator: "small2", Size: 20 * 1024 * 1024, LayerCount: 5},
		{Operator: "small3", Size: 30 * 1024 * 1024, LayerCount: 5},
		{Operator: "medium1", Size: 100 * 1024 * 1024, LayerCount: 15},
		{Operator: "medium2", Size: 150 * 1024 * 1024, LayerCount: 20},
		{Operator: "large1", Size: 300 * 1024 * 1024, LayerCount: 30},
	}

	selected := c.SelectByDistribution(images, 2, 1, 1)

	if len(selected) != 4 {
		t.Errorf("Expected 4 selected images, got %d", len(selected))
	}

	// Verify distribution
	small := 0
	medium := 0
	large := 0

	for _, img := range selected {
		cat := c.CategorizeImage(img)
		switch cat {
		case models.CategorySmall:
			small++
		case models.CategoryMedium:
			medium++
		case models.CategoryLarge:
			large++
		}
	}

	if small != 2 {
		t.Errorf("Expected 2 small, got %d", small)
	}
	if medium != 1 {
		t.Errorf("Expected 1 medium, got %d", medium)
	}
	if large != 1 {
		t.Errorf("Expected 1 large, got %d", large)
	}
}

func TestSelectByDistributionNotEnoughImages(t *testing.T) {
	thresholds := DefaultThresholds()
	c := NewCategorizer(thresholds)

	images := []*models.OperatorImage{
		{Operator: "small1", Size: 10 * 1024 * 1024, LayerCount: 3},
		{Operator: "medium1", Size: 100 * 1024 * 1024, LayerCount: 15},
	}

	// Requesting more images than available
	selected := c.SelectByDistribution(images, 5, 5, 5)

	if len(selected) != 2 {
		t.Errorf("Expected 2 selected images (all available), got %d", len(selected))
	}
}

func TestFilterByCategory(t *testing.T) {
	thresholds := DefaultThresholds()
	c := NewCategorizer(thresholds)

	images := []*models.OperatorImage{
		{Operator: "small1", Size: 10 * 1024 * 1024, LayerCount: 3},
		{Operator: "small2", Size: 30 * 1024 * 1024, LayerCount: 5},
		{Operator: "medium1", Size: 100 * 1024 * 1024, LayerCount: 15},
		{Operator: "large1", Size: 300 * 1024 * 1024, LayerCount: 30},
	}

	smallFiltered := c.FilterByCategory(images, models.CategorySmall)
	mediumFiltered := c.FilterByCategory(images, models.CategoryMedium)
	largeFiltered := c.FilterByCategory(images, models.CategoryLarge)

	if len(smallFiltered) != 2 {
		t.Errorf("Expected 2 small images, got %d", len(smallFiltered))
	}
	if len(mediumFiltered) != 1 {
		t.Errorf("Expected 1 medium image, got %d", len(mediumFiltered))
	}
	if len(largeFiltered) != 1 {
		t.Errorf("Expected 1 large image, got %d", len(largeFiltered))
	}
}

func TestCustomThresholds(t *testing.T) {
	// Custom thresholds: Small < 100MB, Large > 300MB
	thresholds, _ := NewThresholds(100*1024*1024, 300*1024*1024)
	c := NewCategorizer(thresholds)

	tests := []struct {
		name     string
		size     int64
		expected models.ImageCategory
	}{
		{"50MB should be small", 50 * 1024 * 1024, models.CategorySmall},
		{"100MB should be medium", 100 * 1024 * 1024, models.CategoryMedium},
		{"200MB should be medium", 200 * 1024 * 1024, models.CategoryMedium},
		{"300MB should be large", 300 * 1024 * 1024, models.CategoryLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := &models.OperatorImage{Size: tt.size}
			got := c.CategorizeImage(img)
			if got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.SmallThreshold != 52428800 {
		t.Errorf("Default small threshold: expected 52428800 (50MB), got %d", thresholds.SmallThreshold)
	}
	if thresholds.LargeThreshold != 209715200 {
		t.Errorf("Default large threshold: expected 209715200 (200MB), got %d", thresholds.LargeThreshold)
	}

	smallMB, largeMB := thresholds.GetThresholdMB()
	if smallMB != 50.0 {
		t.Errorf("Small threshold in MB: expected 50.0, got %f", smallMB)
	}
	if largeMB != 200.0 {
		t.Errorf("Large threshold in MB: expected 200.0, got %f", largeMB)
	}
}
