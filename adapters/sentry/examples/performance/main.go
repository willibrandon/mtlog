package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/sentry"
	"github.com/willibrandon/mtlog/core"
)

// Mock database for demonstration
type mockDB struct{}

func (db *mockDB) Query(ctx context.Context, query string) error {
	// Simulate database latency
	time.Sleep(time.Duration(20+rand.Intn(30)) * time.Millisecond)
	
	// Randomly fail 10% of queries
	if rand.Float32() < 0.1 {
		return errors.New("database connection timeout")
	}
	return nil
}

// Mock HTTP client for demonstration
type mockHTTPClient struct{}

func (c *mockHTTPClient) Do(ctx context.Context, method, url string) (int, error) {
	// Simulate network latency
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
	
	// Randomly return different status codes
	statuses := []int{200, 200, 200, 200, 404, 500}
	status := statuses[rand.Intn(len(statuses))]
	
	if status >= 500 {
		return status, errors.New("internal server error")
	}
	return status, nil
}

// Mock cache for demonstration
type mockCache struct {
	data map[string]interface{}
}

func (c *mockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// Simulate cache lookup latency
	time.Sleep(2 * time.Millisecond)
	val, ok := c.data[key]
	return val, ok
}

func (c *mockCache) Set(ctx context.Context, key string, value interface{}) {
	// Simulate cache write latency
	time.Sleep(5 * time.Millisecond)
	c.data[key] = value
}

func main() {
	// Create Sentry sink with performance monitoring
	dsn := "https://your-key@sentry.io/project-id"
	sentrySink, err := sentry.NewSentrySink(dsn,
		sentry.WithEnvironment("production"),
		sentry.WithRelease("v2.0.0"),
		sentry.WithMinLevel(core.InformationLevel),
		sentry.WithStackTraceCacheSize(500), // Cache stack traces for performance
	)
	if err != nil {
		log.Fatalf("Failed to create Sentry sink: %v", err)
	}
	defer sentrySink.Close()

	// Create logger
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSink(sentrySink),
	)

	// Initialize mock services
	db := &mockDB{}
	httpClient := &mockHTTPClient{}
	cache := &mockCache{data: make(map[string]interface{})}

	fmt.Println("Starting performance monitoring example...")
	fmt.Println("This simulates an order processing workflow with transaction tracking.")

	// Process multiple orders with transaction tracking
	for i := 1; i <= 3; i++ {
		orderID := fmt.Sprintf("ORD-%05d", i)
		processOrder(context.Background(), logger, db, httpClient, cache, orderID)
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\nPerformance monitoring example completed.")
	fmt.Println("Check your Sentry Performance dashboard for transaction details.")
}

func processOrder(ctx context.Context, logger core.Logger, db *mockDB, 
	httpClient *mockHTTPClient, cache *mockCache, orderID string) {
	
	// Start a transaction for the entire order processing
	txCtx := sentry.StartTransaction(ctx, "process-order", "order.process")
	defer func() {
		if tx := sentry.GetTransaction(txCtx); tx != nil {
			tx.Finish()
		}
	}()

	// Add transaction metadata
	sentry.SetSpanTag(txCtx, "order.id", orderID)
	sentry.SetSpanData(txCtx, "order.timestamp", time.Now().Unix())

	logger.WithContext(txCtx).Information("Processing order {OrderId}", orderID)

	// Step 1: Check cache for order details
	cacheCtx, finishCache := sentry.TraceCache(txCtx, "get", orderID)
	_, cacheHit := cache.Get(cacheCtx, orderID)
	finishCache(cacheHit)
	
	if !cacheHit {
		logger.WithContext(cacheCtx).Debug("Cache miss for order {OrderId}", orderID)
		
		// Step 2: Query database for order details
		dbCtx, finishDB := sentry.TraceDatabaseQuery(txCtx, 
			"SELECT * FROM orders WHERE id = ?", "orders_db")
		
		err := db.Query(dbCtx, fmt.Sprintf("SELECT * FROM orders WHERE id = '%s'", orderID))
		finishDB(err)
		
		if err != nil {
			logger.WithContext(dbCtx).Error("Database query failed: {Error}", err)
			sentry.SetSpanStatus(txCtx, "internal_error")
			return
		}
		
		// Cache the result
		cacheSetCtx, finishCacheSet := sentry.TraceCache(txCtx, "set", orderID)
		cache.Set(cacheSetCtx, orderID, map[string]interface{}{
			"id": orderID,
			"status": "pending",
		})
		finishCacheSet(true)
	}

	// Step 3: Validate inventory (batch operation)
	err := sentry.BatchSpan(txCtx, "inventory.validate", 5, func() error {
		// Simulate inventory check for 5 items
		time.Sleep(30 * time.Millisecond)
		return nil
	})
	
	if err != nil {
		logger.WithContext(txCtx).Error("Inventory validation failed: {Error}", err)
		sentry.SetSpanStatus(txCtx, "failed_precondition")
		return
	}

	// Step 4: Process payment via external API
	paymentCtx, finishPayment := sentry.TraceHTTPRequest(txCtx, 
		"POST", "https://payment-gateway.example.com/charge")
	
	statusCode, err := httpClient.Do(paymentCtx, "POST", 
		"https://payment-gateway.example.com/charge")
	finishPayment(statusCode)
	
	if err != nil {
		logger.WithContext(paymentCtx).Error(
			"Payment processing failed with status {Status}: {Error}", 
			statusCode, err)
		sentry.SetSpanStatus(txCtx, "failed_precondition")
		return
	}

	// Step 5: Update order status (measured operation)
	err = sentry.MeasureSpan(txCtx, "order.update_status", func() error {
		// Simulate status update
		time.Sleep(10 * time.Millisecond)
		
		// Randomly fail 5% of updates
		if rand.Float32() < 0.05 {
			return errors.New("optimistic locking conflict")
		}
		return nil
	})
	
	if err != nil {
		logger.WithContext(txCtx).Warning("Status update failed: {Error}", err)
		// Don't fail the transaction, we can retry this
	}

	// Step 6: Send confirmation email (async operation)
	emailCtx, finishEmail := sentry.StartSpan(txCtx, "email.send")
	go func() {
		defer finishEmail()
		
		// Simulate email sending
		time.Sleep(200 * time.Millisecond)
		
		logger.WithContext(emailCtx).Information(
			"Confirmation email sent for order {OrderId}", orderID)
		sentry.SetSpanStatus(emailCtx, "ok")
	}()

	// Transaction completed successfully
	sentry.SetSpanStatus(txCtx, "ok")
	logger.WithContext(txCtx).Information(
		"Order {OrderId} processed successfully", orderID)
}

// Example of using transaction middleware for HTTP handlers
func exampleHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// This would be wrapped with TransactionMiddleware
		ctx := r.Context()
		
		// The middleware automatically creates a transaction
		middleware := sentry.TransactionMiddleware("http.handler")
		
		err := middleware(ctx, func(txCtx context.Context) error {
			// Your handler logic here
			// All operations within use txCtx for span tracking
			return nil
		})
		
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		
		w.WriteHeader(http.StatusOK)
	}
}