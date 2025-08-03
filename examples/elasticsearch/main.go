package main

import (
	"context"
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	fmt.Println("=== Elasticsearch Examples ===")
	fmt.Println()

	// Basic example
	fmt.Println("1. Basic Elasticsearch Logging")
	basicExample()
	fmt.Println()

	// With authentication
	fmt.Println("2. Elasticsearch with Authentication")
	authExample()
	fmt.Println()

	// Advanced configuration
	fmt.Println("3. Advanced Elasticsearch Configuration")
	advancedExample()
	fmt.Println()

	// ECS format example
	fmt.Println("4. ECS (Elastic Common Schema) Format")
	ecsExample()
}

func basicExample() {
	// Create logger with Elasticsearch sink
	esSink, err := sinks.NewElasticsearchSink("http://localhost:9200")
	if err != nil {
		fmt.Printf("Failed to create Elasticsearch sink: %v\n", err)
		return
	}
	defer esSink.Close()
	
	log := mtlog.New(
		mtlog.WithConsoleProperties(), // Also log to console
		mtlog.WithSink(esSink),
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

	fmt.Println("Events logged to Elasticsearch (default index: logs-YYYY.MM.DD)")
}

func authExample() {
	// Skip if no auth configured
	// In production, these would come from environment variables or config
	apiKey := "" // Set your API key here
	username := "" // Or use basic auth
	password := ""

	if apiKey == "" && (username == "" || password == "") {
		fmt.Println("No authentication configured, skipping example")
		return
	}

	var esSink *sinks.ElasticsearchSink
	var err error
	
	if apiKey != "" {
		// Using API key authentication
		esSink, err = sinks.NewElasticsearchSink("http://localhost:9200",
			sinks.WithElasticsearchAPIKey(apiKey))
	} else {
		// Using basic authentication
		esSink, err = sinks.NewElasticsearchSink("http://localhost:9200",
			sinks.WithElasticsearchBasicAuth(username, password))
	}
	
	if err != nil {
		fmt.Printf("Failed to create Elasticsearch sink: %v\n", err)
		return
	}
	defer esSink.Close()
	
	authMethod := "Basic"
	if apiKey != "" {
		authMethod = "APIKey"
	}
	
	log := mtlog.New(
		mtlog.WithSink(esSink),
		mtlog.WithProperty("AuthMethod", authMethod),
	)

	log.Information("Authenticated connection test successful")
}

func advancedExample() {
	// Create Elasticsearch sink with advanced options
	esSink, err := sinks.NewElasticsearchSink("http://localhost:9200",
		sinks.WithElasticsearchIndex("mtlog-advanced"),
		sinks.WithElasticsearchBatchSize(50),
		sinks.WithElasticsearchBatchTimeout(2*time.Second),
		sinks.WithElasticsearchURLs("http://localhost:9200", "http://localhost:9201"), // Multiple nodes
		sinks.WithElasticsearchPipeline("my-ingest-pipeline"), // Use ingest pipeline
	)
	if err != nil {
		fmt.Printf("Failed to create Elasticsearch sink: %v\n", err)
		return
	}

	log := mtlog.New(
		mtlog.WithSink(esSink),
		mtlog.WithConsoleProperties(),
		mtlog.WithMachineName(),
		mtlog.WithThreadId(),
		mtlog.WithProperty("Environment", "Production"),
		mtlog.WithProperty("Service", "OrderService"),
	)
	defer esSink.Close()

	// Simulate high-volume logging to test batching
	for i := 0; i < 100; i++ {
		log.Information("Processing order {OrderId} for customer {CustomerId}", 
			fmt.Sprintf("ORD-%04d", i), 
			fmt.Sprintf("CUST-%03d", i%10))
		
		if i%10 == 0 {
			log.Debug("Batch progress: {Count} orders processed", i)
		}
	}

	// Force flush remaining events
	fmt.Println("Flushing remaining events...")
}

func ecsExample() {
	// Elasticsearch sink automatically adds ECS-compatible fields
	esSink, err := sinks.NewElasticsearchSink("http://localhost:9200",
		sinks.WithElasticsearchIndex("ecs-logs"),
		sinks.WithElasticsearchDataStreams(), // Use data streams for better indexing
	)
	if err != nil {
		fmt.Printf("Failed to create Elasticsearch sink: %v\n", err)
		return
	}
	defer esSink.Close()
	
	log := mtlog.New(
		mtlog.WithSink(esSink),
		mtlog.WithConsoleProperties(),
		mtlog.WithProperty("service.name", "mtlog-example"),
		mtlog.WithProperty("service.version", "1.0.0"),
		mtlog.WithProperty("host.name", "prod-server-01"),
	)

	// Create a context with trace information
	ctx := context.Background()
	ctx = context.WithValue(ctx, "trace.id", "abc123def456")
	ctx = context.WithValue(ctx, "span.id", "789xyz")

	// Log with context (ECS fields will be added automatically)
	contextLog := log.WithContext(ctx)
	
	// The sink automatically maps to ECS fields:
	// - level -> log.level
	// - @timestamp -> @timestamp
	// - Error property -> error.message
	contextLog.Information("Request received for endpoint {http.request.method} {url.path}", 
		"GET", "/api/orders")
	
	// Error logging - automatically mapped to error.message
	dbErr := fmt.Errorf("connection refused")
	contextLog.Error("Database connection failed: {Error}", dbErr)

	// Log with standard ECS fields
	contextLog.Information("User authentication successful",
		"user.id", "user-456",
		"user.name", "john.doe",
		"user.email", "john.doe@example.com",
		"event.action", "user-login",
		"event.outcome", "success")

	fmt.Println("Events logged in ECS format to Elasticsearch")
}