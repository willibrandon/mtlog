package main

import (
	"fmt"
	"time"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// Order represents a sample order
type Order struct {
	ID         string
	CustomerID int
	Total      float64
	Items      []OrderItem
	Status     string
	CreatedAt  time.Time
}

// OrderItem represents an item in an order
type OrderItem struct {
	ProductID   string
	ProductName string
	Quantity    int
	Price       float64
}

func main() {
	// Example 1: Basic Seq integration
	fmt.Println("=== Basic Seq Integration ===")
	basicExample()
	
	// Example 2: Seq with API key and batching
	fmt.Println("\n=== Seq with API Key ===")
	apiKeyExample()
	
	// Example 3: Advanced Seq configuration
	fmt.Println("\n=== Advanced Seq Configuration ===")
	advancedExample()
	
	// Example 4: Structured logging to Seq
	fmt.Println("\n=== Structured Logging to Seq ===")
	structuredExample()
}

func basicExample() {
	// Create logger with Seq sink
	log := mtlog.New(
		mtlog.WithConsoleProperties(), // Also log to console
		mtlog.WithSeq("http://localhost:5341"),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithMachineName(),
		mtlog.WithTimestamp(),
	)
	
	// Simple logging
	log.Information("Application started")
	log.Debug("Debug information: {DebugValue}", 42)
	log.Warning("This is a warning with {Count} items", 5)
	
	// Ensure logs are flushed
	time.Sleep(100 * time.Millisecond)
}

func apiKeyExample() {
	// Create logger with authenticated Seq sink
	log := mtlog.New(
		mtlog.WithSeqAPIKey("http://localhost:5341", "your-api-key-here"),
		mtlog.WithConsoleProperties(),
		mtlog.WithProperty("Application", "mtlog-example"),
		mtlog.WithProperty("Environment", "development"),
		mtlog.WithMachineName(),
	)
	
	// Log with context
	requestLogger := log.ForContext("RequestId", "req-123")
	requestLogger.Information("Processing request")
	requestLogger.Information("Request completed in {Duration}ms", 145)
}

func advancedExample() {
	// Create logger with advanced Seq configuration
	log := mtlog.New(
		mtlog.WithSeqAdvanced("http://localhost:5341",
			sinks.WithSeqAPIKey("your-api-key"),
			sinks.WithSeqBatchSize(50),
			sinks.WithSeqBatchTimeout(2*time.Second),
			sinks.WithSeqCompression(true),
			sinks.WithSeqRetry(3, time.Second),
		),
		mtlog.WithConsoleProperties(),
		mtlog.WithEnricher(&customEnricher{}),
		mtlog.WithDestructuring(),
	)
	
	// Log some events
	for i := 0; i < 10; i++ {
		log.Information("Event {EventNumber} of {Total}", i+1, 10)
		time.Sleep(50 * time.Millisecond)
	}
	
	// Wait for batch to flush
	time.Sleep(3 * time.Second)
}

func structuredExample() {
	// Create logger for structured data
	log := mtlog.New(
		mtlog.WithSeq("http://localhost:5341"),
		mtlog.WithConsoleProperties(),
		mtlog.WithDestructuring(),
		mtlog.WithTimestamp(),
		mtlog.WithCallersInfo(),
	)
	
	// Create sample order
	order := Order{
		ID:         "ORD-12345",
		CustomerID: 789,
		Total:      299.99,
		Status:     "pending",
		CreatedAt:  time.Now(),
		Items: []OrderItem{
			{
				ProductID:   "PROD-001",
				ProductName: "Widget Pro",
				Quantity:    2,
				Price:       49.99,
			},
			{
				ProductID:   "PROD-002",
				ProductName: "Gadget Max",
				Quantity:    1,
				Price:       199.99,
			},
		},
	}
	
	// Log structured data - Seq will index all properties
	log.Information("Order placed: {@Order}", order)
	
	// Log with multiple properties
	log.Information("Processing order {OrderId} for customer {CustomerId} with {ItemCount} items",
		order.ID, order.CustomerID, len(order.Items))
	
	// Error scenario
	err := processOrder(order)
	if err != nil {
		log.Error("Failed to process order {OrderId}: {Error}",
			order.ID, err)
	}
	
	// Performance logging
	start := time.Now()
	// Simulate work
	time.Sleep(150 * time.Millisecond)
	duration := time.Since(start)
	
	log.Information("Order {OrderId} processed in {Duration}",
		order.ID, duration)
	
	// Using different log levels
	log.Verbose("Detailed order information: {@Order}", order)
	log.Debug("Order status: {Status}", order.Status)
	log.Warning("Order {OrderId} total {Total} exceeds threshold",
		order.ID, order.Total)
	
	// Wait for final flush
	time.Sleep(time.Second)
}

// processOrder simulates order processing
func processOrder(order Order) error {
	if order.Total > 1000 {
		return fmt.Errorf("order total exceeds maximum allowed amount")
	}
	return nil
}

// customEnricher adds custom properties to all events
type customEnricher struct{}

func (e *customEnricher) Enrich(event *core.LogEvent, factory core.LogEventPropertyFactory) {
	event.AddPropertyIfAbsent(factory.CreateProperty("Version", "1.0.0"))
	event.AddPropertyIfAbsent(factory.CreateProperty("Component", "seq-example"))
}