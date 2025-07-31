package main

import (
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	fmt.Println("=== Splunk Examples ===")
	fmt.Println()

	// Basic example
	fmt.Println("1. Basic Splunk Logging")
	basicExample()
	fmt.Println()

	// Advanced configuration
	fmt.Println("2. Advanced Splunk Configuration")
	advancedExample()
	fmt.Println()

	// Custom fields example
	fmt.Println("3. Splunk with Custom Fields")
	customFieldsExample()
}

func basicExample() {
	// Splunk HEC token - in production, use environment variable or secure config
	splunkURL := "http://localhost:8088"
	splunkToken := "your-hec-token-here"

	// Create logger with Splunk sink
	splunkSink, err := sinks.NewSplunkSink(splunkURL, splunkToken)
	if err != nil {
		fmt.Printf("Failed to create Splunk sink: %v\n", err)
		return
	}
	defer splunkSink.Close()

	log := mtlog.New(
		mtlog.WithConsoleProperties(), // Also log to console
		mtlog.WithSink(splunkSink),
		mtlog.WithMinimumLevel(core.InformationLevel),
		mtlog.WithMachineName(),
		mtlog.WithProcessInfo(),
	)

	// Log some events
	log.Information("Application started")
	log.Information("User {UserId} logged in from {IPAddress}", "user-123", "192.168.1.100")

	// Log structured data
	order := struct {
		ID       string
		Customer string
		Total    float64
		Items    int
	}{
		ID:       "ORD-2025-001",
		Customer: "ACME Corp",
		Total:    1299.99,
		Items:    3,
	}
	log.Information("Order placed: {@Order}", order)

	// Log with different levels
	log.Warning("Low disk space on {Drive}: {PercentUsed:P} used", "C:", 0.85)
	log.Error("Failed to connect to database: {Error}", "Connection timeout")

	fmt.Println("Events sent to Splunk HEC")
}

func advancedExample() {
	splunkURL := "http://localhost:8088"
	splunkToken := "your-hec-token-here"

	// Create Splunk sink with advanced options
	splunkSink, err := sinks.NewSplunkSink(splunkURL, splunkToken,
		sinks.WithSplunkIndex("application_logs"),
		sinks.WithSplunkSource("mtlog-app"),
		sinks.WithSplunkSourceType("_json"),
		sinks.WithSplunkHost("prod-server-01"),
		sinks.WithSplunkBatchSize(50),
		sinks.WithSplunkBatchTimeout(2*time.Second),
	)
	if err != nil {
		fmt.Printf("Failed to create Splunk sink: %v\n", err)
		return
	}

	log := mtlog.New(
		mtlog.WithSink(splunkSink),
		mtlog.WithConsoleProperties(),
		mtlog.WithMachineName(),
		mtlog.WithThreadId(),
		mtlog.WithProperty("Environment", "Production"),
		mtlog.WithProperty("Service", "OrderService"),
		mtlog.WithProperty("Version", "2.1.0"),
	)
	defer splunkSink.Close()

	// Simulate high-volume logging to test batching
	for i := range 100 {
		log.Information("Processing order {OrderId} for customer {CustomerId}",
			fmt.Sprintf("ORD-%04d", i),
			fmt.Sprintf("CUST-%03d", i%10))

		if i%10 == 0 {
			log.Debug("Batch progress: {Count} orders processed", i)
		}

		// Simulate some warnings and errors
		if i%15 == 0 && i > 0 {
			log.Warning("Order {OrderId} processing slow: {Duration}ms",
				fmt.Sprintf("ORD-%04d", i), 1500+i*10)
		}

		if i%25 == 0 && i > 0 {
			log.Error("Failed to validate order {OrderId}: {Reason}",
				fmt.Sprintf("ORD-%04d", i), "Invalid shipping address")
		}
	}

	fmt.Println("Batch processing complete - events sent to Splunk")
}

func customFieldsExample() {
	splunkURL := "http://localhost:8088"
	splunkToken := "your-hec-token-here"

	// Create Splunk sink with metadata
	splunkSink, err := sinks.NewSplunkSink(splunkURL, splunkToken,
		sinks.WithSplunkIndex("security_logs"),
		sinks.WithSplunkSourceType("security:auth"),
	)
	if err != nil {
		fmt.Printf("Failed to create Splunk sink: %v\n", err)
		return
	}
	defer splunkSink.Close()

	log := mtlog.New(
		mtlog.WithSink(splunkSink),
		mtlog.WithConsoleProperties(),
		mtlog.WithProperty("app", "auth-service"),
		mtlog.WithProperty("dc", "us-east-1"),
		mtlog.WithProperty("cluster", "prod-k8s"),
		mtlog.WithProperty("pod", "auth-service-7d9f8c6b5-xvz2m"),
	)

	// Security event logging with rich fields
	log.Information("User authentication attempt",
		"event.action", "user-login",
		"event.category", "authentication",
		"event.outcome", "success",
		"user.id", "user-456",
		"user.name", "john.doe",
		"user.email", "john.doe@example.com",
		"source.ip", "203.0.113.42",
		"source.geo.country", "US",
		"source.geo.city", "New York",
		"user_agent.original", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
		"session.id", "sess-789abc",
		"mfa.type", "totp",
		"risk.score", 15)

	// Failed authentication
	log.Warning("User authentication failed",
		"event.action", "user-login",
		"event.category", "authentication",
		"event.outcome", "failure",
		"user.name", "jane.smith",
		"source.ip", "198.51.100.23",
		"source.geo.country", "CN",
		"error.code", "INVALID_CREDENTIALS",
		"risk.score", 75,
		"threat.indicator", true)

	// Suspicious activity
	log.Error("Potential security threat detected",
		"event.action", "brute-force-attempt",
		"event.category", "intrusion_detection",
		"event.severity", 8,
		"source.ip", "192.0.2.99",
		"source.geo.country", "RU",
		"threat.tactic.name", "Credential Access",
		"threat.technique.name", "Brute Force",
		"user.target", "admin",
		"attempt.count", 50,
		"time.window", "5m",
		"action.taken", "ip-blocked")

	fmt.Println("Security events with custom fields sent to Splunk")
}
