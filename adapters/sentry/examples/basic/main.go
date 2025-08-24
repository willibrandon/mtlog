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
	// Get DSN from environment or use a test DSN
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Fatal("Please set SENTRY_DSN environment variable")
	}

	// Create Sentry sink
	sentrySink, err := sentry.WithSentry(dsn,
		sentry.WithEnvironment("development"),
		sentry.WithRelease("v1.0.0"),
		sentry.WithMinLevel(core.ErrorLevel), // Only send errors and above to Sentry
	)
	if err != nil {
		log.Fatalf("Failed to create Sentry sink: %v", err)
	}
	
	// Create logger with Sentry sink
	logger := mtlog.New(
		mtlog.WithConsole(), // Also log to console
		mtlog.WithSink(sentrySink),
	)
	defer logger.Close()

	// Normal logging - goes to console only
	logger.Information("Application started")
	logger.Debug("Debug information: {DebugValue}", 42)

	// Simulate some work
	userID := "user-123"
	logger.Information("Processing request for user {UserId}", userID)

	// Warning - still only goes to console
	logger.Warning("Cache miss for user {UserId}", userID)

	// Error - this goes to both console and Sentry
	dbErr := errors.New("database connection failed")
	logger.Error("Failed to fetch user data: {Error}", dbErr)

	// Fatal error - also goes to Sentry
	criticalErr := errors.New("critical system failure")
	logger.Fatal("System critical error: {Error}", criticalErr)

	// Give time for events to be sent
	time.Sleep(2 * time.Second)
}