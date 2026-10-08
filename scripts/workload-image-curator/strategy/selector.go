package strategy

import "github.com/acm-deploy-load/workload-image-curator/models"

// StrategySelector selects images to match a given strategy distribution
type StrategySelector struct{}

// NewStrategySelector creates a new strategy selector
func NewStrategySelector() *StrategySelector {
	return &StrategySelector{}
}

// SelectBySize selects images from categorized groups to match the strategy distribution
// Returns selected images matching the size-based strategy
func (s *StrategySelector) SelectBySize(
	smallImages []*models.OperatorImage,
	mediumImages []*models.OperatorImage,
	largeImages []*models.OperatorImage,
	smallTarget int,
	mediumTarget int,
	largeTarget int,
) []*models.OperatorImage {
	var selected []*models.OperatorImage

	// Select from each category up to target count
	selected = append(selected, selectUpTo(smallImages, smallTarget)...)
	selected = append(selected, selectUpTo(mediumImages, mediumTarget)...)
	selected = append(selected, selectUpTo(largeImages, largeTarget)...)

	return selected
}

// selectUpTo selects up to n items from a list
func selectUpTo(images []*models.OperatorImage, target int) []*models.OperatorImage {
	if target <= 0 {
		return nil
	}
	if target > len(images) {
		target = len(images)
	}
	return images[:target]
}

// SelectRandom selects n random images from a list, preferring smaller ones first
func (s *StrategySelector) SelectRandom(images []*models.OperatorImage, target int) []*models.OperatorImage {
	if target <= 0 {
		return nil
	}
	if target > len(images) {
		target = len(images)
	}
	// Already sorted by size in selectImages, so just take first N
	return images[:target]
}
