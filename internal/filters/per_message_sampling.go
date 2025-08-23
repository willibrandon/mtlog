package filters

import (
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/cache"
)

// Default constants for sampling configurations
const (
	// DefaultSamplingCacheCapacity is the default capacity for LRU caches used in sampling
	DefaultSamplingCacheCapacity = 10000
	
	// DefaultBackoffFactor is the default multiplication factor for exponential backoff when invalid values are provided
	DefaultBackoffFactor = 2.0
)

// PerMessageSamplingFilter provides per-message sampling capabilities.
type PerMessageSamplingFilter struct {
	// Counter sampling (every nth message)
	counterN     uint64
	counterValue atomic.Uint64

	// Rate sampling (percentage)
	rate        float32
	rateCounter atomic.Uint32
	randSource  *rand.Rand // Per-filter random source to avoid contention

	// Duration sampling (time-based)
	duration       time.Duration
	lastSampleTime atomic.Int64

	// First N sampling
	firstN     uint64
	firstCount atomic.Uint64

	// Group sampling (named groups share counters)
	groupName    string
	groupN       uint64
	groupManager *SamplingGroupManager

	// Conditional sampling
	predicate func() bool
	condN     uint64
	condCount atomic.Uint64

	// Exponential backoff sampling
	backoffKey    string
	backoffFactor float64
	backoffState  *BackoffState

	// Active mode (which sampling type is active)
	mode SamplingMode
	
	// Sampling statistics
	sampledCount atomic.Uint64
	skippedCount atomic.Uint64
}

// SamplingMode represents the type of sampling active on the filter.
type SamplingMode int

const (
	ModeNone SamplingMode = iota
	ModeCounter
	ModeRate
	ModeDuration
	ModeFirst
	ModeGroup
	ModeConditional
	ModeBackoff
)

// SamplingGroupManager manages shared counters for sampling groups with LRU eviction.
type SamplingGroupManager struct {
	cache *cache.LRUCache
	// Cache metrics for monitoring
	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64
}

// NewSamplingGroupManager creates a new sampling group manager with the specified capacity.
func NewSamplingGroupManager(capacity int) *SamplingGroupManager {
	if capacity <= 0 {
		capacity = DefaultSamplingCacheCapacity
	}
	return &SamplingGroupManager{
		cache: cache.NewLRUCache(capacity),
	}
}

// GetOrCreateGroup returns the counter for a group, creating it if necessary.
func (m *SamplingGroupManager) GetOrCreateGroup(name string) *atomic.Uint64 {
	// Check if value exists first (for hit tracking)
	if value, exists := m.cache.Get(name); exists {
		m.cacheHits.Add(1)
		return value.(*atomic.Uint64)
	}
	
	// Cache miss - create new value
	m.cacheMisses.Add(1)
	value := m.cache.GetOrCreate(name, func() interface{} {
		return &atomic.Uint64{}
	})
	return value.(*atomic.Uint64)
}

// ResetGroup resets the counter for a specific group.
func (m *SamplingGroupManager) ResetGroup(name string) {
	if value, exists := m.cache.Get(name); exists {
		counter := value.(*atomic.Uint64)
		counter.Store(0)
	}
}

// ResetAll resets all group counters by clearing the cache.
func (m *SamplingGroupManager) ResetAll() {
	m.cache.Clear()
}

// Warmup pre-populates the cache with common group names to avoid cold-start spikes.
func (m *SamplingGroupManager) Warmup(groupNames []string) {
	m.cache.Warmup(groupNames, func(key string) interface{} {
		return &atomic.Uint64{}
	})
}

// CacheHits returns the number of cache hits.
func (m *SamplingGroupManager) CacheHits() uint64 {
	return m.cacheHits.Load()
}

// CacheMisses returns the number of cache misses.
func (m *SamplingGroupManager) CacheMisses() uint64 {
	return m.cacheMisses.Load()
}

// CacheStats returns both hit and miss statistics.
func (m *SamplingGroupManager) CacheStats() (hits, misses uint64) {
	return m.cacheHits.Load(), m.cacheMisses.Load()
}

// BackoffState manages exponential backoff sampling state with LRU eviction.
type BackoffState struct {
	cache *cache.LRUCache
}

// BackoffCounter tracks state for exponential backoff sampling.
type BackoffCounter struct {
	count     atomic.Uint64
	nextLog   atomic.Uint64
	factor    float64
}

// NewBackoffState creates a new backoff state manager with the specified capacity.
func NewBackoffState(capacity int) *BackoffState {
	if capacity <= 0 {
		capacity = DefaultSamplingCacheCapacity
	}
	return &BackoffState{
		cache: cache.NewLRUCache(capacity),
	}
}

