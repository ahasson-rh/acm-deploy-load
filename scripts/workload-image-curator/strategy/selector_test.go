package strategy

import (
	"testing"

	"github.com/acm-deploy-load/workload-image-curator/models"
)

func TestNewStrategySelector(t *testing.T) {
	selector := NewStrategySelector()
	if selector == nil {
		t.Fatal("NewStrategySelector returned nil")
	}
}

func TestSelectBySize(t *testing.T) {
	selector := NewStrategySelector()

	small := []*models.OperatorImage{
		{Operator: "s1", Size: 10 * 1024 * 1024},
		{Operator: "s2", Size: 20 * 1024 * 1024},
		{Operator: "s3", Size: 30 * 1024 * 1024},
	}

	medium := []*models.OperatorImage{
		{Operator: "m1", Size: 100 * 1024 * 1024},
		{Operator: "m2", Size: 150 * 1024 * 1024},
	}

	large := []*models.OperatorImage{
		{Operator: "l1", Size: 500 * 1024 * 1024},
	}

	selected := selector.SelectBySize(small, medium, large, 2, 1, 1)

	if len(selected) != 4 {
		t.Errorf("SelectBySize returned %d images, want 4", len(selected))
	}

	// Verify counts per category
	smallCount := 0
	mediumCount := 0
	largeCount := 0

	for _, img := range selected {
		if img.Size < 50*1024*1024 {
			smallCount++
		} else if img.Size < 200*1024*1024 {
			mediumCount++
		} else {
			largeCount++
		}
	}

	if smallCount != 2 {
		t.Errorf("small count = %d, want 2", smallCount)
	}
	if mediumCount != 1 {
		t.Errorf("medium count = %d, want 1", mediumCount)
	}
	if largeCount != 1 {
		t.Errorf("large count = %d, want 1", largeCount)
	}
}

func TestSelectBySize_ZeroTargets(t *testing.T) {
	selector := NewStrategySelector()

	small := []*models.OperatorImage{
		{Operator: "s1", Size: 10 * 1024 * 1024},
	}

	selected := selector.SelectBySize(small, nil, nil, 0, 0, 0)

	if len(selected) != 0 {
		t.Errorf("SelectBySize with 0 targets returned %d images, want 0", len(selected))
	}
}

func TestSelectRandom(t *testing.T) {
	selector := NewStrategySelector()

	images := []*models.OperatorImage{
		{Operator: "op1", Size: 10 * 1024 * 1024},
		{Operator: "op2", Size: 20 * 1024 * 1024},
		{Operator: "op3", Size: 30 * 1024 * 1024},
		{Operator: "op4", Size: 40 * 1024 * 1024},
		{Operator: "op5", Size: 50 * 1024 * 1024},
	}

	selected := selector.SelectRandom(images, 3)

	if len(selected) != 3 {
		t.Errorf("SelectRandom(3) returned %d images, want 3", len(selected))
	}

	// Should select first 3 (since input is pre-sorted by size)
	if selected[0].Operator != "op1" {
		t.Errorf("First selected = %q, want %q", selected[0].Operator, "op1")
	}
}

func TestSelectRandom_MoreThanAvailable(t *testing.T) {
	selector := NewStrategySelector()

	images := []*models.OperatorImage{
		{Operator: "op1", Size: 10 * 1024 * 1024},
		{Operator: "op2", Size: 20 * 1024 * 1024},
	}

	selected := selector.SelectRandom(images, 5)

	if len(selected) != 2 {
		t.Errorf("SelectRandom(5) with 2 available returned %d images, want 2", len(selected))
	}
}
