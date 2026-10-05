package downloader

import (
	"fmt"
	"sync"
	"time"
)

// Progress tracks and reports real-time progress
type Progress struct {
	count     int64
	startTime time.Time
	mu        sync.Mutex
}

// NewProgress creates a new progress tracker
func NewProgress() *Progress {
	return &Progress{
		startTime: time.Now(),
	}
}

// Increment increments the counter
func (p *Progress) Increment() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.count++
}

// Count returns the current count
func (p *Progress) Count() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}

// Elapsed returns the elapsed time
func (p *Progress) Elapsed() time.Duration {
	return time.Since(p.startTime)
}

// Rate returns the operations per second
func (p *Progress) Rate() float64 {
	elapsed := p.Elapsed().Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(p.Count()) / elapsed
}

// String returns a formatted progress string
func (p *Progress) String() string {
	return fmt.Sprintf("[%d] Rate: %.2f img/s | Elapsed: %v",
		p.Count(),
		p.Rate(),
		p.Elapsed().Round(time.Second),
	)
}

// FormatWithTarget returns progress string with target count
func (p *Progress) FormatWithTarget(total int) string {
	return fmt.Sprintf("[%d/%d] Rate: %.2f img/s | Elapsed: %v",
		p.Count(),
		total,
		p.Rate(),
		p.Elapsed().Round(time.Second),
	)
}
