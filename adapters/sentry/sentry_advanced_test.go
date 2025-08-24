package sentry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/willibrandon/mtlog/core"
)

// TestRetryLogic tests the retry mechanism with exponential backoff
func TestRetryLogic(t *testing.T) {
	t.Run("ExponentialBackoff", func(t *testing.T) {
		sink := &SentrySink{
			retryBackoff: 100 * time.Millisecond,
			retryJitter:  0, // No jitter for predictable testing
		}

		testCases := []struct {
			attempt  int
			expected time.Duration
		}{
			{0, 100 * time.Millisecond},
			{1, 200 * time.Millisecond},
			{2, 400 * time.Millisecond},
			{3, 800 * time.Millisecond},
			{10, 30 * time.Second}, // Should cap at 30 seconds
		}

		for _, tc := range testCases {
			backoff := sink.calculateBackoff(tc.attempt)
			if backoff != tc.expected {
				t.Errorf("Attempt %d: expected %v, got %v", tc.attempt, tc.expected, backoff)
			}
		}
	})

	t.Run("BackoffWithJitter", func(t *testing.T) {
		sink := &SentrySink{
			retryBackoff: 1 * time.Second,
			retryJitter:  0.5,
		}

		// Run multiple times to test jitter variation
		for i := 0; i < 10; i++ {
			backoff := sink.calculateBackoff(0)
			// With 50% jitter, backoff should be between 0.5s and 1.5s
			if backoff < 500*time.Millisecond || backoff > 1500*time.Millisecond {
				t.Errorf("Backoff with jitter out of range: %v", backoff)
			}
		}
	})
}

// TestMetricsCollection tests the metrics collection functionality
func TestMetricsCollection(t *testing.T) {
	t.Run("EventMetrics", func(t *testing.T) {
		sink := &SentrySink{
			metrics:       newMetricsCollector(),
			enableMetrics: true,
			hub:           &sentry.Hub{},
			maxRetries:    2,
			retryBackoff:  10 * time.Millisecond,
		}

		// Simulate successful event
		sink.metrics.eventsSent.Add(1)

		// Simulate failed event with retries
		sink.metrics.retryCount.Add(2)
		sink.metrics.eventsFailed.Add(1)
		sink.metrics.networkErrors.Add(1)

		metrics := sink.Metrics()
		if metrics.EventsSent != 1 {
			t.Errorf("Expected 1 event sent, got %d", metrics.EventsSent)
		}
		if metrics.EventsFailed != 1 {
			t.Errorf("Expected 1 event failed, got %d", metrics.EventsFailed)
		}
		if metrics.RetryCount != 2 {
			t.Errorf("Expected 2 retries, got %d", metrics.RetryCount)
		}
		if metrics.NetworkErrors != 1 {
			t.Errorf("Expected 1 network error, got %d", metrics.NetworkErrors)
		}
	})

	t.Run("BatchMetrics", func(t *testing.T) {
		sink := &SentrySink{
			metrics:       newMetricsCollector(),
			enableMetrics: true,
		}

		// Simulate batch processing
		sink.metrics.batchesSent.Add(3)
		sink.metrics.totalBatchSize.Add(150) // 3 batches with total 150 events

		metrics := sink.Metrics()
		if metrics.BatchesSent != 3 {
			t.Errorf("Expected 3 batches sent, got %d", metrics.BatchesSent)
		}
		if metrics.AverageBatchSize != 50.0 {
			t.Errorf("Expected average batch size 50, got %.2f", metrics.AverageBatchSize)
		}
	})

	t.Run("PerformanceMetrics", func(t *testing.T) {
		sink := &SentrySink{
			metrics:       newMetricsCollector(),
			enableMetrics: true,
		}

		// Simulate flush durations
		duration1 := 100 * time.Millisecond
		duration2 := 200 * time.Millisecond

		sink.metrics.lastFlushDuration.Store(int64(duration2))
		sink.metrics.totalFlushTime.Add(int64(duration1 + duration2))

		metrics := sink.Metrics()
		if metrics.LastFlushDuration != duration2 {
			t.Errorf("Expected last flush %v, got %v", duration2, metrics.LastFlushDuration)
		}
		if metrics.TotalFlushTime != duration1+duration2 {
			t.Errorf("Expected total flush time %v, got %v", duration1+duration2, metrics.TotalFlushTime)
		}
	})
}

