// Package sentry provides a Sentry integration sink for mtlog.
// It automatically tracks errors, captures stack traces, and provides
// breadcrumb support for production error monitoring.
package sentry

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

func init() {
	// Seed the random number generator for better jitter distribution in retry backoff
	// This is not security-critical as it's only used for retry timing jitter
	rand.Seed(time.Now().UnixNano())
}

var (
	// builderPool is a pool of string builders for message rendering
	builderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
)

// stackTraceCache implements an LRU cache for stack traces
type stackTraceCache struct {
	mu      sync.RWMutex
	cache   map[string]*sentry.Stacktrace
	order   []string // Track insertion order for LRU
	maxSize int
}

func newStackTraceCache(maxSize int) *stackTraceCache {
	return &stackTraceCache{
		cache:   make(map[string]*sentry.Stacktrace),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

func (c *stackTraceCache) get(key string) (*sentry.Stacktrace, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st, ok := c.cache[key]
	return st, ok
}

func (c *stackTraceCache) set(key string, st *sentry.Stacktrace) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, just update
	if _, exists := c.cache[key]; exists {
		c.cache[key] = st
		return
	}

	// LRU eviction if at capacity
	if len(c.cache) >= c.maxSize {
		// Remove oldest entry (first in order)
		if len(c.order) > 0 {
			oldest := c.order[0]
			delete(c.cache, oldest)
			c.order = c.order[1:]
		}
	}

	c.cache[key] = st
	c.order = append(c.order, key)
}

func (c *stackTraceCache) size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

func (c *stackTraceCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*sentry.Stacktrace)
	c.order = make([]string, 0, c.maxSize)
}

// SentrySink sends log events to Sentry for error tracking and monitoring.
// It supports batching, breadcrumb collection, and custom fingerprinting.
type SentrySink struct {
	client *sentry.Client
	hub    *sentry.Hub

	// Configuration
	minLevel         core.LogEventLevel
	breadcrumbLevel  core.LogEventLevel
	sampleRate       float64
	environment      string
	release          string
	serverName       string
	maxBreadcrumbs   int

	// Breadcrumbs
	breadcrumbs *BreadcrumbBuffer

	// Fingerprinting
	fingerprinter Fingerprinter

	// Batching
	batchSize    int
	batchTimeout time.Duration
	batch        []*sentry.Event
	batchMu      sync.Mutex
	stopCh       chan struct{}
	flushCh      chan struct{}
	wg           sync.WaitGroup

	// BeforeSend hook
	beforeSend sentry.EventProcessor

	// Retry configuration
	maxRetries   int
	retryBackoff time.Duration
	retryJitter  float64 // Jitter factor (0.0 to 1.0)

	// Metrics
	metrics       *metricsCollector
	enableMetrics bool

	// Stack trace caching
	stackTraceCache     *stackTraceCache
	stackTraceCacheSize int
	
	// Sampling
	samplingConfig *SamplingConfig
	sampler        *sampler
}

// Fingerprinter is a function that generates fingerprints for error grouping.
type Fingerprinter func(*core.LogEvent) []string

// WithSentry creates a new Sentry sink with the given DSN and options.
// This is a convenience function that returns a core.LogEventSink.
func WithSentry(dsn string, opts ...Option) (core.LogEventSink, error) {
	return NewSentrySink(dsn, opts...)
}

