package middleware

import (
	"net/http"
	"sync"
	"time"
)

// MetricsRecorder records HTTP metrics for observability
type MetricsRecorder interface {
	// RecordRequest records metrics for a completed HTTP request
	RecordRequest(method, path string, statusCode int, duration time.Duration)
	
	// RecordPanic records a panic occurrence
	RecordPanic(method, path string)
	
	// RecordBodySize records request/response body sizes
	RecordBodySize(direction string, size int)
}

// NoOpMetricsRecorder is a metrics recorder that does nothing
type NoOpMetricsRecorder struct{}

func (n *NoOpMetricsRecorder) RecordRequest(method, path string, statusCode int, duration time.Duration) {}
func (n *NoOpMetricsRecorder) RecordPanic(method, path string)                                          {}
func (n *NoOpMetricsRecorder) RecordBodySize(direction string, size int)                                 {}

// PrometheusMetricsRecorder records metrics for Prometheus
// This is a stub that users can implement with actual Prometheus client
type PrometheusMetricsRecorder struct {
	// Users would add prometheus.Histogram, prometheus.Counter etc here
	// We don't want to add prometheus as a dependency
}

func (p *PrometheusMetricsRecorder) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	// User implementation would call prometheus metrics here
	// e.g., p.requestDuration.WithLabelValues(method, path, strconv.Itoa(statusCode)).Observe(duration.Seconds())
}

func (p *PrometheusMetricsRecorder) RecordPanic(method, path string) {
	// User implementation would increment panic counter
	// e.g., p.panicCounter.WithLabelValues(method, path).Inc()
}

func (p *PrometheusMetricsRecorder) RecordBodySize(direction string, size int) {
	// User implementation would record body size
	// e.g., p.bodySize.WithLabelValues(direction).Observe(float64(size))
}

// SimpleMetricsRecorder provides basic in-memory metrics (useful for testing)
type SimpleMetricsRecorder struct {
	mu            sync.Mutex
	requests      []RequestMetric
	panics        []PanicMetric
	bodySizes     []BodySizeMetric
	maxRequests   int
}

type RequestMetric struct {
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	Timestamp  time.Time
}

type PanicMetric struct {
	Method    string
	Path      string
	Timestamp time.Time
}

type BodySizeMetric struct {
	Direction string
	Size      int
	Timestamp time.Time
}

// NewSimpleMetricsRecorder creates a simple in-memory metrics recorder
func NewSimpleMetricsRecorder(maxRequests int) *SimpleMetricsRecorder {
	return &SimpleMetricsRecorder{
		maxRequests: maxRequests,
	}
}

func (s *SimpleMetricsRecorder) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.requests = append(s.requests, RequestMetric{
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Duration:   duration,
		Timestamp:  time.Now(),
	})
	
	// Keep only last N requests to avoid unbounded growth
	if len(s.requests) > s.maxRequests {
		s.requests = s.requests[len(s.requests)-s.maxRequests:]
	}
}

func (s *SimpleMetricsRecorder) RecordPanic(method, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.panics = append(s.panics, PanicMetric{
		Method:    method,
		Path:      path,
		Timestamp: time.Now(),
	})
	
	if len(s.panics) > 100 {
		s.panics = s.panics[len(s.panics)-100:]
	}
}

func (s *SimpleMetricsRecorder) RecordBodySize(direction string, size int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.bodySizes = append(s.bodySizes, BodySizeMetric{
		Direction: direction,
		Size:      size,
		Timestamp: time.Now(),
	})
	
	if len(s.bodySizes) > 1000 {
		s.bodySizes = s.bodySizes[len(s.bodySizes)-1000:]
	}
}

// GetMetrics returns a copy of the collected metrics
func (s *SimpleMetricsRecorder) GetMetrics() ([]RequestMetric, []PanicMetric, []BodySizeMetric) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	reqCopy := make([]RequestMetric, len(s.requests))
	copy(reqCopy, s.requests)
	
	panicCopy := make([]PanicMetric, len(s.panics))
	copy(panicCopy, s.panics)
	
	bodyCopy := make([]BodySizeMetric, len(s.bodySizes))
	copy(bodyCopy, s.bodySizes)
	
	return reqCopy, panicCopy, bodyCopy
}

