package sentry

import (
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
)

// BreadcrumbBuffer is a thread-safe ring buffer for storing breadcrumbs.
// It automatically evicts old breadcrumbs based on age and capacity.
type BreadcrumbBuffer struct {
	mu      sync.RWMutex
	items   []breadcrumbEntry
	maxSize int
	head    int
	tail    int
	size    int
	maxAge  time.Duration
}

type breadcrumbEntry struct {
	breadcrumb sentry.Breadcrumb
	addedAt    time.Time
}

// NewBreadcrumbBuffer creates a new breadcrumb buffer with the specified capacity.
func NewBreadcrumbBuffer(maxSize int) *BreadcrumbBuffer {
	if maxSize < 1 {
		maxSize = 1
	}
	return &BreadcrumbBuffer{
		items:   make([]breadcrumbEntry, maxSize),
		maxSize: maxSize,
		maxAge:  5 * time.Minute, // Default max age
	}
}

// Add adds a breadcrumb to the buffer.
func (b *BreadcrumbBuffer) Add(breadcrumb sentry.Breadcrumb) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry := breadcrumbEntry{
		breadcrumb: breadcrumb,
		addedAt:    time.Now(),
	}

	if b.size < b.maxSize {
		// Buffer not full yet
		b.items[b.tail] = entry
		b.tail = (b.tail + 1) % b.maxSize
		b.size++
	} else {
		// Buffer full, overwrite oldest
		b.items[b.head] = entry
		b.head = (b.head + 1) % b.maxSize
		b.tail = (b.tail + 1) % b.maxSize
	}
}

// GetAll returns all valid breadcrumbs in chronological order.
func (b *BreadcrumbBuffer) GetAll() []*sentry.Breadcrumb {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == 0 {
		return nil
	}

	result := make([]*sentry.Breadcrumb, 0, b.size)
	now := time.Now()
	cutoff := now.Add(-b.maxAge)

	// Iterate through the buffer in order
	for i := 0; i < b.size; i++ {
		idx := (b.head + i) % b.maxSize
		entry := b.items[idx]

		// Skip breadcrumbs that are too old
		if entry.addedAt.Before(cutoff) {
			continue
		}

		// Return pointer to breadcrumb
		breadcrumb := entry.breadcrumb
		result = append(result, &breadcrumb)
	}

	return result
}

// Clear removes all breadcrumbs from the buffer.
func (b *BreadcrumbBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.head = 0
	b.tail = 0
	b.size = 0
}

// SetMaxAge sets the maximum age for breadcrumbs.
func (b *BreadcrumbBuffer) SetMaxAge(maxAge time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.maxAge = maxAge
}

// Size returns the current number of breadcrumbs in the buffer.
func (b *BreadcrumbBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size
}