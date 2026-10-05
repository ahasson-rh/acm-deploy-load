package downloader

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// Job represents a unit of work
type Job struct {
	fn func() (interface{}, error)
}

// WorkerPool manages concurrent operations with rate limiting
type WorkerPool struct {
	workers   int
	limiter   *rate.Limiter
	eg        *errgroup.Group
	ctx       context.Context
	cancel    context.CancelFunc
	progress  *Progress
	jobsChan  chan Job
	resultsCh chan interface{}
	errors    []error
	mu        sync.Mutex
}

// NewWorkerPool creates a new worker pool with rate limiting
func NewWorkerPool(ctx context.Context, workers int, rateLimit float64, showProgress bool) *WorkerPool {
	workerCtx, cancel := context.WithCancel(ctx)
	eg, egCtx := errgroup.WithContext(workerCtx)

	var prog *Progress
	if showProgress {
		prog = NewProgress()
	}

	pool := &WorkerPool{
		workers:   workers,
		limiter:   rate.NewLimiter(rate.Limit(rateLimit), 1),
		eg:        eg,
		ctx:       egCtx,
		cancel:    cancel,
		progress:  prog,
		jobsChan:  make(chan Job, workers*2),
		resultsCh: make(chan interface{}, workers*2),
		errors:    []error{},
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		pool.eg.Go(pool.worker)
	}

	return pool
}

// worker processes jobs from the job channel
func (wp *WorkerPool) worker() error {
	for job := range wp.jobsChan {
		// Wait for rate limit token
		if err := wp.limiter.Wait(wp.ctx); err != nil {
			return err
		}

		result, err := job.fn()
		if err != nil {
			wp.mu.Lock()
			wp.errors = append(wp.errors, err)
			wp.mu.Unlock()
			continue // Continue processing other jobs
		}

		if result != nil {
			wp.resultsCh <- result
		}
		if wp.progress != nil {
			wp.progress.Increment()
		}
	}
	return nil
}

// Do enqueues a job for execution
func (wp *WorkerPool) Do(fn func() (interface{}, error)) error {
	select {
	case wp.jobsChan <- Job{fn: fn}:
		return nil
	case <-wp.ctx.Done():
		return wp.ctx.Err()
	}
}

// Wait waits for all workers to complete
func (wp *WorkerPool) Wait() error {
	close(wp.jobsChan)
	if err := wp.eg.Wait(); err != nil {
		return err
	}
	close(wp.resultsCh)

	if len(wp.errors) > 0 {
		return fmt.Errorf("worker pool errors: %v", wp.errors)
	}

	return nil
}

// Cancel cancels all pending operations
func (wp *WorkerPool) Cancel() {
	wp.cancel()
}

// Results returns the results channel
func (wp *WorkerPool) Results() <-chan interface{} {
	return wp.resultsCh
}

// Errors returns collected errors
func (wp *WorkerPool) Errors() []error {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	errs := make([]error, len(wp.errors))
	copy(errs, wp.errors)
	return errs
}
