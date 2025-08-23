package enrichers

import (
	"context"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// DeadlineOption configures deadline enricher behavior.
type DeadlineOption func(*DeadlineEnricher)

// DeadlineEnricher enriches log events with context deadline information.
// It detects when operations are approaching context deadlines and adds
// relevant properties to help diagnose timeout-related issues.
type DeadlineEnricher struct {
	// Configuration
	threshold           time.Duration  // Absolute time threshold for warnings
	percentageThreshold float64        // Percentage-based threshold (0.0-1.0)
	upgradeLevel        bool           // Upgrade Info→Warning when approaching
	addProperties       bool           // Add deadline properties to events
	emitMetrics         bool           // Emit metrics about deadline approaches
	customHandler       func(*core.LogEvent, time.Duration) // Custom deadline handler

	// Cache management
	deadlineCache     *deadlineLRUCache
	firstWarningSet   *firstWarningSet  // Separate, larger set for tracking first warnings
	cacheMaxSize      int
	cacheTTL          time.Duration
}

// deadlineInfo holds cached deadline information for a context.
type deadlineInfo struct {
	deadline          time.Time
	hasDeadline       bool
	startTime         time.Time // When we first saw this context
	lastCheck         time.Time // Last time we checked this context
	exceededLogged    bool      // Whether we've logged that deadline was exceeded
}

// NewDeadlineEnricher creates a new deadline enricher with the specified threshold.
func NewDeadlineEnricher(threshold time.Duration, opts ...DeadlineOption) *DeadlineEnricher {
	e := &DeadlineEnricher{
		threshold:           threshold,
		percentageThreshold: 0.0, // Disabled by default
		upgradeLevel:        true,
		addProperties:       true,
		emitMetrics:         false,
		cacheMaxSize:        1000,
		cacheTTL:            5 * time.Minute,
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Initialize cache
	e.deadlineCache = newDeadlineLRUCache(e.cacheMaxSize, e.cacheTTL)
	
	// Initialize first warning set with 10x the cache size
	// This ensures first warnings are tracked even after cache eviction
	firstWarningSize := e.cacheMaxSize * 10
	if firstWarningSize > 100000 {
		firstWarningSize = 100000 // Cap at 100k to prevent excessive memory
	}
	e.firstWarningSet = newFirstWarningSet(firstWarningSize)

	return e
}

// WithDeadlinePercentageThreshold sets a percentage-based threshold.
// When deadline approaches this percentage of total time, warnings are triggered.
// For example, 0.1 means warn when 10% of time remains.
func WithDeadlinePercentageThreshold(percent float64) DeadlineOption {
	return func(e *DeadlineEnricher) {
		if percent > 0 && percent <= 1.0 {
			e.percentageThreshold = percent
		}
	}
}

// WithDeadlineUpgradeLevel controls whether to upgrade log level when deadline approaches.
func WithDeadlineUpgradeLevel(upgrade bool) DeadlineOption {
	return func(e *DeadlineEnricher) {
		e.upgradeLevel = upgrade
	}
}

// WithDeadlineAddProperties controls whether to add deadline properties to events.
func WithDeadlineAddProperties(add bool) DeadlineOption {
	return func(e *DeadlineEnricher) {
		e.addProperties = add
	}
}

// WithDeadlineMetrics enables metric emission for deadline approaches.
func WithDeadlineMetrics(enabled bool) DeadlineOption {
	return func(e *DeadlineEnricher) {
		e.emitMetrics = enabled
	}
}

// WithDeadlineCustomHandler sets a custom handler for deadline approaching events.
func WithDeadlineCustomHandler(handler func(*core.LogEvent, time.Duration)) DeadlineOption {
	return func(e *DeadlineEnricher) {
		e.customHandler = handler
	}
}

// WithDeadlineCacheSize sets the maximum cache size.
func WithDeadlineCacheSize(size int) DeadlineOption {
	return func(e *DeadlineEnricher) {
		if size > 0 {
			e.cacheMaxSize = size
		}
	}
}

// WithDeadlineCacheTTL sets the cache TTL.
func WithDeadlineCacheTTL(ttl time.Duration) DeadlineOption {
	return func(e *DeadlineEnricher) {
		if ttl > 0 {
			e.cacheTTL = ttl
		}
	}
}

// Enrich adds deadline information to the log event if context has a deadline.
func (e *DeadlineEnricher) Enrich(event *core.LogEvent, factory core.LogEventPropertyFactory) {
	// Extract context from event if available
	ctx, ok := event.Properties["__context__"].(context.Context)
	if !ok {
		// No context available, nothing to do
		return
	}

	// Remove internal context property
	delete(event.Properties, "__context__")

	// Check for deadline
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		// No deadline, zero cost exit
		return
	}

	now := time.Now()
	
	// Get or create cache entry for this context
	info := e.deadlineCache.getOrCreate(ctx)
	if info.startTime.IsZero() {
		// First time seeing this context, record start time
		info.startTime = now
		info.deadline = deadline
		info.hasDeadline = true
		e.deadlineCache.put(ctx, info)
	}

	// Calculate remaining time
	remaining := deadline.Sub(now)

	// Check if deadline has been exceeded
	if remaining < 0 {
		e.handleDeadlineExceeded(event, factory, ctx, -remaining)
		return
	}

	// Check if we should warn about approaching deadline
	shouldWarn := false
	
	// Check absolute threshold
	if e.threshold > 0 && remaining <= e.threshold {
		shouldWarn = true
	}

	// Check percentage threshold if configured
	if !shouldWarn && e.percentageThreshold > 0 {
		// For percentage calculation, we need to estimate total duration
		// If we have a cached start time, use it; otherwise estimate
		if !info.startTime.IsZero() {
			totalDuration := deadline.Sub(info.startTime)
			if totalDuration > 0 {
				percentRemaining := float64(remaining) / float64(totalDuration)
				// Warn if remaining percentage is at or below threshold
				if percentRemaining <= e.percentageThreshold {
					shouldWarn = true
				}
			}
		} else {
			// We don't have the exact start time, so we can't do percentage-based warnings accurately
			// This is a limitation of detecting context midway through its lifetime
			// We could estimate, but that would be inaccurate
		}
	}

	if !shouldWarn {
		// Early exit - not within any threshold
		return
	}

	// Check if this is the first warning for this context
	// Use the separate first warning set which is much larger than the cache
	isFirstWarning := e.firstWarningSet.markWarned(ctx)

	// Add properties if configured
	if e.addProperties {
		// Use OTEL-style dotted names
		event.AddProperty("deadline.remaining_ms", remaining.Milliseconds())
		event.AddProperty("deadline.at", deadline.Format(time.RFC3339Nano))
		event.AddProperty("deadline.approaching", true)
		
		if isFirstWarning {
			event.AddProperty("deadline.first_warning", true)
		}
	}

	// Upgrade level if configured and it's Info or below
	if e.upgradeLevel && event.Level <= core.InformationLevel {
		event.Level = core.WarningLevel
	}

	// Call custom handler if configured
	if e.customHandler != nil {
		e.customHandler(event, remaining)
	}

	// Emit metrics if configured
	if e.emitMetrics && selflog.IsEnabled() {
		selflog.Printf("[deadline] Context deadline approaching: remaining=%v, first=%v", remaining, isFirstWarning)
	}
}

// handleDeadlineExceeded handles events that occur after the deadline.
func (e *DeadlineEnricher) handleDeadlineExceeded(event *core.LogEvent, factory core.LogEventPropertyFactory, ctx context.Context, exceededBy time.Duration) {
	// Check if we've already logged exceeded for this context
	info := e.deadlineCache.get(ctx)
	isFirstExceeded := true
	if info != nil && info.exceededLogged {
		isFirstExceeded = false
	}

	// Mark as exceeded
	if info != nil && !info.exceededLogged {
		info.exceededLogged = true
		e.deadlineCache.put(ctx, info)
	}

	// Add properties (only on first exceeded or if always showing)
	if e.addProperties && isFirstExceeded {
		event.AddProperty("deadline.exceeded", true)
		event.AddProperty("deadline.exceeded_by_ms", exceededBy.Milliseconds())
	}

	// Always upgrade to at least Warning for exceeded deadlines
	// This happens for all events after deadline, not just the first
	if event.Level < core.WarningLevel {
		event.Level = core.WarningLevel
	}

	// Call custom handler if configured (only on first exceeded)
	if e.customHandler != nil && isFirstExceeded {
		e.customHandler(event, -exceededBy)
	}

	// Emit metrics if configured
	if e.emitMetrics && selflog.IsEnabled() {
		selflog.Printf("[deadline] Context deadline exceeded by %v", exceededBy)
	}
}

// EnrichWithContext enriches the event with deadline information from the provided context.
// This is used when context is passed explicitly to logging methods.
func (e *DeadlineEnricher) EnrichWithContext(ctx context.Context, event *core.LogEvent, factory core.LogEventPropertyFactory) {
	// Store context temporarily in event properties for Enrich method
	event.Properties["__context__"] = ctx
	e.Enrich(event, factory)
}

// Stats returns current statistics for monitoring and debugging.
// This is useful for understanding cache usage and first warning tracking.
func (e *DeadlineEnricher) Stats() core.DeadlineStats {
	if e == nil {
		return core.DeadlineStats{}
	}
	
	return core.DeadlineStats{
		CacheSize:           e.deadlineCache.size(),
		CacheCapacity:       e.cacheMaxSize,
		FirstWarningCount:   e.firstWarningSet.size(),
		FirstWarningCapacity: e.firstWarningSet.maxSize,
		CacheTTL:            e.cacheTTL,
	}
}