// BatchMetricsRecorder batches metrics for high-throughput scenarios
type BatchMetricsRecorder struct {
	batch     []RequestMetric
	batchMu   sync.Mutex
	flush     func([]RequestMetric)
	interval  time.Duration
	maxBatch  int
	ticker    *time.Ticker
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewBatchMetricsRecorder creates a new batch metrics recorder
func NewBatchMetricsRecorder(flush func([]RequestMetric), interval time.Duration, maxBatch int) *BatchMetricsRecorder {
	if maxBatch <= 0 {
		maxBatch = 1000
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	
	b := &BatchMetricsRecorder{
		batch:    make([]RequestMetric, 0, maxBatch),
		flush:    flush,
		interval: interval,
		maxBatch: maxBatch,
		ticker:   time.NewTicker(interval),
		stopCh:   make(chan struct{}),
	}
	
	// Start the flush goroutine
	b.wg.Add(1)
	go b.flushLoop()
	
	return b
}

// flushLoop periodically flushes the batch
func (b *BatchMetricsRecorder) flushLoop() {
	defer b.wg.Done()
	
	for {
		select {
		case <-b.ticker.C:
			b.flushBatch()
		case <-b.stopCh:
			b.flushBatch() // Final flush
			return
		}
	}
}

// flushBatch flushes the current batch
func (b *BatchMetricsRecorder) flushBatch() {
	b.batchMu.Lock()
	if len(b.batch) == 0 {
		b.batchMu.Unlock()
		return
	}
	
	// Copy batch for flushing
	toFlush := make([]RequestMetric, len(b.batch))
	copy(toFlush, b.batch)
	b.batch = b.batch[:0]
	b.batchMu.Unlock()
	
	// Flush outside the lock
	if b.flush != nil {
		b.flush(toFlush)
	}
}

// RecordRequest records a request metric
func (b *BatchMetricsRecorder) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	var metric *RequestMetric
	if EnablePooling {
		metric = getRequestMetric()
	} else {
		metric = &RequestMetric{}
	}
	metric.Method = method
	metric.Path = path
	metric.StatusCode = statusCode
	metric.Duration = duration
	metric.Timestamp = time.Now()
	
	b.batchMu.Lock()
	b.batch = append(b.batch, *metric)
	// Return metric to pool after copying to batch
	if EnablePooling {
		putRequestMetric(metric)
	}
	shouldFlush := len(b.batch) >= b.maxBatch
	b.batchMu.Unlock()
	
	if shouldFlush {
		b.flushBatch()
	}
}

// RecordPanic records a panic metric
func (b *BatchMetricsRecorder) RecordPanic(method, path string) {
	// For panics, record immediately as high priority
	metric := RequestMetric{
		Method:     method,
		Path:       path,
		StatusCode: 500,
		Duration:   0,
		Timestamp:  time.Now(),
	}
	
	// Flush single panic metric immediately
	if b.flush != nil {
		b.flush([]RequestMetric{metric})
	}
}

// RecordBodySize records body size (could be batched separately if needed)
func (b *BatchMetricsRecorder) RecordBodySize(direction string, size int) {
	// For now, we'll ignore body size in batch recorder
	// Could implement a separate batch for body metrics if needed
}

// Close stops the batch recorder and flushes remaining metrics
func (b *BatchMetricsRecorder) Close() error {
	close(b.stopCh)
	b.ticker.Stop()
	b.wg.Wait()
	return nil
}

// Stats returns current batch statistics
func (b *BatchMetricsRecorder) Stats() (batchSize int, lastFlush time.Time) {
	b.batchMu.Lock()
	batchSize = len(b.batch)
	b.batchMu.Unlock()
	return batchSize, time.Now() // Could track last flush time if needed
}

// MetricsMiddleware wraps a handler to record metrics
func MetricsMiddleware(recorder MetricsRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Wrap response writer to capture status
			rw := newResponseWriter(w)
			
			// Handle panics
			defer func() {
				if err := recover(); err != nil {
					// Record both panic and request metrics for panicked requests
					duration := time.Since(start)
					recorder.RecordPanic(r.Method, r.URL.Path)
					recorder.RecordRequest(r.Method, r.URL.Path, 500, duration) // Assume 500 for panics
					panic(err) // Re-panic after recording
				}
			}()
			
			// Process request
			next.ServeHTTP(rw, r)
			
			// Record metrics for successful requests
			duration := time.Since(start)
			recorder.RecordRequest(r.Method, r.URL.Path, rw.Status(), duration)
			
			if rw.Size() > 0 {
				recorder.RecordBodySize("response", rw.Size())
			}
		})
	}
}