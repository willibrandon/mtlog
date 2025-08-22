package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func TestBeforeRequestInterceptor(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var interceptorCalled bool
	var capturedMethod string
	var capturedPath string
	
	opts := &Options{
		Logger:       logger,
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
		BeforeRequest: func(r *http.Request, log core.Logger) {
			interceptorCalled = true
			capturedMethod = r.Method
			capturedPath = r.URL.Path
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if !interceptorCalled {
		t.Error("BeforeRequest interceptor was not called")
	}
	if capturedMethod != "POST" {
		t.Errorf("Captured method = %v, want POST", capturedMethod)
	}
	if capturedPath != "/test" {
		t.Errorf("Captured path = %v, want /test", capturedPath)
	}
}

func TestAfterRequestInterceptor(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var interceptorCalled bool
	var capturedStatus int
	var capturedDuration time.Duration
	
	opts := &Options{
		Logger:       logger,
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			interceptorCalled = true
			capturedStatus = statusCode
			capturedDuration = duration
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate some work
		w.WriteHeader(http.StatusCreated)
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if !interceptorCalled {
		t.Error("AfterRequest interceptor was not called")
	}
	if capturedStatus != http.StatusCreated {
		t.Errorf("Captured status = %v, want %v", capturedStatus, http.StatusCreated)
	}
	if capturedDuration < 10*time.Millisecond {
		t.Errorf("Captured duration = %v, should be at least 10ms", capturedDuration)
	}
}

func TestBothInterceptors(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var beforeCalled, afterCalled bool
	var beforeTime, afterTime time.Time
	
	opts := &Options{
		Logger:       logger,
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
		BeforeRequest: func(r *http.Request, log core.Logger) {
			beforeCalled = true
			beforeTime = time.Now()
		},
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			afterCalled = true
			afterTime = time.Now()
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if !beforeCalled {
		t.Error("BeforeRequest interceptor was not called")
	}
	if !afterCalled {
		t.Error("AfterRequest interceptor was not called")
	}
	if !afterTime.After(beforeTime) {
		t.Error("AfterRequest should be called after BeforeRequest")
	}
}

func TestInterceptorWithPanic(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var beforeCalled, afterCalled bool
	var capturedStatus int
	
	opts := &Options{
		Logger:       logger,
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
		BeforeRequest: func(r *http.Request, log core.Logger) {
			beforeCalled = true
		},
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			afterCalled = true
			capturedStatus = statusCode
		},
		PanicHandler: func(w http.ResponseWriter, r *http.Request, err any) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if !beforeCalled {
		t.Error("BeforeRequest should be called even when handler panics")
	}
	if !afterCalled {
		t.Error("AfterRequest should be called even when handler panics")
	}
	if capturedStatus != http.StatusInternalServerError {
		t.Errorf("Status after panic = %v, want %v", capturedStatus, http.StatusInternalServerError)
	}
}

func TestInterceptorOrder(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var order []string
	
	opts := &Options{
		Logger:       logger,
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
		BeforeRequest: func(r *http.Request, log core.Logger) {
			order = append(order, "before")
		},
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			order = append(order, "after")
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	expectedOrder := []string{"before", "handler", "after"}
	if len(order) != len(expectedOrder) {
		t.Fatalf("Order length = %v, want %v", len(order), len(expectedOrder))
	}
	
	for i, expected := range expectedOrder {
		if order[i] != expected {
			t.Errorf("Order[%d] = %v, want %v", i, order[i], expected)
		}
	}
}

func TestInterceptorAccess(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var loggerNotNil bool
	var hasRequestID bool
	
	opts := &Options{
		Logger:            logger,
		GenerateRequestID: true,
		LatencyField:      "duration_ms",
		LatencyUnit:       "ms",
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			loggerNotNil = log != nil
			
			// Check if request ID is accessible from context
			ctx := r.Context()
			if id, ok := ctx.Value(RequestIDContextKey).(string); ok && id != "" {
				hasRequestID = true
			}
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if !loggerNotNil {
		t.Error("Logger should be provided to interceptor")
	}
	if !hasRequestID {
		t.Error("Request ID should be accessible in interceptor")
	}
}

func TestInterceptorConcurrency(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var beforeCount int32
	var afterCount int32
	
	opts := &Options{
		Logger:       logger,
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
		BeforeRequest: func(r *http.Request, log core.Logger) {
			atomic.AddInt32(&beforeCount, 1)
		},
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			atomic.AddInt32(&afterCount, 1)
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// Run 100 concurrent requests
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			done <- true
		}()
	}
	
	// Wait for all requests to complete
	for i := 0; i < 100; i++ {
		<-done
	}
	
	if bc := atomic.LoadInt32(&beforeCount); bc != 100 {
		t.Errorf("BeforeRequest count = %v, want 100", bc)
	}
	if ac := atomic.LoadInt32(&afterCount); ac != 100 {
		t.Errorf("AfterRequest count = %v, want 100", ac)
	}
}

func TestInterceptorWithSkippedPaths(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	var interceptorCalled bool
	
	opts := &Options{
		Logger:       logger,
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
		SkipPaths:    []string{"/health", "/metrics"},
		BeforeRequest: func(r *http.Request, log core.Logger) {
			interceptorCalled = true
		},
	}
	
	handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// Test skipped path
	interceptorCalled = false
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	if interceptorCalled {
		t.Error("Interceptor should not be called for skipped paths")
	}
	
	// Test non-skipped path
	interceptorCalled = false
	req = httptest.NewRequest("GET", "/api/users", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	if !interceptorCalled {
		t.Error("Interceptor should be called for non-skipped paths")
	}
}

func BenchmarkInterceptors(b *testing.B) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	b.Run("NoInterceptors", func(b *testing.B) {
		opts := &Options{
			Logger:       logger,
			LatencyField: "duration_ms",
			LatencyUnit:  "ms",
		}
		
		handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest("GET", "/test", nil)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
	
	b.Run("WithInterceptors", func(b *testing.B) {
		opts := &Options{
			Logger:       logger,
			LatencyField: "duration_ms",
			LatencyUnit:  "ms",
			BeforeRequest: func(r *http.Request, log core.Logger) {
				// No-op
			},
			AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
				// No-op
			},
		}
		
		handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest("GET", "/test", nil)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}