package main

import (
	"time"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// Order represents an order with type safety
type Order struct {
	ID         int
	CustomerID int
	Total      float64
	Items      []OrderItem
	CreatedAt  time.Time
}

// OrderItem represents an item in an order
type OrderItem struct {
	ProductID int
	Quantity  int
	Price     float64
}

// UserAction represents a user action event
type UserAction struct {
	UserID    int
	Action    string
	Timestamp time.Time
	Metadata  map[string]string
}

func main() {
	// Example 1: Type-safe logger for Orders
	orderLogger := mtlog.NewTyped[Order](
		mtlog.WithConsoleProperties(),
		mtlog.WithDestructuring(),
	)
	
	order := Order{
		ID:         12345,
		CustomerID: 789,
		Total:      299.99,
		Items: []OrderItem{
			{ProductID: 1, Quantity: 2, Price: 49.99},
			{ProductID: 2, Quantity: 1, Price: 199.99},
		},
		CreatedAt: time.Now(),
	}
	
	// Type-safe logging - compiler ensures first parameter is an Order
	orderLogger.InformationT("Order placed: {@Order}", order)
	orderLogger.DebugT("Processing order {@Order} for shipping", order, "warehouse", "EAST")
	
	// Example 2: Structured logger with property bag
	structured := mtlog.NewStructured(
		mtlog.WithConsoleProperties(),
	)
	
	// Build a log entry with typed properties
	structured.LogWith().
		Property("OrderId", order.ID).
		Property("CustomerId", order.CustomerID).
		Property("Total", order.Total).
		Property("ItemCount", len(order.Items)).
		Level(core.InformationLevel).
		Message("Order summary for {OrderId} - {ItemCount} items totaling ${Total}").
		Write()
	
	// Example 3: Type-safe logger for user actions
	actionLogger := mtlog.NewTyped[UserAction](
		mtlog.WithConsoleProperties(),
		mtlog.WithTimestamp(),
	)
	
	action := UserAction{
		UserID:    789,
		Action:    "checkout_completed",
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"order_id": "12345",
			"payment":  "credit_card",
		},
	}
	
	actionLogger.InformationT("User action: {@Action}", action)
	
	// Example 4: Using property bag for complex scenarios
	props := core.NewPropertyBag()
	core.AddTyped(props, "OrderId", order.ID)
	core.AddTyped(props, "Processing", true)
	core.AddTyped(props, "Priority", "high")
	
	// Type-safe retrieval
	if orderId, ok := core.GetTyped[int](props, "OrderId"); ok {
		structured.LogWith().
			Property("OrderId", orderId).
			Property("Status", "processing").
			Level(core.DebugLevel).
			Message("Order {OrderId} status: {Status}").
			Write()
	}
	
	// Example 5: Chaining typed context with same type
	// For different types, use the regular logger
	regularLogger := mtlog.New(
		mtlog.WithConsoleProperties(),
	)
	sessionLogger := regularLogger.ForContext("SessionId", "abc-123")
	sessionLogger.Information("Order {@Order} processed in session", order)
}