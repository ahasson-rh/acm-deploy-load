package downloader

import (
	"testing"
	"time"
)

func TestNewWorkerProgressTracker(t *testing.T) {
	tracker := NewWorkerProgressTracker(100, 5)

	if tracker.totalImages != 100 {
		t.Errorf("Expected 100 images, got %d", tracker.totalImages)
	}

	if len(tracker.workers) != 5 {
		t.Errorf("Expected 5 workers, got %d", len(tracker.workers))
	}
}

func TestUpdateWorkerImage(t *testing.T) {
	tracker := NewWorkerProgressTracker(10, 2)

	tracker.UpdateWorkerImage(1, "quay.io/test/image1:latest")

	stats := tracker.GetStats()
	if stats[1].CurrentImage != "quay.io/test/image1:latest" {
		t.Errorf("Expected image not set correctly")
	}
}

func TestRecordSuccess(t *testing.T) {
	tracker := NewWorkerProgressTracker(10, 2)

	tracker.RecordSuccess(1)
	tracker.RecordSuccess(1)
	tracker.RecordSuccess(2)

	stats := tracker.GetStats()
	if stats[1].SuccessCount != 2 {
		t.Errorf("Expected 2 successes for worker 1, got %d", stats[1].SuccessCount)
	}
	if stats[2].SuccessCount != 1 {
		t.Errorf("Expected 1 success for worker 2, got %d", stats[2].SuccessCount)
	}
}

func TestRecordFailure(t *testing.T) {
	tracker := NewWorkerProgressTracker(10, 2)

	tracker.RecordFailure(1)
	tracker.RecordFailure(2)
	tracker.RecordFailure(2)

	stats := tracker.GetStats()
	if stats[1].FailureCount != 1 {
		t.Errorf("Expected 1 failure for worker 1, got %d", stats[1].FailureCount)
	}
	if stats[2].FailureCount != 2 {
		t.Errorf("Expected 2 failures for worker 2, got %d", stats[2].FailureCount)
	}
}

func TestSummary(t *testing.T) {
	tracker := NewWorkerProgressTracker(10, 3)

	tracker.UpdateWorkerImage(1, "image1")
	tracker.RecordSuccess(1)
	tracker.RecordSuccess(2)
	tracker.RecordFailure(3)

	summary := tracker.Summary()
	if len(summary) == 0 {
		t.Error("Summary is empty")
	}

	if testing.Verbose() {
		t.Logf("Summary: %s", summary)
	}
}

func TestWorkerStatus(t *testing.T) {
	tracker := NewWorkerProgressTracker(10, 2)

	tracker.UpdateWorkerImage(1, "image1")
	tracker.RecordSuccess(1)
	tracker.RecordFailure(2)

	status1 := tracker.WorkerStatus(1)
	if len(status1) == 0 {
		t.Error("Worker 1 status is empty")
	}

	status2 := tracker.WorkerStatus(2)
	if len(status2) == 0 {
		t.Error("Worker 2 status is empty")
	}

	if testing.Verbose() {
		t.Logf("Worker 1: %s", status1)
		t.Logf("Worker 2: %s", status2)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tracker := NewWorkerProgressTracker(1000, 10)

	// Simulate concurrent worker updates
	done := make(chan bool, 10)

	for w := 1; w <= 10; w++ {
		go func(workerID int) {
			for i := 0; i < 100; i++ {
				tracker.UpdateWorkerImage(workerID, "image_"+string(rune(i)))
				tracker.RecordSuccess(workerID)
				time.Sleep(1 * time.Millisecond) // Simulate work
			}
			done <- true
		}(w)
	}

	// Wait for all workers
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify totals
	stats := tracker.GetStats()
	totalSuccess := int64(0)
	for _, ws := range stats {
		totalSuccess += ws.SuccessCount
	}

	if totalSuccess != 1000 {
		t.Errorf("Expected 1000 total successes, got %d", totalSuccess)
	}
}
