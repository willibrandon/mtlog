package main

import (
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Create console sink with template and Literate theme
	consoleSink, err := sinks.NewConsoleSinkWithTemplateAndTheme(
		"[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message:lj}",
		sinks.LiterateTheme(),
	)
	if err != nil {
		panic(err)
	}
	
	logger := mtlog.New(
		mtlog.WithSink(consoleSink),
		mtlog.WithAutoSourceContext(),
		mtlog.WithMinimumLevel(core.DebugLevel),
	)
	defer logger.Close()

	// Service startup
	logger.Information("🚀 Application started successfully on port {Port:000}", 8080)
	logger.Debug("Configuration loaded from {ConfigFile}", "config/app.yaml")
	
	// User authentication
	logger.Information("User {Username} logged in with role {Role}", "alice", "admin")
	
	// API request logging
	logger.Information("HTTP {Method} {Path} responded {StatusCode:000} in {Duration:F2}ms", 
		"GET", "/api/v1/products", 200, 45.3)
	
	// Business operations
	logger.Information("Processing order {OrderId} for user {UserId} with total ${Total:F2}", 
		"ORD-2025-789", 12345, 299.99)
	
	// Warning scenarios
	logger.Warning("Inventory low for product {ProductId}, only {Remaining} items left", 
		"SKU-1234", 5)
	
	// Performance monitoring
	logger.Information("Database query completed in {QueryTime:F3}ms for {RowCount} rows", 
		123.456, 1500)
	
	// Cache operations
	logger.Debug("Cache hit for key {CacheKey} with TTL {TTL}s remaining", 
		"product:popular:today", 3600)
	logger.Warning("Cache miss for key {CacheKey}, fetching from database", 
		"user:preferences:12345")
	
	// Error scenarios
	logger.Error("Failed to connect to payment gateway {Gateway}: connection timeout after {Timeout}s", 
		"stripe", 30)
	
	// System metrics with percentage formatting
	logger.Information("System health: CPU={CpuUsage:P1}, Memory={MemoryUsage:P0}, Disk={DiskUsage:P1}", 
		0.653, 0.89, 0.421)
	
	// Status monitoring
	logger.Information("Health check passed: Database={DbStatus}, Cache={CacheStatus}, Queue={QueueStatus}",
		"healthy", "healthy", "degraded")
	
	// Batch processing with zero-padding
	logger.Information("Batch job completed: Processed={Processed:000}, Failed={Failed:000}, Duration={Duration:F1}s",
		42, 3, 12.7)
	
	// Request rate metrics
	logger.Information("API metrics: {RequestsPerSecond:F1} req/s, {AverageLatency:F0}ms, {ErrorRate:P2}",
		125.3, 45.2, 0.0234)
	
	// Shutdown
	logger.Information("🛑 Graceful shutdown completed in {Duration:F2}s", 2.34)
}