package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/sentry"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

func main() {
	// Enable self-diagnostics to see retry attempts
	selflog.Enable(log.Writer())
	defer selflog.Disable()

	// Create Sentry sink with retry configuration
	dsn := "https://your-key@sentry.io/project-id"
	sentrySink, err := sentry.NewSentrySink(dsn,
		sentry.WithEnvironment("development"),
		sentry.WithRelease("v1.0.0"),
		
		// Configure retry with exponential backoff
		sentry.WithRetry(3, 1*time.Second),     // 3 retries, starting at 1 second
		sentry.WithRetryJitter(0.2),            // 20% jitter to prevent thundering herd
		
		// Configure batching for efficiency
		sentry.WithBatchSize(10),
		sentry.WithBatchTimeout(2*time.Second),
		
		// Enable metrics to track retry behavior
		sentry.WithMetrics(true),
		sentry.WithMetricsCallback(5*time.Second, func(m sentry.Metrics) {
			fmt.Printf("\n=== Sentry Metrics ===\n")
			fmt.Printf("Events sent: %d\n", m.EventsSent)
			fmt.Printf("Events failed: %d\n", m.EventsFailed)
			fmt.Printf("Events retried: %d\n", m.EventsRetried)
			fmt.Printf("Retry count: %d\n", m.RetryCount)
			fmt.Printf("Network errors: %d\n", m.NetworkErrors)
			if m.EventsSent > 0 {
				retryRate := float64(m.RetryCount) / float64(m.EventsSent) * 100
				fmt.Printf("Retry rate: %.2f%%\n", retryRate)
			}
			fmt.Printf("===================\n\n")
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create Sentry sink: %v", err)
	}
	defer sentrySink.Close()

	// Create logger with Sentry
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSink(sentrySink),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	// Simulate various network conditions
	fmt.Println("Simulating various error conditions with retry logic...")

	// Transient error that would benefit from retry
	logger.Error("Database connection failed: {Error}", 
		errors.New("connection refused"))

	// Multiple errors in quick succession (will batch)
	for i := 0; i < 5; i++ {
		logger.Error("Request {RequestId} failed with status {Status}",
			fmt.Sprintf("req-%d", i),
			500+i)
		time.Sleep(100 * time.Millisecond)
	}

	// Critical error with context
	logger.Fatal("Critical system failure: {Component} is down for {UserId} in {TenantId}",
		"payment-gateway", "user-123", "tenant-456")

	// Wait for batch to flush
	fmt.Println("\nWaiting for events to be sent with retry logic...")
	time.Sleep(5 * time.Second)

	// Get final metrics
	metrics := sentrySink.Metrics()
	fmt.Printf("\n=== Final Metrics ===\n")
	fmt.Printf("Total events sent: %d\n", metrics.EventsSent)
	fmt.Printf("Total events failed: %d\n", metrics.EventsFailed)
	fmt.Printf("Average batch size: %.2f\n", metrics.AverageBatchSize)
	fmt.Printf("Last flush duration: %v\n", metrics.LastFlushDuration)
	fmt.Printf("Total flush time: %v\n", metrics.TotalFlushTime)
	
	fmt.Println("\nRetry example completed. Check your Sentry dashboard for events.")
}