// NewSentrySink creates a new Sentry sink with the given DSN and options.
func NewSentrySink(dsn string, opts ...Option) (*SentrySink, error) {
	// Fall back to environment variable if DSN not provided
	if dsn == "" {
		dsn = os.Getenv("SENTRY_DSN")
		if dsn == "" {
			return nil, fmt.Errorf("Sentry DSN not provided and SENTRY_DSN environment variable not set")
		}
	}

	s := &SentrySink{
		minLevel:            core.ErrorLevel,
		breadcrumbLevel:     core.DebugLevel,
		sampleRate:          1.0,
		maxBreadcrumbs:      100,
		batchSize:           100,
		batchTimeout:        5 * time.Second,
		stopCh:              make(chan struct{}),
		flushCh:             make(chan struct{}),
		metrics:             newMetricsCollector(),
		enableMetrics:       true, // Default to enabled
		stackTraceCacheSize: 1000, // Default cache size
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create Sentry client
	clientOpts := sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      s.environment,
		Release:          s.release,
		ServerName:       s.serverName,
		SampleRate:       s.sampleRate,
		AttachStacktrace: true,
		BeforeSend:       s.beforeSend,
	}

	client, err := sentry.NewClient(clientOpts)
	if err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[sentry] failed to create client: %v", err)
		}
		return nil, fmt.Errorf("failed to create Sentry client: %w", err)
	}

	s.client = client
	s.hub = sentry.NewHub(client, sentry.NewScope())
	s.breadcrumbs = NewBreadcrumbBuffer(s.maxBreadcrumbs)
	s.batch = make([]*sentry.Event, 0, s.batchSize)
	s.stackTraceCache = newStackTraceCache(s.stackTraceCacheSize)

	// Start background worker
	s.wg.Add(1)
	go s.worker()

	return s, nil
}

// Emit sends a log event to Sentry.
func (s *SentrySink) Emit(event *core.LogEvent) {
	if event == nil {
		return
	}

	// Check if this should be a breadcrumb
	if event.Level < s.minLevel && event.Level >= s.breadcrumbLevel {
		s.addBreadcrumb(event)
		return
	}

	// Only send events at or above minimum level
	if event.Level < s.minLevel {
		return
	}
	
	// Apply sampling decision
	if s.sampler != nil && !s.sampler.shouldSample(event) {
		if s.enableMetrics && s.metrics != nil {
			s.metrics.eventsDropped.Add(1)
		}
		return
	}

	// Convert to Sentry event
	sentryEvent := s.convertToSentryEvent(event)
	if sentryEvent == nil {
		return
	}

	// Add to batch
	s.batchMu.Lock()
	s.batch = append(s.batch, sentryEvent)
	shouldFlush := len(s.batch) >= s.batchSize
	s.batchMu.Unlock()

	if shouldFlush {
		select {
		case s.flushCh <- struct{}{}:
		default:
		}
	}
}

// Metrics returns a snapshot of the current metrics.
func (s *SentrySink) Metrics() Metrics {
	if s.metrics == nil {
		return Metrics{}
	}
	return s.metrics.snapshot()
}

// Close flushes any pending events and closes the sink.
func (s *SentrySink) Close() error {
	close(s.stopCh)
	s.wg.Wait()

	// Final flush
	s.flush()

	// Flush Sentry client
	if s.client != nil {
		if !s.client.Flush(2 * time.Second) {
			if selflog.IsEnabled() {
				selflog.Printf("[sentry] timeout during final flush")
			}
		}
	}

	return nil
}

// worker handles batching and periodic flushing.
func (s *SentrySink) worker() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.flush()
		case <-s.flushCh:
			s.flush()
			ticker.Reset(s.batchTimeout)
		}
	}
}

// flush sends all batched events to Sentry.
func (s *SentrySink) flush() {
	start := time.Now()
	
	s.batchMu.Lock()
	if len(s.batch) == 0 {
		s.batchMu.Unlock()
		return
	}
	events := s.batch
	s.batch = make([]*sentry.Event, 0, s.batchSize)
	s.batchMu.Unlock()

	// Track metrics
	defer func() {
		if s.enableMetrics {
			duration := time.Since(start)
			s.metrics.lastFlushDuration.Store(int64(duration))
			s.metrics.totalFlushTime.Add(int64(duration))
			s.metrics.batchesSent.Add(1)
			s.metrics.totalBatchSize.Add(int64(len(events)))
		}
	}()

	for _, event := range events {
		// Attach current breadcrumbs
		event.Breadcrumbs = s.breadcrumbs.GetAll()

		// Send with retry logic if configured
		if s.maxRetries > 0 {
			s.sendWithRetry(event)
		} else {
			// Original behavior without retry
			eventID := s.hub.CaptureEvent(event)
			if eventID == nil && selflog.IsEnabled() {
				selflog.Printf("[sentry] failed to capture event: %s", event.Message)
			}
		}
	}
}

