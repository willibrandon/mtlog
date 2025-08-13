package parser

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

// Constants for cache configuration
const (
	defaultMaxSize       = 10_000
	maxShardCount        = 64
	targetEntriesPerShard = 250
)

// cacheStats tracks cache performance metrics
type cacheStats struct {
	hits       atomic.Uint64
	misses     atomic.Uint64
	evictions  atomic.Uint64
	expirations atomic.Uint64
}

// Stats returns a snapshot of cache statistics
type Stats struct {
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Expirations uint64
	Size        int
	MaxSize     int
}

// lruEntry represents an entry in the LRU cache
type lruEntry struct {
	key      string
	value    *MessageTemplate
	element  *list.Element
	expireAt int64 // Unix nano, 0 means no expiration
}

// lruShard is a single shard of the sharded LRU cache
type lruShard struct {
	mu       sync.Mutex
	entries  map[string]*lruEntry
	eviction *list.List
	maxSize  int
	ttlNanos int64
}

// TemplateCache is a concurrent, sharded LRU cache with optional TTL
type TemplateCache struct {
	shards    []*lruShard
	shardMask uint32
	stats     cacheStats
	
	// Configuration (immutable after creation)
	maxSize  int
	ttlNanos int64
	
	// Cleanup management
	cleanupStop chan struct{}
	cleanupOnce sync.Once
}

var (
	// Global template cache instance using atomic pointer for safe reconfiguration
	globalCache atomic.Pointer[TemplateCache]
	globalCacheOnce sync.Once
	globalCacheConfigured atomic.Bool
)

// CacheOption configures the template cache
type CacheOption func(*TemplateCache)

// WithMaxSize sets the maximum number of entries (default: 10,000)
func WithMaxSize(size int) CacheOption {
	return func(c *TemplateCache) {
		if size > 0 {
			c.maxSize = size
		}
	}
}

// WithTTL sets the time-to-live for cache entries
func WithTTL(ttl time.Duration) CacheOption {
	return func(c *TemplateCache) {
		if ttl > 0 {
			c.ttlNanos = ttl.Nanoseconds()
		}
	}
}

// initGlobalCache initializes the global cache with options
func initGlobalCache() {
	// Only initialize with default settings if not already configured
	if globalCacheConfigured.Load() {
		return
	}
	globalCacheOnce.Do(func() {
		cache := NewTemplateCache()
		globalCache.Store(cache)
	})
}

// NewTemplateCache creates a new template cache
func NewTemplateCache(opts ...CacheOption) *TemplateCache {
	c := &TemplateCache{
		maxSize: defaultMaxSize,
	}
	
	for _, opt := range opts {
		opt(c)
	}
	
	// Calculate optimal number of shards (power of 2)
	numShards := 1
	for numShards<<1 <= maxShardCount && numShards<<1 <= max(1, c.maxSize/targetEntriesPerShard) {
		numShards <<= 1
	}
	
	// Distribute capacity exactly across shards
	c.shards = make([]*lruShard, numShards)
	c.shardMask = uint32(numShards - 1)
	
	base := c.maxSize / numShards
	extra := c.maxSize % numShards
	
	for i := 0; i < numShards; i++ {
		capacity := base
		if i < extra {
			capacity++
		}
		c.shards[i] = &lruShard{
			entries:  make(map[string]*lruEntry),
			eviction: list.New(),
			maxSize:  capacity,
			ttlNanos: c.ttlNanos,
		}
	}
	
	// Start cleanup if TTL is configured
	if c.ttlNanos > 0 {
		c.cleanupStop = make(chan struct{})
		go c.cleanupLoop()
	}
	
	return c
}

// getShard returns the shard for a given key using FNV-1a hash
func (c *TemplateCache) getShard(key string) *lruShard {
	// FNV-1a 32-bit hash for better distribution
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	// Mix bits for better avalanche effect
	hash ^= hash >> 16
	hash *= 0x85ebca6b
	hash ^= hash >> 13
	hash *= 0xc2b2ae35
	hash ^= hash >> 16
	
	return c.shards[hash&c.shardMask]
}

// Get retrieves a template from the cache
func (c *TemplateCache) Get(key string) (*MessageTemplate, bool) {
	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	
	entry, ok := shard.entries[key]
	if !ok {
		c.stats.misses.Add(1)
		return nil, false
	}
	
	// Check expiration
	if entry.expireAt > 0 && time.Now().UnixNano() > entry.expireAt {
		shard.removeEntry(entry)
		c.stats.expirations.Add(1)
		c.stats.misses.Add(1)
		return nil, false
	}
	
	// Move to front (most recently used)
	shard.eviction.MoveToFront(entry.element)
	c.stats.hits.Add(1)
	return entry.value, true
}

