package downloader

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerStats tracks per-worker download statistics
type WorkerStats struct {
	WorkerID      int
	CurrentImage  string
	SuccessCount  int64
	FailureCount  int64
	BytesTransfer int64
	StartTime     time.Time
	LastUpdate    time.Time
	mu            sync.RWMutex
}

// WorkerPool tracks multiple worker statistics
type WorkerProgressTracker struct {
	totalImages int
	workers     map[int]*WorkerStats
	mu          sync.RWMutex
}

// NewWorkerProgressTracker creates a new progress tracker
func NewWorkerProgressTracker(totalImages, numWorkers int) *WorkerProgressTracker {
	workers := make(map[int]*WorkerStats)
	for i := 1; i <= numWorkers; i++ {
		workers[i] = &WorkerStats{
			WorkerID:  i,
			StartTime: time.Now(),
		}
	}

	return &WorkerProgressTracker{
		totalImages: totalImages,
		workers:     workers,
	}
}

// UpdateWorkerImage updates the current image being processed by a worker
func (wpt *WorkerProgressTracker) UpdateWorkerImage(workerID int, imageRef string) {
	wpt.mu.RLock()
	ws, exists := wpt.workers[workerID]
	wpt.mu.RUnlock()

	if !exists {
		return
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.CurrentImage = imageRef
	ws.LastUpdate = time.Now()
}

// RecordSuccess increments success count for a worker
func (wpt *WorkerProgressTracker) RecordSuccess(workerID int) {
	wpt.mu.RLock()
	ws, exists := wpt.workers[workerID]
	wpt.mu.RUnlock()

	if !exists {
		return
	}

	atomic.AddInt64(&ws.SuccessCount, 1)
	ws.mu.Lock()
	ws.LastUpdate = time.Now()
	ws.mu.Unlock()
}

// RecordFailure increments failure count for a worker
func (wpt *WorkerProgressTracker) RecordFailure(workerID int) {
	wpt.mu.RLock()
	ws, exists := wpt.workers[workerID]
	wpt.mu.RUnlock()

	if !exists {
		return
	}

	atomic.AddInt64(&ws.FailureCount, 1)
	ws.mu.Lock()
	ws.LastUpdate = time.Now()
	ws.mu.Unlock()
}

// GetStats returns current statistics for all workers
func (wpt *WorkerProgressTracker) GetStats() map[int]*WorkerStats {
	wpt.mu.RLock()
	defer wpt.mu.RUnlock()

	stats := make(map[int]*WorkerStats)
	for id, ws := range wpt.workers {
		ws.mu.RLock()
		statsCopy := &WorkerStats{
			WorkerID:      ws.WorkerID,
			CurrentImage:  ws.CurrentImage,
			SuccessCount:  atomic.LoadInt64(&ws.SuccessCount),
			FailureCount:  atomic.LoadInt64(&ws.FailureCount),
			BytesTransfer: atomic.LoadInt64(&ws.BytesTransfer),
			StartTime:     ws.StartTime,
			LastUpdate:    ws.LastUpdate,
		}
		ws.mu.RUnlock()
		stats[id] = statsCopy
	}

	return stats
}

// Summary returns aggregated summary statistics
func (wpt *WorkerProgressTracker) Summary() string {
	stats := wpt.GetStats()

	totalSuccess := int64(0)
	totalFailure := int64(0)
	activeWorkers := 0

	for _, ws := range stats {
		totalSuccess += atomic.LoadInt64(&ws.SuccessCount)
		totalFailure += atomic.LoadInt64(&ws.FailureCount)
		if ws.CurrentImage != "" {
			activeWorkers++
		}
	}

	elapsed := time.Since(stats[1].StartTime)
	rate := 0.0
	if elapsed.Seconds() > 0 {
		rate = float64(totalSuccess) / elapsed.Seconds()
	}

	return fmt.Sprintf("[%d/%d] Success: %d | Failed: %d | Active: %d | Rate: %.2f img/s",
		totalSuccess+totalFailure, wpt.totalImages, totalSuccess, totalFailure, activeWorkers, rate)
}

// WorkerStatus returns a formatted status string for a specific worker
func (wpt *WorkerProgressTracker) WorkerStatus(workerID int) string {
	wpt.mu.RLock()
	ws, exists := wpt.workers[workerID]
	wpt.mu.RUnlock()

	if !exists {
		return ""
	}

	ws.mu.RLock()
	defer ws.mu.RUnlock()

	status := ""
	if ws.CurrentImage != "" {
		status = fmt.Sprintf("Worker %d: %s (S:%d F:%d)", workerID, ws.CurrentImage, ws.SuccessCount, ws.FailureCount)
	} else {
		status = fmt.Sprintf("Worker %d: idle (S:%d F:%d)", workerID, ws.SuccessCount, ws.FailureCount)
	}

	return status
}