// sendWithRetry sends an event to Sentry with retry logic
func (s *SentrySink) sendWithRetry(event *sentry.Event) {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		eventID := s.hub.CaptureEvent(event)
		if eventID != nil {
			// Success
			if s.enableMetrics {
				s.metrics.eventsSent.Add(1)
				if attempt > 0 {
					s.metrics.eventsRetried.Add(1)
				}
			}
			return
		}

		lastErr = fmt.Errorf("failed to capture event: %s", event.Message)

		if attempt < s.maxRetries {
			// Calculate exponential backoff with jitter
			delay := s.calculateBackoff(attempt)

			if s.enableMetrics {
				s.metrics.retryCount.Add(1)
			}

			if selflog.IsEnabled() {
				selflog.Printf("[sentry] retry attempt %d/%d for event, waiting %v",
					attempt+1, s.maxRetries, delay)
			}

			time.Sleep(delay)
		}
	}

	// All retries exhausted
	if s.enableMetrics {
		s.metrics.eventsFailed.Add(1)
		s.metrics.networkErrors.Add(1)
	}

	if selflog.IsEnabled() {
		selflog.Printf("[sentry] failed to send event after %d attempts: %v",
			s.maxRetries+1, lastErr)
	}
}

// calculateBackoff calculates exponential backoff with jitter
func (s *SentrySink) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: backoff * 2^attempt
	backoff := float64(s.retryBackoff) * math.Pow(2, float64(attempt))

	// Add jitter
	if s.retryJitter > 0 {
		jitter := (rand.Float64()*2 - 1) * s.retryJitter // -jitter to +jitter
		backoff = backoff * (1 + jitter)
	}

	// Cap at 30 seconds
	maxBackoff := 30 * time.Second
	if time.Duration(backoff) > maxBackoff {
		return maxBackoff
	}

	return time.Duration(backoff)
}

// addBreadcrumb adds a log event as a breadcrumb.
func (s *SentrySink) addBreadcrumb(event *core.LogEvent) {
	breadcrumb := sentry.Breadcrumb{
		Type:      "default",
		Category:  levelToCategory(event.Level),
		Message:   s.renderMessage(event),
		Level:     levelToSentryLevel(event.Level),
		Timestamp: event.Timestamp,
	}

	// Add properties as data
	if len(event.Properties) > 0 {
		breadcrumb.Data = make(map[string]interface{})
		for k, v := range event.Properties {
			breadcrumb.Data[k] = v
		}
	}

	s.breadcrumbs.Add(breadcrumb)

	if s.enableMetrics {
		s.metrics.breadcrumbsAdded.Add(1)
	}
}

// convertToSentryEvent converts a log event to a Sentry event.
func (s *SentrySink) convertToSentryEvent(event *core.LogEvent) *sentry.Event {
	sentryEvent := &sentry.Event{
		Message:   s.renderMessage(event),
		Level:     levelToSentryLevel(event.Level),
		Timestamp: event.Timestamp,
		Extra:     make(map[string]interface{}),
		Tags:      make(map[string]string),
	}

	// Add message template as tag for grouping
	sentryEvent.Tags["message.template"] = event.MessageTemplate

	// Add properties as extra data
	for k, v := range event.Properties {
		// Check for special properties
		switch k {
		case "error", "err", "Error":
			if err, ok := v.(error); ok {
				sentryEvent.Exception = s.extractException(err)
			}
		case "user", "User":
			if user, ok := v.(sentry.User); ok {
				sentryEvent.User = user
			}
		default:
			sentryEvent.Extra[k] = v
		}
	}

	// Apply custom fingerprinting
	if s.fingerprinter != nil {
		sentryEvent.Fingerprint = s.fingerprinter(event)
	} else {
		// Default fingerprint based on template and error
		sentryEvent.Fingerprint = []string{event.MessageTemplate}
		if len(sentryEvent.Exception) > 0 {
			sentryEvent.Fingerprint = append(sentryEvent.Fingerprint, 
				sentryEvent.Exception[0].Type)
		}
	}
	
	// Apply group-based sampling
	if s.sampler != nil && s.samplingConfig != nil && s.samplingConfig.GroupSampling {
		fingerprint := fmt.Sprintf("%v", sentryEvent.Fingerprint)
		if !s.sampler.groupSample(fingerprint) {
			if s.enableMetrics && s.metrics != nil {
				s.metrics.eventsDropped.Add(1)
			}
			return nil
		}
	}

	return sentryEvent
}