// Put adds a template to the cache
func (c *TemplateCache) Put(key string, value *MessageTemplate) {
	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	
	// Check if already exists
	if entry, ok := shard.entries[key]; ok {
		entry.value = value
		if shard.ttlNanos > 0 {
			entry.expireAt = time.Now().UnixNano() + shard.ttlNanos
		}
		shard.eviction.MoveToFront(entry.element)
		return
	}
	
	// Evict if at capacity
	if len(shard.entries) >= shard.maxSize {
		oldest := shard.eviction.Back()
		if oldest != nil {
			shard.removeEntry(oldest.Value.(*lruEntry))
			c.stats.evictions.Add(1)
		}
	}
	
	// Add new entry
	entry := &lruEntry{
		key:   key,
		value: value,
	}
	if shard.ttlNanos > 0 {
		entry.expireAt = time.Now().UnixNano() + shard.ttlNanos
	}
	entry.element = shard.eviction.PushFront(entry)
	shard.entries[key] = entry
}

// Delete removes an entry from the cache
func (c *TemplateCache) Delete(key string) bool {
	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	
	entry, ok := shard.entries[key]
	if !ok {
		return false
	}
	
	shard.removeEntry(entry)
	return true
}

// removeEntry removes an entry from the shard (must be called with lock held)
func (s *lruShard) removeEntry(entry *lruEntry) {
	delete(s.entries, entry.key)
	s.eviction.Remove(entry.element)
}

// Stats returns current cache statistics
func (c *TemplateCache) Stats() Stats {
	size := 0
	for _, shard := range c.shards {
		shard.mu.Lock()
		size += len(shard.entries)
		shard.mu.Unlock()
	}
	
	return Stats{
		Hits:        c.stats.hits.Load(),
		Misses:      c.stats.misses.Load(),
		Evictions:   c.stats.evictions.Load(),
		Expirations: c.stats.expirations.Load(),
		Size:        size,
		MaxSize:     c.maxSize,
	}
}

// Clear removes all entries from the cache and resets statistics
func (c *TemplateCache) Clear() {
	for _, shard := range c.shards {
		shard.mu.Lock()
		shard.entries = make(map[string]*lruEntry)
		shard.eviction = list.New()
		shard.mu.Unlock()
	}
	
	// Reset statistics
	c.stats.hits.Store(0)
	c.stats.misses.Store(0)
	c.stats.evictions.Store(0)
	c.stats.expirations.Store(0)
}

// Close stops the cleanup goroutine if running
func (c *TemplateCache) Close() {
	c.cleanupOnce.Do(func() {
		if c.cleanupStop != nil {
			close(c.cleanupStop)
		}
	})
}

// cleanupLoop runs periodic cleanup for expired entries
func (c *TemplateCache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.cleanupStop:
			return
		}
	}
}

// cleanupExpired removes expired entries from all shards
func (c *TemplateCache) cleanupExpired() {
	now := time.Now().UnixNano()
	
	for _, shard := range c.shards {
		shard.mu.Lock()
		
		// Iterate through LRU list from back (oldest)
		for elem := shard.eviction.Back(); elem != nil; {
			entry := elem.Value.(*lruEntry)
			if entry.expireAt == 0 || entry.expireAt > now {
				break // Rest of list is newer
			}
			
			prev := elem.Prev()
			shard.removeEntry(entry)
			c.stats.expirations.Add(1)
			elem = prev
		}
		
		shard.mu.Unlock()
	}
}

// ConfigureGlobalCache sets up the global cache with options
// This should only be called at application startup before any cache usage
// Panics if called more than once to prevent runtime reconfiguration issues
func ConfigureGlobalCache(opts ...CacheOption) {
	if !globalCacheConfigured.CompareAndSwap(false, true) {
		panic("mtlog: global template cache already configured - reconfiguration not allowed")
	}
	
	cache := NewTemplateCache(opts...)
	oldCache := globalCache.Swap(cache)
	if oldCache != nil {
		oldCache.Close()
	}
}

// ResetGlobalCacheForTesting allows tests to reset the configuration flag
// This should ONLY be used in tests, never in production code
func ResetGlobalCacheForTesting() {
	globalCacheConfigured.Store(false)
	globalCacheOnce = sync.Once{}
}

// GetGlobalCache returns the current global cache instance
func GetGlobalCache() *TemplateCache {
	initGlobalCache()
	return globalCache.Load()
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}