// GetOrCreateCounter returns the backoff counter for a key.
func (b *BackoffState) GetOrCreateCounter(key string, factor float64) *BackoffCounter {
	value := b.cache.GetOrCreate(key, func() interface{} {
		counter := &BackoffCounter{
			factor: factor,
		}
		counter.nextLog.Store(1) // First message should be logged
		return counter
	})
	return value.(*BackoffCounter)
}

// Warmup pre-populates the cache with common backoff keys to avoid cold-start spikes.
func (b *BackoffState) Warmup(keys []string, defaultFactor float64) {
	b.cache.Warmup(keys, func(key string) interface{} {
		counter := &BackoffCounter{
			factor: defaultFactor,
		}
		counter.nextLog.Store(1)
		return counter
	})
}

// NewCounterSamplingFilter creates a filter that samples every nth message.
func NewCounterSamplingFilter(n uint64) *PerMessageSamplingFilter {
	// Validate n - zero means no sampling (all messages filtered out)
	// This is intentional behavior, but we document it
	return &PerMessageSamplingFilter{
		counterN: n,
		mode:     ModeCounter,
	}
}

// NewRateSamplingFilter creates a filter that samples a percentage of messages.
func NewRateSamplingFilter(rate float32) *PerMessageSamplingFilter {
	if rate < 0.0 {
		rate = 0.0
	} else if rate > 1.0 {
		rate = 1.0
	}

	return &PerMessageSamplingFilter{
		rate:       rate,
		randSource: rand.New(rand.NewSource(time.Now().UnixNano())),
		mode:       ModeRate,
	}
}

// NewDurationSamplingFilter creates a filter that samples at most once per duration.
func NewDurationSamplingFilter(duration time.Duration) *PerMessageSamplingFilter {
	// Validate duration - negative or zero duration means no time-based sampling
	if duration <= 0 {
		duration = 1 * time.Nanosecond // Minimal valid duration
	}
	return &PerMessageSamplingFilter{
		duration: duration,
		mode:     ModeDuration,
	}
}

// NewFirstNSamplingFilter creates a filter that logs only the first n occurrences.
func NewFirstNSamplingFilter(n uint64) *PerMessageSamplingFilter {
	// Note: n=0 means no messages will be logged, which is valid behavior
	return &PerMessageSamplingFilter{
		firstN: n,
		mode:   ModeFirst,
	}
}

// NewGroupSamplingFilter creates a filter that samples within a named group.
func NewGroupSamplingFilter(groupName string, n uint64, manager *SamplingGroupManager) *PerMessageSamplingFilter {
	// Validate inputs
	if groupName == "" {
		groupName = "default-group" // Use default group name if empty
	}
	if manager == nil {
		// This should not happen in normal usage, but we handle it gracefully
		manager = NewSamplingGroupManager(DefaultSamplingCacheCapacity)
	}
	
	return &PerMessageSamplingFilter{
		groupName:    groupName,
		groupN:       n,
		groupManager: manager,
		mode:         ModeGroup,
	}
}

// NewConditionalSamplingFilter creates a filter that samples based on a predicate.
func NewConditionalSamplingFilter(predicate func() bool, n uint64) *PerMessageSamplingFilter {
	// Validate predicate
	if predicate == nil {
		// No predicate means never sample
		predicate = func() bool { return false }
	}
	
	return &PerMessageSamplingFilter{
		predicate: predicate,
		condN:     n,
		mode:      ModeConditional,
	}
}

// NewBackoffSamplingFilter creates a filter with exponential backoff sampling.
func NewBackoffSamplingFilter(key string, factor float64, state *BackoffState) *PerMessageSamplingFilter {
	// Validate inputs
	if key == "" {
		key = "default-backoff-key" // Use default key if empty
	}
	if factor <= 1.0 {
		factor = DefaultBackoffFactor // Use default factor for invalid values
	}
	if state == nil {
		// This should not happen in normal usage, but we handle it gracefully
		state = NewBackoffState(DefaultSamplingCacheCapacity)
	}
	
	return &PerMessageSamplingFilter{
		backoffKey:   key,
		backoffFactor: factor,
		backoffState:  state,
		mode:         ModeBackoff,
	}
}

