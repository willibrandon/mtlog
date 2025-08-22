package sinks

import (
	"context"
	"fmt"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// HealthyTestSink always reports healthy
type HealthyTestSink struct {
	MemorySink
}

func (h *HealthyTestSink) HealthCheck(ctx context.Context) error {
	return nil
}

// UnhealthyTestSink always reports unhealthy
type UnhealthyTestSink struct {
	MemorySink
	err error
}

func (u *UnhealthyTestSink) HealthCheck(ctx context.Context) error {
	if u.err != nil {
		return u.err
	}
	return fmt.Errorf("sink is unhealthy")
}

func TestRouterHealthCheck(t *testing.T) {
	t.Run("checks health of all routes", func(t *testing.T) {
		healthySink := &HealthyTestSink{}
		unhealthySink := &UnhealthyTestSink{err: fmt.Errorf("database connection failed")}
		
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "healthy",
				Predicate: LevelPredicate(core.ErrorLevel),
				Sink:      healthySink,
			},
			Route{
				Name:      "unhealthy",
				Predicate: LevelPredicate(core.WarningLevel),
				Sink:      unhealthySink,
			},
			Route{
				Name:      "no-health-check",
				Predicate: LevelPredicate(core.InformationLevel),
				Sink:      NewMemorySink(), // Doesn't implement HealthCheckable
			},
		)
		
		ctx := context.Background()
		results := router.CheckHealth(ctx)
		
		// Should have results for all routes
		if len(results) != 3 {
			t.Errorf("Expected 3 health results, got %d", len(results))
		}
		
		// Check individual results
		if result, ok := results["healthy"]; !ok || !result.Healthy {
			t.Error("Expected healthy route to be healthy")
		}
		
		if result, ok := results["unhealthy"]; !ok || result.Healthy {
			t.Error("Expected unhealthy route to be unhealthy")
		}
		
		if result, ok := results["unhealthy"]; ok && result.Error == nil {
			t.Error("Expected unhealthy route to have error")
		}
		
		// Route without health check should be assumed healthy
		if result, ok := results["no-health-check"]; !ok || !result.Healthy {
			t.Error("Expected route without health check to be assumed healthy")
		}
	})
	
	t.Run("checks default sink health", func(t *testing.T) {
		defaultSink := &UnhealthyTestSink{err: fmt.Errorf("default sink error")}
		router := NewRouterSinkWithDefault(FirstMatch, defaultSink)
		
		ctx := context.Background()
		results := router.CheckHealth(ctx)
		
		if result, ok := results["<default>"]; !ok {
			t.Error("Expected default sink health result")
		} else if result.Healthy {
			t.Error("Expected default sink to be unhealthy")
		}
	})
	
	t.Run("periodic health check calls callback", func(t *testing.T) {
		healthySink := &HealthyTestSink{}
		router := NewRouterSink(FirstMatch,
			Route{
				Name:      "test",
				Predicate: func(*core.LogEvent) bool { return true },
				Sink:      healthySink,
			},
		)
		
		callbackCalled := make(chan bool, 1)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		router.PeriodicHealthCheck(ctx, 50*time.Millisecond, func(results map[string]HealthStatus) {
			if len(results) > 0 {
				callbackCalled <- true
			}
		})
		
		select {
		case <-callbackCalled:
			// Success
		case <-time.After(200 * time.Millisecond):
			t.Error("Callback not called within timeout")
		}
	})
}

func TestHealthCheckWrapper(t *testing.T) {
	t.Run("wraps sink with custom health check", func(t *testing.T) {
		memSink := NewMemorySink()
		healthy := true
		
		wrapped := NewHealthCheckWrapper(memSink, func(ctx context.Context) error {
			if !healthy {
				return fmt.Errorf("custom check failed")
			}
			return nil
		})
		
		// Test emit works
		wrapped.Emit(&core.LogEvent{Level: core.InformationLevel})
		if len(memSink.Events()) != 1 {
			t.Error("Emit not forwarded to wrapped sink")
		}
		
		// Test health check
		ctx := context.Background()
		if err := wrapped.HealthCheck(ctx); err != nil {
			t.Errorf("Expected healthy, got %v", err)
		}
		
		// Make unhealthy
		healthy = false
		if err := wrapped.HealthCheck(ctx); err == nil {
			t.Error("Expected unhealthy")
		}
	})
}