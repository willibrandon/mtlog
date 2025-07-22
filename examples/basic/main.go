package main

import (
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Create a new logger
	log := mtlog.New()
	
	// Add console sink
	log.AddSink(sinks.NewConsoleSink())
	
	// Set minimum level to Debug
	log.SetMinimumLevel(core.DebugLevel)
	
	// Log some messages
	log.Information("Application started")
	
	// Log with properties
	userId := 123
	userName := "Alice"
	log.Information("User {UserId} logged in with name {UserName}", userId, userName)
	
	// Different log levels
	log.Debug("Debug message - processing user {UserId}", userId)
	log.Warning("Warning - user {UserId} has {AttemptCount} failed login attempts", userId, 3)
	log.Error("Error - failed to process order {OrderId} for user {UserId}", "ORD-789", userId)
	
	// Test destructuring hints
	user := map[string]interface{}{
		"id":    userId,
		"name":  userName,
		"email": "alice@example.com",
	}
	log.Information("Processing user {@User}", user)
	
	// Test context properties
	ctxLog := log.ForContext("Environment", "Production")
	ctxLog.Information("Context-enriched log message")
	
	// Test with multiple properties
	ctxLog2 := ctxLog.ForContext("Version", "1.0.0")
	ctxLog2.Information("Multiple context properties")
	
	// Test escaped braces
	log.Information("Use {{double braces}} to show literal braces")
	
	// Test adjacent properties
	log.Information("Name: {FirstName}{LastName}", "John", "Doe")
	
	// Test verbose level (should not appear with current minimum level)
	log.Verbose("This verbose message should not appear")
	
	// Lower the minimum level and try again
	log.SetMinimumLevel(core.VerboseLevel)
	log.Verbose("Now this verbose message should appear")
	
	log.Information("Application finished")
}