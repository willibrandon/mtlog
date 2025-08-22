package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func BenchmarkMiddlewareWithInterceptors(b *testing.B) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	opts := &Options{
		Logger: logger,
		BeforeRequest: func(r *http.Request, log core.Logger) {
			// Simulate some work
			_ = r.URL.Path
		},
		AfterRequest: func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
			// Simulate some work
			_ = statusCode
		},
	}
	
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
}

func BenchmarkMiddlewareWithBodyLogging(b *testing.B) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	requestBody := []byte(`{"username":"test","password":"secret"}`)
	
	b.Run("NoBodyLogging", func(b *testing.B) {
		opts := &Options{
			Logger: logger,
		}
		
		handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"ok"}`))
		}))
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/test", bytes.NewReader(requestBody))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
	
	b.Run("WithBodyLogging", func(b *testing.B) {
		opts := &Options{
			Logger:          logger,
			LogRequestBody:  true,
			LogResponseBody: true,
			MaxBodySize:     1024,
			BodySanitizer:   DefaultBodySanitizer,
		}
		
		handler := Middleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"ok"}`))
		}))
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/test", bytes.NewReader(requestBody))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkMetricsRecorderComparison(b *testing.B) {
	b.Run("SimpleRecorder", func(b *testing.B) {
		recorder := NewSimpleMetricsRecorder(10000)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			recorder.RecordRequest("GET", "/test", 200, 10*time.Millisecond)
		}
	})
	
	b.Run("BatchRecorder", func(b *testing.B) {
		recorder := NewBatchMetricsRecorder(
			func(metrics []RequestMetric) {
				// No-op flush
			},
			1*time.Second,
			1000,
		)
		defer recorder.Close()
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			recorder.RecordRequest("GET", "/test", 200, 10*time.Millisecond)
		}
	})
}

func BenchmarkDynamicSampling(b *testing.B) {
	sampler := NewDynamicPathSampler([]PathSamplingRule{
		{Pattern: "/health", Rate: 0.0},
		{Pattern: "/metrics", Rate: 0.0},
		{Pattern: "/api/v1/*", Rate: 1.0},
		{Pattern: "/api/v2/**", Rate: 0.5},
		{Pattern: "/debug/*", Rate: 1.0},
		{Pattern: "*", Rate: 0.1},
	})
	
	paths := []string{
		"/health",
		"/metrics",
		"/api/v1/users",
		"/api/v2/orders/123/items",
		"/debug/pprof",
		"/random/path",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", paths[i%len(paths)], nil)
		_ = sampler.ShouldSample(req)
	}
}

func BenchmarkStructuredErrors(b *testing.B) {
	b.Run("CreateError", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			err := NewValidationError("validation failed", map[string]string{
				"field": "required",
			})
			_ = err
		}
	})
	
	b.Run("CreateErrorWithPooling", func(b *testing.B) {
		oldPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = oldPooling }()
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			err := NewValidationError("validation failed", map[string]string{
				"field": "required",
			})
			putError(err)
		}
	})
}

func BenchmarkPathMatching(b *testing.B) {
	patterns := []struct {
		name    string
		pattern string
		path    string
	}{
		{"Simple", "/api/users", "/api/users"},
		{"SingleStar", "/api/*/items", "/api/123/items"},
		{"DoubleStar", "/api/**/items", "/api/v1/orders/123/items"},
		{"Escaped", "/api/\\{id\\}", "/api/{id}"},
	}
	
	for _, p := range patterns {
		b.Run(p.name, func(b *testing.B) {
			sampler := NewDynamicPathSampler([]PathSamplingRule{
				{Pattern: p.pattern, Rate: 1.0},
			})
			req := httptest.NewRequest("GET", p.path, nil)
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_ = sampler.ShouldSample(req)
			}
		})
	}
}
