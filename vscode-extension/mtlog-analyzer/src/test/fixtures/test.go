package main

import "github.com/willibrandon/mtlog"

func main() {
	logger := mtlog.New(mtlog.WithConsole())

	// This should trigger an error: template has 2 properties but 1 argument
	logger.Information("User {UserId} logged in from {IP}", 123)

	// This should trigger a warning: using @ with basic type
	logger.Debug("Count is {@Count}", 42)

	// This should trigger a suggestion: property name should be PascalCase
	logger.Information("User {userId} completed action", "user123")

	// This is correct
	logger.Information("User {@User} has {Count} items", struct{ Name string }{"Alice"}, 5)
}