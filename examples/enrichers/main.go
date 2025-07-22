package main

import (
	"path/filepath"
	"time"
	
	"github.com/willibrandon/mtlog"
)

func main() {
	// Create a new logger with console, file output, enrichers, and debug level
	logPath := filepath.Join("logs", "app.log")
	log := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithFile(logPath),
		mtlog.WithTimestamp(),
		mtlog.WithMachineName(),
		mtlog.WithProcess(),
		mtlog.Debug(),
	)
	
	// Log some messages
	log.Information("Application started with enrichers")
	
	// Demonstrate different features
	userId := 456
	userName := "Bob"
	
	log.Debug("Debug: Initializing user session for {UserId}", userId)
	log.Information("User {UserId} ({UserName}) logged in successfully", userId, userName)
	
	// Simulate some operations
	orders := []string{"ORD-001", "ORD-002", "ORD-003"}
	for _, orderId := range orders {
		log.Information("Processing order {OrderId} for user {UserId}", orderId, userId)
		time.Sleep(100 * time.Millisecond)
	}
	
	// Context logging
	serviceLog := log.ForContext("Service", "OrderProcessor").
		ForContext("Version", "2.1.0")
	
	serviceLog.Information("Service initialized")
	serviceLog.Warning("Low memory warning: {MemoryMB} MB remaining", 512)
	
	// Error scenario
	log.Error("Failed to connect to database after {Attempts} attempts", 3)
	
	// Structured logging
	orderDetails := map[string]interface{}{
		"orderId":    "ORD-004",
		"userId":     userId,
		"items":      5,
		"totalValue": 299.99,
	}
	log.Information("Order completed: {@OrderDetails}", orderDetails)
	
	log.Information("Application shutting down gracefully")
}