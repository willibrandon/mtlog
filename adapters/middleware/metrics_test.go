package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNoOpMetricsRecorder(t *testing.T) {
	recorder := &NoOpMetricsRecorder{}
	
	// All methods should not panic and return nothing
	recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
	recorder.RecordPanic("GET", "/test")
	recorder.RecordBodySize("request", 1024)
	
	// Test passes if no panic occurs
}

func TestPrometheusMetricsRecorder(t *testing.T) {
	recorder := &PrometheusMetricsRecorder{}
	
	// All methods should not panic (they're stubs for user implementation)
	recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
	recorder.RecordPanic("GET", "/test")
	recorder.RecordBodySize("request", 1024)
	
	// Test passes if no panic occurs
}

func TestSimpleMetricsRecorder(t *testing.T) {
	t.Run("basic recording", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(100)
		
		// Record some requests
		recorder.RecordRequest("GET", "/users", 200, 100*time.Millisecond)
		recorder.RecordRequest("POST", "/users", 201, 200*time.Millisecond)
		recorder.RecordRequest("GET", "/users/1", 404, 50*time.Millisecond)
		
		// Record some panics
		recorder.RecordPanic("GET", "/panic")
		recorder.RecordPanic("POST", "/panic")
		
		// Record some body sizes
		recorder.RecordBodySize("request", 1024)
		recorder.RecordBodySize("response", 2048)
		
		// Check metrics
		requests, panics, bodySizes := recorder.GetMetrics()
		
		if len(requests) != 3 {
			t.Errorf("Expected 3 requests, got %d", len(requests))
		}
		
		if len(panics) != 2 {
			t.Errorf("Expected 2 panics, got %d", len(panics))
		}
		
		if len(bodySizes) != 2 {
			t.Errorf("Expected 2 body sizes, got %d", len(bodySizes))
		}
		
		// Check first request
		req := requests[0]
		if req.Method != "GET" || req.Path != "/users" || req.StatusCode != 200 {
			t.Errorf("First request: got %+v", req)
		}
		if req.Duration != 100*time.Millisecond {
			t.Errorf("Expected duration 100ms, got %v", req.Duration)
		}
		
		// Check first panic
		panic := panics[0]
		if panic.Method != "GET" || panic.Path != "/panic" {
			t.Errorf("First panic: got %+v", panic)
		}
		
		// Check first body size
		body := bodySizes[0]
		if body.Direction != "request" || body.Size != 1024 {
			t.Errorf("First body size: got %+v", body)
		}
	})
	
	t.Run("request limit enforcement", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(2)
		
		// Record more requests than the limit
		for i := 0; i < 5; i++ {
			recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
		}
		
		requests, _, _ := recorder.GetMetrics()
		
		// Should only keep the last 2 requests
		if len(requests) != 2 {
			t.Errorf("Expected 2 requests (limit), got %d", len(requests))
		}
	})
	
	t.Run("panic limit enforcement", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(10)
		
		// Record more panics than the limit (100)
		for i := 0; i < 150; i++ {
			recorder.RecordPanic("GET", "/panic")
		}
		
		_, panics, _ := recorder.GetMetrics()
		
		// Should only keep the last 100 panics
		if len(panics) != 100 {
			t.Errorf("Expected 100 panics (limit), got %d", len(panics))
		}
	})
	
	t.Run("body size limit enforcement", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(10)
		
		// Record more body sizes than the limit (1000)
		for i := 0; i < 1200; i++ {
			recorder.RecordBodySize("request", 1024)
		}
		
		_, _, bodySizes := recorder.GetMetrics()
		
		// Should only keep the last 1000 body sizes
		if len(bodySizes) != 1000 {
			t.Errorf("Expected 1000 body sizes (limit), got %d", len(bodySizes))
		}
	})
	
	t.Run("metrics are copied", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(100)
		recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
		
		requests1, _, _ := recorder.GetMetrics()
		requests2, _, _ := recorder.GetMetrics()
		
		// Should be different slices
		if &requests1[0] == &requests2[0] {
			t.Errorf("GetMetrics should return copies, not the same slice")
		}
		
		// But with the same content
		if requests1[0].Method != requests2[0].Method {
			t.Errorf("Copied metrics should have same content")
		}
	})
}

