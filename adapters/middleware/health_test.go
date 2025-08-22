package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func TestHealthCheckHandler(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	handler := NewHealthCheckHandler(logger).
		WithVersion("1.0.0").
		WithEnvironment("test").
		WithMetrics(true)
	
	// Add some checks
	handler.AddCheck("database", func() Check {
		return Check{
			Status:  "healthy",
			Message: "Database is OK",
		}
	})
	
	handler.AddCheck("cache", func() Check {
		return Check{
			Status:  "unhealthy",
			Message: "Cache is down",
			Error:   "connection refused",
		}
	})
	
	// Simulate some requests and errors
	handler.IncrementRequestCount()
	handler.IncrementRequestCount()
	handler.IncrementErrorCount()
	
	// Test the health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %v, want %v (unhealthy due to cache)", rec.Code, http.StatusServiceUnavailable)
	}
	
	// Parse response
	var health HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	// Verify basic fields
	if health.Status != "unhealthy" {
		t.Errorf("Status = %v, want unhealthy", health.Status)
	}
	if health.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", health.Version)
	}
	if health.Environment != "test" {
		t.Errorf("Environment = %v, want test", health.Environment)
	}
	
	// Verify checks
	if len(health.Checks) != 2 {
		t.Errorf("Checks count = %v, want 2", len(health.Checks))
	}
	if health.Checks["database"].Status != "healthy" {
		t.Errorf("Database status = %v, want healthy", health.Checks["database"].Status)
	}
	if health.Checks["cache"].Status != "unhealthy" {
		t.Errorf("Cache status = %v, want unhealthy", health.Checks["cache"].Status)
	}
	
	// Verify metrics
	if health.Metrics == nil {
		t.Fatal("Metrics should be included")
	}
	if health.Metrics.RequestsTotal != 2 {
		t.Errorf("RequestsTotal = %v, want 2", health.Metrics.RequestsTotal)
	}
	if health.Metrics.ErrorRate != 0.5 {
		t.Errorf("ErrorRate = %v, want 0.5", health.Metrics.ErrorRate)
	}
}

func TestHealthCheckHandlerAllHealthy(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	
	handler := NewHealthCheckHandler(logger).
		AddCheck("service1", func() Check {
			return Check{Status: "healthy"}
		}).
		AddCheck("service2", func() Check {
			return Check{Status: "ok"}
		})
	
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("StatusCode = %v, want %v", rec.Code, http.StatusOK)
	}
	
	var health HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if health.Status != "healthy" {
		t.Errorf("Status = %v, want healthy", health.Status)
	}
}

func TestLivenessHandler(t *testing.T) {
	handler := LivenessHandler()
	
	req := httptest.NewRequest("GET", "/liveness", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("StatusCode = %v, want %v", rec.Code, http.StatusOK)
	}
	
	if rec.Body.String() != "OK" {
		t.Errorf("Body = %v, want OK", rec.Body.String())
	}
	
	if rec.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type = %v, want text/plain", rec.Header().Get("Content-Type"))
	}
}

func TestReadinessHandler(t *testing.T) {
	t.Run("Ready", func(t *testing.T) {
		handler := ReadinessHandler(
			func() Check { return Check{Status: "healthy"} },
			func() Check { return Check{Status: "ok"} },
		)
		
		req := httptest.NewRequest("GET", "/readiness", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusOK {
			t.Errorf("StatusCode = %v, want %v", rec.Code, http.StatusOK)
		}
		
		var result map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&result)
		
		if result["status"] != "ready" {
			t.Errorf("Status = %v, want ready", result["status"])
		}
	})
	
	t.Run("NotReady", func(t *testing.T) {
		handler := ReadinessHandler(
			func() Check { return Check{Status: "healthy"} },
			func() Check { return Check{Status: "unhealthy", Error: "service down"} },
		)
		
		req := httptest.NewRequest("GET", "/readiness", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("StatusCode = %v, want %v", rec.Code, http.StatusServiceUnavailable)
		}
		
		var result map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&result)
		
		if result["status"] != "not ready" {
			t.Errorf("Status = %v, want 'not ready'", result["status"])
		}
	})
}

func TestDatabaseHealthChecker(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		checker := DatabaseHealthChecker("postgres", func() error {
			return nil
		})
		
		check := checker()
		
		if check.Status != "healthy" {
			t.Errorf("Status = %v, want healthy", check.Status)
		}
		if check.Message != "postgres connection ok" {
			t.Errorf("Message = %v, want 'postgres connection ok'", check.Message)
		}
		if check.Error != "" {
			t.Errorf("Error = %v, want empty", check.Error)
		}
	})
	
	t.Run("Unhealthy", func(t *testing.T) {
		checker := DatabaseHealthChecker("mysql", func() error {
			return errors.New("connection refused")
		})
		
		check := checker()
		
		if check.Status != "unhealthy" {
			t.Errorf("Status = %v, want unhealthy", check.Status)
		}
		if check.Message != "mysql connection failed" {
			t.Errorf("Message = %v, want 'mysql connection failed'", check.Message)
		}
		if check.Error != "connection refused" {
			t.Errorf("Error = %v, want 'connection refused'", check.Error)
		}
	})
}

