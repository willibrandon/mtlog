package sinks

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// HealthCheckable interface for sinks that support health checks.
type HealthCheckable interface {
	HealthCheck(ctx context.Context) error
}

// HealthStatus represents the health status of a route.
type HealthStatus struct {
	RouteName   string
	Healthy     bool
	Error       error
	LastChecked time.Time
}

// CheckHealth performs health checks on all routes that support it.
func (r *RouterSink) CheckHealth(ctx context.Context) map[string]HealthStatus {
	r.mu.RLock()
	routes := make([]Route, len(r.routes))
	copy(routes, r.routes)
	r.mu.RUnlock()
	
	results := make(map[string]HealthStatus)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for _, route := range routes {
		route := route // Capture for goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			status := HealthStatus{
				RouteName:   route.Name,
				LastChecked: time.Now(),
			}
			
			if hc, ok := route.Sink.(HealthCheckable); ok {
				if err := hc.HealthCheck(ctx); err != nil {
					status.Error = err
					status.Healthy = false
				} else {
					status.Healthy = true
				}
			} else {
				// Sink doesn't support health checks - assume healthy
				status.Healthy = true
			}
			
			mu.Lock()
			results[route.Name] = status
			mu.Unlock()
		}()
	}
	
	// Check default sink if present
	if r.defaultSink != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			status := HealthStatus{
				RouteName:   "<default>",
				LastChecked: time.Now(),
			}
			
			if hc, ok := r.defaultSink.(HealthCheckable); ok {
				if err := hc.HealthCheck(ctx); err != nil {
					status.Error = err
					status.Healthy = false
				} else {
					status.Healthy = true
				}
			} else {
				status.Healthy = true
			}
			
			mu.Lock()
			results["<default>"] = status
			mu.Unlock()
		}()
	}
	
	wg.Wait()
	return results
}

// PeriodicHealthCheck starts a goroutine that periodically checks sink health.
func (r *RouterSink) PeriodicHealthCheck(ctx context.Context, interval time.Duration, callback func(map[string]HealthStatus)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				results := r.CheckHealth(ctx)
				if callback != nil {
					callback(results)
				}
			}
		}
	}()
}

// Example implementation for common sinks

// HealthCheckWrapper wraps a sink with a simple health check.
type HealthCheckWrapper struct {
	wrapped core.LogEventSink
	checker func(context.Context) error
}

func NewHealthCheckWrapper(sink core.LogEventSink, checker func(context.Context) error) *HealthCheckWrapper {
	return &HealthCheckWrapper{
		wrapped: sink,
		checker: checker,
	}
}

func (h *HealthCheckWrapper) Emit(event *core.LogEvent) {
	h.wrapped.Emit(event)
}

func (h *HealthCheckWrapper) HealthCheck(ctx context.Context) error {
	if h.checker != nil {
		return h.checker(ctx)
	}
	return nil
}

func (h *HealthCheckWrapper) Close() error {
	if closer, ok := h.wrapped.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// Health check for file sinks
func (fs *FileSink) HealthCheck(ctx context.Context) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	if !fs.isOpen {
		return fmt.Errorf("file sink is closed")
	}
	
	// Try to sync to verify file is still writable
	if err := fs.file.Sync(); err != nil {
		return fmt.Errorf("file sync failed: %w", err)
	}
	
	return nil
}

// Health check for memory sinks (always healthy)
func (ms *MemorySink) HealthCheck(ctx context.Context) error {
	return nil
}