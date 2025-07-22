package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	fmt.Println("=== Async Sink Examples ===")

	// Example 1: Basic async logging
	fmt.Println("1. Basic Async Logging")
	demoBasicAsync()

	// Example 2: Async with batching
	fmt.Println("\n2. Async with Batching")
	demoAsyncBatching()

	// Example 3: Overflow strategies
	fmt.Println("\n3. Overflow Strategies")
	demoOverflowStrategies()

	// Example 4: Production configuration
	fmt.Println("\n4. Production Configuration")
	demoProductionAsync()

	// Example 5: Performance comparison
	fmt.Println("\n5. Performance Comparison")
	demoPerformanceComparison()
}

func demoBasicAsync() {
	// Create a console sink
	consoleSink := sinks.NewConsoleSink()

	// Wrap it with async for non-blocking logging
	asyncSink := sinks.NewAsyncSink(consoleSink, sinks.AsyncOptions{
		BufferSize: 100,
	})
	defer asyncSink.Close()

	logger := mtlog.New(mtlog.WithSink(asyncSink))

	// Log events - returns immediately
	start := time.Now()
	for i := 0; i < 10; i++ {
		logger.Information("Async log message {Number}", i)
	}
	fmt.Printf("Logged 10 messages in %v (non-blocking)\n", time.Since(start))

	// Wait for processing
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	asyncSink.WaitForEmpty(ctx)
}

func demoAsyncBatching() {
	// Create logs directory
	os.MkdirAll("logs", 0755)
	
	// Create a file sink for better batching demo
	fileSink, err := sinks.NewFileSink("logs/async-batch.log")
	if err != nil {
		fmt.Printf("Failed to create file sink: %v\n", err)
		return
	}

	// Wrap with async sink that batches events
	asyncSink := sinks.NewAsyncSink(fileSink, sinks.AsyncOptions{
		BufferSize:    1000,
		BatchSize:     50,      // Batch up to 50 events
		FlushInterval: 100 * time.Millisecond, // Or flush every 100ms
	})
	defer asyncSink.Close()

	logger := mtlog.New(mtlog.WithSink(asyncSink))

	// Generate many events quickly
	fmt.Println("Generating 200 events...")
	for i := 0; i < 200; i++ {
		logger.Information("Batched event {EventId} at {Timestamp}", i, time.Now().Format("15:04:05.000"))
	}

	// Show metrics
	metrics := asyncSink.GetMetrics()
	fmt.Printf("Async metrics: Processed=%d, Dropped=%d, Buffer=%d/%d\n",
		metrics.Processed, metrics.Dropped, metrics.BufferSize, metrics.BufferCap)
}

func demoOverflowStrategies() {
	// Simulate a slow sink
	slowSink := &slowConsoleSink{delay: 50 * time.Millisecond}

	strategies := []struct {
		name     string
		strategy sinks.OverflowStrategy
	}{
		{"Block", sinks.OverflowBlock},
		{"Drop", sinks.OverflowDrop},
		{"DropOldest", sinks.OverflowDropOldest},
	}

	for _, s := range strategies {
		fmt.Printf("\nTesting %s strategy:\n", s.name)

		asyncSink := sinks.NewAsyncSink(slowSink, sinks.AsyncOptions{
			BufferSize:       5, // Small buffer
			OverflowStrategy: s.strategy,
			OnError: func(err error) {
				fmt.Printf("Error: %v\n", err)
			},
		})

		logger := mtlog.New(mtlog.WithSink(asyncSink))

		// Try to log 10 events quickly
		start := time.Now()
		for i := 0; i < 10; i++ {
			logger.Information("Event {Number} with {Strategy}", i, s.name)
		}
		elapsed := time.Since(start)

		// Get metrics before closing
		metrics := asyncSink.GetMetrics()
		
		asyncSink.Close()

		fmt.Printf("  Time: %v, Processed: %d, Dropped: %d\n",
			elapsed, metrics.Processed, metrics.Dropped)
	}
}

