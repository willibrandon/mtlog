package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// Integration test combining multiple features
func TestIntegrationFullPipeline(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	// Metrics recorder
	var requestCount int32
	metricsRecorder := &testMetricsRecorder{
		requestCount: &requestCount,
	}
	
	// Interceptors
	var beforeCalled, afterCalled bool
	
	// Error handler
	var errorHandled bool
	
	opts := &Options{
		Logger:            logger,
		GenerateRequestID: true,
		RequestIDHeader:   "X-Request-Id",
		LogRequestBody:    true,
		LogResponseBody:   true,
		MaxBodySize:       1024,
		BodySanitizer:     DefaultBodySanitizer,
		Sampler:           NewRateSampler(1.0), // Log everything
		MetricsRecorder:   metricsRecorder,
		LatencyField:      "duration_ms",
		LatencyUnit:       "ms",
		BeforeRequest: func(r *http.Request, log core.Logger) {
			beforeCalled = true
		},
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			afterCalled = true
		},
		ErrorHandler: func(err *MiddlewareError) (int, interface{}) {
			errorHandled = true
			return DefaultErrorHandler(err)
		},
		CustomFields: []FieldExtractor{
			{
				Name: "CustomField",
				Extract: func(r *http.Request) any {
					return "custom-value"
				},
			},
		},
	}
	
	// Create middleware chain with correlation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context values are available
		ctx := r.Context()
		if logger := FromContext(ctx); logger == nil {
			t.Error("Logger should be in context")
		}
		if id := RequestIDFromContext(ctx); id == "" {
			t.Error("Request ID should be in context")
		}
		
		// Write JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
		})
	})
	
	// Apply correlation and logging middleware
	finalHandler := PropagateTraceContext(
		Middleware(opts)(handler),
		CorrelationOptions{
			GenerateTraceID: true,
			GenerateSpanID:  true,
		},
	)
	
	// Create request with JSON body
	body := map[string]string{
		"username": "testuser",
		"password": "secret123",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	
	rec := httptest.NewRecorder()
	finalHandler.ServeHTTP(rec, req)
	
	// Verify all features worked
	if !beforeCalled {
		t.Error("BeforeRequest interceptor not called")
	}
	if !afterCalled {
		t.Error("AfterRequest interceptor not called")
	}
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Error("Metrics not recorded")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", rec.Code, http.StatusOK)
	}
	
	// Verify correlation headers
	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("Request ID header not set")
	}
	
	// errorHandled should be false for successful request
	if errorHandled {
		t.Error("Error handler should not be called for successful request")
	}
}

func TestIntegrationPanicRecovery(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var panicRecovered bool
	var metricsRecorded bool
	
	opts := &Options{
		Logger:            logger,
		GenerateRequestID: true,
		LatencyField:      "duration_ms",
		LatencyUnit:       "ms",
		MetricsRecorder: &testMetricsRecorder{
			onPanic: func() {
				metricsRecorded = true
			},
		},
		PanicHandler: func(w http.ResponseWriter, r *http.Request, err any) {
			panicRecovered = true
			panicErr := NewPanicError(err, r.Method, r.URL.Path, RequestIDFromContext(r.Context()))
			status, response := DevelopmentErrorHandler(panicErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(response)
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	
	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if !panicRecovered {
		t.Error("Panic was not recovered")
	}
	if !metricsRecorded {
		t.Error("Panic metrics not recorded")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %v, want %v", rec.Code, http.StatusInternalServerError)
	}
	
	// Verify error response
	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["error"] == nil {
		t.Error("Error message not in response")
	}
}

func TestIntegrationHighLoad(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	// Use batch metrics recorder
	var flushCount int32
	batchRecorder := NewBatchMetricsRecorder(
		func(metrics []RequestMetric) {
			atomic.AddInt32(&flushCount, 1)
		},
		100*time.Millisecond,
		30, // Flush every 30 requests
	)
	defer batchRecorder.Close()
	
	// Use dynamic sampling
	sampler := NewDynamicPathSampler([]PathSamplingRule{
		{Pattern: "/health", Rate: 0.0},
		{Pattern: "*", Rate: 1.0}, // Log all non-health requests
	})
	
	opts := &Options{
		Logger:          logger,
		Sampler:         sampler,
		MetricsRecorder: batchRecorder,
		LatencyField:    "duration_ms",
		LatencyUnit:     "ms",
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// Send 100 concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "/api/test"
			if i%10 == 0 {
				path = "/health"
			}
			req := httptest.NewRequest("GET", path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}(i)
	}
	
	wg.Wait()
	
	// Wait for final flush
	time.Sleep(150 * time.Millisecond)
	
	// Should have flushed at least 3 times (30, 60, 90 requests + final flush)
	if fc := atomic.LoadInt32(&flushCount); fc < 3 {
		t.Errorf("Flush count = %v, want at least 3", fc)
	}
}

func TestIntegrationMemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	// Enable pooling
	oldPooling := EnablePooling
	EnablePooling = true
	defer func() { EnablePooling = oldPooling }()
	
	opts := &Options{
		Logger:            logger,
		GenerateRequestID: true,
		LogRequestBody:    true,
		LogResponseBody:   true,
		MetricsRecorder:   NewSimpleMetricsRecorder(1000),
		LatencyField:      "duration_ms",
		LatencyUnit:       "ms",
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("response body"))
	}))
	
	// Measure initial memory
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	// Run many requests
	for i := 0; i < 10000; i++ {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("request body")))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
	
	// Measure final memory
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	
	// Calculate memory growth (handle potential underflow)
	var memGrowth int64
	if m2.Alloc >= m1.Alloc {
		memGrowth = int64(m2.Alloc - m1.Alloc)
	} else {
		// Memory was freed, growth is negative (which is good)
		memGrowth = -int64(m1.Alloc - m2.Alloc)
	}
	memGrowthMB := float64(memGrowth) / 1024 / 1024
	
	// Should not grow more than 10MB for 10k requests
	if memGrowthMB > 10 {
		t.Errorf("Memory growth = %.2f MB, should be less than 10 MB", memGrowthMB)
	}
}

func TestIntegrationWithRealServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real server test in short mode")
	}
	
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	// Create health handler
	healthHandler := NewHealthCheckHandler(logger).
		WithVersion("1.0.0").
		AddCheck("test", func() Check {
			return Check{Status: "healthy"}
		})
	
	// Setup middleware
	opts := DefaultOptions(logger)
	opts.GenerateRequestID = true
	opts.MetricsRecorder = NewSimpleMetricsRecorder(100)
	
	// Create router
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler)
	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "echo",
		})
	})
	
	// Apply middleware
	handler := Middleware(opts)(mux)
	
	// Start test server
	server := httptest.NewServer(handler)
	defer server.Close()
	
	// Test health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health status = %v, want %v", resp.StatusCode, http.StatusOK)
	}
	
	// Test API endpoint
	resp, err = http.Get(server.URL + "/api/echo")
	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("API status = %v, want %v", resp.StatusCode, http.StatusOK)
	}
	
	// Verify request ID header
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("Request ID header not present")
	}
}

// Test helper - metrics recorder for testing
type testMetricsRecorder struct {
	requestCount *int32
	onPanic      func()
}

func (t *testMetricsRecorder) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	if t.requestCount != nil {
		atomic.AddInt32(t.requestCount, 1)
	}
}

func (t *testMetricsRecorder) RecordPanic(method, path string) {
	if t.onPanic != nil {
		t.onPanic()
	}
}

func (t *testMetricsRecorder) RecordBodySize(direction string, size int) {}

func BenchmarkIntegrationFullMiddleware(b *testing.B) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	opts := &Options{
		Logger:            logger,
		GenerateRequestID: true,
		LogRequestBody:    true,
		LogResponseBody:   true,
		MetricsRecorder:   NewSimpleMetricsRecorder(1000),
		Sampler:           NewRateSampler(0.5),
		LatencyField:      "duration_ms",
		LatencyUnit:       "ms",
		BeforeRequest: func(r *http.Request, log core.Logger) {
			// No-op
		},
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			// No-op
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}))
	
	req := httptest.NewRequest("POST", "/api/test", bytes.NewReader([]byte(`{"data":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkIntegrationWithPooling(b *testing.B) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	b.Run("WithoutPooling", func(b *testing.B) {
		oldPooling := EnablePooling
		EnablePooling = false
		defer func() { EnablePooling = oldPooling }()
		
		opts := DefaultOptions(logger)
		handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest("GET", "/test", nil)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
	
	b.Run("WithPooling", func(b *testing.B) {
		oldPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = oldPooling }()
		
		opts := DefaultOptions(logger)
		handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest("GET", "/test", nil)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}