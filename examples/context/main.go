package main

import (
	"context"
	"os"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/enrichers"
)

func main() {
	// Set some environment variables for demonstration
	os.Setenv("ENVIRONMENT", "production")
	os.Setenv("SERVICE_NAME", "api-gateway")
	os.Setenv("SERVICE_VERSION", "v1.2.3")
	
	// Create logger with various enrichers
	log := mtlog.New(
		mtlog.WithConsoleProperties(), // Use console sink that shows properties
		mtlog.WithCommonEnvironment(),
		mtlog.WithMachineName(),
		mtlog.WithProcess(),
		mtlog.WithThreadId(),
		mtlog.WithCallers(9), // Skip 9 frames to get to the actual caller
		mtlog.Debug(),
	)
	
	log.Information("Application started with all enrichers")
	
	// Demonstrate context-based logging
	ctx := context.Background()
	ctx = enrichers.WithCorrelationId(ctx, "req-123-456")
	ctx = enrichers.WithRequestId(ctx, "req-789")
	ctx = enrichers.WithUserId(ctx, "user-42")
	ctx = enrichers.WithSessionId(ctx, "session-xyz")
	
	// Create a logger with context
	ctxLog := log.WithContext(ctx)
	ctxLog.Information("Processing request with context")
	
	// Simulate a service call
	processOrder(ctxLog, "ORD-001")
	
	// Demonstrate correlation ID enricher
	correlatedLog := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithCorrelationId("batch-job-123"),
		mtlog.Information(),
	)
	
	correlatedLog.Information("Starting batch job")
	correlatedLog.Information("Processing item {ItemId}", "ITEM-001")
	correlatedLog.Information("Batch job completed")
	
	// Demonstrate custom context values
	type contextKey string
	const tenantKey contextKey = "tenant"
	
	tenantCtx := context.WithValue(context.Background(), tenantKey, "acme-corp")
	tenantLog := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithEnricher(enrichers.NewContextValueEnricher(tenantCtx, tenantKey, "TenantId")),
		mtlog.Information(),
	)
	
	tenantLog.Information("Processing tenant-specific operation")
}

func processOrder(log core.Logger, orderId string) {
	log.Information("Processing order {OrderId}", orderId)
	
	// Simulate some work
	log.Debug("Validating order {OrderId}", orderId)
	log.Debug("Checking inventory for order {OrderId}", orderId)
	log.Information("Order {OrderId} processed successfully", orderId)
}