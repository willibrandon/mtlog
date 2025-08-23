package enrichers

import (
	"context"
	"sync"
	"time"
	"unsafe"
)

// deadlineCacheEntry represents an entry in the deadline LRU cache.
type deadlineCacheEntry struct {
	key        context.Context
	info       *deadlineInfo
	prev       *deadlineCacheEntry
	next       *deadlineCacheEntry
	expiration time.Time
}

// deadlineLRUCacheShard represents a single shard of the LRU cache.
type deadlineLRUCacheShard struct {
	mu      sync.RWMutex
	items   map[context.Context]*deadlineCacheEntry
	head    *deadlineCacheEntry // Most recently used
	tail    *deadlineCacheEntry // Least recently used
	size    int
	maxSize int
	ttl     time.Duration
}

// deadlineLRUCache is a sharded LRU cache for deadline information.
// It uses multiple shards to reduce lock contention.
type deadlineLRUCache struct {
	shards   []*deadlineLRUCacheShard
	numShards int
	maxSize   int
	ttl       time.Duration
}

// newDeadlineLRUCache creates a new sharded LRU cache.
func newDeadlineLRUCache(maxSize int, ttl time.Duration) *deadlineLRUCache {
	// Use 16 shards for good concurrency without too much overhead
	numShards := 16
	if maxSize < numShards {
		numShards = 1
	}

	shardSize := maxSize / numShards
	if shardSize < 1 {
		shardSize = 1
	}

	cache := &deadlineLRUCache{
		shards:    make([]*deadlineLRUCacheShard, numShards),
		numShards: numShards,
		maxSize:   maxSize,
		ttl:       ttl,
	}

	for i := 0; i < numShards; i++ {
		cache.shards[i] = &deadlineLRUCacheShard{
			items:   make(map[context.Context]*deadlineCacheEntry),
			maxSize: shardSize,
			ttl:     ttl,
		}
	}

	return cache
}

// getShard returns the shard for the given context.
func (c *deadlineLRUCache) getShard(ctx context.Context) *deadlineLRUCacheShard {
	// Use pointer address for sharding
	// This provides good distribution across shards
	ptr := uintptr(unsafe.Pointer(&ctx))
	index := int(ptr % uintptr(c.numShards))
	return c.shards[index]
}

// get retrieves deadline information from the cache.
func (c *deadlineLRUCache) get(ctx context.Context) *deadlineInfo {
	shard := c.getShard(ctx)
	return shard.get(ctx)
}

// put stores deadline information in the cache.
func (c *deadlineLRUCache) put(ctx context.Context, info *deadlineInfo) {
	shard := c.getShard(ctx)
	shard.put(ctx, info)
}

// getOrCreate retrieves or creates deadline information for a context.
func (c *deadlineLRUCache) getOrCreate(ctx context.Context) *deadlineInfo {
	shard := c.getShard(ctx)
	return shard.getOrCreate(ctx)
}

// Shard methods

func (s *deadlineLRUCacheShard) get(ctx context.Context) *deadlineInfo {
	s.mu.RLock()
	entry, ok := s.items[ctx]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	// Check TTL
	if s.ttl > 0 && time.Now().After(entry.expiration) {
		// Entry expired, remove it
		s.mu.Lock()
		s.removeEntry(entry)
		delete(s.items, ctx)
		s.size--
		s.mu.Unlock()
		return nil
	}

	// Move to front (most recently used)
	s.mu.Lock()
	s.moveToFront(entry)
	s.mu.Unlock()

	return entry.info
}

func (s *deadlineLRUCacheShard) put(ctx context.Context, info *deadlineInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	if entry, ok := s.items[ctx]; ok {
		// Update existing entry
		entry.info = info
		entry.expiration = time.Now().Add(s.ttl)
		s.moveToFront(entry)
		return
	}

	// Check if we need to evict
	if s.size >= s.maxSize {
		// Evict least recently used (tail)
		if s.tail != nil {
			delete(s.items, s.tail.key)
			s.removeEntry(s.tail)
			s.size--
		}
	}

	// Create new entry
	entry := &deadlineCacheEntry{
		key:        ctx,
		info:       info,
		expiration: time.Now().Add(s.ttl),
	}

	s.items[ctx] = entry
	s.addToFront(entry)
	s.size++
}

func (s *deadlineLRUCacheShard) getOrCreate(ctx context.Context) *deadlineInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	if entry, ok := s.items[ctx]; ok {
		// Check TTL
		if s.ttl > 0 && time.Now().After(entry.expiration) {
			// Entry expired, create new one with fresh start time
			entry.info = &deadlineInfo{
				startTime: time.Now(),
				lastCheck: time.Now(),
			}
			entry.expiration = time.Now().Add(s.ttl)
		}
		s.moveToFront(entry)
		return entry.info
	}

	// Check if we need to evict
	if s.size >= s.maxSize {
		// Evict least recently used (tail)
		if s.tail != nil {
			delete(s.items, s.tail.key)
			s.removeEntry(s.tail)
			s.size--
		}
	}

	// Create new entry - startTime will be set by enricher on first use
	info := &deadlineInfo{
		lastCheck: time.Now(),
	}
	entry := &deadlineCacheEntry{
		key:        ctx,
		info:       info,
		expiration: time.Now().Add(s.ttl),
	}

	s.items[ctx] = entry
	s.addToFront(entry)
	s.size++

	return info
}

// moveToFront moves an entry to the front of the LRU list.
func (s *deadlineLRUCacheShard) moveToFront(entry *deadlineCacheEntry) {
	if s.head == entry {
		return // Already at front
	}
	s.removeEntry(entry)
	s.addToFront(entry)
}

// removeEntry removes an entry from the LRU list.
func (s *deadlineLRUCacheShard) removeEntry(entry *deadlineCacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		s.head = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		s.tail = entry.prev
	}

	entry.prev = nil
	entry.next = nil
}

// addToFront adds an entry to the front of the LRU list.
func (s *deadlineLRUCacheShard) addToFront(entry *deadlineCacheEntry) {
	entry.next = s.head
	entry.prev = nil

	if s.head != nil {
		s.head.prev = entry
	}
	s.head = entry

	if s.tail == nil {
		s.tail = entry
	}
}

// size returns the total number of entries across all shards.
func (c *deadlineLRUCache) size() int {
	if c == nil {
		return 0
	}
	
	total := 0
	for _, shard := range c.shards {
		shard.mu.RLock()
		total += shard.size
		shard.mu.RUnlock()
	}
	return total
}