func TestBatchMetricsRecorder(t *testing.T) {
	t.Run("basic batching", func(t *testing.T) {
		var flushedMetrics []RequestMetric
		var flushMutex sync.Mutex
		
		flushFunc := func(metrics []RequestMetric) {
			flushMutex.Lock()
			defer flushMutex.Unlock()
			flushedMetrics = append(flushedMetrics, metrics...)
		}
		
		recorder := NewBatchMetricsRecorder(flushFunc, 100*time.Millisecond, 3)
		defer recorder.Close()
		
		// Record requests to trigger batch flush
		recorder.RecordRequest("GET", "/test1", 200, time.Millisecond)
		recorder.RecordRequest("GET", "/test2", 201, time.Millisecond)
		recorder.RecordRequest("GET", "/test3", 202, time.Millisecond) // Should trigger flush
		
		// Wait a bit for flush
		time.Sleep(50 * time.Millisecond)
		
		flushMutex.Lock()
		count := len(flushedMetrics)
		flushMutex.Unlock()
		
		if count != 3 {
			t.Errorf("Expected 3 flushed metrics, got %d", count)
		}
	})
	
	t.Run("time-based flushing", func(t *testing.T) {
		var flushedMetrics []RequestMetric
		var flushMutex sync.Mutex
		
		flushFunc := func(metrics []RequestMetric) {
			flushMutex.Lock()
			defer flushMutex.Unlock()
			flushedMetrics = append(flushedMetrics, metrics...)
		}
		
		recorder := NewBatchMetricsRecorder(flushFunc, 50*time.Millisecond, 10)
		defer recorder.Close()
		
		// Record one request (won't trigger size-based flush)
		recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
		
		// Wait for time-based flush
		time.Sleep(100 * time.Millisecond)
		
		flushMutex.Lock()
		count := len(flushedMetrics)
		flushMutex.Unlock()
		
		if count != 1 {
			t.Errorf("Expected 1 flushed metric from time-based flush, got %d", count)
		}
	})
	
	t.Run("panic recording", func(t *testing.T) {
		var flushedMetrics []RequestMetric
		var flushMutex sync.Mutex
		
		flushFunc := func(metrics []RequestMetric) {
			flushMutex.Lock()
			defer flushMutex.Unlock()
			flushedMetrics = append(flushedMetrics, metrics...)
		}
		
		recorder := NewBatchMetricsRecorder(flushFunc, time.Second, 10)
		defer recorder.Close()
		
		// Record panic
		recorder.RecordPanic("GET", "/panic")
		
		// Wait a bit for immediate flush
		time.Sleep(50 * time.Millisecond)
		
		flushMutex.Lock()
		defer flushMutex.Unlock()
		
		if len(flushedMetrics) != 1 {
			t.Errorf("Expected 1 panic metric to be flushed immediately, got %d", len(flushedMetrics))
		}
		
		metric := flushedMetrics[0]
		if metric.Method != "GET" || metric.Path != "/panic" || metric.StatusCode != 500 {
			t.Errorf("Panic metric: got %+v", metric)
		}
	})
	
	t.Run("body size recording is no-op", func(t *testing.T) {
		recorder := NewBatchMetricsRecorder(nil, time.Second, 10)
		defer recorder.Close()
		
		// Should not panic
		recorder.RecordBodySize("request", 1024)
	})
	
	t.Run("close flushes remaining metrics", func(t *testing.T) {
		var flushedMetrics []RequestMetric
		var flushMutex sync.Mutex
		
		flushFunc := func(metrics []RequestMetric) {
			flushMutex.Lock()
			defer flushMutex.Unlock()
			flushedMetrics = append(flushedMetrics, metrics...)
		}
		
		recorder := NewBatchMetricsRecorder(flushFunc, time.Hour, 10) // Long interval
		
		// Record some metrics
		recorder.RecordRequest("GET", "/test1", 200, time.Millisecond)
		recorder.RecordRequest("GET", "/test2", 201, time.Millisecond)
		
		// Close should flush remaining metrics
		recorder.Close()
		
		flushMutex.Lock()
		count := len(flushedMetrics)
		flushMutex.Unlock()
		
		if count != 2 {
			t.Errorf("Expected 2 metrics flushed on close, got %d", count)
		}
	})
	
	t.Run("stats reporting", func(t *testing.T) {
		recorder := NewBatchMetricsRecorder(nil, time.Hour, 10)
		defer recorder.Close()
		
		// Record some metrics
		recorder.RecordRequest("GET", "/test1", 200, time.Millisecond)
		recorder.RecordRequest("GET", "/test2", 201, time.Millisecond)
		
		batchSize, _ := recorder.Stats()
		if batchSize != 2 {
			t.Errorf("Expected batch size 2, got %d", batchSize)
		}
	})
	
	t.Run("default values", func(t *testing.T) {
		recorder := NewBatchMetricsRecorder(nil, 0, 0) // Invalid values
		defer recorder.Close()
		
		// Should use defaults without panicking
		recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
	})
	
	t.Run("nil flush function", func(t *testing.T) {
		recorder := NewBatchMetricsRecorder(nil, 10*time.Millisecond, 1)
		defer recorder.Close()
		
		// Should not panic with nil flush function
		recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
		time.Sleep(20 * time.Millisecond) // Wait for potential flush
	})
}

