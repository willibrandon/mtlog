package main

import (
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create a logger with console output
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.InformationLevel),
	)

	// Example 1: Basic With() usage
	logger.With("service", "auth", "version", "1.0").Info("Service started")

	// Example 2: Chaining With() calls
	logger.
		With("environment", "production").
		With("region", "us-west-2").
		Info("Deployment complete")

	// Example 3: Creating a common logger with shared fields
	apiLogger := logger.With(
		"component", "api",
		"host", "api-server-01",
	)

	// Use the common logger for multiple operations
	apiLogger.Info("Handling request")
	apiLogger.With("endpoint", "/users").Info("GET /users")
	apiLogger.With("endpoint", "/products", "method", "POST").Info("POST /products")

	// Example 4: Request-scoped logging
	requestLogger := apiLogger.With(
		"request_id", "abc-123",
		"user_id", 456,
		"timestamp", time.Now().Unix(),
	)

	requestLogger.Info("Request started")
	requestLogger.With("duration_ms", 42).Info("Request completed")

	// Example 5: Combining With() and ForContext()
	logger.
		With("service", "payment").
		ForContext("transaction_id", "tx-789").
		With("amount", 99.99, "currency", "USD").
		Info("Payment processed")

	// Example 6: Error logging with context
	err := processOrder("order-123")
	if err != nil {
		logger.
			With("order_id", "order-123").
			With("error_type", "validation").
			Error("Failed to process order: {Error}", err)
	}

	// Example 7: Structured logging for metrics
	metricsLogger := logger.With("metric_type", "performance")
	
	metricsLogger.With(
		"operation", "database_query",
		"duration_ms", 15,
		"rows_returned", 42,
	).Info("Query executed")

	metricsLogger.With(
		"operation", "cache_lookup",
		"hit", true,
		"latency_ns", 1500,
	).Info("Cache accessed")

	// Example 8: Dynamic field addition
	baseLogger := logger.With("app", "example")
	for i := 0; i < 3; i++ {
		baseLogger.With("iteration", i, "timestamp", time.Now().UnixNano()).
			Info("Processing iteration")
		time.Sleep(100 * time.Millisecond)
	}
}

func processOrder(orderID string) error {
	// Simulate order processing
	_ = orderID // Mark as used
	return nil
}