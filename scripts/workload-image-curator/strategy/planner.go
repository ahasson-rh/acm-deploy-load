package strategy

import (
	"fmt"

	"github.com/acm-deploy-load/workload-image-curator/categorizer"
	"github.com/acm-deploy-load/workload-image-curator/models"
	"github.com/acm-deploy-load/workload-image-curator/registry"
)

// PreAssessmentResult contains results from pre-run registry assessment
type PreAssessmentResult struct {
	ExistingCount     int                              // Total images already in target registry
	ExistingBySize    map[models.ImageCategory]int    // Breakdown of existing images by size
	SelectedCount     int                              // Total images selected for download
	SelectedBySize    map[models.ImageCategory]int    // Breakdown of selected images by size
	TotalDownloadSize int64                           // Total bytes to download
	SkipDownload      bool                            // True if strategy already satisfied
}

// Planner performs pre-run assessment of target registry state
type Planner struct {
	inspector  *registry.MirrorInspector
	categorizer *categorizer.Categorizer
}

// NewPlanner creates a new pre-assessment planner
func NewPlanner(inspector *registry.MirrorInspector, thresholds *categorizer.SizeThresholds) *Planner {
	return &Planner{
		inspector:  inspector,
		categorizer: categorizer.NewCategorizer(thresholds),
	}
}

// AssessAndSelect performs pre-assessment and selects images needed to satisfy strategy
// Returns the filtered image list and assessment results
func (p *Planner) AssessAndSelect(
	allImages []*models.OperatorImage,
	strategy *models.SelectionStrategy,
	existing map[string]*registry.ExistingImage,
) ([]*models.OperatorImage, *PreAssessmentResult) {

	result := &PreAssessmentResult{
		ExistingBySize: make(map[models.ImageCategory]int),
		SelectedBySize: make(map[models.ImageCategory]int),
	}

	// Count existing images by size category
	existingByCategory := make(map[models.ImageCategory][]*models.OperatorImage)
	for _, existImg := range existing {
		cat := p.categorizer.CategorizeImage(existImg.Image)
		existingByCategory[cat] = append(existingByCategory[cat], existImg.Image)
		result.ExistingCount++
		result.ExistingBySize[cat]++
	}

	// Calculate remaining needed per category based on strategy
	needed := calculateRemaining(strategy, result.ExistingBySize)

	// If nothing needed, skip download
	if totalNeeded(needed) == 0 {
		result.SkipDownload = true
		return nil, result
	}

	// Filter out existing images and categorize remaining
	remainingByCategory := make(map[models.ImageCategory][]*models.OperatorImage)
	for _, img := range allImages {
		if _, alreadyExists := existing[img.ShaDigest]; alreadyExists {
			continue // Skip images already in target
		}

		cat := p.categorizer.CategorizeImage(img)
		remainingByCategory[cat] = append(remainingByCategory[cat], img)
	}

	// Select images from remaining to satisfy needed counts
	var selected []*models.OperatorImage
	for cat, count := range needed {
		if count > 0 && len(remainingByCategory[cat]) > 0 {
			available := remainingByCategory[cat]
			if count > len(available) {
				count = len(available)
			}
			selected = append(selected, available[:count]...)
			result.SelectedBySize[cat] = count

			// Calculate download size
			for i := 0; i < count; i++ {
				result.TotalDownloadSize += available[i].Size
			}
		}
	}

	result.SelectedCount = len(selected)

	// Check if strategy is satisfied
	if result.SelectedCount == 0 && result.ExistingCount >= strategy.Total() {
		result.SkipDownload = true
	}

	return selected, result
}

// calculateRemaining determines how many more images are needed per size category
func calculateRemaining(strategy *models.SelectionStrategy, existing map[models.ImageCategory]int) map[models.ImageCategory]int {
	needed := make(map[models.ImageCategory]int)

	if strategy.RandomMode {
		// In random mode, just track total needed
		total := strategy.Count - existing[models.CategorySmall] - existing[models.CategoryMedium] - existing[models.CategoryLarge]
		if total < 0 {
			total = 0
		}
		// Distribute evenly across categories
		perCategory := total / 3
		remainder := total % 3
		needed[models.CategorySmall] = perCategory
		needed[models.CategoryMedium] = perCategory
		needed[models.CategoryLarge] = perCategory + remainder
		return needed
	}

	// Size-based mode: calculate per-category needs
	needed[models.CategorySmall] = strategy.Small() - existing[models.CategorySmall]
	if needed[models.CategorySmall] < 0 {
		needed[models.CategorySmall] = 0
	}

	needed[models.CategoryMedium] = strategy.Medium() - existing[models.CategoryMedium]
	if needed[models.CategoryMedium] < 0 {
		needed[models.CategoryMedium] = 0
	}

	needed[models.CategoryLarge] = strategy.Large() - existing[models.CategoryLarge]
	if needed[models.CategoryLarge] < 0 {
		needed[models.CategoryLarge] = 0
	}

	return needed
}

// totalNeeded returns the sum of all needed images across categories
func totalNeeded(needed map[models.ImageCategory]int) int {
	total := 0
	for _, count := range needed {
		total += count
	}
	return total
}

// FormatSize converts bytes to human-readable format (GB, MB, KB)
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