// IsEnabled determines if the event should be logged based on the sampling mode.
func (f *PerMessageSamplingFilter) IsEnabled(event *core.LogEvent) bool {
	var shouldSample bool
	
	switch f.mode {
	case ModeCounter:
		shouldSample = f.shouldSampleCounter()
	case ModeRate:
		shouldSample = f.shouldSampleRate()
	case ModeDuration:
		shouldSample = f.shouldSampleDuration()
	case ModeFirst:
		shouldSample = f.shouldSampleFirst()
	case ModeGroup:
		shouldSample = f.shouldSampleGroup()
	case ModeConditional:
		shouldSample = f.shouldSampleConditional()
	case ModeBackoff:
		shouldSample = f.shouldSampleBackoff()
	default:
		shouldSample = true
	}
	
	// Track statistics
	if shouldSample {
		f.sampledCount.Add(1)
	} else {
		f.skippedCount.Add(1)
	}
	
	return shouldSample
}

// shouldSampleCounter samples every nth message.
func (f *PerMessageSamplingFilter) shouldSampleCounter() bool {
	if f.counterN <= 1 {
		return true
	}

	count := f.counterValue.Add(1)
	return count%f.counterN == 1
}

// shouldSampleRate samples a percentage of messages using true random sampling.
func (f *PerMessageSamplingFilter) shouldSampleRate() bool {
	if f.rate >= 1.0 {
		return true
	}
	if f.rate <= 0.0 {
		return false
	}

	// Use per-filter random source to avoid contention
	// randSource is always initialized in NewRateSamplingFilter
	return f.randSource.Float32() < f.rate
}

// shouldSampleDuration samples at most once per duration.
func (f *PerMessageSamplingFilter) shouldSampleDuration() bool {
	if f.duration <= 0 {
		return true
	}

	now := time.Now().UnixNano()
	lastSample := f.lastSampleTime.Load()

	if now-lastSample >= int64(f.duration) {
		// Try to update the last sample time
		return f.lastSampleTime.CompareAndSwap(lastSample, now)
	}
	return false
}

// shouldSampleFirst logs only the first n occurrences.
func (f *PerMessageSamplingFilter) shouldSampleFirst() bool {
	if f.firstN == 0 {
		return false
	}

	count := f.firstCount.Add(1)
	return count <= f.firstN
}

// shouldSampleGroup samples within a named group.
func (f *PerMessageSamplingFilter) shouldSampleGroup() bool {
	if f.groupN <= 1 || f.groupManager == nil {
		return true
	}

	counter := f.groupManager.GetOrCreateGroup(f.groupName)
	count := counter.Add(1)
	return count%f.groupN == 1
}

// shouldSampleConditional samples based on a predicate.
func (f *PerMessageSamplingFilter) shouldSampleConditional() bool {
	// First check the predicate
	if f.predicate != nil && !f.predicate() {
		return false
	}

	// Then apply counter sampling
	if f.condN <= 1 {
		return true
	}

	count := f.condCount.Add(1)
	return count%f.condN == 1
}

// shouldSampleBackoff samples with exponential backoff.
func (f *PerMessageSamplingFilter) shouldSampleBackoff() bool {
	if f.backoffState == nil {
		return true
	}

	counter := f.backoffState.GetOrCreateCounter(f.backoffKey, f.backoffFactor)
	
	count := counter.count.Add(1)
	nextLog := counter.nextLog.Load()

	if count >= nextLog {
		// Calculate next log occurrence using exponential backoff
		// Log at: 1, 2, 4, 8, 16, 32, ...
		next := nextLog * uint64(f.backoffFactor)
		if next <= nextLog { // Overflow protection
			next = nextLog + 1
		}
		counter.nextLog.CompareAndSwap(nextLog, next)
		return true
	}

	return false
}

// Reset resets the sampling counters.
func (f *PerMessageSamplingFilter) Reset() {
	switch f.mode {
	case ModeCounter:
		f.counterValue.Store(0)
	case ModeRate:
		f.rateCounter.Store(0)
	case ModeDuration:
		f.lastSampleTime.Store(0)
	case ModeFirst:
		f.firstCount.Store(0)
	case ModeGroup:
		if f.groupManager != nil {
			f.groupManager.ResetGroup(f.groupName)
		}
	case ModeConditional:
		f.condCount.Store(0)
	case ModeBackoff:
		// Backoff doesn't reset individual counters, only through state manager
	}
	
	// Reset statistics
	f.sampledCount.Store(0)
	f.skippedCount.Store(0)
}

// GetStats returns the current sampling statistics.
func (f *PerMessageSamplingFilter) GetStats() SamplingStats {
	return SamplingStats{
		Sampled: f.sampledCount.Load(),
		Skipped: f.skippedCount.Load(),
	}
}

// SamplingStats contains sampling statistics.
type SamplingStats struct {
	Sampled uint64
	Skipped uint64
}

