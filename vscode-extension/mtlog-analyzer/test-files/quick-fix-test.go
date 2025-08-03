package main

import "github.com/willibrandon/mtlog"

func main() {
	log := mtlog.New(mtlog.WithConsole())
	
	// PascalCase warnings - property names should be PascalCase
	log.Information("User {user_id} logged in", 123)
	log.Information("Order {order_id} processed for {customer_name}", "12345", "John")
	log.Debug("Request from {ip_address} with {user_agent}", "192.168.1.1", "Chrome")
	log.Warning("High {cpu_usage} detected on {server_name}", 85.5, "web-01")
	
	// Argument mismatch errors - too few arguments
	log.Information("User {UserId} action {Action}", 123)
	log.Error("Failed to process {OrderId} for customer {CustomerId}")
	log.Warning("Temperature {Temp} exceeded threshold {Threshold}", 75.5)
	
	// Argument mismatch errors - too many arguments  
	log.Information("Hello {Name}", "Alice", "Bob", "Charlie")
	log.Debug("Processing started", 123, 456)
	log.Error("Operation failed for {Id}", 789, "extra", "args")
	
	// Mixed issues - both PascalCase and argument mismatch
	log.Information("User {user_id} performed {action_type}", 123) // missing one arg
	log.Warning("Server {server_id} CPU at {cpu_percent}%", "srv-01", 95, "extra") // too many args
}