// extractException extracts exception information from an error.
func (s *SentrySink) extractException(err error) []sentry.Exception {
	// Create cache key from error type and message
	cacheKey := fmt.Sprintf("%T:%s", err, err.Error())

	// Check cache
	if s.stackTraceCache != nil {
		if cached, ok := s.stackTraceCache.get(cacheKey); ok {
			// Cache hit - return cached stack trace
			return []sentry.Exception{
				{
					Type:       fmt.Sprintf("%T", err),
					Value:      err.Error(),
					Stacktrace: cached,
				},
			}
		}
	}

	// Extract new stack trace
	stacktrace := sentry.ExtractStacktrace(err)

	// Cache it
	if s.stackTraceCache != nil && stacktrace != nil {
		s.stackTraceCache.set(cacheKey, stacktrace)
	}

	return []sentry.Exception{
		{
			Type:       fmt.Sprintf("%T", err),
			Value:      err.Error(),
			Stacktrace: stacktrace,
		},
	}
}

// renderMessage renders the message template with actual property values.
func (s *SentrySink) renderMessage(event *core.LogEvent) string {
	template := event.MessageTemplate
	
	// Get builder from pool
	builder := builderPool.Get().(*strings.Builder)
	defer func() {
		builder.Reset()
		builderPool.Put(builder)
	}()
	
	// Preallocate capacity based on template length + estimated property expansion
	estimatedSize := len(template) + len(event.Properties)*20
	builder.Grow(estimatedSize)

	// Replace {PropertyName} with actual values
	i := 0
	for i < len(template) {
		if i < len(template)-1 && template[i] == '{' {
			// Find closing brace
			j := i + 1
			for j < len(template) && template[j] != '}' {
				j++
			}

			if j < len(template) {
				// Extract property name (handle format specifiers)
				propContent := template[i+1 : j]
				propName := propContent
				
				// Remove format specifiers (e.g., {Price:F2} -> Price)
				if colonIdx := strings.IndexByte(propName, ':'); colonIdx != -1 {
					propName = propName[:colonIdx]
				}
				
				// Remove capturing hints
				propName = strings.TrimPrefix(propName, "@")
				propName = strings.TrimPrefix(propName, "$")

				// Look up property value
				if val, ok := event.Properties[propName]; ok {
					// Format the value
					switch v := val.(type) {
					case error:
						builder.WriteString(v.Error())
					case time.Time:
						builder.WriteString(v.Format(time.RFC3339))
					case *time.Time:
						if v != nil {
							builder.WriteString(v.Format(time.RFC3339))
						} else {
							builder.WriteString("<nil>")
						}
					case fmt.Stringer:
						builder.WriteString(v.String())
					default:
						builder.WriteString(fmt.Sprint(v))
					}
				} else {
					// Keep the placeholder if no value found
					builder.WriteString(template[i : j+1])
				}

				i = j + 1
				continue
			}
		}

		builder.WriteByte(template[i])
		i++
	}

	return builder.String()
}

// levelToSentryLevel converts mtlog level to Sentry level.
func levelToSentryLevel(level core.LogEventLevel) sentry.Level {
	switch level {
	case core.VerboseLevel, core.DebugLevel:
		return sentry.LevelDebug
	case core.InformationLevel:
		return sentry.LevelInfo
	case core.WarningLevel:
		return sentry.LevelWarning
	case core.ErrorLevel:
		return sentry.LevelError
	case core.FatalLevel:
		return sentry.LevelFatal
	default:
		return sentry.LevelInfo
	}
}

// levelToCategory returns a breadcrumb category for a log level.
func levelToCategory(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel, core.DebugLevel:
		return "debug"
	case core.InformationLevel:
		return "info"
	case core.WarningLevel:
		return "warning"
	case core.ErrorLevel:
		return "error"
	case core.FatalLevel:
		return "fatal"
	default:
		return "log"
	}
}