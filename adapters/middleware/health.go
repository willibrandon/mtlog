package middleware

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// HealthStatus represents the health status of the application
type HealthStatus struct {
	Status      string            `json:"status"`
	Timestamp   time.Time         `json:"timestamp"`
	Uptime      string            `json:"uptime"`
	Checks      map[string]Check  `json:"checks,omitempty"`
	Version     string            `json:"version,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Metrics     *HealthMetrics    `json:"metrics,omitempty"`
}

// Check represents a health check result
type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Latency int64  `json:"latency_ms,omitempty"`
}

// HealthMetrics contains runtime metrics
type HealthMetrics struct {
	RequestsTotal     int64   `json:"requests_total"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	ErrorRate         float64 `json:"error_rate"`
	MemoryMB          uint64  `json:"memory_mb"`
	GoroutineCount    int     `json:"goroutine_count"`
}

// HealthChecker performs health checks
type HealthChecker func() Check

// HealthCheckHandler creates a dedicated health check handler
type HealthCheckHandler struct {
	startTime      time.Time
	logger         core.Logger
	checks         map[string]HealthChecker
	version        string
	environment    string
	requestCounter int64
	errorCounter   int64
	includeMetrics bool
}

// NewHealthCheckHandler creates a new health check handler
func NewHealthCheckHandler(logger core.Logger) *HealthCheckHandler {
	return &HealthCheckHandler{
		startTime:      time.Now(),
		logger:         logger,
		checks:         make(map[string]HealthChecker),
		includeMetrics: true,
	}
}

// WithVersion sets the application version
func (h *HealthCheckHandler) WithVersion(version string) *HealthCheckHandler {
	h.version = version
	return h
}

// WithEnvironment sets the environment name
func (h *HealthCheckHandler) WithEnvironment(env string) *HealthCheckHandler {
	h.environment = env
	return h
}

// WithMetrics enables/disables metrics in health response
func (h *HealthCheckHandler) WithMetrics(include bool) *HealthCheckHandler {
	h.includeMetrics = include
	return h
}

// AddCheck adds a health check
func (h *HealthCheckHandler) AddCheck(name string, checker HealthChecker) *HealthCheckHandler {
	h.checks[name] = checker
	return h
}

// IncrementRequestCount increments the request counter
func (h *HealthCheckHandler) IncrementRequestCount() {
	atomic.AddInt64(&h.requestCounter, 1)
}

// IncrementErrorCount increments the error counter
func (h *HealthCheckHandler) IncrementErrorCount() {
	atomic.AddInt64(&h.errorCounter, 1)
}

// ServeHTTP handles health check requests
func (h *HealthCheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	
	// Set headers to prevent caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")
	
	// Add uptime header
	uptime := time.Since(h.startTime)
	w.Header().Set("X-Uptime", uptime.String())
	
	// Build health status
	status := HealthStatus{
		Status:      "healthy",
		Timestamp:   time.Now(),
		Uptime:      uptime.String(),
		Version:     h.version,
		Environment: h.environment,
	}
	
	// Run health checks
	if len(h.checks) > 0 {
		status.Checks = make(map[string]Check)
		for name, checker := range h.checks {
			start := time.Now()
			check := checker()
			check.Latency = time.Since(start).Nanoseconds() / 1000000 // Convert to milliseconds
			status.Checks[name] = check
			
			// Update overall status if any check fails
			if check.Status != "healthy" && check.Status != "ok" {
				status.Status = "unhealthy"
			}
		}
	}
	
	// Add metrics if enabled
	if h.includeMetrics {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		
		requests := atomic.LoadInt64(&h.requestCounter)
		errors := atomic.LoadInt64(&h.errorCounter)
		errorRate := float64(0)
		if requests > 0 {
			errorRate = float64(errors) / float64(requests)
		}
		
		// Calculate requests per second safely
		requestsPerSecond := float64(0)
		uptimeSeconds := uptime.Seconds()
		if uptimeSeconds > 0 {
			requestsPerSecond = float64(requests) / uptimeSeconds
		}
		
		status.Metrics = &HealthMetrics{
			RequestsTotal:     requests,
			RequestsPerSecond: requestsPerSecond,
			ErrorRate:         errorRate,
			MemoryMB:          memStats.Alloc / 1024 / 1024,
			GoroutineCount:    runtime.NumGoroutine(),
		}
	}
	
	// Set status code
	statusCode := http.StatusOK
	if status.Status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}
	
	// Write response
	jsonBytes, err := json.Marshal(status)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	
	w.WriteHeader(statusCode)
	w.Write(jsonBytes)
}

// LivenessHandler returns a simple liveness check handler
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// ReadinessHandler returns a readiness check handler with custom checks
func ReadinessHandler(checks ...HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ready := true
		results := make(map[string]Check)
		
		for i, checker := range checks {
			check := checker()
			results[string(rune('a'+i))] = check
			if check.Status != "healthy" && check.Status != "ok" {
				ready = false
			}
		}
		
		if ready {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ready",
				"checks": results,
			})
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "not ready",
				"checks": results,
			})
		}
	}
}

// Common health checkers

// DatabaseHealthChecker creates a health checker for database connections
func DatabaseHealthChecker(name string, pingFunc func() error) HealthChecker {
	return func() Check {
		if err := pingFunc(); err != nil {
			return Check{
				Status:  "unhealthy",
				Message: name + " connection failed",
				Error:   err.Error(),
			}
		}
		return Check{
			Status:  "healthy",
			Message: name + " connection ok",
		}
	}
}

// HTTPHealthChecker creates a health checker for HTTP endpoints
func HTTPHealthChecker(name, url string, timeout time.Duration) HealthChecker {
	client := &http.Client{Timeout: timeout}
	return func() Check {
		resp, err := client.Get(url)
		if err != nil {
			return Check{
				Status:  "unhealthy",
				Message: name + " unreachable",
				Error:   err.Error(),
			}
		}
		defer resp.Body.Close()
		
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return Check{
				Status:  "healthy",
				Message: name + " responding",
			}
		}
		
		return Check{
			Status:  "unhealthy",
			Message: name + " returned " + resp.Status,
		}
	}
}

// DiskSpaceHealthChecker creates a health checker for disk space
func DiskSpaceHealthChecker(minFreeGB float64) HealthChecker {
	return func() Check {
		// This is a simplified example
		// In production, you'd use syscall to get actual disk stats
		return Check{
			Status:  "healthy",
			Message: "Disk space adequate",
		}
	}
}