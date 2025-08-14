package main

import (
	"context"
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	otelmtlog "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
)

// testConsoleSink is a simple sink for testing
type testConsoleSink struct{}

func (s *testConsoleSink) Emit(event *core.LogEvent) {
	fmt.Printf("[%d] %s\n", int(event.Level), event.MessageTemplate)
}

func (s *testConsoleSink) Close() error {
	return nil
}

func main() {
	fmt.Println("OpenTelemetry Sampling Examples")
	fmt.Println("================================")

	ctx := context.Background()

	// Example 1: Rate-based sampling (10% of events)
	example1RateSampling(ctx)
	
	// Example 2: Counter-based sampling (every 5th event)
	example2CounterSampling(ctx)
	
	// Example 3: Level-based sampling (Warning and above)
	example3LevelSampling(ctx)
	
	// Example 4: Adaptive sampling (maintain 10 events/sec)
	example4AdaptiveSampling(ctx)
	
	// Example 5: Composite sampling (complex rules)
	example5CompositeSampling(ctx)
	
	fmt.Println("\n✅ Sampling examples completed!")
}

func example1RateSampling(ctx context.Context) {
	fmt.Println("\n=== Example 1: Rate Sampling (10%) ===")
	
	// Create OTLP sink with rate sampling
	sink, _ := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint("localhost:4317"),
		otelmtlog.WithOTLPSampling(otelmtlog.NewRateSampler(0.1)), // 10% sampling
	)
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(sink),
		mtlog.WithConsole(),
	)
	
	// Generate 100 events, expect ~10 to be sent to OTLP
	for i := 0; i < 100; i++ {
		logger.Information("Rate sampled event {Index}", i)
	}
	
	fmt.Println("Generated 100 events with 10% sampling")
}

func example2CounterSampling(ctx context.Context) {
	fmt.Println("\n=== Example 2: Counter Sampling (every 5th) ===")
	
	// Create a sampling sink wrapper  
	consoleSink := &testConsoleSink{}
	
	samplingSink := otelmtlog.NewSamplingSink(
		consoleSink,
		otelmtlog.NewCounterSampler(5), // Every 5th event
	)
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(samplingSink),
		mtlog.WithConsole(),
	)
	
	// Generate 20 events, expect 4 to be sent (5th, 10th, 15th, 20th)
	for i := 1; i <= 20; i++ {
		logger.Information("Counter sampled event {Index}", i)
	}
	
	sampled, dropped := samplingSink.GetStats()
	fmt.Printf("Sampled: %d, Dropped: %d\n", sampled, dropped)
}

func example3LevelSampling(ctx context.Context) {
	fmt.Println("\n=== Example 3: Level Sampling (Warning+) ===")
	
	// Only sample Warning and above
	consoleSink := &testConsoleSink{}
	
	samplingSink := otelmtlog.NewSamplingSink(
		consoleSink,
		otelmtlog.NewLevelSampler(core.WarningLevel),
	)
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(samplingSink),
		mtlog.WithConsole(),
	)
	
	// These won't be sent to OTLP
	logger.Debug("Debug message - not sampled")
	logger.Information("Info message - not sampled")
	
	// These will be sent to OTLP
	logger.Warning("Warning message - sampled")
	logger.Error("Error message - sampled")
	
	sampled, dropped := samplingSink.GetStats()
	fmt.Printf("Sampled: %d, Dropped: %d\n", sampled, dropped)
}

func example4AdaptiveSampling(ctx context.Context) {
	fmt.Println("\n=== Example 4: Adaptive Sampling (10 events/sec) ===")
	
	// Maintain 10 events per second
	consoleSink := &testConsoleSink{}
	
	samplingSink := otelmtlog.NewSamplingSink(
		consoleSink,
		otelmtlog.NewAdaptiveSampler(10), // Target 10 events/sec
	)
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(samplingSink),
		mtlog.WithConsole(),
	)
	
	// Burst of events - sampling rate should decrease
	fmt.Println("High volume burst...")
	for i := 0; i < 100; i++ {
		logger.Information("Burst event {Index}", i)
		time.Sleep(1 * time.Millisecond)
	}
	
	// Wait for adaptive window
	time.Sleep(1100 * time.Millisecond)
	
	// Slow events - sampling rate should increase
	fmt.Println("Low volume period...")
	for i := 0; i < 5; i++ {
		logger.Information("Slow event {Index}", i)
		time.Sleep(200 * time.Millisecond)
	}
	
	sampled, dropped := samplingSink.GetStats()
	fmt.Printf("Total - Sampled: %d, Dropped: %d\n", sampled, dropped)
}

func example5CompositeSampling(ctx context.Context) {
	fmt.Println("\n=== Example 5: Composite Sampling ===")
	
	// Complex sampling: (Warning+ OR (Info AND 20% rate))
	compositeSampler := otelmtlog.NewCompositeSampler(
		otelmtlog.AnyMode,
		otelmtlog.NewLevelSampler(core.WarningLevel), // Always sample warnings+
		otelmtlog.NewCompositeSampler(
			otelmtlog.AllMode,
			otelmtlog.NewLevelSampler(core.InformationLevel), // Info level
			otelmtlog.NewRateSampler(0.2),                    // AND 20% rate
		),
	)
	
	consoleSink := &testConsoleSink{}
	
	samplingSink := otelmtlog.NewSamplingSink(consoleSink, compositeSampler)
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(samplingSink),
		mtlog.WithConsole(),
	)
	
	// Debug - never sampled
	for i := 0; i < 10; i++ {
		logger.Debug("Debug {Index} - never sampled", i)
	}
	
	// Info - sampled at 20%
	for i := 0; i < 50; i++ {
		logger.Information("Info {Index} - 20% sampled", i)
	}
	
	// Warning - always sampled
	for i := 0; i < 5; i++ {
		logger.Warning("Warning {Index} - always sampled", i)
	}
	
	sampled, dropped := samplingSink.GetStats()
	fmt.Printf("Sampled: %d, Dropped: %d\n", sampled, dropped)
	fmt.Println("Expected: ~10 info (20% of 50) + 5 warnings = ~15 sampled")
}