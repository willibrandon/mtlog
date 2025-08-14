package otel_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mtlogotel "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

// TestHighConcurrencyEnricher tests enrichers under high concurrent load
func TestHighConcurrencyEnricher(t *testing.T) {
	// Setup tracer
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("concurrency-test")

	tests := []struct {
		name           string
		enricherFactory func(context.Context) core.LogEventEnricher
		goroutines     int
		eventsPerRoutine int
	}{
		{
			name: "FastOTELEnricher",
			enricherFactory: func(ctx context.Context) core.LogEventEnricher {
				return mtlogotel.NewFastOTELEnricher(ctx)
			},
			goroutines: 100,
			eventsPerRoutine: 1000,
		},
		{
			name: "StaticOTELEnricher",
			enricherFactory: func(ctx context.Context) core.LogEventEnricher {
				return mtlogotel.NewStaticOTELEnricher(ctx)
			},
			goroutines: 100,
			eventsPerRoutine: 1000,
		},
		{
			name: "CachingOTELEnricher",
			enricherFactory: func(ctx context.Context) core.LogEventEnricher {
				return mtlogotel.NewOTELEnricher(ctx)
			},
			goroutines: 100,
			eventsPerRoutine: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, span := tracer.Start(context.Background(), "test-span")
			defer span.End()

			enricher := tt.enricherFactory(ctx)
			factory := &mockPropertyFactory{}

			var wg sync.WaitGroup
			wg.Add(tt.goroutines)

			var errors atomic.Int64
			var processed atomic.Int64

			start := time.Now()

			for i := 0; i < tt.goroutines; i++ {
				go func(id int) {
					defer wg.Done()
					
					for j := 0; j < tt.eventsPerRoutine; j++ {
						event := &core.LogEvent{
							Properties: make(map[string]any),
						}
						
						// Catch any panics
						func() {
							defer func() {
								if r := recover(); r != nil {
									errors.Add(1)
									t.Errorf("Panic in goroutine %d: %v", id, r)
								}
							}()
							
							enricher.Enrich(event, factory)
							processed.Add(1)
						}()
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(start)

			t.Logf("%s: Processed %d events in %v (%.0f events/sec)",
				tt.name, processed.Load(), duration,
				float64(processed.Load())/duration.Seconds())

			if errors.Load() > 0 {
				t.Errorf("Encountered %d errors during concurrent processing", errors.Load())
			}
		})
	}
}

// TestConcurrentSinkOperations tests concurrent operations on the OTLP sink
func TestConcurrentSinkOperations(t *testing.T) {
	sink, err := mtlogotel.NewOTLPSink(
		mtlogotel.WithOTLPEndpoint("localhost:4317"),
		mtlogotel.WithOTLPInsecure(), // Use insecure connection for testing
		mtlogotel.WithOTLPBatching(50, 100*time.Millisecond),
		mtlogotel.WithOTLPMaxQueueSize(10000),
		mtlogotel.WithOTLPTimeout(1*time.Second), // Short timeout for tests
		mtlogotel.WithOTLPRetry(100*time.Millisecond, 1*time.Second), // Minimal retry
	)
	if err != nil {
		t.Fatalf("Failed to create sink: %v", err)
	}
	defer sink.Close()

	const (
		writers  = 50
		flushers = 10
		readers  = 5
		duration = 5 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	var totalEvents atomic.Int64
	var flushCount atomic.Int64
	var metricsCount atomic.Int64
	var healthChecks atomic.Int64

	// Start writers
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					event := &core.LogEvent{
						Timestamp:       time.Now(),
						Level:           core.InformationLevel,
						MessageTemplate: "Event from writer {Writer}",
						Properties: map[string]any{
							"Writer": id,
						},
					}
					sink.Emit(event)
					totalEvents.Add(1)
					runtime.Gosched() // Yield to increase contention
				}
			}
		}(i)
	}

	// Start flushers
	wg.Add(flushers)
	for i := 0; i < flushers; i++ {
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = sink.Flush()
					flushCount.Add(1)
				}
			}
		}(i)
	}

	// Start metrics readers
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = sink.GetMetrics()
					metricsCount.Add(1)
				}
			}
		}(i)
	}

	// Start health checkers
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					checkCtx, checkCancel := context.WithTimeout(context.Background(), 1*time.Second)
					_ = sink.HealthCheck(checkCtx)
					checkCancel()
					healthChecks.Add(1)
				}
			}
		}(i)
	}

	// Wait for completion
	wg.Wait()

	// Final metrics
	metrics := sink.GetMetrics()
	t.Logf("Concurrent test results:")
	t.Logf("  Total events sent: %d", totalEvents.Load())
	t.Logf("  Events exported: %d", metrics["exported"])
	t.Logf("  Events dropped: %d", metrics["dropped"])
	t.Logf("  Errors: %d", metrics["errors"])
	t.Logf("  Flush operations: %d", flushCount.Load())
	t.Logf("  Metrics reads: %d", metricsCount.Load())
	t.Logf("  Health checks: %d", healthChecks.Load())
}

