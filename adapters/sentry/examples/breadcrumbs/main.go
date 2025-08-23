package main

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/sentry"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Get DSN from environment
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Fatal("Please set SENTRY_DSN environment variable")
	}

	// Create Sentry sink with breadcrumb support
	sentrySink, err := sentry.WithSentry(dsn,
		sentry.WithEnvironment("development"),
		sentry.WithMinLevel(core.ErrorLevel),       // Errors go to Sentry
		sentry.WithBreadcrumbLevel(core.DebugLevel), // Debug and above become breadcrumbs
		sentry.WithMaxBreadcrumbs(100),              // Keep last 100 breadcrumbs
	)
	if err != nil {
		log.Fatalf("Failed to create Sentry sink: %v", err)
	}
	
	// Create logger with breadcrumb support
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSink(sentrySink),
	)
	defer logger.Close()

	// Simulate a user flow - these become breadcrumbs in Sentry
	logger.Debug("User session started")
	logger.Information("User {UserId} logged in", "user-456")
	
	// Simulate navigation
	logger.Debug("Navigating to dashboard")
	logger.Information("Loading user preferences")
	
	// Simulate data operations
	logger.Debug("Fetching recent transactions")
	logger.Information("Found {Count} transactions", 25)
	
	// Simulate a warning that becomes a breadcrumb
	logger.Warning("Slow query detected: {Duration}ms", 1500)
	
	// More operations
	logger.Debug("Applying filters")
	logger.Information("Rendering transaction list")
	
	// Now when an error occurs, all the above breadcrumbs are attached
	// This provides context about what led to the error
	txErr := processTransaction()
	if txErr != nil {
		logger.Error("Transaction processing failed: {Error}", txErr)
	}

	// Give time for events to be sent
	time.Sleep(2 * time.Second)
}

func processTransaction() error {
	// Simulate a failure
	return errors.New("insufficient funds")
}