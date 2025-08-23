package enrichers

import (
	"container/list"
	"context"
	"sync"
)

// firstWarningSet is a bounded LRU set that tracks which contexts have had their first warning.
// It's designed to be much larger than the deadline cache to maintain semantic correctness
// even when the main cache evicts entries.
type firstWarningSet struct {
	mu       sync.RWMutex
	maxSize  int
	items    map[context.Context]*list.Element
	lruList  *list.List
}

// newFirstWarningSet creates a new first warning tracking set.
func newFirstWarningSet(maxSize int) *firstWarningSet {
	return &firstWarningSet{
		maxSize:  maxSize,
		items:    make(map[context.Context]*list.Element),
		lruList:  list.New(),
	}
}

// markWarned marks a context as having received its first warning.
// Returns true if this was the first time marking this context.
func (s *firstWarningSet) markWarned(ctx context.Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already warned
	if elem, exists := s.items[ctx]; exists {
		// Move to front (most recently used)
		s.lruList.MoveToFront(elem)
		return false // Already warned
	}

	// Add new entry
	elem := s.lruList.PushFront(ctx)
	s.items[ctx] = elem

	// Evict oldest if over capacity
	if s.lruList.Len() > s.maxSize {
		oldest := s.lruList.Back()
		if oldest != nil {
			s.lruList.Remove(oldest)
			delete(s.items, oldest.Value.(context.Context))
		}
	}

	return true // First warning
}

// size returns the current number of contexts tracked.
func (s *firstWarningSet) size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.lruList.Len()
}