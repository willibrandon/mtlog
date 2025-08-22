package sinks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int32

const (
	// CircuitClosed allows all events through (normal operation).
	CircuitClosed CircuitState = iota
	// CircuitOpen blocks all events (failure detected).
	CircuitOpen
	// CircuitHalfOpen allows one test event through.
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerSink wraps a sink with circuit breaker protection.
type CircuitBreakerSink struct {
	wrapped          core.LogEventSink
	name             string
	failureThreshold int
	successThreshold int // Successes needed in half-open to close
	resetTimeout     time.Duration
	
	state        atomic.Int32 // CircuitState
	failures     atomic.Int32
	successes    atomic.Int32
	lastFailTime atomic.Int64 // Unix nano
	
	mu         sync.Mutex // For state transitions
	fallback   core.LogEventSink // Optional fallback sink when circuit is open
	onStateChange func(from, to CircuitState) // Optional callback
}

// CircuitBreakerOptions configures a circuit breaker sink.
type CircuitBreakerOptions struct {
	Name             string
	FailureThreshold int           // Failures before opening circuit
	SuccessThreshold int           // Successes in half-open before closing
	ResetTimeout     time.Duration // Time before trying half-open
	FallbackSink     core.LogEventSink
	OnStateChange    func(from, to CircuitState)
}

// NewCircuitBreakerSink creates a new circuit breaker sink with default options.
func NewCircuitBreakerSink(wrapped core.LogEventSink) *CircuitBreakerSink {
	return NewCircuitBreakerSinkWithOptions(wrapped, CircuitBreakerOptions{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		ResetTimeout:     30 * time.Second,
	})
}

// NewCircuitBreakerSinkWithOptions creates a circuit breaker with custom options.
func NewCircuitBreakerSinkWithOptions(wrapped core.LogEventSink, opts CircuitBreakerOptions) *CircuitBreakerSink {
	if opts.FailureThreshold <= 0 {
		opts.FailureThreshold = 5
	}
	if opts.SuccessThreshold <= 0 {
		opts.SuccessThreshold = 2
	}
	if opts.ResetTimeout <= 0 {
		opts.ResetTimeout = 30 * time.Second
	}
	if opts.Name == "" {
		opts.Name = "circuit-breaker"
	}
	
	cb := &CircuitBreakerSink{
		wrapped:          wrapped,
		name:             opts.Name,
		failureThreshold: opts.FailureThreshold,
		successThreshold: opts.SuccessThreshold,
		resetTimeout:     opts.ResetTimeout,
		fallback:         opts.FallbackSink,
		onStateChange:    opts.OnStateChange,
	}
	
	cb.state.Store(int32(CircuitClosed))
	return cb
}

// Emit sends the event through the circuit breaker.
func (cb *CircuitBreakerSink) Emit(event *core.LogEvent) {
	if event == nil {
		return
	}
	
	state := cb.getState()
	
	switch state {
	case CircuitOpen:
		// Check if we should transition to half-open
		if cb.shouldAttemptReset() {
			cb.transitionToHalfOpen()
			cb.attemptEmit(event)
		} else {
			// Circuit is open - use fallback or drop
			if cb.fallback != nil {
				cb.fallback.Emit(event)
			} else if selflog.IsEnabled() {
				selflog.Printf("[CircuitBreaker:%s] dropping event - circuit open", cb.name)
			}
		}
		
	case CircuitHalfOpen:
		// Try the request
		cb.attemptEmit(event)
		
	case CircuitClosed:
		// Normal operation
		cb.attemptEmit(event)
	}
}

// attemptEmit tries to emit through the wrapped sink and handles success/failure.
func (cb *CircuitBreakerSink) attemptEmit(event *core.LogEvent) {
	// Wrap the emit in a panic recovery
	success := true
	func() {
		defer func() {
			if r := recover(); r != nil {
				success = false
				if selflog.IsEnabled() {
					selflog.Printf("[CircuitBreaker:%s] wrapped sink panicked: %v", cb.name, r)
				}
			}
		}()
		
		// If the sink is HealthCheckable, check its health first
		if hc, ok := cb.wrapped.(HealthCheckable); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := hc.HealthCheck(ctx); err != nil {
				success = false
				if selflog.IsEnabled() {
					selflog.Printf("[CircuitBreaker:%s] health check failed: %v", cb.name, err)
				}
				return
			}
		}
		
		cb.wrapped.Emit(event)
	}()
	
	if success {
		cb.recordSuccess()
	} else {
		cb.recordFailure()
	}
}

