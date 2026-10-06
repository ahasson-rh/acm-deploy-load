package categorizer

import (
	"sort"

	"github.com/acm-deploy-load/workload-image-curator/models"
)

// Categorizer provides image size-based categorization
type Categorizer struct {
	thresholds *SizeThresholds
}

// NewCategorizer creates a new image categorizer
func NewCategorizer(thresholds *SizeThresholds) *Categorizer {
	if thresholds == nil {
		thresholds = DefaultThresholds()
	}
	return &Categorizer{
		thresholds: thresholds,
	}
}

// CategorizeImage classifies a single image by size
func (c *Categorizer) CategorizeImage(img *models.OperatorImage) models.ImageCategory {
	if img.Size < c.thresholds.SmallThreshold {
		return models.CategorySmall
	}
	if img.Size < c.thresholds.LargeThreshold {
		return models.CategoryMedium
	}
	return models.CategoryLarge
}

// CategorizedImages holds images grouped by size category
type CategorizedImages struct {
	Small  []*models.OperatorImage
	Medium []*models.OperatorImage
	Large  []*models.OperatorImage
}

// CategoryStats tracks statistics per category
type CategoryStats struct {
	Count        int
	TotalSize    int64
	AverageSize  int64
	SmallestSize int64
	LargestSize  int64
	LayerCount   int
}

// Categorize groups images by size and returns statistics
func (c *Categorizer) Categorize(images []*models.OperatorImage) (*CategorizedImages, map[models.ImageCategory]*CategoryStats) {
	categorized := &CategorizedImages{
		Small:  make([]*models.OperatorImage, 0),
		Medium: make([]*models.OperatorImage, 0),
		Large:  make([]*models.OperatorImage, 0),
	}

	stats := make(map[models.ImageCategory]*CategoryStats)
	stats[models.CategorySmall] = &CategoryStats{SmallestSize: -1, LargestSize: -1}
	stats[models.CategoryMedium] = &CategoryStats{SmallestSize: -1, LargestSize: -1}
	stats[models.CategoryLarge] = &CategoryStats{SmallestSize: -1, LargestSize: -1}

	// Categorize each image
	for _, img := range images {
		category := c.CategorizeImage(img)

		switch category {
		case models.CategorySmall:
			categorized.Small = append(categorized.Small, img)
		case models.CategoryMedium:
			categorized.Medium = append(categorized.Medium, img)
		case models.CategoryLarge:
			categorized.Large = append(categorized.Large, img)
		}

		// Update stats
		s := stats[category]
		s.Count++
		s.TotalSize += img.Size
		if s.SmallestSize < 0 || img.Size < s.SmallestSize {
			s.SmallestSize = img.Size
		}
		if img.Size > s.LargestSize {
			s.LargestSize = img.Size
		}
		s.LayerCount += img.LayerCount
	}

	// Calculate averages
	for _, s := range stats {
		if s.Count > 0 {
			s.AverageSize = s.TotalSize / int64(s.Count)
		}
	}

	// Sort within each category by size (smallest first for faster downloads)
	sort.Slice(categorized.Small, func(i, j int) bool {
		return categorized.Small[i].Size < categorized.Small[j].Size
	})
	sort.Slice(categorized.Medium, func(i, j int) bool {
		return categorized.Medium[i].Size < categorized.Medium[j].Size
	})
	sort.Slice(categorized.Large, func(i, j int) bool {
		return categorized.Large[i].Size < categorized.Large[j].Size
	})

	return categorized, stats
}

// SelectByDistribution selects images to match a distribution strategy
// Prioritizes smaller images within each category for faster overall completion
func (c *Categorizer) SelectByDistribution(
	images []*models.OperatorImage,
	smallNeeded, mediumNeeded, largeNeeded int) []*models.OperatorImage {

	categorized, _ := c.Categorize(images)

	selected := make([]*models.OperatorImage, 0)

	// Select from each category (already sorted smallest first)
	count := 0
	for i := 0; i < len(categorized.Small) && count < smallNeeded; i++ {
		selected = append(selected, categorized.Small[i])
		count++
	}

	count = 0
	for i := 0; i < len(categorized.Medium) && count < mediumNeeded; i++ {
		selected = append(selected, categorized.Medium[i])
		count++
	}

	count = 0
	for i := 0; i < len(categorized.Large) && count < largeNeeded; i++ {
		selected = append(selected, categorized.Large[i])
		count++
	}

	return selected
}

// FilterByCategory returns images in a specific category
func (c *Categorizer) FilterByCategory(images []*models.OperatorImage, category models.ImageCategory) []*models.OperatorImage {
	filtered := make([]*models.OperatorImage, 0)
	for _, img := range images {
		if c.CategorizeImage(img) == category {
			filtered = append(filtered, img)
		}
	}
	return filtered
}

// GetCategoryName returns a human-readable category name
func (c *Categorizer) GetCategoryDescription(category models.ImageCategory) string {
	switch category {
	case models.CategorySmall:
		return "small"
	case models.CategoryMedium:
		return "medium"
	case models.CategoryLarge:
		return "large"
	default:
		return "unknown"
	}
}
