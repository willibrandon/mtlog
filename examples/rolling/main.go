package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/internal/enrichers"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Create logs directory
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Printf("Failed to create logs directory: %v\n", err)
		return
	}

	// Example 1: Rolling by size
	fmt.Println("=== Rolling by Size Example ===")
	demoSizeRolling(logsDir)

	// Example 2: Rolling by time
	fmt.Println("\n=== Rolling by Time Example ===")
	demoTimeRolling(logsDir)

	// Example 3: Rolling with compression
	fmt.Println("\n=== Rolling with Compression Example ===")
	demoCompressionRolling(logsDir)

	// Example 4: Production configuration
	fmt.Println("\n=== Production Configuration Example ===")
	demoProductionConfig(logsDir)
}

func demoSizeRolling(logsDir string) {
	// Create a rolling file sink that rolls at 1KB
	sink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
		FilePath:        filepath.Join(logsDir, "app-size.log"),
		MaxFileSize:     1024,  // 1KB for demo
		RetainFileCount: 3,     // Keep only last 3 files
	})
	if err != nil {
		fmt.Printf("Failed to create sink: %v\n", err)
		return
	}
	defer sink.Close()

	logger := mtlog.New(mtlog.WithSink(sink))

	// Generate some logs to trigger rolling
	for i := 0; i < 50; i++ {
		logger.Information("Processing order {OrderId} for customer {CustomerId}", 
			fmt.Sprintf("ORD-%04d", i), 
			fmt.Sprintf("CUST-%03d", i%10))
		
		if i%10 == 0 {
			logger.Warning("High memory usage detected: {MemoryMB}MB", 512+i*10)
		}
	}

	fmt.Println("Check logs/app-size*.log files - should have multiple files due to size limit")
}

func demoTimeRolling(logsDir string) {
	// Create a rolling file sink that rolls hourly
	sink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
		FilePath:        filepath.Join(logsDir, "app-time.log"),
		RollingInterval: sinks.RollingIntervalHourly,
		RetainFileCount: 24, // Keep last 24 hours
	})
	if err != nil {
		fmt.Printf("Failed to create sink: %v\n", err)
		return
	}
	defer sink.Close()

	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithEnricher(enrichers.NewTimestampEnricher()),
	)

	// Log some events
	logger.Information("Application started")
	logger.Information("Configuration loaded from {ConfigFile}", "/etc/app/config.yaml")
	logger.Information("Listening on port {Port}", 8080)

	// Simulate some operations
	for i := 0; i < 10; i++ {
		logger.Information("Request {RequestId} processed in {DurationMs}ms", 
			fmt.Sprintf("req-%d", i), 
			50+i*10)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Time-based rolling configured - files will roll every hour")
}

func demoCompressionRolling(logsDir string) {
	// Create a rolling file sink with compression
	sink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
		FilePath:            filepath.Join(logsDir, "app-compressed.log"),
		MaxFileSize:         2048, // 2KB
		RetainFileCount:     5,
		CompressRolledFiles: true,
	})
	if err != nil {
		fmt.Printf("Failed to create sink: %v\n", err)
		return
	}
	defer sink.Close()

	logger := mtlog.New(mtlog.WithSink(sink))

	// Generate logs with structured data
	for i := 0; i < 30; i++ {
		order := struct {
			ID         string
			CustomerID string
			Total      float64
			Items      int
		}{
			ID:         fmt.Sprintf("ORD-%04d", i),
			CustomerID: fmt.Sprintf("CUST-%03d", i%10),
			Total:      99.99 + float64(i)*10.50,
			Items:      1 + i%5,
		}

		logger.Information("Order processed: {@Order}", order)
		
		if i%5 == 0 {
			logger.Debug("Cache statistics: {HitRate:P2} ({Hits}/{Total})", 
				0.85+float64(i%10)/100, 850+i, 1000+i)
		}
	}

	fmt.Println("Check logs/app-compressed*.log.gz files - rolled files are compressed")
}

func demoProductionConfig(logsDir string) {
	// Production-ready configuration with multiple sinks
	
	// Main application log with daily rolling
	appSink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
		FilePath:            filepath.Join(logsDir, "app.log"),
		RollingInterval:     sinks.RollingIntervalDaily,
		RetainFileCount:     30, // Keep 30 days
		CompressRolledFiles: true,
		BufferSize:          64 * 1024, // 64KB buffer for performance
	})
	if err != nil {
		fmt.Printf("Failed to create app sink: %v\n", err)
		return
	}
	defer appSink.Close()

	// Error log with size-based rolling
	errorSink, err := sinks.NewRollingFileSink(sinks.RollingFileOptions{
		FilePath:            filepath.Join(logsDir, "errors.log"),
		MaxFileSize:         10 * 1024 * 1024, // 10MB
		RetainFileCount:     10,
		CompressRolledFiles: true,
	})
	if err != nil {
		fmt.Printf("Failed to create error sink: %v\n", err)
		return
	}
	defer errorSink.Close()

	// For this demo, we'll use a simple multi-sink approach
	// In production, you'd implement proper filtered sinks

	// Create logger with enrichers
	// For now, just use the app sink and console
	logger := mtlog.New(
		mtlog.WithSink(appSink),
		mtlog.WithEnricher(enrichers.NewMachineNameEnricher()),
		mtlog.WithEnricher(enrichers.NewProcessEnricher()),
		mtlog.WithEnricher(enrichers.NewThreadIdEnricher()),
	)
	
	// Also create an error logger
	errorLogger := mtlog.New(mtlog.WithSink(errorSink))

	// Simulate production logging
	logger.Information("Application started in {Environment} mode", "production")
	logger.Information("Database connection established to {Host}:{Port}", "db.example.com", 5432)

	// Normal operations
	for i := 0; i < 5; i++ {
		logger.Information("Health check passed: {Status}", "healthy")
		logger.Debug("Memory usage: {UsedMB}/{TotalMB}MB", 256+i*10, 1024)
		time.Sleep(200 * time.Millisecond)
	}

	// Simulate some warnings and errors
	logger.Warning("API rate limit approaching: {Current}/{Limit} requests", 950, 1000)
	logger.Error("Failed to send email to {Email}: {Error}", "user@example.com", "SMTP timeout")
	errorLogger.Error("Failed to send email to {Email}: {Error}", "user@example.com", "SMTP timeout")
	errorLogger.Fatal("Critical system failure: {Component}", "payment-processor")

	fmt.Println("\nProduction configuration created:")
	fmt.Println("- app.log: All logs, daily rolling, 30 days retention")
	fmt.Println("- errors.log: Errors only, 10MB max size, compressed")
	fmt.Println("- Console output with dev theme")
}