// TestStackTraceCache tests the stack trace caching mechanism
func TestStackTraceCache(t *testing.T) {
	t.Run("CacheHitMiss", func(t *testing.T) {
		cache := newStackTraceCache(10)

		// Create a mock stacktrace
		st := &sentry.Stacktrace{
			Frames: []sentry.Frame{
				{Function: "TestFunc", Filename: "test.go", Lineno: 42},
			},
		}

		// Test cache miss
		if _, ok := cache.get("key1"); ok {
			t.Error("Expected cache miss for new key")
		}

		// Test cache set and hit
		cache.set("key1", st)
		if cached, ok := cache.get("key1"); !ok {
			t.Error("Expected cache hit after set")
		} else if cached != st {
			t.Error("Cached stacktrace doesn't match original")
		}

		// Verify size
		if cache.size() != 1 {
			t.Errorf("Expected cache size 1, got %d", cache.size())
		}
	})

	t.Run("LRUEviction", func(t *testing.T) {
		cache := newStackTraceCache(3)

		// Fill cache beyond capacity
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("key%d", i)
			st := &sentry.Stacktrace{
				Frames: []sentry.Frame{
					{Function: fmt.Sprintf("Func%d", i)},
				},
			}
			cache.set(key, st)
		}

		// Cache should only have last 3 entries
		if cache.size() != 3 {
			t.Errorf("Expected cache size 3, got %d", cache.size())
		}

		// First two entries should be evicted
		if _, ok := cache.get("key0"); ok {
			t.Error("Expected key0 to be evicted")
		}
		if _, ok := cache.get("key1"); ok {
			t.Error("Expected key1 to be evicted")
		}

		// Last three should still be present
		for i := 2; i < 5; i++ {
			key := fmt.Sprintf("key%d", i)
			if _, ok := cache.get(key); !ok {
				t.Errorf("Expected %s to be in cache", key)
			}
		}
	})

	t.Run("CacheClear", func(t *testing.T) {
		cache := newStackTraceCache(10)

		// Add some entries
		for i := 0; i < 5; i++ {
			cache.set(fmt.Sprintf("key%d", i), &sentry.Stacktrace{})
		}

		// Clear cache
		cache.clear()

		if cache.size() != 0 {
			t.Errorf("Expected cache size 0 after clear, got %d", cache.size())
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		cache := newStackTraceCache(100)
		var wg sync.WaitGroup

		// Concurrent writes
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					key := fmt.Sprintf("key-%d-%d", id, j)
					cache.set(key, &sentry.Stacktrace{})
				}
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					key := fmt.Sprintf("key-%d-%d", id, j)
					cache.get(key)
				}
			}(i)
		}

		wg.Wait()

		// Should not panic and size should be reasonable
		size := cache.size()
		if size < 0 || size > 100 {
			t.Errorf("Unexpected cache size after concurrent access: %d", size)
		}
	})
}

// TestStringBuilderPooling tests the string builder pool performance
func TestStringBuilderPooling(t *testing.T) {
	sink := &SentrySink{}

	// Test that builders are reused from pool
	event := &core.LogEvent{
		MessageTemplate: "Test {Value1} and {Value2}",
		Properties: map[string]interface{}{
			"Value1": "first",
			"Value2": "second",
		},
	}

	// Run multiple times to test pool reuse
	for i := 0; i < 100; i++ {
		result := sink.renderMessage(event)
		expected := "Test first and second"
		if result != expected {
			t.Errorf("Iteration %d: expected '%s', got '%s'", i, expected, result)
		}
	}
}

