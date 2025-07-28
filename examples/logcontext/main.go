// Package main demonstrates using LogContext for scoped properties.
package main

import (
	"context"
	"fmt"
	"time"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create a logger with console output that shows properties
	logger := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithMinimumLevel(core.DebugLevel),
	)
	
	fmt.Println("=== LogContext Example ===")
	fmt.Println()
	
	// Simulate a web service handling requests
	simulateWebService(logger)
	
	fmt.Println()
	fmt.Println("=== Nested Context Example ===")
	fmt.Println()
	
	// Demonstrate nested contexts
	demonstrateNestedContexts(logger)
}

func simulateWebService(logger core.Logger) {
	// Simulate handling an HTTP request
	ctx := context.Background()
	
	// Add request-level properties that will be included in all logs
	ctx = mtlog.PushProperty(ctx, "RequestId", "req-12345")
	ctx = mtlog.PushProperty(ctx, "ClientIP", "192.168.1.100")
	ctx = mtlog.PushProperty(ctx, "Method", "POST")
	ctx = mtlog.PushProperty(ctx, "Path", "/api/users")
	
	// Create a logger that includes context properties
	log := logger.WithContext(ctx)
	
	log.Information("Received HTTP request")
	
	// Simulate user authentication
	userId := authenticateUser(ctx, log)
	if userId != "" {
		// Add user context
		ctx = mtlog.PushProperty(ctx, "UserId", userId)
		log = logger.WithContext(ctx) // Recreate logger with updated context
		
		log.Information("User authenticated successfully")
		
		// Process the request
		processUserRequest(ctx, log, userId)
	}
	
	log.Information("Request completed")
}

func authenticateUser(ctx context.Context, log core.Logger) string {
	log.Debug("Authenticating user")
	
	// Simulate authentication delay
	time.Sleep(50 * time.Millisecond)
	
	// Simulate successful authentication
	return "user-789"
}

func processUserRequest(ctx context.Context, log core.Logger, userId string) {
	// Add operation-specific context
	ctx = mtlog.PushProperty(ctx, "Operation", "UpdateProfile")
	opLog := log.WithContext(ctx)
	
	opLog.Debug("Loading user profile")
	time.Sleep(30 * time.Millisecond)
	
	opLog.Information("Updating user profile")
	time.Sleep(50 * time.Millisecond)
	
	// Simulate validation
	if err := validateProfileUpdate(ctx, opLog); err != nil {
		opLog.Warning("Profile validation failed: {Error}", err)
		return
	}
	
	opLog.Information("Profile updated successfully")
}

func validateProfileUpdate(ctx context.Context, log core.Logger) error {
	// Add validation context
	ctx = mtlog.PushProperty(ctx, "ValidationStep", "EmailFormat")
	valLog := log.WithContext(ctx)
	
	valLog.Debug("Validating email format")
	
	// Simulate validation with a 30% chance of failure for demonstration
	if time.Now().UnixNano()%10 < 3 {
		err := fmt.Errorf("invalid email format: missing @ symbol")
		valLog.Error("Validation failed: {Error}", err)
		return err
	}
	
	valLog.Debug("Email format validation passed")
	return nil
}

func demonstrateNestedContexts(logger core.Logger) {
	// Base context with tenant information
	ctx := context.Background()
	ctx = mtlog.PushProperty(ctx, "TenantId", "acme-corp")
	ctx = mtlog.PushProperty(ctx, "Environment", "production")
	
	// Simulate processing jobs for a tenant
	processJobs(ctx, logger)
}

func processJobs(ctx context.Context, logger core.Logger) {
	// Add job batch context
	ctx = mtlog.PushProperty(ctx, "BatchId", "batch-001")
	ctx = mtlog.PushProperty(ctx, "BatchSize", 3)
	
	batchLog := logger.WithContext(ctx)
	batchLog.Information("Starting job batch processing")
	
	// Process individual jobs
	for i := 1; i <= 3; i++ {
		// Each job gets its own context with job-specific properties
		jobCtx := mtlog.PushProperty(ctx, "JobId", fmt.Sprintf("job-%03d", i))
		jobCtx = mtlog.PushProperty(jobCtx, "JobType", getJobType(i))
		
		jobLog := logger.WithContext(jobCtx)
		processJob(jobCtx, jobLog, i)
	}
	
	batchLog.Information("Job batch completed")
}

func getJobType(jobNum int) string {
	types := []string{"DataImport", "ReportGeneration", "EmailNotification"}
	return types[(jobNum-1)%len(types)]
}

func processJob(ctx context.Context, log core.Logger, jobNum int) {
	log.Information("Starting job")
	
	// Simulate job processing
	time.Sleep(time.Duration(20+jobNum*10) * time.Millisecond)
	
	// Add progress information
	ctx = mtlog.PushProperty(ctx, "Progress", "50%")
	midLog := log.WithContext(ctx)
	midLog.Debug("Job halfway complete")
	
	// Complete the job
	time.Sleep(time.Duration(20+jobNum*10) * time.Millisecond)
	
	// Some jobs might have warnings
	if jobNum == 2 {
		log.Warning("Job completed with warnings: {WarningCount}", 3)
	} else {
		log.Information("Job completed successfully")
	}
}