func TestHTTPHealthChecker(t *testing.T) {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthy" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer testServer.Close()
	
	t.Run("Healthy", func(t *testing.T) {
		checker := HTTPHealthChecker("api", testServer.URL+"/healthy", 5*time.Second)
		check := checker()
		
		if check.Status != "healthy" {
			t.Errorf("Status = %v, want healthy", check.Status)
		}
	})
	
	t.Run("Unhealthy", func(t *testing.T) {
		checker := HTTPHealthChecker("api", testServer.URL+"/unhealthy", 5*time.Second)
		check := checker()
		
		if check.Status != "unhealthy" {
			t.Errorf("Status = %v, want unhealthy", check.Status)
		}
		if !contains(check.Message, "500") {
			t.Errorf("Message should contain status code 500: %v", check.Message)
		}
	})
	
	t.Run("Unreachable", func(t *testing.T) {
		checker := HTTPHealthChecker("api", "http://localhost:99999/health", 1*time.Second)
		check := checker()
		
		if check.Status != "unhealthy" {
			t.Errorf("Status = %v, want unhealthy", check.Status)
		}
		if check.Error == "" {
			t.Error("Error should not be empty for unreachable service")
		}
	})
}

func TestHealthCheckConcurrency(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	handler := NewHealthCheckHandler(logger).WithMetrics(true)
	
	// Add a slow check
	var checkCount int32
	handler.AddCheck("slow", func() Check {
		atomic.AddInt32(&checkCount, 1)
		time.Sleep(10 * time.Millisecond)
		return Check{Status: "healthy"}
	})
	
	// Run concurrent health checks
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}()
	}
	
	wg.Wait()
	
	// Verify check was called 10 times
	if count := atomic.LoadInt32(&checkCount); count != 10 {
		t.Errorf("Check count = %v, want 10", count)
	}
}

func TestHealthCheckHeaders(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	handler := NewHealthCheckHandler(logger)
	
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	
	handler.ServeHTTP(rec, req)
	
	// Check cache headers
	headers := rec.Header()
	if headers.Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
		t.Error("Cache-Control header not set correctly")
	}
	if headers.Get("Pragma") != "no-cache" {
		t.Error("Pragma header not set correctly")
	}
	if headers.Get("Expires") != "0" {
		t.Error("Expires header not set correctly")
	}
	if headers.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header not set correctly")
	}
	if headers.Get("X-Uptime") == "" {
		t.Error("X-Uptime header should be set")
	}
}

func TestHealthMetricsCalculation(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	handler := NewHealthCheckHandler(logger).WithMetrics(true)
	
	// Simulate traffic
	for i := 0; i < 100; i++ {
		handler.IncrementRequestCount()
		if i%4 == 0 { // 25% error rate
			handler.IncrementErrorCount()
		}
	}
	
	// Wait a bit to have measurable uptime
	time.Sleep(100 * time.Millisecond)
	
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	var health HealthStatus
	json.NewDecoder(rec.Body).Decode(&health)
	
	if health.Metrics.RequestsTotal != 100 {
		t.Errorf("RequestsTotal = %v, want 100", health.Metrics.RequestsTotal)
	}
	
	if health.Metrics.ErrorRate != 0.25 {
		t.Errorf("ErrorRate = %v, want 0.25", health.Metrics.ErrorRate)
	}
	
	if health.Metrics.RequestsPerSecond <= 0 {
		t.Error("RequestsPerSecond should be positive")
	}
	
	if health.Metrics.GoroutineCount <= 0 {
		t.Error("GoroutineCount should be positive")
	}
}

func TestHealthCheckWithoutMetrics(t *testing.T) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	handler := NewHealthCheckHandler(logger).WithMetrics(false)
	
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	var health HealthStatus
	json.NewDecoder(rec.Body).Decode(&health)
	
	if health.Metrics != nil {
		t.Error("Metrics should be nil when disabled")
	}
}

func TestDiskSpaceHealthChecker(t *testing.T) {
	// This is mostly a placeholder test since the actual implementation
	// would need system calls
	checker := DiskSpaceHealthChecker(10.0)
	check := checker()
	
	if check.Status != "healthy" {
		t.Errorf("Status = %v, want healthy (placeholder test)", check.Status)
	}
}

func BenchmarkHealthCheck(b *testing.B) {
	logger := mtlog.New(mtlog.WithSink(&discardSink{}))
	handler := NewHealthCheckHandler(logger).
		WithMetrics(true).
		AddCheck("db", func() Check {
			return Check{Status: "healthy"}
		}).
		AddCheck("cache", func() Check {
			return Check{Status: "healthy"}
		})
	
	req := httptest.NewRequest("GET", "/health", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// discardSink for testing
type discardSink struct{}

func (d *discardSink) Emit(event *core.LogEvent) {}
func (d *discardSink) Close() error { return nil }