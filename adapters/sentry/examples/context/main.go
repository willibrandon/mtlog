package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
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

	// Create Sentry sink with context enrichment
	sentrySink, err := sentry.WithSentry(dsn,
		sentry.WithEnvironment("production"),
		sentry.WithRelease("v2.1.0"),
		sentry.WithServerName("api-server-01"),
	)
	if err != nil {
		log.Fatalf("Failed to create Sentry sink: %v", err)
	}
	
	// Create logger with context enrichment
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSink(sentrySink),
	)
	defer logger.Close()

	// Example 1: User context
	ctx := context.Background()
	ctx = sentry.WithUser(ctx, sentrygo.User{
		ID:       "user-789",
		Email:    "john.doe@example.com",
		Username: "johndoe",
	})

	// Log with user context - if this errors, Sentry will know which user was affected
	logger.WithContext(ctx).Information("Processing payment for user")
	
	// Simulate payment error - Sentry will associate this with the user
	paymentErr := errors.New("payment gateway timeout")
	logger.WithContext(ctx).Error("Payment failed: {Error}", paymentErr)

	// Example 2: Request context with tags
	requestCtx := context.Background()
	requestCtx = sentry.WithTags(requestCtx, map[string]string{
		"request.id":     "req-12345",
		"request.method": "POST",
		"request.path":   "/api/v1/payments",
		"client.ip":      "192.168.1.100",
		"region":         "us-west-2",
	})

	// Add user to request context
	requestCtx = sentry.WithUser(requestCtx, sentrygo.User{
		ID:    "user-890",
		Email: "jane.smith@example.com",
	})

	// Log with rich context
	logger.WithContext(requestCtx).Information("API request received")
	
	// If an error occurs, Sentry will have all the context
	apiErr := errors.New("invalid payment method")
	logger.WithContext(requestCtx).Error("API request failed: {Error}", apiErr)

	// Example 3: Custom context data
	deviceCtx := context.Background()
	deviceCtx = sentry.WithContext(deviceCtx, "device", map[string]interface{}{
		"model":    "iPhone 14 Pro",
		"os":       "iOS 17.1",
		"app_version": "3.2.1",
	})
	deviceCtx = sentry.WithContext(deviceCtx, "location", map[string]interface{}{
		"country": "USA",
		"city":    "Seattle",
		"timezone": "PST",
	})

	// Log with device and location context
	logger.WithContext(deviceCtx).Warning("App crash detected on mobile device")

	// Example 4: Custom fingerprinting for error grouping
	customSink, err := sentry.WithSentry(dsn,
		sentry.WithFingerprinter(func(event *core.LogEvent) []string {
			// Group errors by template and error type
			fingerprint := []string{event.MessageTemplate}
			if err, ok := event.Properties["Error"].(error); ok {
				if unwrapped := errors.Unwrap(err); unwrapped != nil {
					fingerprint = append(fingerprint, unwrapped.Error())
				} else {
					fingerprint = append(fingerprint, err.Error())
				}
			}
			return fingerprint
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create custom Sentry sink: %v", err)
	}
	
	customLogger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSink(customSink),
	)
	defer customLogger.Close()

	// These errors will be grouped together in Sentry
	for i := 0; i < 3; i++ {
		customLogger.Error("Database query failed: {Error}", errors.New("connection timeout"))
	}

	// Give time for events to be sent
	time.Sleep(2 * time.Second)
}