// TestTransactionTracking tests the performance monitoring features
func TestTransactionTracking(t *testing.T) {
	t.Run("TransactionCreation", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-transaction", "test.operation")

		tx := GetTransaction(txCtx)
		if tx == nil {
			t.Fatal("Expected transaction to be created")
		}

		if tx.Name != "test-transaction" {
			t.Errorf("Expected transaction name 'test-transaction', got %s", tx.Name)
		}
	})

	t.Run("SpanCreation", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-tx", "test")

		spanCtx, finish := StartSpan(txCtx, "db.query")
		defer finish()

		span := GetSpan(spanCtx)
		if span == nil {
			t.Fatal("Expected span to be created")
		}
	})

	t.Run("SpanDataAndTags", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-tx", "test")

		spanCtx, finish := StartSpan(txCtx, "operation")
		defer finish()

		SetSpanTag(spanCtx, "user.id", "123")
		SetSpanData(spanCtx, "query", "SELECT * FROM users")
		SetSpanStatus(spanCtx, "ok")

		// Verify span was modified (would need access to span internals in real test)
		span := GetSpan(spanCtx)
		if span == nil {
			t.Fatal("Expected span to exist")
		}
	})

	t.Run("MeasureSpan", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-tx", "test")

		// Test successful operation
		err := MeasureSpan(txCtx, "successful.op", func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Test failed operation
		expectedErr := errors.New("operation failed")
		err = MeasureSpan(txCtx, "failed.op", func() error {
			return expectedErr
		})
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("BatchSpan", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-tx", "test")

		itemCount := 100
		err := BatchSpan(txCtx, "batch.process", itemCount, func() error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("TraceHTTPRequest", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-tx", "test")

		spanCtx, finish := TraceHTTPRequest(txCtx, "GET", "https://api.example.com/users")

		// Simulate HTTP response
		finish(200) // Success

		// Test with error status
		spanCtx2, finish2 := TraceHTTPRequest(txCtx, "POST", "https://api.example.com/users")
		finish2(500) // Server error
		
		// Verify spans were created
		if GetSpan(spanCtx) == nil {
			t.Error("Expected HTTP span to be created")
		}
		if GetSpan(spanCtx2) == nil {
			t.Error("Expected second HTTP span to be created")
		}
	})

	t.Run("TraceDatabaseQuery", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-tx", "test")

		query := "SELECT * FROM orders WHERE status = ?"
		spanCtx, finish := TraceDatabaseQuery(txCtx, query, "orders_db")

		// Simulate successful query
		finish(nil)

		// Test with error
		spanCtx2, finish2 := TraceDatabaseQuery(txCtx, query, "orders_db")
		finish2(errors.New("connection timeout"))

		// Verify spans were created
		if GetSpan(spanCtx) == nil {
			t.Error("Expected DB span to be created")
		}
		if GetSpan(spanCtx2) == nil {
			t.Error("Expected second DB span to be created")
		}
	})

	t.Run("TraceCache", func(t *testing.T) {
		ctx := context.Background()
		txCtx := StartTransaction(ctx, "test-tx", "test")

		// Test cache hit
		spanCtx, finish := TraceCache(txCtx, "get", "user:123")
		finish(true) // Cache hit

		// Test cache miss
		spanCtx2, finish2 := TraceCache(txCtx, "get", "user:456")
		finish2(false) // Cache miss

		// Verify spans were created
		if GetSpan(spanCtx) == nil {
			t.Error("Expected cache span to be created")
		}
		if GetSpan(spanCtx2) == nil {
			t.Error("Expected second cache span to be created")
		}
	})

	t.Run("TransactionMiddleware", func(t *testing.T) {
		middleware := TransactionMiddleware("test.handler")

		ctx := context.Background()
		
		// Test successful handler
		err := middleware(ctx, func(txCtx context.Context) error {
			// Verify transaction exists
			if GetTransaction(txCtx) == nil {
				t.Error("Expected transaction in handler context")
			}
			return nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Test failed handler
		expectedErr := errors.New("handler error")
		err = middleware(ctx, func(txCtx context.Context) error {
			return expectedErr
		})
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

// TestEnvironmentVariable tests environment variable support
func TestEnvironmentVariable(t *testing.T) {
	// This would require setting/unsetting environment variables
	// which should be done carefully in tests
	t.Run("DSNFromEnvironment", func(t *testing.T) {
		// Save original value
		originalDSN := os.Getenv("SENTRY_DSN")
		defer os.Setenv("SENTRY_DSN", originalDSN)

		// Set test DSN
		testDSN := "https://test@sentry.io/123"
		os.Setenv("SENTRY_DSN", testDSN)

		// Create sink without explicit DSN
		sink, err := NewSentrySink("")
		if err == nil {
			defer sink.Close()
			t.Skip("Skipping as it would connect to real Sentry")
		}

		// Clear environment variable
		os.Unsetenv("SENTRY_DSN")

		// Should fail without DSN
		_, err = NewSentrySink("")
		if err == nil {
			t.Error("Expected error when DSN not provided and env var not set")
		}
	})
}

// Benchmark tests for new features
func BenchmarkRetryCalculation(b *testing.B) {
	sink := &SentrySink{
		retryBackoff: 100 * time.Millisecond,
		retryJitter:  0.2,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sink.calculateBackoff(i % 5)
	}
}

func BenchmarkStackTraceCaching(b *testing.B) {
	cache := newStackTraceCache(100)
	errors := make([]error, 10)
	for i := range errors {
		errors[i] = fmt.Errorf("error %d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := errors[i%len(errors)]
		key := fmt.Sprintf("%T:%s", err, err.Error())
		
		if _, ok := cache.get(key); !ok {
			cache.set(key, &sentry.Stacktrace{})
		}
	}
}

func BenchmarkMetricsCollection(b *testing.B) {
	metrics := newMetricsCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.eventsSent.Add(1)
		metrics.breadcrumbsAdded.Add(1)
		if i%10 == 0 {
			_ = metrics.snapshot()
		}
	}
}

func BenchmarkTransactionCreation(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txCtx := StartTransaction(ctx, "bench-tx", "benchmark")
		if tx := GetTransaction(txCtx); tx != nil {
			tx.Finish()
		}
	}
}