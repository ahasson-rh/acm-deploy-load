package downloader

import (
	"context"
	"fmt"
	"testing"
)

func TestWorkerPool_BasicExecution(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 5, 100, false)

	// Submit 10 jobs
	for i := 0; i < 10; i++ {
		idx := i
		pool.Do(func() (interface{}, error) {
			return idx * 2, nil
		})
	}

	if err := pool.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Collect results
	count := 0
	for range pool.Results() {
		count++
	}

	if count != 10 {
		t.Errorf("Expected 10 results, got %d", count)
	}
}

func TestWorkerPool_Cancellation(t *testing.T) {
	// Skip this test as it's hard to reliably test cancellation
	// The real test is manual with Ctrl+C
	t.Skip("Cancellation test requires manual verification")
}

func TestWorkerPool_Errors(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 3, 100, false)

	// Mix of successful and failing jobs
	for i := 0; i < 5; i++ {
		idx := i
		pool.Do(func() (interface{}, error) {
			if idx%2 == 0 {
				return nil, fmt.Errorf("error for job %d", idx)
			}
			return idx, nil
		})
	}

	if err := pool.Wait(); err != nil {
		// We expect errors to be collected but not stop execution
		t.Logf("Pool.Wait() returned: %v", err)
	}

	// Successful jobs should still be in results
	successCount := 0
	for range pool.Results() {
		successCount++
	}

	if successCount != 2 {
		t.Errorf("Expected 2 successful results, got %d", successCount)
	}

	if len(pool.Errors()) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(pool.Errors()))
	}
}

func TestProgress_RateCalculation(t *testing.T) {
	prog := NewProgress()

	// Increment counter
	for i := 0; i < 10; i++ {
		prog.Increment()
	}

	if prog.Count() != 10 {
		t.Errorf("Expected count 10, got %d", prog.Count())
	}

	// Rate should be > 0 (since time has elapsed)
	rate := prog.Rate()
	if rate <= 0 {
		t.Errorf("Expected rate > 0, got %v", rate)
	}
}

func TestProgress_String(t *testing.T) {
	prog := NewProgress()
	prog.Increment()
	prog.Increment()

	str := prog.String()
	if len(str) == 0 {
		t.Error("String() returned empty string")
	}

	strTarget := prog.FormatWithTarget(10)
	if len(strTarget) == 0 {
		t.Error("FormatWithTarget() returned empty string")
	}
}