// TestRaceConditionRegression tests specific race conditions that were fixed
func TestRaceConditionRegression(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping race condition regression test in short mode")
	}

	// Test the timer race condition fix
	t.Run("TimerRace", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			sink, err := mtlogotel.NewOTLPSink(
				mtlogotel.WithOTLPEndpoint("localhost:4317"),
				mtlogotel.WithOTLPInsecure(), // Use insecure connection for testing
				mtlogotel.WithOTLPBatching(10, 1*time.Millisecond),
				mtlogotel.WithOTLPTimeout(500*time.Millisecond), // Short timeout for tests
				mtlogotel.WithOTLPRetry(100*time.Millisecond, 500*time.Millisecond), // Minimal retry
			)
			if err != nil {
				t.Fatalf("Failed to create sink: %v", err)
			}

			var wg sync.WaitGroup
			wg.Add(2)

			// Goroutine 1: Rapidly send events
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					event := &core.LogEvent{
						Timestamp:       time.Now(),
						Level:           core.InformationLevel,
						MessageTemplate: "Event",
						Properties:      map[string]any{},
					}
					sink.Emit(event)
				}
			}()

			// Goroutine 2: Rapidly flush
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					_ = sink.Flush()
					time.Sleep(time.Microsecond)
				}
			}()

			wg.Wait()
			sink.Close()
		}
	})

	// Test the atomic.Bool cache fix in enricher
	t.Run("EnricherCacheRace", func(t *testing.T) {
		tp := trace.NewTracerProvider()
		otel.SetTracerProvider(tp)
		tracer := otel.Tracer("race-test")

		for i := 0; i < 100; i++ {
			ctx, span := tracer.Start(context.Background(), "test")
			enricher := mtlogotel.NewOTELEnricher(ctx)
			factory := &mockPropertyFactory{}

			var wg sync.WaitGroup
			wg.Add(10)

			for j := 0; j < 10; j++ {
				go func() {
					defer wg.Done()
					for k := 0; k < 100; k++ {
						event := &core.LogEvent{
							Properties: make(map[string]any),
						}
						enricher.Enrich(event, factory)
					}
				}()
			}

			wg.Wait()
			span.End()
		}
	})
}

// BenchmarkConcurrentEnrichment benchmarks enrichment under concurrent load
func BenchmarkConcurrentEnrichment(b *testing.B) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("bench")
	
	ctx, span := tracer.Start(context.Background(), "benchmark")
	defer span.End()

	enrichers := []struct {
		name string
		enricher core.LogEventEnricher
	}{
		{"Fast", mtlogotel.NewFastOTELEnricher(ctx)},
		{"Static", mtlogotel.NewStaticOTELEnricher(ctx)},
		{"Caching", mtlogotel.NewOTELEnricher(ctx)},
	}

	for _, e := range enrichers {
		b.Run(e.name, func(b *testing.B) {
			factory := &mockPropertyFactory{}
			
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					event := &core.LogEvent{
						Properties: make(map[string]any),
					}
					e.enricher.Enrich(event, factory)
				}
			})
		})
	}
}