func TestMetricsMiddleware(t *testing.T) {
	t.Run("basic metrics recording", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(100)
		middleware := MetricsMiddleware(recorder)
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add small delay to ensure measurable duration on all platforms
			// Windows timer resolution may require a larger delay
			time.Sleep(time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		})
		
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		// Process request through middleware
		middleware(handler).ServeHTTP(w, req)
		
		// Check recorded metrics
		requests, panics, bodySizes := recorder.GetMetrics()
		
		if len(requests) != 1 {
			t.Errorf("Expected 1 request metric, got %d", len(requests))
		}
		
		if len(panics) != 0 {
			t.Errorf("Expected 0 panic metrics, got %d", len(panics))
		}
		
		if len(bodySizes) != 1 {
			t.Errorf("Expected 1 body size metric, got %d", len(bodySizes))
		}
		
		// Check request metric details
		req_metric := requests[0]
		if req_metric.Method != "GET" {
			t.Errorf("Expected method GET, got %s", req_metric.Method)
		}
		if req_metric.Path != "/test" {
			t.Errorf("Expected path /test, got %s", req_metric.Path)
		}
		if req_metric.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", req_metric.StatusCode)
		}
		if req_metric.Duration <= 0 {
			t.Errorf("Expected positive duration, got %v", req_metric.Duration)
		}
		
		// Check body size metric
		bodyMetric := bodySizes[0]
		if bodyMetric.Direction != "response" {
			t.Errorf("Expected direction 'response', got %s", bodyMetric.Direction)
		}
		if bodyMetric.Size != len("test response") {
			t.Errorf("Expected size %d, got %d", len("test response"), bodyMetric.Size)
		}
	})
	
	t.Run("panic recording", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(100)
		middleware := MetricsMiddleware(recorder)
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})
		
		req := httptest.NewRequest("GET", "/panic", nil)
		w := httptest.NewRecorder()
		
		// Process request through middleware, expect panic to be re-raised
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Expected panic to be re-raised")
				}
			}()
			middleware(handler).ServeHTTP(w, req)
		}()
		
		// Check recorded metrics
		requests, panics, _ := recorder.GetMetrics()
		
		if len(requests) != 1 {
			t.Errorf("Expected 1 request metric, got %d", len(requests))
		}
		
		if len(panics) != 1 {
			t.Errorf("Expected 1 panic metric, got %d", len(panics))
		}
		
		// Check panic metric
		panicMetric := panics[0]
		if panicMetric.Method != "GET" {
			t.Errorf("Expected method GET, got %s", panicMetric.Method)
		}
		if panicMetric.Path != "/panic" {
			t.Errorf("Expected path /panic, got %s", panicMetric.Path)
		}
	})
	
	t.Run("various status codes", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(100)
		middleware := MetricsMiddleware(recorder)
		
		statusCodes := []int{200, 201, 400, 404, 500}
		
		for _, statusCode := range statusCodes {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
			})
			
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			
			middleware(handler).ServeHTTP(w, req)
		}
		
		// Check all requests were recorded
		requests, _, _ := recorder.GetMetrics()
		if len(requests) != len(statusCodes) {
			t.Errorf("Expected %d request metrics, got %d", len(statusCodes), len(requests))
		}
		
		// Check status codes are recorded correctly
		for i, expectedStatus := range statusCodes {
			if requests[i].StatusCode != expectedStatus {
				t.Errorf("Request %d: expected status %d, got %d", i, expectedStatus, requests[i].StatusCode)
			}
		}
	})
	
	t.Run("timing accuracy", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(100)
		middleware := MetricsMiddleware(recorder)
		
		sleepDuration := 50 * time.Millisecond
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(sleepDuration)
			w.WriteHeader(http.StatusOK)
		})
		
		req := httptest.NewRequest("GET", "/slow", nil)
		w := httptest.NewRecorder()
		
		middleware(handler).ServeHTTP(w, req)
		
		requests, _, _ := recorder.GetMetrics()
		if len(requests) != 1 {
			t.Fatalf("Expected 1 request, got %d", len(requests))
		}
		
		duration := requests[0].Duration
		// Duration should be at least the sleep duration
		if duration < sleepDuration {
			t.Errorf("Expected duration >= %v, got %v", sleepDuration, duration)
		}
		
		// Duration should be reasonable (not too much overhead)
		// Allow more tolerance on CI systems which may have timing variations
		maxOverhead := 20 * time.Millisecond
		if duration > sleepDuration+maxOverhead {
			t.Errorf("Duration seems too long: %v (expected ~%v with max %v overhead)", duration, sleepDuration, maxOverhead)
		}
	})
}

