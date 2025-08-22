package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/willibrandon/mtlog"
)

func TestAllocations(t *testing.T) {
	// Create logger with discard sink
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	options := DefaultOptions(logger)
	mw := Middleware(options)
	
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	
	// Run once to warm up
	handler.ServeHTTP(rec, req)
	
	// Now measure allocations
	allocs := testing.AllocsPerRun(100, func() {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	})
	
	t.Logf("Allocations per request: %.1f", allocs)
}

func TestAllocationBreakdown(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	tests := []struct {
		name   string
		setup  func(*Options)
	}{
		{
			name:  "Minimal",
			setup: func(o *Options) {},
		},
		{
			name: "WithRequestID",
			setup: func(o *Options) {
				o.GenerateRequestID = true
			},
		},
		{
			name: "WithAllRequestFields",
			setup: func(o *Options) {
				o.RequestFields = []string{"method", "path", "ip", "user_agent", "referer", "proto", "host"}
			},
		},
		{
			name: "WithCustomFields",
			setup: func(o *Options) {
				o.CustomFields = []FieldExtractor{
					{Name: "Field1", Extract: func(r *http.Request) any { return "value1" }},
					{Name: "Field2", Extract: func(r *http.Request) any { return "value2" }},
				}
			},
		},
		{
			name: "WithBodyLogging",
			setup: func(o *Options) {
				o.LogRequestBody = true
				o.LogResponseBody = true
			},
		},
		{
			name: "WithSampling",
			setup: func(o *Options) {
				o.Sampler = NewRateSampler(0.5)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := DefaultOptions(logger)
			tt.setup(options)
			
			mw := Middleware(options)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}))
			
			req := httptest.NewRequest("POST", "/test", nil)
			req.Header.Set("User-Agent", "TestAgent")
			req.Header.Set("Referer", "https://example.com")
			
			// Warm up
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			
			// Measure
			allocs := testing.AllocsPerRun(100, func() {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
			})
			
			t.Logf("%s: %.1f allocations", tt.name, allocs)
		})
	}
}

func TestAllocationsByComponent(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	t.Run("ResponseWriter wrapper", func(t *testing.T) {
		allocs := testing.AllocsPerRun(1000, func() {
			w := httptest.NewRecorder()
			rw := newResponseWriter(w)
			rw.WriteHeader(200)
			rw.Write([]byte("test"))
		})
		t.Logf("ResponseWriter wrapper: %.1f allocations", allocs)
	})
	
	t.Run("UUID generation", func(t *testing.T) {
		allocs := testing.AllocsPerRun(1000, func() {
			_ = uuid.New().String()
		})
		t.Logf("UUID generation: %.1f allocations", allocs)
	})
	
	t.Run("Context with values", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		allocs := testing.AllocsPerRun(1000, func() {
			ctx := req.Context()
			ctx = context.WithValue(ctx, LoggerContextKey, logger)
			ctx = context.WithValue(ctx, RequestIDContextKey, "test-id")
			_ = req.WithContext(ctx)
		})
		t.Logf("Context with values: %.1f allocations", allocs)
	})
	
	t.Run("Logger.With", func(t *testing.T) {
		allocs := testing.AllocsPerRun(1000, func() {
			_ = logger.With("Key", "Value")
		})
		t.Logf("Logger.With: %.1f allocations", allocs)
	})
	
	t.Run("Logger.With multiple", func(t *testing.T) {
		allocs := testing.AllocsPerRun(1000, func() {
			_ = logger.With("Key1", "Value1").
				With("Key2", "Value2").
				With("Key3", "Value3")
		})
		t.Logf("Logger.With multiple: %.1f allocations", allocs)
	})
	
	t.Run("Template args slice", func(t *testing.T) {
		allocs := testing.AllocsPerRun(1000, func() {
			args := []any{
				"GET",
				"/test",
				200,
				15.5,
			}
			_ = args
		})
		t.Logf("Template args slice: %.1f allocations", allocs)
	})
}

func TestMemoryUsage(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	options := DefaultOptions(logger)
	options.GenerateRequestID = true
	options.RequestFields = []string{"method", "path", "ip", "user_agent"}
	
	mw := Middleware(options)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	// Run 1000 requests
	for i := 0; i < 1000; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
	
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	totalAlloc := m2.TotalAlloc - m1.TotalAlloc
	avgPerRequest := totalAlloc / 1000
	
	t.Logf("Total memory allocated for 1000 requests: %d bytes", totalAlloc)
	t.Logf("Average per request: %d bytes", avgPerRequest)
	t.Logf("Number of GCs: %d", m2.NumGC-m1.NumGC)
}