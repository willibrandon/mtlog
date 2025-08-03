package main

import (
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Create a logger with Literate theme for nice output
	sink, err := sinks.NewConsoleSinkWithTemplateAndTheme(
		"[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message}",
		sinks.LiterateTheme(),
	)
	if err != nil {
		panic(err)
	}
	
	logger := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithAutoSourceContext(),
	)
	defer logger.Close()

	// Properties work with both syntaxes
	userId := 42
	username := "gopher"
	ip := "192.168.1.100"
	
	// Demonstrate Go template syntax {{.Property}}
	logger.Information("Go template syntax: User {{.UserId}} logged in", userId)
	
	// You can mix both syntaxes
	logger.Information("Mixed syntax: User {UserId} ({{.Username}}) from {IP}", userId, username, ip)
	
	// Both syntaxes work identically
	logger.Information("User {{.UserId}} ({{.Username}}) logged in from {{.IP}}", userId, username, ip)
	logger.Information("Mixed: User {UserId} ({{.Username}}) from {IP}", userId, username, ip)
	
	// Go templates with complex data
	type User struct {
		ID   int
		Name string
		Role string
	}
	
	user := User{ID: 123, Name: "Alice", Role: "admin"}
	logger.Information("User {{.User}} has role {{.Role}}", user, user.Role)
	
	// Note: Format specifiers still work with regular syntax
	amount := 99.95
	logger.Information("Payment of {Amount:F2} processed for {{.Username}}", amount, username)
	
	// Multiple properties
	logger.Information("{{.Action}} by {{.User}} at {{.Time}} - Status: {{.Status}}", 
		"Login", username, "2024-01-02 15:04:05", "Success")
}