// recordSuccess records a successful emission.
func (cb *CircuitBreakerSink) recordSuccess() {
	state := CircuitState(cb.state.Load())
	
	switch state {
	case CircuitHalfOpen:
		successes := cb.successes.Add(1)
		if int(successes) >= cb.successThreshold {
			cb.transitionToClosed()
		}
		
	case CircuitClosed:
		// Reset failure count on success in closed state
		cb.failures.Store(0)
	}
}

// recordFailure records a failed emission.
func (cb *CircuitBreakerSink) recordFailure() {
	cb.lastFailTime.Store(time.Now().UnixNano())
	
	state := CircuitState(cb.state.Load())
	
	switch state {
	case CircuitClosed:
		failures := cb.failures.Add(1)
		if int(failures) >= cb.failureThreshold {
			cb.transitionToOpen()
		}
		
	case CircuitHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.transitionToOpen()
	}
}

// shouldAttemptReset checks if enough time has passed to try half-open.
func (cb *CircuitBreakerSink) shouldAttemptReset() bool {
	lastFail := cb.lastFailTime.Load()
	if lastFail == 0 {
		return false
	}
	
	elapsed := time.Since(time.Unix(0, lastFail))
	return elapsed >= cb.resetTimeout
}

// State transitions

func (cb *CircuitBreakerSink) transitionToOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	oldState := CircuitState(cb.state.Load())
	if oldState != CircuitOpen {
		cb.state.Store(int32(CircuitOpen))
		cb.failures.Store(0)
		cb.successes.Store(0)
		
		if selflog.IsEnabled() {
			selflog.Printf("[CircuitBreaker:%s] circuit opened (was %s)", cb.name, oldState)
		}
		
		if cb.onStateChange != nil {
			cb.onStateChange(oldState, CircuitOpen)
		}
	}
}

func (cb *CircuitBreakerSink) transitionToHalfOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	oldState := CircuitState(cb.state.Load())
	if oldState == CircuitOpen {
		cb.state.Store(int32(CircuitHalfOpen))
		cb.successes.Store(0)
		cb.failures.Store(0)
		
		if selflog.IsEnabled() {
			selflog.Printf("[CircuitBreaker:%s] circuit half-open (was open)", cb.name)
		}
		
		if cb.onStateChange != nil {
			cb.onStateChange(oldState, CircuitHalfOpen)
		}
	}
}

func (cb *CircuitBreakerSink) transitionToClosed() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	oldState := CircuitState(cb.state.Load())
	if oldState != CircuitClosed {
		cb.state.Store(int32(CircuitClosed))
		cb.failures.Store(0)
		cb.successes.Store(0)
		cb.lastFailTime.Store(0)
		
		if selflog.IsEnabled() {
			selflog.Printf("[CircuitBreaker:%s] circuit closed (was %s)", cb.name, oldState)
		}
		
		if cb.onStateChange != nil {
			cb.onStateChange(oldState, CircuitClosed)
		}
	}
}

func (cb *CircuitBreakerSink) getState() CircuitState {
	return CircuitState(cb.state.Load())
}

// GetState returns the current circuit state.
func (cb *CircuitBreakerSink) GetState() CircuitState {
	return cb.getState()
}

// GetStats returns current circuit breaker statistics.
func (cb *CircuitBreakerSink) GetStats() CircuitBreakerStats {
	return CircuitBreakerStats{
		State:        cb.getState(),
		Failures:     cb.failures.Load(),
		Successes:    cb.successes.Load(),
		LastFailTime: time.Unix(0, cb.lastFailTime.Load()),
	}
}

// CircuitBreakerStats contains circuit breaker statistics.
type CircuitBreakerStats struct {
	State        CircuitState
	Failures     int32
	Successes    int32
	LastFailTime time.Time
}

// Close closes the wrapped sink.
func (cb *CircuitBreakerSink) Close() error {
	if closer, ok := cb.wrapped.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// HealthCheck checks the health of the wrapped sink if it supports it.
func (cb *CircuitBreakerSink) HealthCheck(ctx context.Context) error {
	state := cb.getState()
	
	// Report circuit state as part of health
	switch state {
	case CircuitOpen:
		return fmt.Errorf("circuit breaker is open")
	case CircuitHalfOpen:
		// Try to check wrapped sink health
		if hc, ok := cb.wrapped.(HealthCheckable); ok {
			return hc.HealthCheck(ctx)
		}
		return nil
	case CircuitClosed:
		// Check wrapped sink health if possible
		if hc, ok := cb.wrapped.(HealthCheckable); ok {
			return hc.HealthCheck(ctx)
		}
		return nil
	}
	
	return nil
}