func TestMetricsRecorderConcurrency(t *testing.T) {
	t.Run("SimpleMetricsRecorder concurrency", func(t *testing.T) {
		recorder := NewSimpleMetricsRecorder(1000)
		
		const numGoroutines = 100
		const numOperationsPerGoroutine = 10
		
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperationsPerGoroutine; j++ {
					recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
					recorder.RecordPanic("GET", "/panic")
					recorder.RecordBodySize("request", 1024)
				}
			}(i)
		}
		
		wg.Wait()
		
		// All operations should complete without race conditions
		requests, panics, bodySizes := recorder.GetMetrics()
		
		totalExpected := numGoroutines * numOperationsPerGoroutine
		
		// Note: Due to limits, we might not have all records
		if len(requests) == 0 || len(panics) == 0 || len(bodySizes) == 0 {
			t.Errorf("Expected some metrics to be recorded")
		}
		
		// But we shouldn't have more than expected
		if len(requests) > totalExpected {
			t.Errorf("Too many requests recorded: %d > %d", len(requests), totalExpected)
		}
		if len(panics) > totalExpected {
			t.Errorf("Too many panics recorded: %d > %d", len(panics), totalExpected)
		}
		if len(bodySizes) > totalExpected {
			t.Errorf("Too many body sizes recorded: %d > %d", len(bodySizes), totalExpected)
		}
	})
	
	t.Run("BatchMetricsRecorder concurrency", func(t *testing.T) {
		var flushedCount int
		var flushMutex sync.Mutex
		
		flushFunc := func(metrics []RequestMetric) {
			flushMutex.Lock()
			flushedCount += len(metrics)
			flushMutex.Unlock()
		}
		
		recorder := NewBatchMetricsRecorder(flushFunc, 10*time.Millisecond, 5)
		defer recorder.Close()
		
		const numGoroutines = 50
		const numOperationsPerGoroutine = 5
		
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperationsPerGoroutine; j++ {
					recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Wait for potential final flush
		time.Sleep(50 * time.Millisecond)
		
		flushMutex.Lock()
		count := flushedCount
		flushMutex.Unlock()
		
		expected := numGoroutines * numOperationsPerGoroutine
		if count != expected {
			t.Errorf("Expected %d flushed metrics, got %d", expected, count)
		}
	})
}

func BenchmarkSimpleMetricsRecorder(b *testing.B) {
	recorder := NewSimpleMetricsRecorder(10000)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
		}
	})
}

func BenchmarkBatchMetricsRecorder(b *testing.B) {
	flushFunc := func(metrics []RequestMetric) {
		// Simulate some processing
		_ = len(metrics)
	}
	
	recorder := NewBatchMetricsRecorder(flushFunc, time.Second, 1000)
	defer recorder.Close()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder.RecordRequest("GET", "/test", 200, time.Millisecond)
		}
	})
}

func BenchmarkMetricsMiddleware(b *testing.B) {
	recorder := &NoOpMetricsRecorder{}
	middleware := MetricsMiddleware(recorder)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := middleware(handler)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)
		}
	})
}