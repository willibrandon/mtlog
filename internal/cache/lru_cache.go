package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

// LRUCache provides a thread-safe LRU cache with optional TTL.
type LRUCache struct {
	capacity  int
	items     map[string]*list.Element
	evictList *list.List
	mu        sync.RWMutex
	
	// Statistics
	hits   atomic.Uint64
	misses atomic.Uint64
	evictions atomic.Uint64
}

// entry represents a cache entry.
type entry struct {
	key       string
	value     interface{}
	timestamp time.Time
	ttl       time.Duration
}

// NewLRUCache creates a new LRU cache with the specified capacity.
func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		capacity = 10000 // Default capacity
	}
	
	return &LRUCache{
		capacity:  capacity,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	elem, exists := c.items[key]
	if !exists {
		c.misses.Add(1)
		return nil, false
	}
	
	ent := elem.Value.(*entry)
	
	// Check TTL if set
	if ent.ttl > 0 && time.Since(ent.timestamp) > ent.ttl {
		c.evictList.Remove(elem)
		delete(c.items, key)
		c.evictions.Add(1)
		c.misses.Add(1)
		return nil, false
	}
	
	// Move to front (most recently used)
	c.evictList.MoveToFront(elem)
	c.hits.Add(1)
	return ent.value, true
}

// GetOrCreate retrieves a value from the cache or creates it using the provided function.
func (c *LRUCache) GetOrCreate(key string, createFunc func() interface{}) interface{} {
	if value, exists := c.Get(key); exists {
		return value
	}
	
	value := createFunc()
	c.Put(key, value, 0)
	return value
}

// Put adds or updates a value in the cache.
func (c *LRUCache) Put(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if key exists
	if elem, exists := c.items[key]; exists {
		// Update existing entry
		c.evictList.MoveToFront(elem)
		ent := elem.Value.(*entry)
		ent.value = value
		ent.timestamp = time.Now()
		ent.ttl = ttl
		return
	}
	
	// Add new entry
	ent := &entry{
		key:       key,
		value:     value,
		timestamp: time.Now(),
		ttl:       ttl,
	}
	
	elem := c.evictList.PushFront(ent)
	c.items[key] = elem
	
	// Evict oldest if over capacity
	if c.evictList.Len() > c.capacity {
		c.evictOldest()
	}
}

// evictOldest removes the least recently used item.
func (c *LRUCache) evictOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.evictList.Remove(elem)
		ent := elem.Value.(*entry)
		delete(c.items, ent.key)
		c.evictions.Add(1)
	}
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if elem, exists := c.items[key]; exists {
		c.evictList.Remove(elem)
		delete(c.items, key)
	}
}

// Clear removes all items from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[string]*list.Element)
	c.evictList.Init()
}

// Len returns the number of items in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats returns cache statistics.
func (c *LRUCache) Stats() CacheStats {
	return CacheStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
		Size:      c.Len(),
		Capacity:  c.capacity,
	}
}

// CacheStats contains cache statistics.
type CacheStats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
	Size      int
	Capacity  int
}

// Warmup pre-populates the cache with the given keys using the provided factory function.
// This helps avoid cold-start allocation spikes in high-traffic applications.
func (c *LRUCache) Warmup(keys []string, factory func(key string) interface{}) {
	for _, key := range keys {
		if _, exists := c.Get(key); !exists {
			value := factory(key)
			c.Put(key, value, 0)
		}
	}
}