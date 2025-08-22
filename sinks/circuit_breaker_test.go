package sinks

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// FailingSink simulates a sink that can fail on demand
type FailingSink struct {
	shouldFail atomic.Bool
	emitCount  atomic.Int32
}

func (fs *FailingSink) Emit(event *core.LogEvent) {
	fs.emitCount.Add(1)
	if fs.shouldFail.Load() {
		panic("simulated failure")
	}
}

func (fs *FailingSink) Close() error {
	return nil
}

func (fs *FailingSink) HealthCheck(ctx context.Context) error {
	if fs.shouldFail.Load() {
		return fmt.Errorf("sink is unhealthy")
	}
	return nil
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("circuit opens after threshold failures", func(t *testing.T) {
		failingSink := &FailingSink{}
		fallbackSink := NewMemorySink()
		
		cb := NewCircuitBreakerSinkWithOptions(failingSink, CircuitBreakerOptions{
			Name:             "test",
			FailureThreshold: 3,
			SuccessThreshold: 2,
			ResetTimeout:     100 * time.Millisecond,
			FallbackSink:     fallbackSink,
		})
		
		// Make sink fail
		failingSink.shouldFail.Store(true)
		
		// Send events until circuit opens
		for i := 0; i < 5; i++ {
			cb.Emit(&core.LogEvent{Level: core.InformationLevel})
		}
		
		// Circuit should be open
		if cb.GetState() != CircuitOpen {
			t.Errorf("Expected circuit to be open, got %s", cb.GetState())
		}
		
		// Events should go to fallback
		events := fallbackSink.Events()
		if len(events) < 2 {
			t.Errorf("Expected at least 2 events in fallback, got %d", len(events))
		}
	})
	
	t.Run("circuit transitions to half-open after timeout", func(t *testing.T) {
		failingSink := &FailingSink{}
		
		cb := NewCircuitBreakerSinkWithOptions(failingSink, CircuitBreakerOptions{
			Name:             "test",
			FailureThreshold: 2,
			ResetTimeout:     50 * time.Millisecond,
		})
		
		// Open the circuit
		failingSink.shouldFail.Store(true)
		for i := 0; i < 3; i++ {
			cb.Emit(&core.LogEvent{})
		}
		
		if cb.GetState() != CircuitOpen {
			t.Fatal("Circuit should be open")
		}
		
		// Wait for reset timeout
		time.Sleep(60 * time.Millisecond)
		
		// Make sink healthy again
		failingSink.shouldFail.Store(false)
		
		// Next emit should transition to half-open
		cb.Emit(&core.LogEvent{})
		
		// Circuit should be half-open or closed (if success threshold is 1)
		state := cb.GetState()
		if state == CircuitOpen {
			t.Errorf("Circuit should not still be open after timeout")
		}
	})
	
	t.Run("circuit closes after success threshold in half-open", func(t *testing.T) {
		failingSink := &FailingSink{}
		
		cb := NewCircuitBreakerSinkWithOptions(failingSink, CircuitBreakerOptions{
			Name:             "test",
			FailureThreshold: 2,
			SuccessThreshold: 3,
			ResetTimeout:     50 * time.Millisecond,
		})
		
		// Open the circuit
		failingSink.shouldFail.Store(true)
		for i := 0; i < 3; i++ {
			cb.Emit(&core.LogEvent{})
		}
		
		// Wait for reset timeout
		time.Sleep(60 * time.Millisecond)
		
		// Make sink healthy
		failingSink.shouldFail.Store(false)
		
		// Send successful events
		for i := 0; i < 3; i++ {
			cb.Emit(&core.LogEvent{})
		}
		
		// Circuit should be closed
		if cb.GetState() != CircuitClosed {
			t.Errorf("Expected circuit to be closed, got %s", cb.GetState())
		}
	})
	
	t.Run("circuit reopens on failure in half-open", func(t *testing.T) {
		failingSink := &FailingSink{}
		
		cb := NewCircuitBreakerSinkWithOptions(failingSink, CircuitBreakerOptions{
			Name:             "test",
			FailureThreshold: 2,
			SuccessThreshold: 3,
			ResetTimeout:     50 * time.Millisecond,
		})
		
		// Open the circuit
		failingSink.shouldFail.Store(true)
		for i := 0; i < 3; i++ {
			cb.Emit(&core.LogEvent{})
		}
		
		// Wait for reset timeout
		time.Sleep(60 * time.Millisecond)
		
		// First emit transitions to half-open and fails
		cb.Emit(&core.LogEvent{})
		
		// Circuit should be open again
		if cb.GetState() != CircuitOpen {
			t.Errorf("Expected circuit to be open after half-open failure, got %s", cb.GetState())
		}
	})
	
	t.Run("state change callback", func(t *testing.T) {
		failingSink := &FailingSink{}
		var transitions []string
		
		cb := NewCircuitBreakerSinkWithOptions(failingSink, CircuitBreakerOptions{
			Name:             "test",
			FailureThreshold: 2,
			OnStateChange: func(from, to CircuitState) {
				transitions = append(transitions, fmt.Sprintf("%s->%s", from, to))
			},
		})
		
		// Open the circuit
		failingSink.shouldFail.Store(true)
		for i := 0; i < 3; i++ {
			cb.Emit(&core.LogEvent{})
		}
		
		if len(transitions) != 1 || transitions[0] != "closed->open" {
			t.Errorf("Expected closed->open transition, got %v", transitions)
		}
	})
}

func TestCircuitBreakerHealthCheck(t *testing.T) {
	t.Run("reports unhealthy when open", func(t *testing.T) {
		failingSink := &FailingSink{}
		cb := NewCircuitBreakerSinkWithOptions(failingSink, CircuitBreakerOptions{
			FailureThreshold: 1,
		})
		
		// Open the circuit
		failingSink.shouldFail.Store(true)
		cb.Emit(&core.LogEvent{})
		
		// Health check should report unhealthy
		ctx := context.Background()
		err := cb.HealthCheck(ctx)
		if err == nil {
			t.Error("Expected health check to fail when circuit is open")
		}
	})
	
	t.Run("checks wrapped sink health when closed", func(t *testing.T) {
		failingSink := &FailingSink{}
		cb := NewCircuitBreakerSink(failingSink)
		
		// Healthy sink
		ctx := context.Background()
		err := cb.HealthCheck(ctx)
		if err != nil {
			t.Errorf("Expected healthy, got %v", err)
		}
		
		// Make sink unhealthy
		failingSink.shouldFail.Store(true)
		err = cb.HealthCheck(ctx)
		if err == nil {
			t.Error("Expected unhealthy when wrapped sink is unhealthy")
		}
	})
}