func demoProductionAsync() {
	// Create a rolling file sink
	fileSink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
		FilePath:        "logs/production.log",
		MaxFileSize:     10 * 1024 * 1024, // 10MB
		RollingInterval: sinks.RollingIntervalDaily,
		RetainFileCount: 30,
	})
	if err != nil {
		fmt.Printf("Failed to create rolling file sink: %v\n", err)
		return
	}

	// Wrap with async for high-performance logging
	asyncSink := sinks.NewAsyncSink(fileSink, sinks.AsyncOptions{
		BufferSize:       10000,              // Large buffer for bursts
		BatchSize:        100,                // Batch writes
		FlushInterval:    50 * time.Millisecond,
		OverflowStrategy: sinks.OverflowDrop, // Drop rather than block in production
		OnError: func(err error) {
			// In production, send to monitoring
			fmt.Fprintf(os.Stderr, "Logging error: %v\n", err)
		},
		ShutdownTimeout: 30 * time.Second, // Allow time for shutdown
	})
	defer asyncSink.Close()

	logger := mtlog.New(mtlog.WithSink(asyncSink))

	// Simulate production logging patterns
	fmt.Println("Simulating production load...")
	
	// Burst of events
	for i := 0; i < 1000; i++ {
		logger.Information("Request {RequestId} processed", fmt.Sprintf("req-%06d", i))
	}

	// Normal operation
	for i := 0; i < 10; i++ {
		logger.Information("Health check passed")
		time.Sleep(100 * time.Millisecond)
	}

	// Error scenario
	for i := 0; i < 5; i++ {
		logger.Error("Database connection failed: {Error}", "timeout after 30s")
	}

	// Show final metrics
	metrics := asyncSink.GetMetrics()
	fmt.Printf("\nFinal metrics:\n")
	fmt.Printf("  Processed: %d events\n", metrics.Processed)
	fmt.Printf("  Dropped: %d events\n", metrics.Dropped)
	fmt.Printf("  Errors: %d\n", metrics.Errors)
	fmt.Printf("  Buffer utilization: %d/%d\n", metrics.BufferSize, metrics.BufferCap)
}

func demoPerformanceComparison() {
	// Create a null sink for pure performance testing
	nullSink := &nullSink{}
	
	// Test direct logging
	directLogger := mtlog.New(mtlog.WithSink(nullSink))
	
	start := time.Now()
	for i := 0; i < 100000; i++ {
		directLogger.Information("Direct log {Number}", i)
	}
	directTime := time.Since(start)
	
	// Test async logging
	asyncSink := sinks.NewAsyncSink(nullSink, sinks.AsyncOptions{
		BufferSize: 10000,
		BatchSize:  100,
	})
	defer asyncSink.Close()
	
	asyncLogger := mtlog.New(mtlog.WithSink(asyncSink))
	
	start = time.Now()
	for i := 0; i < 100000; i++ {
		asyncLogger.Information("Async log {Number}", i)
	}
	asyncTime := time.Since(start)
	
	// Wait for async to finish
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	asyncSink.WaitForEmpty(ctx)
	
	fmt.Printf("\nPerformance comparison (100k events):\n")
	fmt.Printf("  Direct: %v (%.0f events/sec)\n", directTime, 100000/directTime.Seconds())
	fmt.Printf("  Async: %v (%.0f events/sec)\n", asyncTime, 100000/asyncTime.Seconds())
	fmt.Printf("  Speedup: %.1fx\n", directTime.Seconds()/asyncTime.Seconds())
}

// Helper sinks for demos

type slowConsoleSink struct {
	delay time.Duration
}

func (s *slowConsoleSink) Emit(event *core.LogEvent) {
	time.Sleep(s.delay)
	fmt.Printf("[SLOW] %s\n", event.Properties["Number"])
}

func (s *slowConsoleSink) Close() error {
	return nil
}

type nullSink struct{}

func (n *nullSink) Emit(event *core.LogEvent) {
	// Do nothing - for performance testing
}

func (n *nullSink) Close() error {
	return nil
}